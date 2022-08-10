package ctxio

import (
	"context"
	"io"
	"sync"
	"time"
)

type writeDeadlineSetter interface {
	SetWriteDeadline(t time.Time) error
}

func NewWriter(writer io.Writer) WriteCloser {
	if setter, ok := writer.(writeDeadlineSetter); ok {
		if err := setter.SetWriteDeadline(time.Time{}); err == nil {
			return newWatchWriter(writer, setter)
		}
	}
	return newGoWriter(writer)
}

type watchWriter struct {
	w        io.Writer
	setter   writeDeadlineSetter
	watcher  chan<- context.Context
	finished chan<- struct{}

	closed    chan struct{}
	closeOnce sync.Once

	mu  sync.Mutex
	err error
}

func newWatchWriter(writer io.Writer, setter writeDeadlineSetter) WriteCloser {
	watcher := make(chan context.Context, 1)
	finished := make(chan struct{})
	closed := make(chan struct{})

	w := &watchWriter{
		w:        writer,
		setter:   setter,
		watcher:  watcher,
		finished: finished,
		closed:   closed,
	}

	go func() {
		for {
			var ctx context.Context
			select {
			case ctx = <-watcher:
			case <-closed:
				return
			}

			done := ctx.Done()
		START:
			select {
			case <-done:
				w.cancel(ctx.Err())
				done = nil
				goto START
			case <-finished:
			case <-closed:
				return
			}
		}
	}()

	return w
}

func (w *watchWriter) WriteContext(ctx context.Context, data []byte) (n int, err error) {
	if w.watchCancel(ctx); err != nil {
		return 0, err
	}

	n, err = w.w.Write(data)
	if err != nil {
		canceled := w.canceled()
		if canceled != nil {
			err = canceled
		}
	}
	w.finish()
	return
}

func (w *watchWriter) Close() error {
	w.closeOnce.Do(func() {
		close(w.closed)
	})
	return nil
}

func (w *watchWriter) watchCancel(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// clear the error
	w.mu.Lock()
	w.err = nil
	w.mu.Unlock()

	// start to watch
	w.watcher <- ctx
	return nil
}

func (w *watchWriter) finish() {
	select {
	case w.finished <- struct{}{}:
	case <-w.closed:
	}
}

func (w *watchWriter) cancel(err error) {
	w.mu.Lock()
	w.err = err
	w.mu.Unlock()

	w.setter.SetWriteDeadline(aLongTimeAgo)
}

func (w *watchWriter) canceled() error {
	w.mu.Lock()
	err := w.err
	w.mu.Unlock()
	return err
}

func newGoWriter(writer io.Writer) WriteCloser {
	return nil
}
