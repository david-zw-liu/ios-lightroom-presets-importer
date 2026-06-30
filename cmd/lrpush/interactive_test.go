package main

import (
	"reflect"
	"testing"

	"github.com/davidliu/lrpush/internal/afcfs"
)

func TestParseSelection(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want []int
		err  bool
	}{
		{"", 5, nil, false},
		{"1 3 5", 5, []int{0, 2, 4}, false},
		{"1,3", 5, []int{0, 2}, false},
		{"all", 3, []int{0, 1, 2}, false},
		{"2 2 2", 5, []int{1}, false}, // dedup
		{"0", 5, nil, true},           // below range
		{"6", 5, nil, true},           // above range
		{"x", 5, nil, true},           // non-numeric
	}
	for _, c := range cases {
		got, err := parseSelection(c.in, c.n)
		if c.err {
			if err == nil {
				t.Errorf("parseSelection(%q,%d): want error, got %v", c.in, c.n, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSelection(%q,%d): unexpected error %v", c.in, c.n, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseSelection(%q,%d) = %v, want %v", c.in, c.n, got, c.want)
		}
	}
}

func TestListUserStylesEntries(t *testing.T) {
	m := afcfs.NewMemFS()
	us := "Documents/123/settings-acr/userStyles"
	m.AddFile(us+"/Index.dat", 1)
	m.AddDir(us + "/GroupB")
	m.AddDir(us + "/GroupA")
	m.AddFile(us+"/loose.xmp", 1)

	entries, err := listUserStylesEntries(m, us)
	if err != nil {
		t.Fatal(err)
	}
	// sorted: GroupA, GroupB, Index.dat, loose.xmp
	want := []entryChoice{
		{"GroupA", true},
		{"GroupB", true},
		{"Index.dat", false},
		{"loose.xmp", false},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %+v, want %+v", entries, want)
	}
}
