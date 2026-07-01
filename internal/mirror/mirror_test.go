package mirror

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidliu/lrpush/internal/afcfs"
)

func TestSafeRelRejectsEscapes(t *testing.T) {
	bad := []string{"", ".", "..", "../x", "/abs", "a/../../b"}
	for _, r := range bad {
		if _, err := safeRel(r); err == nil {
			t.Errorf("safeRel(%q): expected error", r)
		}
	}
	good := map[string]string{"A/foo.xmp": "A/foo.xmp", "a/b/c": "a/b/c", "foo.xmp": "foo.xmp"}
	for in, want := range good {
		got, err := safeRel(in)
		if err != nil || got != want {
			t.Errorf("safeRel(%q) = %q,%v; want %q,nil", in, got, err, want)
		}
	}
}

func TestReconcilePushesNewFile(t *testing.T) {
	fs := afcfs.NewMemFS()
	local := t.TempDir()
	writeLocal(t, local, "A/foo.xmp", "hi")
	root := "Documents/cat/settings-acr/userStyles"

	if err := Reconcile(fs, local, root, []string{"A/foo.xmp"}, map[string]fileSig{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !fs.Has(root + "/A/foo.xmp") {
		t.Error("expected pushed file on device")
	}
}

func TestReconcilePushesNewDirRecursively(t *testing.T) {
	fs := afcfs.NewMemFS()
	local := t.TempDir()
	writeLocal(t, local, "Grp/one.xmp", "a")
	writeLocal(t, local, "Grp/sub/two.xmp", "b")
	root := "Documents/cat/settings-acr/userStyles"

	if err := Reconcile(fs, local, root, []string{"Grp"}, map[string]fileSig{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{root + "/Grp/one.xmp", root + "/Grp/sub/two.xmp"} {
		if !fs.Has(p) {
			t.Errorf("expected %s pushed", p)
		}
	}
}

func TestReconcileDeletesMissingLocalPath(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	fs.AddFile(root+"/Old/gone.xmp", 5)
	local := t.TempDir() // Old/ does not exist locally

	if err := Reconcile(fs, local, root, []string{"Old"}, map[string]fileSig{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if fs.Has(root+"/Old") || fs.Has(root+"/Old/gone.xmp") {
		t.Error("expected device path removed")
	}
}

func TestReconcileSkipsEscapePath(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	fs.AddFile(root+"/keep.xmp", 1)
	local := t.TempDir()

	var logged []string
	if err := Reconcile(fs, local, root, []string{"../evil"}, map[string]fileSig{}, func(s string) { logged = append(logged, s) }); err != nil {
		t.Fatal(err)
	}
	if !fs.Has(root + "/keep.xmp") {
		t.Error("device content must be untouched by an escape path")
	}
	if len(logged) == 0 {
		t.Error("expected a log line for the refused path")
	}
}

// writeLocal creates local/rel with the given content, making parent dirs.
func writeLocal(t *testing.T, local, rel, content string) {
	t.Helper()
	p := filepath.Join(local, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPullReplaceMirrorsTreeAndCreatesLocalDirs(t *testing.T) {
	fs := afcfs.NewMemFS()
	// device userStyles tree
	fs.AddFile("Documents/cat/settings-acr/userStyles/A/foo.xmp", 10)
	fs.AddFile("Documents/cat/settings-acr/userStyles/B/bar.xmp", 20)
	fs.AddFile("Documents/cat/settings-acr/userStyles/Index.dat", 30)

	local := filepath.Join(t.TempDir(), "sync", "com.adobe.lrmobile", "userStyles")
	root := "Documents/cat/settings-acr/userStyles"

	var logged []string
	if err := PullReplace(fs, root, local, func(s string) { logged = append(logged, s) }); err != nil {
		t.Fatal(err)
	}

	// local subdirs created
	for _, d := range []string{"A", "B"} {
		if fi, err := os.Stat(filepath.Join(local, d)); err != nil || !fi.IsDir() {
			t.Errorf("expected local dir %s", d)
		}
	}
	// every device file was pulled to its mirrored local path
	wants := map[string]string{
		root + "/A/foo.xmp": filepath.Join(local, "A", "foo.xmp"),
		root + "/B/bar.xmp": filepath.Join(local, "B", "bar.xmp"),
		root + "/Index.dat": filepath.Join(local, "Index.dat"),
	}
	for src, dst := range wants {
		if got := fs.Pulled[src]; got != dst {
			t.Errorf("Pulled[%q] = %q, want %q", src, got, dst)
		}
	}
	if len(logged) != 3 {
		t.Errorf("expected 3 per-file log lines, got %d: %v", len(logged), logged)
	}
}

func TestReconcileSuppressesEcho(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	local := t.TempDir()
	writeLocal(t, local, "A/foo.xmp", "hello")
	known, err := snapshot(local) // baseline captured right after the "pull"
	if err != nil {
		t.Fatal(err)
	}
	// A macOS echo event fires for the unchanged, just-pulled file.
	if err := Reconcile(fs, local, root, []string{"A/foo.xmp"}, known, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if fs.Has(root + "/A/foo.xmp") {
		t.Error("unchanged file must NOT be pushed (echo suppressed)")
	}
}

func TestReconcilePushesFileChangedBySize(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	local := t.TempDir()
	writeLocal(t, local, "A/foo.xmp", "hello")
	known, _ := snapshot(local)
	writeLocal(t, local, "A/foo.xmp", "hello, longer content now") // size changes
	if err := Reconcile(fs, local, root, []string{"A/foo.xmp"}, known, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !fs.Has(root + "/A/foo.xmp") {
		t.Error("size-changed file must be pushed")
	}
}

func TestReconcilePushesFileChangedByMtimeSameSize(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	local := t.TempDir()
	writeLocal(t, local, "A/foo.xmp", "12345")
	known, _ := snapshot(local)
	// Same byte length, but a real edit bumps mtime (simulate deterministically).
	writeLocal(t, local, "A/foo.xmp", "67890")
	future := time.Unix(0, 0).Add(1000000 * time.Hour)
	if err := os.Chtimes(filepath.Join(local, "A", "foo.xmp"), future, future); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(fs, local, root, []string{"A/foo.xmp"}, known, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !fs.Has(root + "/A/foo.xmp") {
		t.Error("same-size but mtime-changed file must be pushed")
	}
}

func TestReconcileSkipsDSStore(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	local := t.TempDir()
	writeLocal(t, local, ".DS_Store", "junk")
	writeLocal(t, local, "A/.DS_Store", "junk")
	writeLocal(t, local, "A/foo.xmp", "real")
	if err := Reconcile(fs, local, root, []string{".DS_Store", "A"}, map[string]fileSig{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if fs.Has(root + "/.DS_Store") {
		t.Error("top-level .DS_Store must not be pushed")
	}
	if fs.Has(root + "/A/.DS_Store") {
		t.Error("nested .DS_Store must not be pushed")
	}
	if !fs.Has(root + "/A/foo.xmp") {
		t.Error("real file under A should still push")
	}
}

func TestReconcileReplacesDeviceFileWithDir(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	fs.AddFile(root+"/Portrait", 10) // device has a loose FILE named Portrait
	local := t.TempDir()
	writeLocal(t, local, "Portrait/one.xmp", "a") // local now a DIR
	if err := Reconcile(fs, local, root, []string{"Portrait"}, map[string]fileSig{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	fi, err := fs.Stat(root + "/Portrait")
	if err != nil || !fi.IsDir {
		t.Error("Portrait should be a directory on the device now")
	}
	if !fs.Has(root + "/Portrait/one.xmp") {
		t.Error("new dir contents must be pushed")
	}
}

func TestReconcileReplacesDeviceDirWithFile(t *testing.T) {
	fs := afcfs.NewMemFS()
	root := "Documents/cat/settings-acr/userStyles"
	fs.AddFile(root+"/Group/old.xmp", 5) // device has a DIR named Group
	local := t.TempDir()
	writeLocal(t, local, "Group", "now a plain file") // local now a FILE
	if err := Reconcile(fs, local, root, []string{"Group"}, map[string]fileSig{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	fi, err := fs.Stat(root + "/Group")
	if err != nil || fi.IsDir {
		t.Error("Group should be a file on the device now")
	}
	if fs.Has(root + "/Group/old.xmp") {
		t.Error("stale dir contents must be gone")
	}
}
