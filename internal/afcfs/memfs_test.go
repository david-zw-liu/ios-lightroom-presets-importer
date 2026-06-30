package afcfs

import "testing"

func TestMemFSListAndStat(t *testing.T) {
	m := NewMemFS()
	m.AddDir("Documents/123/settings-acr")
	m.AddFile("Documents/123/settings-acr/userStyles/a.xmp", 10)

	entries, err := m.List("Documents/123")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0] != "settings-acr" {
		t.Fatalf("List = %v, want [settings-acr]", entries)
	}
	fi, err := m.Stat("Documents/123/settings-acr/userStyles/a.xmp")
	if err != nil {
		t.Fatal(err)
	}
	if fi.IsDir || fi.Size != 10 || fi.Name != "a.xmp" {
		t.Fatalf("Stat = %+v", fi)
	}
}

func TestMemFSRemoveAll(t *testing.T) {
	m := NewMemFS()
	m.AddFile("a/b/c.txt", 1)
	if err := m.RemoveAll("a/b"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stat("a/b/c.txt"); err == nil {
		t.Fatal("expected c.txt gone after RemoveAll")
	}
}

func TestMemFSPushFileRecorded(t *testing.T) {
	m := NewMemFS()
	if err := m.PushFile("/local/x.xmp", "Documents/123/userStyles/x.xmp"); err != nil {
		t.Fatal(err)
	}
	if got := m.Pushed["Documents/123/userStyles/x.xmp"]; got != "/local/x.xmp" {
		t.Fatalf("Pushed = %v", m.Pushed)
	}
	if _, err := m.Stat("Documents/123/userStyles/x.xmp"); err != nil {
		t.Fatal("pushed file should now exist in MemFS")
	}
}
