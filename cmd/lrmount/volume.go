package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/david-zw-liu/lrmount/internal/mountctl"
)

// appLabel maps a Lightroom bundle id to the human-readable name shown as the
// Finder volume name and used as the mount-path segment.
func appLabel(bundleID string) string {
	switch bundleID {
	case "com.adobe.lrmobilephone":
		return "Lightroom Mobile"
	case "com.adobe.lrmobile":
		return "Lightroom for iPad"
	default:
		return "Lightroom"
	}
}

// sanitizeSeg makes s safe as a single filesystem path segment.
func sanitizeSeg(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == ':' {
			return '-'
		}
		return r
	}, s)
}

// hintPath maps a device-side userStyles path onto its mounted location:
// <mountpoint>/<app>/<path within Documents>. app is the virtual
// subdirectory the Router serves that device's app under.
func hintPath(mountpoint, app, root, devicePath string) string {
	rel := strings.Trim(strings.TrimPrefix(devicePath, strings.Trim(root, "/")), "/")
	return filepath.Join(mountpoint, app, rel)
}

// mountBases lists where to try mounting, in order. A mountpoint is throwaway
// scratch (the data is on the device; Finder names the volume from the NFS
// share), so we prefer the per-user temp dir. That is $TMPDIR
// (/var/folders/.../T on macOS) — NOT the shared /private/tmp, which rejects
// user NFS mounts from an unsigned binary with "Operation not permitted". A
// hidden home directory is the fallback in case the temp mount is refused.
func mountBases() []string {
	var bases []string
	if t := os.TempDir(); t != "" && t != "/tmp" && t != "/private/tmp" {
		bases = append(bases, filepath.Join(t, "lrmount"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		bases = append(bases, filepath.Join(home, ".lrmount"))
	}
	return bases
}

// mountAt mounts the NFS server on port as deviceName, trying each base in
// turn and returning the mountpoint that succeeded. A live leftover mount at
// a candidate path gets a numeric suffix so two volumes never share a dir.
func mountAt(deviceName string, port int) (string, error) {
	var errs []string
	for _, base := range mountBases() {
		mp, err := makeMountpoint(base, deviceName)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if err := mountctl.MountNFS(mp, deviceName, port); err != nil {
			mountctl.Cleanup(mp)
			errs = append(errs, err.Error())
			continue
		}
		return mp, nil
	}
	return "", fmt.Errorf("could not mount %q: %s", deviceName, strings.Join(errs, "; "))
}

// makeMountpoint creates an unused mount directory under base for deviceName.
func makeMountpoint(base, deviceName string) (string, error) {
	for i := 1; i <= 9; i++ {
		leaf := sanitizeSeg(deviceName)
		if i > 1 {
			leaf = fmt.Sprintf("%s %d", leaf, i)
		}
		mp := filepath.Join(base, leaf)
		if mountctl.IsMounted(mp) {
			continue
		}
		if err := os.MkdirAll(mp, 0o755); err != nil {
			return "", err
		}
		return mp, nil
	}
	return "", fmt.Errorf("no usable mountpoint under %q", base)
}
