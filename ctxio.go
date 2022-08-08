package ctxio

import (
	"context"
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

// aLongTimeAgo is a non-zero time, far in the past, used for
// immediate cancellation of network operations.
var aLongTimeAgo = time.Unix(1, 0)
