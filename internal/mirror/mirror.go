// Package mirror mirrors a device Lightroom userStyles folder to a local
// directory and syncs local edits back to the device.
package mirror

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidliu/lrpush/internal/afcfs"
)

// deviceJoinRel joins a device root with a slash-separated relative path.
// An empty rel returns the root unchanged.
func deviceJoinRel(root, rel string) string {
	if rel == "" {
		return root
	}
	return root + "/" + rel
}

// fileSig is a lightweight change signature: size + modtime. It lets the watcher
// tell a genuine edit from a macOS echo of a just-pulled file without hashing.
type fileSig struct {
	size  int64
	mtime time.Time
}

func sigOf(fi os.FileInfo) fileSig { return fileSig{size: fi.Size(), mtime: fi.ModTime()} }

// ignoredName reports OS-junk filenames that must never sync to the device.
func ignoredName(name string) bool { return name == ".DS_Store" }

// snapshot walks root and records a fileSig for every file (keyed by
// slash-relative path), skipping ignored names. It seeds the watcher baseline
// right after the initial pull so unchanged files are not echoed back.
func snapshot(root string) (map[string]fileSig, error) {
	known := map[string]fileSig{}
	err := filepath.WalkDir(root, func(p string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || ignoredName(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		known[filepath.ToSlash(rel)] = sigOf(fi)
		return nil
	})
	return known, err
}

// forgetPrefix removes rel and any keys nested beneath it from the baseline.
func forgetPrefix(known map[string]fileSig, rel string) {
	delete(known, rel)
	prefix := rel + "/"
	for k := range known {
		if strings.HasPrefix(k, prefix) {
			delete(known, k)
		}
	}
}

// safeRel validates that rel is a clean, relative, non-empty slash-separated
// path with no ".." components. It is used by Task 2 (push sync).
func safeRel(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty relative path")
	}
	clean := path.Clean(rel)
	if path.IsAbs(clean) {
		return "", fmt.Errorf("absolute path not allowed: %s", rel)
	}
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("unsafe relative path: %s", rel)
	}
	return clean, nil
}

// PullReplace recreates localDir and recursively pulls the device userStyles
// tree into it, logging one line per file. The device is never written. The
// caller is responsible for wiping ./sync beforehand.
func PullReplace(fs afcfs.FS, deviceUserStyles, localDir string, log func(string)) error {
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return err
	}
	return pullTree(fs, deviceUserStyles, localDir, "", log)
}

// deviceMkdirAll makes a device directory; afcfs.MkDir is already mkdir-p.
func deviceMkdirAll(fs afcfs.FS, dir string) error { return fs.MkDir(dir) }

// Reconcile applies changed local relative paths to the device. For each path:
// missing local → RemoveAll on device; unchanged file (sig matches baseline) →
// skip (suppresses the macOS echo of just-pulled files); changed/new file →
// PushFile; dir → push its files. .DS_Store is never synced. A device node of
// the opposite type (file vs dir) is removed first. Per-path errors and refused
// (escaping) paths are logged and skipped; Reconcile returns nil. known is the
// running (size,mtime) baseline and is updated in place.
func Reconcile(fs afcfs.FS, localDir, deviceUserStyles string, changed []string, known map[string]fileSig, log func(string)) error {
	for _, raw := range changed {
		rel, err := safeRel(raw)
		if err != nil {
			log("skip " + err.Error())
			continue
		}
		if ignoredName(path.Base(rel)) {
			continue // never sync OS junk like .DS_Store
		}
		localPath := filepath.Join(localDir, filepath.FromSlash(rel))
		devPath := deviceJoinRel(deviceUserStyles, rel)

		fi, statErr := os.Stat(localPath)
		if errors.Is(statErr, os.ErrNotExist) {
			if err := fs.RemoveAll(devPath); err != nil {
				log("delete failed " + rel + ": " + err.Error())
				continue
			}
			forgetPrefix(known, rel)
			log("deleted " + rel)
			continue
		}
		if statErr != nil {
			log("stat failed " + rel + ": " + statErr.Error())
			continue
		}
		if fi.IsDir() {
			if err := pushDir(fs, localDir, deviceUserStyles, rel, known, log); err != nil {
				log("push dir failed " + rel + ": " + err.Error())
			}
			continue
		}
		// regular file
		sig := sigOf(fi)
		if prev, ok := known[rel]; ok && prev == sig {
			continue // unchanged since last pull/push — suppress echo
		}
		// device type-change: a directory currently occupies this file path
		if dfi, err := fs.Stat(devPath); err == nil && dfi.IsDir {
			if err := fs.RemoveAll(devPath); err != nil {
				log("replace failed " + rel + ": " + err.Error())
				continue
			}
		}
		if err := deviceMkdirAll(fs, path.Dir(devPath)); err != nil {
			log("mkdir failed " + rel + ": " + err.Error())
			continue
		}
		if err := fs.PushFile(localPath, devPath); err != nil {
			log("push failed " + rel + ": " + err.Error())
			continue
		}
		known[rel] = sig
		log("pushed " + rel)
	}
	return nil
}

// pushDir pushes every file under the local subtree at subRel to the device,
// skipping .DS_Store and files whose signature already matches the baseline. A
// device FILE occupying the dir path is removed first (type change).
func pushDir(fs afcfs.FS, rootLocalDir, deviceUserStyles, subRel string, known map[string]fileSig, log func(string)) error {
	deviceDir := deviceJoinRel(deviceUserStyles, subRel)
	if dfi, err := fs.Stat(deviceDir); err == nil && !dfi.IsDir {
		if err := fs.RemoveAll(deviceDir); err != nil {
			return err
		}
	}
	if err := deviceMkdirAll(fs, deviceDir); err != nil {
		return err
	}
	localSub := filepath.Join(rootLocalDir, filepath.FromSlash(subRel))
	return filepath.WalkDir(localSub, func(p string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relFromRoot, err := filepath.Rel(rootLocalDir, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relFromRoot)
		if d.IsDir() {
			return deviceMkdirAll(fs, deviceJoinRel(deviceUserStyles, key))
		}
		if ignoredName(d.Name()) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		sig := sigOf(fi)
		if prev, ok := known[key]; ok && prev == sig {
			return nil // unchanged — suppress echo
		}
		if err := fs.PushFile(p, deviceJoinRel(deviceUserStyles, key)); err != nil {
			return err
		}
		known[key] = sig
		log("pushed " + key)
		return nil
	})
}

func pullTree(fs afcfs.FS, deviceRoot, localRoot, rel string, log func(string)) error {
	entries, err := fs.List(deviceJoinRel(deviceRoot, rel))
	if err != nil {
		return err
	}
	for _, name := range entries {
		if ignoredName(name) {
			continue
		}
		childRel := path.Join(rel, name)
		devPath := deviceJoinRel(deviceRoot, childRel)
		fi, err := fs.Stat(devPath)
		if err != nil {
			return err
		}
		localPath := filepath.Join(localRoot, filepath.FromSlash(childRel))
		if fi.IsDir {
			if err := os.MkdirAll(localPath, 0o755); err != nil {
				return err
			}
			if err := pullTree(fs, deviceRoot, localRoot, childRel, log); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return err
		}
		if err := fs.Pull(devPath, localPath); err != nil {
			return err
		}
		log("pulled " + childRel)
	}
	return nil
}
