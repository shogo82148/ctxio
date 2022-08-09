package ctxio

import (
	"context"
	"io"
	"io/fs"
	"sync"
	"time"
)

type readDeadlineSetter interface {
	SetReadDeadline(t time.Time) error
}

func NewReader(reader io.Reader) ReadCloser {
	if setter, ok := reader.(readDeadlineSetter); ok {
		if err := setter.SetReadDeadline(time.Time{}); err == nil {
			return newWatchReader(reader, setter)
		}
	}
	return newGoReader(reader)
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

func newWatchReader(reader io.Reader, setter readDeadlineSetter) ReadCloser {
	watcher := make(chan context.Context, 1)
	finished := make(chan struct{})
	closed := make(chan struct{})

	r := &watchReader{
		r:        reader,
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
				r.cancel(ctx.Err())
				done = nil
				goto START
			case <-finished:
			case <-closed:
				return
			}
		}
	}()

	return r
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

type goReader struct {
	r         io.Reader
	closed    chan struct{}
	closeOnce sync.Once

	mu    sync.Mutex
	buf   []byte
	start int
	end   int
	req   chan readRequest
	res   chan readResult
}

type readRequest struct {
	n int
}

type readResult struct {
	buf []byte
	n   int
	err error
}

func newGoReader(reader io.Reader) ReadCloser {
	r := &goReader{
		r:      reader,
		closed: make(chan struct{}),
		req:    make(chan readRequest),
		res:    make(chan readResult),
	}
	go r.loop()
	return r
}

func (r *goReader) ReadContext(ctx context.Context, data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.start != r.end {
		// read from buffered data
		end := r.start + len(data)
		if end > r.end {
			end = r.end
		}
		copy(data, r.buf[r.start:end])
		n = end - r.start
		r.start = end
		return
	}

	// send a read request
	var res readResult
	select {
	case r.req <- readRequest{n: len(data)}:
		select {
		case res = <-r.res:
		case <-r.closed:
			return 0, fs.ErrClosed
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	case res = <-r.res:
	case <-r.closed:
		return 0, fs.ErrClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	end := len(data)
	if end > res.n {
		end = res.n
	}
	copy(data, res.buf[:end])
	r.start = end
	r.end = res.n
	r.buf = res.buf
	return end, res.err
}

func (r *goReader) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
	})
	return nil
}

func (r *goReader) loop() {
	buf1 := []byte{}
	buf2 := []byte{}
	for {
		// receive a read request
		var req readRequest
		select {
		case req = <-r.req:
		case <-r.closed:
			return
		}

		// handle the request
		if cap(buf1) < req.n {
			buf1 = make([]byte, req.n)
		}
		n, err := r.r.Read(buf1[:req.n])

		// send a response
		res := readResult{
			buf: buf1[:req.n],
			n:   n,
			err: err,
		}
		select {
		case r.res <- res:
		case <-r.closed:
			return
		}

		buf1, buf2 = buf2, buf1
	}
}
