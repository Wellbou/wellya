package stream

import (
	"errors"
	"io"
	"sync"
	"time"
)

const (
	_BUFFERING_AMOUNT = 64 * 1024
	_BUFFERING_PERIOD = 20 * time.Millisecond
	_BUFFERING_MAX    = 512 << 20
)

var errOutOfSize = errors.New("position is out of data size")
var errBufferTooLarge = errors.New("buffer exceeded size limit")

// BufferedStream is a producer/consumer byte stream.
//
// A single background goroutine (bufferFrames) is the ONLY reader of
// source and appends everything it reads to readBuffer. Consumers
// (Read) only ever serve bytes from readBuffer and wait on a condvar
// while data is on the way. As a result no caller ever blocks on the
// network while holding the mutex, so UI-thread helpers
// (Progress/Seek/IsDone/...) always return instantly.
type BufferedStream struct {
	source       io.ReadCloser
	sourceClosed bool
	bufferTimer  *time.Ticker
	closed       chan bool
	cond         *sync.Cond
	lastError    error
	readBuffer   []byte
	readIndex    int64
	totalSize    int64
	buffered     bool
	eof          bool
	closedFlag   bool
	done         bool
	mux          sync.Mutex
}

func NewBufferedStream(source io.ReadCloser, totalSize int64) *BufferedStream {
	rs := BufferedStream{
		source:    source,
		totalSize: totalSize,
	}
	rs.bufferTimer = time.NewTicker(_BUFFERING_PERIOD)
	rs.closed = make(chan bool)
	rs.cond = sync.NewCond(&rs.mux)

	go rs.bufferFrames(_BUFFERING_AMOUNT)
	return &rs
}

func (h *BufferedStream) Length() int64 {
	if h == nil {
		return 0
	}
	return int64(h.totalSize)
}

func (h *BufferedStream) Close() error {
	h.mux.Lock()
	h.closedFlag = true
	h.stopBuffering()
	h.cond.Broadcast()
	h.closeSourceLocked()
	h.mux.Unlock()
	return nil
}

func (h *BufferedStream) closeSourceLocked() {
	if h.sourceClosed {
		return
	}
	h.sourceClosed = true
	_ = h.source.Close()
}

func (h *BufferedStream) finishLocked() {
	h.stopBuffering()
	h.done = true
	h.eof = true
	h.closeSourceLocked()
	h.cond.Broadcast()
}

func (h *BufferedStream) Read(dest []byte) (n int, err error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	for {
		if h.closedFlag {
			return 0, io.EOF
		}

		readBufLen := int64(len(h.readBuffer))
		if h.readIndex < readBufLen {
			destLen := int64(len(dest))
			endIndex := h.readIndex + destLen
			if endIndex > readBufLen {
				endIndex = readBufLen
			}
			n = copy(dest, h.readBuffer[h.readIndex:endIndex])
			h.readIndex += int64(n)
			return n, nil
		}

		if h.done || h.eof || (h.totalSize > 0 && h.readIndex >= h.totalSize) {
			h.finishLocked()
			return 0, io.EOF
		}

		h.cond.Wait()
	}
}

func (h *BufferedStream) Seek(offset int64, whence int) (pos int64, err error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = h.readIndex + offset
	case io.SeekEnd:
		pos = h.totalSize + offset
	}

	if pos < 0 || (h.totalSize > 0 && pos > h.totalSize) {
		pos = h.readIndex
		err = errOutOfSize
	} else {
		h.done = h.totalSize > 0 && pos == h.totalSize
		h.readIndex = pos
		h.cond.Broadcast()
	}

	return
}

func (h *BufferedStream) IsDone() bool {
	if h == nil {
		return false
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.done
}

func (h *BufferedStream) IsBuffered() bool {
	if h == nil {
		return false
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.buffered
}

func (h *BufferedStream) Progress() float64 {
	if h == nil {
		return 0
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.totalSize <= 0 {
		return 0
	}
	return float64(h.readIndex) / float64(h.totalSize)
}

func (h *BufferedStream) BufferingProgress() float64 {
	if h == nil {
		return 0
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	if h.totalSize <= 0 {
		if h.buffered {
			return 1
		}
		return 0
	}
	return float64(len(h.readBuffer)) / float64(h.totalSize)
}

func (h *BufferedStream) WriteTo(dest io.Writer) (int64, error) {
	h.mux.Lock()
	snapshot := make([]byte, len(h.readBuffer))
	copy(snapshot, h.readBuffer)
	h.mux.Unlock()
	n, err := dest.Write(snapshot)
	return int64(n), err
}

func (h *BufferedStream) Error() error {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.lastError
}

func (h *BufferedStream) stopBuffering() {
	h.buffered = true
	if h.closed != nil {
		h.bufferTimer.Stop()
		close(h.closed)
		h.closed = nil
	}
}

func (h *BufferedStream) bufferFrames(size int64) {
	for {
		h.mux.Lock()

		if h.closedFlag || h.buffered ||
			(h.totalSize > 0 && h.totalSize <= int64(len(h.readBuffer))) ||
			int64(len(h.readBuffer)) >= _BUFFERING_MAX {
			if int64(len(h.readBuffer)) >= _BUFFERING_MAX && !h.closedFlag {
				h.lastError = errBufferTooLarge
				h.eof = true
			}
			h.closeSourceLocked()
			h.stopBuffering()
			h.cond.Broadcast()
			h.mux.Unlock()
			return
		}

		h.mux.Unlock()

		buf := make([]byte, size)
		n, err := io.ReadFull(h.source, buf)

		h.mux.Lock()
		if h.closedFlag {
			h.mux.Unlock()
			return
		}
		h.readBuffer = append(h.readBuffer, buf[:n]...)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			h.eof = true
			h.closeSourceLocked()
			h.stopBuffering()
			h.cond.Broadcast()
			h.mux.Unlock()
			return
		}
		if err != nil {
			h.lastError = err
			h.eof = true
			h.closeSourceLocked()
			h.stopBuffering()
			h.cond.Broadcast()
			h.mux.Unlock()
			return
		}
		h.cond.Broadcast()
		h.mux.Unlock()

		h.mux.Lock()
		if h.closedFlag {
			h.mux.Unlock()
			return
		}
		ch := h.closed
		timerCh := h.bufferTimer.C
		h.mux.Unlock()

		select {
		case <-timerCh:
			continue
		case <-ch:
			return
		}
	}
}
