package stream

import (
	"bytes"
	"io"
	"testing"
)

func testData() []byte {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func TestSeekForwardThenRead(t *testing.T) {
	data := make([]byte, 4<<20)
	for i := range data {
		data[i] = byte(i * 31)
	}
	bs := NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))
	defer bs.Close()

	const pos = 3 << 20
	if _, err := bs.Seek(pos, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	buf := make([]byte, 10)
	n, err := io.ReadFull(bs, buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != 10 {
		t.Fatalf("short read: %d", n)
	}
	for i := 0; i < 10; i++ {
		if buf[i] != data[pos+i] {
			t.Fatalf("byte %d: got %d want %d", i, buf[i], data[pos+i])
		}
	}
}

func TestSeekBackReread(t *testing.T) {
	data := testData()
	bs := NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))
	defer bs.Close()

	first := make([]byte, 64)
	if _, err := io.ReadFull(bs, first); err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := bs.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	second := make([]byte, 64)
	if _, err := io.ReadFull(bs, second); err != nil {
		t.Fatalf("reread: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("reread mismatch")
	}
}

func TestReadToEOF(t *testing.T) {
	data := testData()
	bs := NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))
	defer bs.Close()

	all, err := io.ReadAll(bs)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if !bytes.Equal(all, data) {
		t.Fatalf("data mismatch: got %d bytes want %d", len(all), len(data))
	}
	if !bs.IsDone() {
		t.Fatal("expected done after EOF")
	}
}

func TestSeekPastEnd(t *testing.T) {
	data := testData()
	bs := NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))
	defer bs.Close()

	if _, err := bs.Seek(int64(len(data)+100), io.SeekStart); err == nil {
		t.Fatal("expected out-of-size error")
	}
}
