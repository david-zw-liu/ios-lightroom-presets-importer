package rmsync

import (
	"bytes"
	"errors"
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
