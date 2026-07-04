// internal/afc/file.go
package afc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Open modes, per libimobiledevice afc.h. Note WROnly/WR truncate, and every
// mode except RDOnly creates missing files.
const (
	ModeRDOnly   uint64 = 1 // r
	ModeRW       uint64 = 2 // r+, creates
	ModeWROnly   uint64 = 3 // w, creates + truncates
	ModeWR       uint64 = 4 // w+, creates + truncates
	ModeAppend   uint64 = 5 // a
	ModeRDAppend uint64 = 6 // a+
)

// File is one open handle. The device tracks the position per handle, so
// independent Files never disturb each other; ops on the same File must not
// be concurrent (callers serialize, see nfsgate).
type File struct {
	c      *Conn
	handle uint64
	name   string
}

func (c *Conn) Open(p string, mode uint64) (*File, error) {
	hp := make([]byte, 8)
	binary.LittleEndian.PutUint64(hp, mode)
	hp = append(hp, cstr(p)...)
	resp, err := c.roundTrip(packet{op: opFileOpen, headerPayload: hp})
	if err != nil {
		return nil, pathErr("open", p, err)
	}
	if resp.op != opFileOpenResult || len(resp.headerPayload) < 8 {
		return nil, fmt.Errorf("afc: open %s: unexpected reply op %#x", p, resp.op)
	}
	return &File{c: c, handle: binary.LittleEndian.Uint64(resp.headerPayload), name: p}, nil
}

func (f *File) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	resp, err := f.c.roundTrip(packet{op: opFileRead, headerPayload: le64pair(f.handle, uint64(len(p)))})
	if err != nil {
		return 0, pathErr("read", f.name, err)
	}
	if len(resp.payload) == 0 {
		return 0, io.EOF
	}
	return copy(p, resp.payload), nil
}

func (f *File) Write(p []byte) (int, error) {
	hp := make([]byte, 8)
	binary.LittleEndian.PutUint64(hp, f.handle)
	if _, err := f.c.roundTrip(packet{op: opFileWrite, headerPayload: hp, payload: p}); err != nil {
		return 0, pathErr("write", f.name, err)
	}
	return len(p), nil
}

// Seek moves the device-side position. Whence follows io.Seek* (same values
// AFC uses). For non-SeekStart the new position is fetched with FileTell.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	hp := make([]byte, 24)
	binary.LittleEndian.PutUint64(hp, f.handle)
	binary.LittleEndian.PutUint64(hp[8:], uint64(whence))
	binary.LittleEndian.PutUint64(hp[16:], uint64(offset))
	if _, err := f.c.roundTrip(packet{op: opFileSeek, headerPayload: hp}); err != nil {
		return 0, pathErr("seek", f.name, err)
	}
	if whence == io.SeekStart {
		return offset, nil
	}
	resp, err := f.c.roundTrip(packet{op: opFileTell, headerPayload: le64single(f.handle)})
	if err != nil {
		return 0, pathErr("tell", f.name, err)
	}
	if resp.op != opFileTellResult || len(resp.headerPayload) < 8 {
		return 0, fmt.Errorf("afc: tell %s: unexpected reply op %#x", f.name, resp.op)
	}
	return int64(binary.LittleEndian.Uint64(resp.headerPayload)), nil
}

func (f *File) Truncate(size int64) error {
	if _, err := f.c.roundTrip(packet{op: opFileSetSize, headerPayload: le64pair(f.handle, uint64(size))}); err != nil {
		return pathErr("truncate", f.name, err)
	}
	return nil
}

func (f *File) Close() error {
	if _, err := f.c.roundTrip(packet{op: opFileClose, headerPayload: le64single(f.handle)}); err != nil {
		return pathErr("close", f.name, err)
	}
	return nil
}

func le64single(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func le64pair(a, b uint64) []byte {
	out := make([]byte, 16)
	binary.LittleEndian.PutUint64(out, a)
	binary.LittleEndian.PutUint64(out[8:], b)
	return out
}
