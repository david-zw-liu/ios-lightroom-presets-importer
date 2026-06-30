package rmsync

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/davidliu/lrpush/internal/afcfs"
)

// pullFailFS wraps a MemFS but always returns an error from Pull.
type pullFailFS struct{ *afcfs.MemFS }

func (f pullFailFS) Pull(deviceSrc, localDst string) error {
	return errors.New("pull: simulated failure")
}

func TestPlanRmResolvesAndStats(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddFile("U/a.xmp", 1)
	m.AddDir("U/folder")

	targets := PlanRm(m, "U", []string{"a.xmp", "folder", "missing.xmp"})
	if len(targets) != 3 {
		t.Fatalf("got %d targets", len(targets))
	}
	if targets[0].Device != "U/a.xmp" || !targets[0].Exists || targets[0].IsDir {
		t.Fatalf("a.xmp target wrong: %+v", targets[0])
	}
	if !targets[1].IsDir || !targets[1].Exists {
		t.Fatalf("folder target wrong: %+v", targets[1])
	}
	if targets[2].Exists {
		t.Fatalf("missing target should not exist: %+v", targets[2])
	}
}

func TestExecuteDryRunNoMutate(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddFile("U/a.xmp", 1)
	targets := PlanRm(m, "U", []string{"a.xmp"})
	var buf bytes.Buffer
	if err := Execute(m, targets, ExecOptions{BackupDir: "/bk", Commit: false, Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if !m.Has("U/a.xmp") {
		t.Fatal("dry-run must not delete")
	}
}

func TestExecuteCommitBacksUpAndDeletes(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddFile("U/a.xmp", 1)
	targets := PlanRm(m, "U", []string{"a.xmp", "missing"})
	var buf bytes.Buffer
	if err := Execute(m, targets, ExecOptions{BackupDir: "/bk", Commit: true, Out: &buf}); err != nil {
		t.Fatal(err)
	}
	if m.Has("U/a.xmp") {
		t.Fatal("a.xmp should be deleted")
	}
	if m.Pulled["U/a.xmp"] == "" {
		t.Fatal("a.xmp should have been backed up before delete")
	}
}

func TestPlanRmRejectsUnsafePaths(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddFile("U/good.xmp", 1)

	targets := PlanRm(m, "U", []string{"..", "../escape", "/abs", "good.xmp"})
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(targets))
	}

	// First three must be unsafe.
	for i, label := range []string{"..", "../escape", "/abs"} {
		if !targets[i].Unsafe {
			t.Errorf("target[%d] (%q) should be Unsafe", i, label)
		}
		if targets[i].Device != "" {
			t.Errorf("target[%d] (%q) Device should be empty, got %q", i, label, targets[i].Device)
		}
		if targets[i].Exists {
			t.Errorf("target[%d] (%q) Exists should be false", i, label)
		}
	}

	// good.xmp must not be unsafe and must resolve correctly.
	if targets[3].Unsafe {
		t.Error("target[3] (good.xmp) must not be Unsafe")
	}
	if targets[3].Device != "U/good.xmp" {
		t.Errorf("target[3] Device = %q, want U/good.xmp", targets[3].Device)
	}
	if !targets[3].Exists {
		t.Error("target[3] (good.xmp) should Exist")
	}

	// Execute with Commit:true should return an error (3 unsafe = 3 failures),
	// print "refused" for unsafe targets, and only Pull the safe target.
	var buf bytes.Buffer
	err := Execute(m, targets, ExecOptions{BackupDir: "/bk", Commit: true, Out: &buf})
	if err == nil {
		t.Fatal("Execute should return an error when unsafe targets are present")
	}
	if !strings.Contains(buf.String(), "refused") {
		t.Errorf("output should contain 'refused', got: %s", buf.String())
	}
	// Only good.xmp should have been pulled; unsafe paths must not be pulled.
	if _, ok := m.Pulled["U/good.xmp"]; !ok {
		t.Error("good.xmp should have been pulled (backed up)")
	}
	if len(m.Pulled) != 1 {
		t.Errorf("only 1 file should be pulled, got %d: %v", len(m.Pulled), m.Pulled)
	}
	// good.xmp should be deleted; unsafe targets must not have mutated the FS.
	if m.Has("U/good.xmp") {
		t.Error("good.xmp should have been deleted after successful backup")
	}
}

func TestExecuteBackupFailureSkipsDelete(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddFile("U/a.xmp", 1)
	fs := pullFailFS{m}
	targets := PlanRm(fs, "U", []string{"a.xmp"})
	var buf bytes.Buffer
	err := Execute(fs, targets, ExecOptions{BackupDir: "/bk", Commit: true, Out: &buf})
	if err == nil {
		t.Fatal("Execute should return an error when backup fails")
	}
	if !m.Has("U/a.xmp") {
		t.Fatal("U/a.xmp must NOT be deleted when its backup failed")
	}
}
