// Package mirror mirrors a device Lightroom userStyles folder to a local
// directory and syncs local edits back to the device.
package mirror

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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

func pullTree(fs afcfs.FS, deviceRoot, localRoot, rel string, log func(string)) error {
	entries, err := fs.List(deviceJoinRel(deviceRoot, rel))
	if err != nil {
		return err
	}
	for _, name := range entries {
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
