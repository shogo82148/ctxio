package ctxio

import (
	"context"
	"io"
	"sync"
	"time"
)

type readDeadlineSetter interface {
	SetReadDeadline(t time.Time) error
}

func NewReader(reader io.Reader) ReadCloser {
	setter, _ := reader.(readDeadlineSetter)
	watcher := make(chan context.Context, 1)
	finished := make(chan struct{})
	closed := make(chan struct{})

	r := &watchReader{
		r:        reader,
		setter:   setter,
		watcher:  watcher,
		finished: finished,
	}

	go func() {
		for {
			var ctx context.Context
			select {
			case ctx = <-watcher:
			case <-closed:
			}

			select {
			case <-ctx.Done():
				r.cancel(ctx.Err())
			case <-finished:
			case <-closed:
				return
			}
		}
	}()

	return r
}

type watchReader struct {
	r        io.Reader
	setter   readDeadlineSetter
	watcher  chan<- context.Context
	finished chan<- struct{}
	closed   chan struct{}

	mu  sync.Mutex
	err error
}

func (r *watchReader) ReadContext(ctx context.Context, data []byte) (n int, err error) {
	if r.watchCancel(ctx); err != nil {
		return 0, err
	}

	n, err = r.r.Read(data)
	if err != nil {
		canceled := r.canceled()
		if canceled != nil {
			err = canceled
		}
	}
	r.finish()
	return
}

func (r *watchReader) Close() error {
	r.mu.Lock()
	select {
	case <-r.closed:
		// r.closed is already closed
		// nothing to do here
	default:
		close(r.closed)
	}
	r.mu.Unlock()
	return nil
}

func (r *watchReader) watchCancel(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// clear the error
	r.mu.Lock()
	r.err = nil
	r.mu.Unlock()

	// start to watch
	r.watcher <- ctx
	return nil
}

func (r *watchReader) finish() {
	select {
	case r.finished <- struct{}{}:
	case <-r.closed:
	}
}

func (r *watchReader) cancel(err error) {
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()

	r.setter.SetReadDeadline(aLongTimeAgo)
}

func (r *watchReader) canceled() error {
	r.mu.Lock()
	err := r.err
	r.mu.Unlock()
	return err
}
