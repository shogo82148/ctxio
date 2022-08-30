package ctxio

import (
	"context"
	"errors"
	"io"
	"time"
)

type Reader interface {
	ReadContext(ctx context.Context, data []byte) (n int, err error)
}

type ReadCloser interface {
	Reader
	io.Closer
}

type Writer interface {
	WriteContext(ctx context.Context, data []byte) (n int, err error)
}

type WriteCloser interface {
	Writer
	io.Closer
}

// errInvalidWrite means that a write returned an impossible count.
var errInvalidWrite = errors.New("invalid write result")

// aLongTimeAgo is a non-zero time, far in the past, used for
// immediate cancellation of network operations.
var aLongTimeAgo = time.Unix(1, 0)

// ReadAtLeast reads from r into buf until it has read at least min bytes.
func ReadAtLeast(ctx context.Context, r Reader, buf []byte, min int) (n int, err error) {
	if len(buf) < min {
		return 0, io.ErrShortBuffer
	}
	for n < min && err == nil {
		var nn int
		nn, err = r.ReadContext(ctx, buf[n:])
		n += nn
	}
	if n >= min {
		err = nil
	} else if n > 0 && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

// ReadFull reads exactly len(buf) bytes from r into buf.
func ReadFull(ctx context.Context, r Reader, buf []byte) (n int, err error) {
	return ReadAtLeast(ctx, r, buf, len(buf))
}

// Copy copies from src to dst until either EOF is reached
// on src or an error occurs. It returns the number of bytes
// copied and the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF.
// Because Copy is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
func Copy(ctx context.Context, dst Writer, src Reader) (written int64, err error) {
	return copyBuffer(ctx, dst, src, nil)
}

// CopyBuffer is identical to Copy except that it stages through the
// provided buffer (if one is required) rather than allocating a
// temporary one. If buf is nil, one is allocated; otherwise if it has
// zero length, CopyBuffer panics.
func CopyBuffer(ctx context.Context, dst Writer, src Reader, buf []byte) (written int64, err error) {
	if buf != nil && len(buf) == 0 {
		panic("empty buffer in CopyBuffer")
	}
	return copyBuffer(ctx, dst, src, buf)
}

// copyBuffer is the actual implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
func copyBuffer(ctx context.Context, dst Writer, src Reader, buf []byte) (written int64, err error) {
	if buf == nil {
		size := 32 * 1024
		buf = make([]byte, size)
	}
	for {
		nr, er := src.ReadContext(ctx, buf)
		if nr > 0 {
			nw, ew := dst.WriteContext(ctx, buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

// StringWriter is the interface that wraps the WriteStringContext method.
type StringWriter interface {
	WriteStringContext(ctx context.Context, s string) (n int, err error)
}

func WriteStringContext(ctx context.Context, w Writer, s string) (n int, err error) {
	if sw, ok := w.(StringWriter); ok {
		return sw.WriteStringContext(ctx, s)
	}
	return w.WriteContext(ctx, []byte(s))
}

// NopCloser returns a ReadCloser with a no-op Close method wrapping
// the provided Reader r.
func NopCloser(r Reader) ReadCloser {
	return nopCloser{r}
}

type nopCloser struct {
	Reader
}

func (nopCloser) Close() error { return nil }

// ReadAll reads from r until an error or io.EOF and returns the data it read.
// A successful call returns err == nil, not err == io.EOF. Because ReadAll is
// defined to read from src until io.EOF, it does not treat an io.EOF from Read
// as an error to be reported.
func ReadAll(ctx context.Context, r Reader) ([]byte, error) {
	b := make([]byte, 0, 512)
	for {
		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
		n, err := r.ReadContext(ctx, b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return b, err
		}
	}
}
