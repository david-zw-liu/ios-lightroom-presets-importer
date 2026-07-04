// internal/afc/conn.go
package afc

import (
	"encoding/binary"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Conn is an AFC client over one service connection. AFC is strict
// request/response, so every round trip holds mu for its full duration.
type Conn struct {
	mu        sync.Mutex
	rw        io.ReadWriteCloser
	packetNum uint64
}

func NewConn(rw io.ReadWriteCloser) *Conn { return &Conn{rw: rw} }

func (c *Conn) Close() error { return c.rw.Close() }

func (c *Conn) roundTrip(req packet) (packet, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.packetNum++
	if err := writePacket(c.rw, c.packetNum, req); err != nil {
		return packet{}, err
	}
	resp, err := readPacket(c.rw)
	if err != nil {
		return packet{}, err
	}
	if resp.op == opStatus {
		if len(resp.headerPayload) < 8 {
			return packet{}, errors.New("afc: short status reply")
		}
		if code := binary.LittleEndian.Uint64(resp.headerPayload); code != codeSuccess {
			return packet{}, &Error{Code: code}
		}
	}
	return resp, nil
}

// cstr encodes strings as consecutive NUL-terminated byte runs.
func cstr(parts ...string) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
		b = append(b, 0)
	}
	return b
}

// parseKV decodes the alternating NUL-terminated key/value list AFC uses for
// GetFileInfo and GetDevInfo replies.
func parseKV(payload []byte) map[string]string {
	fields := strings.Split(string(payload), "\x00")
	kv := make(map[string]string, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		kv[fields[i]] = fields[i+1]
	}
	return kv
}

// FileInfo is stat data for one device path.
type FileInfo struct {
	Name    string
	IsDir   bool
	IsLink  bool
	Size    int64
	ModTime time.Time
}

// DeviceInfo describes the vended container's filesystem.
type DeviceInfo struct {
	TotalBytes uint64
	FreeBytes  uint64
}

// List returns the entries of directory p without "." and "..".
func (c *Conn) List(p string) ([]string, error) {
	resp, err := c.roundTrip(packet{op: opReadDir, headerPayload: cstr(p)})
	if err != nil {
		return nil, pathErr("list", p, err)
	}
	var out []string
	for _, name := range strings.Split(string(resp.payload), "\x00") {
		if name == "" || name == "." || name == ".." {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (c *Conn) Stat(p string) (FileInfo, error) {
	resp, err := c.roundTrip(packet{op: opGetFileInfo, headerPayload: cstr(p)})
	if err != nil {
		return FileInfo{}, pathErr("stat", p, err)
	}
	kv := parseKV(resp.payload)
	fi := FileInfo{
		Name:   path.Base("/" + p),
		IsDir:  kv["st_ifmt"] == "S_IFDIR",
		IsLink: kv["st_ifmt"] == "S_IFLNK",
	}
	fi.Size, _ = strconv.ParseInt(kv["st_size"], 10, 64)
	if ns, err := strconv.ParseInt(kv["st_mtime"], 10, 64); err == nil {
		fi.ModTime = time.Unix(0, ns)
	}
	return fi, nil
}

func (c *Conn) DeviceInfo() (DeviceInfo, error) {
	resp, err := c.roundTrip(packet{op: opGetDevInfo})
	if err != nil {
		return DeviceInfo{}, err
	}
	kv := parseKV(resp.payload)
	var di DeviceInfo
	di.TotalBytes, _ = strconv.ParseUint(kv["FSTotalBytes"], 10, 64)
	di.FreeBytes, _ = strconv.ParseUint(kv["FSFreeBytes"], 10, 64)
	return di, nil
}

func (c *Conn) MkDir(p string) error {
	_, err := c.roundTrip(packet{op: opMakeDir, headerPayload: cstr(p)})
	if err != nil {
		return pathErr("mkdir", p, err)
	}
	return nil
}

// Remove deletes one file or one empty directory (AFC fails with
// DirNotEmpty=33 otherwise) — exactly the semantics NFS REMOVE/RMDIR need.
func (c *Conn) Remove(p string) error {
	_, err := c.roundTrip(packet{op: opRemovePath, headerPayload: cstr(p)})
	if err != nil {
		return pathErr("remove", p, err)
	}
	return nil
}

func (c *Conn) Rename(from, to string) error {
	_, err := c.roundTrip(packet{op: opRenamePath, headerPayload: cstr(from, to)})
	if err != nil {
		return pathErr("rename", from, err)
	}
	return nil
}

// SetMtime sets p's modification time (AFC takes nanoseconds since epoch).
func (c *Conn) SetMtime(p string, t time.Time) error {
	hp := make([]byte, 8)
	binary.LittleEndian.PutUint64(hp, uint64(t.UnixNano()))
	hp = append(hp, cstr(p)...)
	_, err := c.roundTrip(packet{op: opSetFileModTime, headerPayload: hp})
	if err != nil {
		return pathErr("chtimes", p, err)
	}
	return nil
}
