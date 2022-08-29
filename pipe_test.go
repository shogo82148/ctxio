package ctxio

import (
	"context"
	"testing"
)

func checkWrite(t *testing.T, w Writer, data []byte, c chan int) {
	n, err := w.WriteContext(context.Background(), data)
	if err != nil {
		t.Errorf("write: %v", err)
	}
	if n != len(data) {
		t.Errorf("short write: %d != %d", n, len(data))
	}
	c <- 0
}

// Test a single read/write pair.
func TestPipe1(t *testing.T) {
	c := make(chan int)
	r, w := Pipe()
	var buf = make([]byte, 64)
	go checkWrite(t, w, []byte("hello, world"), c)
	n, err := r.ReadContext(context.Background(), buf)
	if err != nil {
		t.Errorf("read: %v", err)
	} else if n != 12 || string(buf[0:12]) != "hello, world" {
		t.Errorf("bad read: got %q", buf[0:n])
	}
	<-c
	r.Close()
	w.Close()
}
