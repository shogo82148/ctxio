package ctxio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

func TestWatchWriter(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	data := bytes.Repeat([]byte("foobar01"), 1024*1024)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer r.Close()
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			t.Error(err)
		}
		if n != int64(len(data)) {
			t.Errorf("want %d, got %d", len(data), n)
		}
	}()

	ww := NewWriter(w)

	n, err := ww.WriteContext(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("want %d, got %d", len(data), n)
	}
	if err := ww.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func TestGoWriter(t *testing.T) {
	r, w := io.Pipe()
	data := bytes.Repeat([]byte("foobar01"), 1024*1024)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer r.Close()
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			t.Error(err)
		}
		if n != int64(len(data)) {
			t.Errorf("want %d, got %d", len(data), n)
		}
	}()

	ww := NewWriter(w)

	n, err := ww.WriteContext(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("want %d, got %d", len(data), n)
	}
	if err := ww.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func TestGoWriter_Timeout(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	data := bytes.Repeat([]byte("foobar01"), 1024*1024)
	ww := NewWriter(w)

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		n, err := ww.WriteContext(ctx, data)
		if n != writeBufferSize {
			t.Errorf("want %d, got %d", writeBufferSize, n)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("want context.DeadlineExceeded, got %v", err)
		}
	}()

	go func() {
		defer r.Close()
		io.Copy(io.Discard, r)
	}()

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		n, err := ww.WriteContext(ctx, data)
		if n != len(data) {
			t.Errorf("want %d, got %d", len(data), n)
		}
		if err != nil {
			t.Error(err)
		}
	}()

	if err := ww.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
