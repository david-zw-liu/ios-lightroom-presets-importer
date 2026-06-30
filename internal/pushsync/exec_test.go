package pushsync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidliu/lrpush/internal/afcfs"
)

// failPullFS wraps MemFS but makes Pull always return an error.
type failPullFS struct{ *afcfs.MemFS }

func (f failPullFS) Pull(deviceSrc, localDst string) error {
	return fmt.Errorf("simulated backup failure")
}

// failOnePushFS wraps MemFS but makes PushFile fail for one specific device path.
type failOnePushFS struct {
	*afcfs.MemFS
	failDevice string
}

func (f *failOnePushFS) PushFile(localSrc, deviceDst string) error {
	if deviceDst == f.failDevice {
		return fmt.Errorf("simulated push failure for %s", deviceDst)
	}
	return f.MemFS.PushFile(localSrc, deviceDst)
}

func TestExecuteDryRunDoesNotMutate(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddDir("U/my-presets") // existing target dir
	dir := t.TempDir()
	local := filepath.Join(dir, "my-presets", "a.xmp")
	os.MkdirAll(filepath.Dir(local), 0o755)
	os.WriteFile(local, []byte("x"), 0o644)

	plan, _ := PlanPush(filepath.Join(dir, "my-presets"), "U")
	var buf bytes.Buffer
	err := Execute(m, plan, ExecOptions{UserStylesDir: "U", BackupDir: "/tmp/bk", Commit: false, Out: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Pushed) != 0 {
		t.Fatalf("dry-run pushed files: %v", m.Pushed)
	}
	if len(m.Pulled) != 0 {
		t.Fatalf("dry-run backed up: %v", m.Pulled)
	}
	if !m.Has("U/my-presets") {
		t.Fatal("dry-run must not RemoveAll existing dir")
	}
}

func TestExecuteCommitBacksUpReplacesAndPushes(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddFile("U/my-presets/old.xmp", 1) // stale file that must NOT survive
	m.AddFile("U/keep.xmp", 1)           // unrelated existing file, must survive

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "my-presets")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.xmp"), []byte("x"), 0o644)

	plan, _ := PlanPush(srcDir, "U")
	var buf bytes.Buffer
	err := Execute(m, plan, ExecOptions{UserStylesDir: "U", BackupDir: "/tmp/bk", Commit: true, Out: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if m.Pulled["U"] != "/tmp/bk" {
		t.Fatalf("expected backup Pull of U, got %v", m.Pulled)
	}
	if m.Has("U/my-presets/old.xmp") {
		t.Fatal("stale old.xmp should be gone after replace")
	}
	if !m.Has("U/my-presets/a.xmp") {
		t.Fatal("a.xmp should have been pushed")
	}
	if !m.Has("U/keep.xmp") {
		t.Fatal("unrelated keep.xmp must survive")
	}
}

func TestExecuteBackupFailureAbortsBeforeMutation(t *testing.T) {
	underlying := afcfs.NewMemFS()
	underlying.AddFile("U/my-presets/old.xmp", 1)
	underlying.AddFile("U/keep.xmp", 1)

	fs := failPullFS{underlying}

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "my-presets")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "new.xmp"), []byte("x"), 0o644)

	plan, err := PlanPush(srcDir, "U")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	execErr := Execute(fs, plan, ExecOptions{UserStylesDir: "U", BackupDir: "/bk", Commit: true, Out: &buf})
	if execErr == nil {
		t.Fatal("expected Execute to return an error when backup fails")
	}
	if len(underlying.Pushed) != 0 {
		t.Fatalf("nothing should be pushed when backup fails, got: %v", underlying.Pushed)
	}
	if !underlying.Has("U/my-presets/old.xmp") {
		t.Fatal("old.xmp must still exist: RemoveAll must not run when backup fails")
	}
}

func TestExecutePerFilePushFailureContinuesAndErrors(t *testing.T) {
	underlying := afcfs.NewMemFS()

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "my-presets")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.xmp"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "b.xmp"), []byte("y"), 0o644)

	plan, err := PlanPush(srcDir, "U")
	if err != nil {
		t.Fatal(err)
	}

	// Fail pushes for a.xmp; b.xmp should still succeed.
	fs := &failOnePushFS{MemFS: underlying, failDevice: "U/my-presets/a.xmp"}

	var buf bytes.Buffer
	execErr := Execute(fs, plan, ExecOptions{UserStylesDir: "U", BackupDir: "/bk", Commit: true, Out: &buf})
	if execErr == nil {
		t.Fatal("expected Execute to return an error when a per-file push fails")
	}
	if underlying.Has("U/my-presets/a.xmp") {
		t.Fatal("a.xmp should NOT have been pushed (it was the failing file)")
	}
	if !underlying.Has("U/my-presets/b.xmp") {
		t.Fatal("b.xmp should have been pushed despite a.xmp failing (loop must not abort)")
	}
}
