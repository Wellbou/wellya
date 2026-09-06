package api

import (
	"context"
	"io"
	"time"
)

// TimeLimitedReader ties the body lifetime to the context: Close cancels
// the request context so no goroutine/connection leaks. There is
// deliberately no per-read idle timeout: playback legitimately stops
// reading while paused, and any stall killer here would abort resume.
type TimeLimitedReader struct {
	cancel context.CancelFunc
	ctx    context.Context
	body   io.ReadCloser
}

func NewTimeLimitedReader(r io.ReadCloser, ctx context.Context, ctxCancel context.CancelFunc, timeout time.Duration) *TimeLimitedReader {
	return &TimeLimitedReader{
		cancel: ctxCancel,
		ctx:    ctx,
		body:   r,
	}
}

func (r *TimeLimitedReader) Read(dest []byte) (int, error) {
	return r.body.Read(dest)
}

func (r *TimeLimitedReader) Close() error {
	r.cancel()
	return r.body.Close()
}
