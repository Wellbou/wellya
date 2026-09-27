package api

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrStreamStalled = errors.New("track stream stalled: no data from server")

// TimeLimitedReader ties the body lifetime to the request context and
// guards against dead connections: if a single Read returns nothing for
// `timeout`, the request is cancelled and Read fails instead of hanging
// the buffering goroutine forever. The timer is armed only while a Read
// is in flight, so an idle (paused) consumer never trips it.
type TimeLimitedReader struct {
	cancel  context.CancelFunc
	ctx     context.Context
	body    io.ReadCloser
	timeout time.Duration
}

func NewTimeLimitedReader(r io.ReadCloser, ctx context.Context, ctxCancel context.CancelFunc, timeout time.Duration) *TimeLimitedReader {
	return &TimeLimitedReader{
		cancel:  ctxCancel,
		ctx:     ctx,
		body:    r,
		timeout: timeout,
	}
}

func (r *TimeLimitedReader) Read(dest []byte) (int, error) {
	if r.timeout <= 0 {
		return r.body.Read(dest)
	}
	watchdog := time.AfterFunc(r.timeout, r.cancel)
	n, err := r.body.Read(dest)
	watchdog.Stop()
	if err != nil && r.ctx.Err() != nil && n == 0 {
		return 0, ErrStreamStalled
	}
	return n, err
}

func (r *TimeLimitedReader) Close() error {
	r.cancel()
	return r.body.Close()
}
