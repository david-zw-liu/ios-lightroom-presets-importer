package mirror

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidliu/lrpush/internal/afcfs"
)

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
