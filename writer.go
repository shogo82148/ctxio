package ctxio

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"time"
)

const writeBufferSize = 32 * 1024

type writeDeadlineSetter interface {
	SetWriteDeadline(t time.Time) error
}

type writeCloser struct {
	Writer
}

func (w writeCloser) Close() error {
	if c, ok := w.Writer.(io.Closer); ok {
		_ = c.Close()
	}
	return nil
}

func NewWriter(writer io.Writer) WriteCloser {
	switch w := writer.(type) {
	case bufio.ReadWriter:
		return &nopWriter{w}
	case *bufio.Writer:
		return &nopWriter{w}
	case *bytes.Buffer:
		return &nopWriter{w}
	case *strings.Builder:
		return &nopWriter{w}
	case Writer:
		return writeCloser{w}
	}
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

type writeRequest struct {
	data []byte
	ch   chan writeResponse
}

type writeResponse struct {
	n   int
	err error
}

type goWriter struct {
	w         io.Writer
	closed    chan struct{}
	closeOnce sync.Once

	mu   sync.Mutex
	buf1 []byte
	buf2 []byte
	ch   chan writeRequest
}

func newGoWriter(writer io.Writer) WriteCloser {
	w := &goWriter{
		w:      writer,
		closed: make(chan struct{}),
		buf1:   make([]byte, writeBufferSize),
		buf2:   make([]byte, writeBufferSize),
		ch:     make(chan writeRequest),
	}
	go w.loop()
	return w
}

func (w *goWriter) WriteContext(ctx context.Context, data []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for n < len(data) {
		m, err := w.writeContext(ctx, data[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return
}

func (w *goWriter) writeContext(ctx context.Context, data []byte) (n int, err error) {
	buf := w.buf1
	w.buf1, w.buf2 = w.buf2, w.buf1
	ch := make(chan writeResponse, 1)
	n = copy(buf, data)

	req := writeRequest{
		data: buf[:n],
		ch:   ch,
	}
	select {
	case w.ch <- req:
	case <-ctx.Done():
		return n, ctx.Err()
	}

	select {
	case res := <-ch:
		return res.n, res.err
	case <-ctx.Done():
		return n, ctx.Err()
	}
}

func (w *goWriter) Close() error {
	w.closeOnce.Do(func() {
		close(w.closed)
	})
	return nil
}

func (w *goWriter) loop() {
	for {
		var req writeRequest
		select {
		case req = <-w.ch:
		case <-w.closed:
			return
		}
		n, err := w.w.Write(req.data)

		res := writeResponse{
			n:   n,
			err: err,
		}
		req.ch <- res
	}
}

type nopWriter struct {
	io.Writer
}

func (w *nopWriter) WriteContext(ctx context.Context, data []byte) (int, error) {
	return w.Write(data)
}

func (w *nopWriter) Close() error {
	return nil
}
