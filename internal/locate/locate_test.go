package locate

import (
	"testing"

	"github.com/davidliu/lrpush/internal/afcfs"
)

func TestDocumentsRootContainer(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddDir("Documents/123/settings-acr")
	got, err := DocumentsRoot(m, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Documents" {
		t.Fatalf("DocumentsRoot = %q, want Documents", got)
	}
}

func TestDocumentsRootIsDocuments(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddDir("123/settings-acr") // root already is Documents
	got, err := DocumentsRoot(m, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("DocumentsRoot = %q, want empty", got)
	}
}

func TestFindCatalogs(t *testing.T) {
	m := afcfs.NewMemFS()
	m.AddDir("Documents/aaa/settings-acr")
	m.AddFile("Documents/aaa/settings-acr/userStyles/p1.xmp", 1)
	m.AddFile("Documents/aaa/settings-acr/userStyles/p2.xmp", 1)
	m.AddDir("Documents/bbb/settings-acr/userStyles")
	m.AddDir("Documents/ccc/other") // not a catalog

	cands, err := FindCatalogs(m, "Documents")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("got %d catalogs, want 2: %+v", len(cands), cands)
	}
	byName := map[string]Catalog{}
	for _, c := range cands {
		byName[c.Name] = c
	}
	if byName["aaa"].PresetCount != 2 {
		t.Fatalf("aaa preset count = %d, want 2", byName["aaa"].PresetCount)
	}
	if byName["aaa"].UserStyles != "Documents/aaa/settings-acr/userStyles" {
		t.Fatalf("aaa userStyles = %q", byName["aaa"].UserStyles)
	}
}

func TestSelectCatalogSingleAuto(t *testing.T) {
	cands := []Catalog{{Name: "aaa"}}
	got, err := SelectCatalog(cands, "", nil)
	if err != nil || got.Name != "aaa" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSelectCatalogFlag(t *testing.T) {
	cands := []Catalog{{Name: "aaa"}, {Name: "bbb"}}
	got, err := SelectCatalog(cands, "bbb", nil)
	if err != nil || got.Name != "bbb" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSelectCatalogFlagMissing(t *testing.T) {
	cands := []Catalog{{Name: "aaa"}}
	if _, err := SelectCatalog(cands, "zzz", nil); err == nil {
		t.Fatal("expected error for unknown catalog")
	}
}

func TestSelectCatalogMultiUsesPicker(t *testing.T) {
	cands := []Catalog{{Name: "aaa"}, {Name: "bbb"}}
	got, err := SelectCatalog(cands, "", func(c []Catalog) (int, error) { return 1, nil })
	if err != nil || got.Name != "bbb" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSelectCatalogNone(t *testing.T) {
	if _, err := SelectCatalog(nil, "", nil); err == nil {
		t.Fatal("expected error for no candidates")
	}
}
