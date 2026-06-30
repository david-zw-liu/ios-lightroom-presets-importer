// Package locate finds the Lightroom userStyles directory inside the app container.
package locate

import (
	"fmt"
	"strings"

	"github.com/davidliu/lrpush/internal/afcfs"
)

func join(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, strings.Trim(p, "/"))
		}
	}
	return strings.Join(nonEmpty, "/")
}

// DocumentsRoot determines the path prefix that contains the catalog folders.
// override wins. Otherwise: if the AFC root contains a "Documents" dir, the root
// is the container and we return "Documents"; else the root already is Documents
// and we return "".
func DocumentsRoot(fs afcfs.FS, override string) (string, error) {
	if override != "" {
		return strings.Trim(override, "/"), nil
	}
	entries, err := fs.List("")
	if err != nil {
		return "", fmt.Errorf("list AFC root: %w", err)
	}
	for _, e := range entries {
		if e == "Documents" {
			if fi, err := fs.Stat("Documents"); err == nil && fi.IsDir {
				return "Documents", nil
			}
		}
	}
	return "", nil
}

// Catalog is one Lightroom catalog/account folder that has a settings-acr dir.
type Catalog struct {
	Name        string
	Dir         string
	UserStyles  string
	PresetCount int
}

// FindCatalogs lists docsRoot's children and keeps those containing settings-acr.
func FindCatalogs(fs afcfs.FS, docsRoot string) ([]Catalog, error) {
	children, err := fs.List(docsRoot)
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", docsRoot, err)
	}
	var out []Catalog
	for _, name := range children {
		dir := join(docsRoot, name)
		fi, err := fs.Stat(dir)
		if err != nil || !fi.IsDir {
			continue
		}
		settings := join(dir, "settings-acr")
		if sfi, err := fs.Stat(settings); err != nil || !sfi.IsDir {
			continue
		}
		userStyles := join(settings, "userStyles")
		count := 0
		if entries, err := fs.List(userStyles); err == nil {
			count = len(entries)
		}
		out = append(out, Catalog{Name: name, Dir: dir, UserStyles: userStyles, PresetCount: count})
	}
	return out, nil
}

// SelectCatalog picks one catalog. catalogFlag forces a name; otherwise single
// candidate auto-selects and multiple candidates call picker (interactive menu).
func SelectCatalog(cands []Catalog, catalogFlag string, picker func([]Catalog) (int, error)) (Catalog, error) {
	if len(cands) == 0 {
		return Catalog{}, fmt.Errorf("no catalog with a settings-acr folder found; app may not have created a catalog yet, or set --path-prefix")
	}
	if catalogFlag != "" {
		for _, c := range cands {
			if c.Name == catalogFlag {
				return c, nil
			}
		}
		var names []string
		for _, c := range cands {
			names = append(names, c.Name)
		}
		return Catalog{}, fmt.Errorf("catalog %q not found; available: %s", catalogFlag, strings.Join(names, ", "))
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	if picker == nil {
		return Catalog{}, fmt.Errorf("%d catalogs found; pass --catalog <name> to choose", len(cands))
	}
	i, err := picker(cands)
	if err != nil {
		return Catalog{}, err
	}
	if i < 0 || i >= len(cands) {
		return Catalog{}, fmt.Errorf("invalid catalog selection %d", i)
	}
	return cands[i], nil
}
