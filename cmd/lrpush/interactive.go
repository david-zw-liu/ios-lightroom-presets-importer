package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/davidliu/lrpush/internal/afcfs"
)

// entryChoice is one first-level userStyles entry offered in the rm menu.
type entryChoice struct {
	Name  string
	IsDir bool
}

// listUserStylesEntries returns the first-level entries (dirs and files) of
// userStyles, sorted by name for a stable menu.
func listUserStylesEntries(fs afcfs.FS, userStyles string) ([]entryChoice, error) {
	names, err := fs.List(userStyles)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]entryChoice, 0, len(names))
	for _, name := range names {
		fi, err := fs.Stat(userStyles + "/" + name)
		out = append(out, entryChoice{Name: name, IsDir: err == nil && fi.IsDir})
	}
	return out, nil
}

// parseSelection parses a menu selection line into 0-based indices into a list
// of size n. Accepts space/comma/tab-separated 1-based numbers, or "all". An
// empty line selects nothing. Out-of-range or non-numeric tokens are an error.
// Duplicates are collapsed; order follows first appearance.
func parseSelection(line string, n int) ([]int, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	if strings.EqualFold(line, "all") {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return idx, nil
	}
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	seen := make(map[int]bool)
	var out []int
	for _, f := range fields {
		num, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q (use numbers like '1 3 5' or 'all')", f)
		}
		if num < 1 || num > n {
			return nil, fmt.Errorf("selection %d out of range 1..%d", num, n)
		}
		i := num - 1
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out, nil
}
