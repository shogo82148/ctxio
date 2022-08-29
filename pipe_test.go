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

type pipeReturn struct {
	n   int
	err error
}

// Test a large write that requires multiple reads to satisfy.
func writer(w WriteCloser, buf []byte, c chan pipeReturn) {
	n, err := w.WriteContext(context.Background(), buf)
	w.Close()
	c <- pipeReturn{n, err}
}

func TestPipe3(t *testing.T) {
	c := make(chan pipeReturn)
	r, w := Pipe()
	var wdat = make([]byte, 128)
	for i := 0; i < len(wdat); i++ {
		wdat[i] = byte(i)
	}
	go writer(w, wdat, c)
	var rdat = make([]byte, 1024)
	tot := 0
	for n := 1; n <= 256; n *= 2 {
		nn, err := r.ReadContext(context.Background(), rdat[tot:tot+n])
		if err != nil && err != io.EOF {
			t.Fatalf("read: %v", err)
		}

		// only final two reads should be short - 1 byte, then 0
		expect := n
		if n == 128 {
			expect = 1
		} else if n == 256 {
			expect = 0
			if err != io.EOF {
				t.Fatalf("read at end: %v", err)
			}
		}
		if nn != expect {
			t.Fatalf("read %d, expected %d, got %d", n, expect, nn)
		}
		tot += nn
	}
	pr := <-c
	if pr.n != 128 || pr.err != nil {
		t.Fatalf("write 128: %d, %v", pr.n, pr.err)
	}
	if tot != 128 {
		t.Fatalf("total read %d != 128", tot)
	}
	for i := 0; i < 128; i++ {
		if rdat[i] != byte(i) {
			t.Fatalf("rdat[%d] = %d", i, rdat[i])
		}
	}
}
