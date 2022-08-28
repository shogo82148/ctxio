package ctxio

import (
	"bytes"
	"context"
	"testing"
)

type Buffer struct {
	bytes.Buffer
}

func (buf *Buffer) ReadContext(ctx context.Context, data []byte) (int, error) {
	return buf.Read(data)
}

func (buf *Buffer) WriteContext(ctx context.Context, data []byte) (int, error) {
	return buf.Write(data)
}

func TestCopy(t *testing.T) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	Copy(context.Background(), wb, rb)
	if wb.String() != "hello, world." {
		t.Errorf("Copy did not work properly")
	}
}

func TestCopyBuffer(t *testing.T) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	CopyBuffer(context.Background(), wb, rb, make([]byte, 1)) // Tiny buffer to keep it honest.
	if wb.String() != "hello, world." {
		t.Errorf("CopyBuffer did not work properly")
	}
}

func TestCopyBufferNil(t *testing.T) {
	rb := new(Buffer)
	wb := new(Buffer)
	rb.WriteString("hello, world.")
	CopyBuffer(context.Background(), wb, rb, nil) // Should allocate a buffer.
	if wb.String() != "hello, world." {
		t.Errorf("CopyBuffer did not work properly")
	}
}

func TestReadAll(t *testing.T) {
	rb := new(Buffer)
	rb.WriteString("hello, world.")
	data, err := ReadAll(context.Background(), rb)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello, world." {
		t.Errorf("ReadAll did not work properly")
	}
}
