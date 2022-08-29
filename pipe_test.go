package ctxio

import (
	"context"
	"io"
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

func reader(t *testing.T, r Reader, c chan int) {
	var buf = make([]byte, 64)
	for {
		n, err := r.ReadContext(context.Background(), buf)
		if err == io.EOF {
			c <- 0
			break
		}
		if err != nil {
			t.Errorf("read: %v", err)
		}
		c <- n
	}
}

// Test a sequence of read/write pairs.
func TestPipe2(t *testing.T) {
	c := make(chan int)
	r, w := Pipe()
	go reader(t, r, c)
	var buf = make([]byte, 64)
	for i := 0; i < 5; i++ {
		p := buf[0 : 5+i*10]
		n, err := w.WriteContext(context.Background(), p)
		if n != len(p) {
			t.Errorf("wrote %d, got %d", len(p), n)
		}
		if err != nil {
			t.Errorf("write: %v", err)
		}
		nn := <-c
		if nn != n {
			t.Errorf("wrote %d, read got %d", n, nn)
		}
	}
	w.Close()
	nn := <-c
	if nn != 0 {
		t.Errorf("final read got %d", nn)
	}
}
