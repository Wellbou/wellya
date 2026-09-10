package tracker

import (
	"io"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	mp3 "github.com/dece2183/go-stream-mp3"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/stream"
)

const (
	_PROGRESS_UPDATE_PERIOD = 33 * time.Millisecond
)

type readWrapper struct {
	program        *tea.Program
	decoder        *mp3.Decoder
	trackBuffer    *stream.BufferedStream
	trackBuffered  bool
	trackDone      bool
	lastUpdateTime time.Time
	mux            sync.Mutex
}

func (w *readWrapper) NewReaderWithDecoder(reader *stream.BufferedStream, decoder *mp3.Decoder) {
	w.mux.Lock()
	defer w.mux.Unlock()
	w.trackBuffered = false
	w.trackDone = false
	w.trackBuffer = reader
	w.decoder = decoder
	w.lastUpdateTime = time.Now()
}

func (w *readWrapper) Close() {
	w.mux.Lock()
	buffer := w.trackBuffer
	w.decoder = nil
	w.trackBuffer = nil
	w.mux.Unlock()

	if buffer != nil {
		buffer.Close()
	}
}

func (w *readWrapper) Read(dest []byte) (n int, err error) {
	w.mux.Lock()
	buffer := w.trackBuffer
	decoder := w.decoder
	w.mux.Unlock()

	if buffer == nil || decoder == nil {
		err = io.EOF
		return
	}

	n, err = decoder.Read(dest)
	if err != nil && err != io.EOF {
		if buffer.Error() != nil {
			err = buffer.Error()
			log.Print(log.LVL_ERROR, "buffering error: %s", err)
			go w.program.Send(STOP)
			return
		}
		log.Print(log.LVL_WARNING, "mp3 decoding error: %s", err)
		err = nil
	}

	w.mux.Lock()
	if buffer.IsBuffered() && !w.trackBuffered {
		w.trackBuffered = true
		go w.program.Send(BUFFERING_COMPLETE)
	}

	if buffer.IsDone() && !w.trackDone {
		w.trackDone = true
		decoder.Seek(0, io.SeekStart)
		buffer.Close()
		go w.program.Send(NEXT)
	} else if !w.trackDone && time.Since(w.lastUpdateTime) > _PROGRESS_UPDATE_PERIOD {
		w.lastUpdateTime = time.Now()
		fraction := ProgressControl(buffer.Progress())
		go w.program.Send(fraction)
	}
	w.mux.Unlock()

	return
}

func (w *readWrapper) Buffer() *stream.BufferedStream {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.trackBuffer
}

func (w *readWrapper) Seek(offset int64, whence int) (int64, error) {
	w.mux.Lock()
	decoder := w.decoder
	if decoder != nil {
		w.lastUpdateTime = time.Now()
	}
	w.mux.Unlock()
	if decoder == nil {
		return 0, io.EOF
	}
	return decoder.Seek(offset, whence)
}

func (w *readWrapper) Length() int64 {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.trackBuffer == nil {
		return 0
	}
	return w.trackBuffer.Length()
}

func (w *readWrapper) Progress() float64 {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.trackBuffer == nil {
		return 0
	}
	return w.trackBuffer.Progress()
}
