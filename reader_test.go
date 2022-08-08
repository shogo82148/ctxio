package ctxio

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestWatchReader(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer w.Close()
		if _, err := w.Write([]byte{'!'}); err != nil {
			t.Error(err)
		}
	}()
	defer r.Close()

	rr := NewReader(r)
	defer rr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	buf := make([]byte, 128)
	n, err := rr.ReadContext(ctx, buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("want 1, but got %d", n)
	}
}

func TestWatchReader_Timeout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	defer r.Close()

	rr := NewReader(r)
	defer rr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	buf := make([]byte, 128)
	_, err = rr.ReadContext(ctx, buf)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, but got %v", err)
	}
}
