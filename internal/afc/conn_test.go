// internal/afc/conn_test.go
package afc

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// script serves one request per entry on the device end of a pipe.
type step struct {
	wantOp uint64
	wantHP []byte // nil = don't check
	reply  packet
}

func fakeDevice(t *testing.T, steps []step) *Conn {
	t.Helper()
	cli, dev := net.Pipe()
	go func() {
		defer dev.Close()
		for _, s := range steps {
			req, err := readPacket(dev)
			if err != nil {
				t.Errorf("device read: %v", err)
				return
			}
			if req.op != s.wantOp {
				t.Errorf("op = %#x, want %#x", req.op, s.wantOp)
			}
			if s.wantHP != nil && !bytes.Equal(req.headerPayload, s.wantHP) {
				t.Errorf("headerPayload = %q, want %q", req.headerPayload, s.wantHP)
			}
			if err := writePacket(dev, 1, s.reply); err != nil {
				t.Errorf("device write: %v", err)
				return
			}
		}
	}()
	t.Cleanup(func() { cli.Close() })
	return NewConn(cli)
}

func okStatus() packet { return packet{op: opStatus, headerPayload: le64(codeSuccess)} }

func TestList(t *testing.T) {
	c := fakeDevice(t, []step{{
		wantOp: opReadDir, wantHP: []byte("Documents\x00"),
		reply: packet{op: opData, payload: []byte(".\x00..\x00a.xmp\x00sub\x00")},
	}})
	got, err := c.List("Documents")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a.xmp" || got[1] != "sub" {
		t.Fatalf("got %v", got)
	}
}

func TestStatParsesKVAndNanosecondMtime(t *testing.T) {
	kv := []byte("st_size\x0011\x00st_ifmt\x00S_IFREG\x00st_mtime\x001700000000123456789\x00")
	c := fakeDevice(t, []step{{wantOp: opGetFileInfo, wantHP: []byte("a/b.xmp\x00"),
		reply: packet{op: opData, payload: kv}}})
	fi, err := c.Stat("a/b.xmp")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Name != "b.xmp" || fi.IsDir || fi.Size != 11 {
		t.Fatalf("fi = %+v", fi)
	}
	if want := time.Unix(0, 1700000000123456789); !fi.ModTime.Equal(want) {
		t.Fatalf("mtime = %v, want %v", fi.ModTime, want)
	}
}

func TestStatDir(t *testing.T) {
	kv := []byte("st_size\x0064\x00st_ifmt\x00S_IFDIR\x00st_mtime\x001\x00")
	c := fakeDevice(t, []step{{wantOp: opGetFileInfo, reply: packet{op: opData, payload: kv}}})
	fi, err := c.Stat("Documents")
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir {
		t.Fatal("want IsDir")
	}
}

func TestStatNotFound(t *testing.T) {
	c := fakeDevice(t, []step{{wantOp: opGetFileInfo,
		reply: packet{op: opStatus, headerPayload: le64(codeObjectNotFound)}}})
	_, err := c.Stat("gone")
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("want not-exist, got %v", err)
	}
}

func TestDeviceInfo(t *testing.T) {
	kv := []byte("Model\x00iPhone\x00FSTotalBytes\x001000\x00FSFreeBytes\x00400\x00FSBlockSize\x004096\x00")
	c := fakeDevice(t, []step{{wantOp: opGetDevInfo, reply: packet{op: opData, payload: kv}}})
	di, err := c.DeviceInfo()
	if err != nil {
		t.Fatal(err)
	}
	if di.TotalBytes != 1000 || di.FreeBytes != 400 {
		t.Fatalf("di = %+v", di)
	}
}

func TestRenameSetMtimeMkDirRemove(t *testing.T) {
	mt := time.Unix(0, 42)
	wantMtimeHP := append(le64(42), []byte("f\x00")...)
	c := fakeDevice(t, []step{
		{wantOp: opRenamePath, wantHP: []byte("a\x00b\x00"), reply: okStatus()},
		{wantOp: opSetFileModTime, wantHP: wantMtimeHP, reply: okStatus()},
		{wantOp: opMakeDir, wantHP: []byte("d\x00"), reply: okStatus()},
		{wantOp: opRemovePath, wantHP: []byte("x\x00"), reply: okStatus()},
	})
	if err := c.Rename("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetMtime("f", mt); err != nil {
		t.Fatal(err)
	}
	if err := c.MkDir("d"); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove("x"); err != nil {
		t.Fatal(err)
	}
}

func TestFileOpenReadWriteSeekTruncateClose(t *testing.T) {
	openHP := append(le64(ModeRW), []byte("f\x00")...)
	c := fakeDevice(t, []step{
		{wantOp: opFileOpen, wantHP: openHP,
			reply: packet{op: opFileOpenResult, headerPayload: le64(3)}},
		{wantOp: opFileRead, wantHP: le64(3, 4),
			reply: packet{op: opData, payload: []byte("data")}},
		{wantOp: opFileWrite, wantHP: le64(3), reply: okStatus()},
		{wantOp: opFileSeek, wantHP: le64(3, 0, 5), reply: okStatus()},
		{wantOp: opFileSeek, wantHP: append(le64(3, 2), leI64(-1)...), reply: okStatus()},
		{wantOp: opFileTell, wantHP: le64(3),
			reply: packet{op: opFileTellResult, headerPayload: le64(9)}},
		{wantOp: opFileSetSize, wantHP: le64(3, 5), reply: okStatus()},
		{wantOp: opFileClose, wantHP: le64(3), reply: okStatus()},
	})
	f, err := c.Open("f", ModeRW)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if n, err := f.Read(buf); err != nil || n != 4 || string(buf) != "data" {
		t.Fatalf("read = %d %v %q", n, err, buf)
	}
	if n, err := f.Write([]byte("hi")); err != nil || n != 2 {
		t.Fatalf("write = %d %v", n, err)
	}
	if pos, err := f.Seek(5, io.SeekStart); err != nil || pos != 5 {
		t.Fatalf("seek = %d %v", pos, err)
	}
	if pos, err := f.Seek(-1, io.SeekEnd); err != nil || pos != 9 {
		t.Fatalf("seek end = %d %v", pos, err)
	}
	if err := f.Truncate(5); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadEOF(t *testing.T) {
	c := fakeDevice(t, []step{
		{wantOp: opFileOpen, reply: packet{op: opFileOpenResult, headerPayload: le64(3)}},
		{wantOp: opFileRead, reply: packet{op: opData}},
	})
	f, err := c.Open("f", ModeRDOnly)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read(make([]byte, 4)); err != io.EOF {
		t.Fatalf("want io.EOF, got %v", err)
	}
}

func leI64(v int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return b
}
