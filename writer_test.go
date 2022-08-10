package ctxio

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"testing"
)

func TestWatchWriter(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

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
