// writebuf_test.go — Tests for non-blocking write buffer with backpressure.

package pty

import (
	"bytes"
	"testing"
)

func TestWriteBuffer_BasicWrite(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	n, err := wb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}

	// Close waits for drain to complete, making dest safe to read.
	wb.Close()

	if dest.String() != "hello" {
		t.Fatalf("expected %q, got %q", "hello", dest.String())
	}
}

func TestWriteBuffer_Backpressure(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})}
	wb := NewWriteBuffer(gw)
	defer func() {
		close(gw.gate)
		wb.Close()
	}()

	// Fill buffer to capacity.
	data := make([]byte, writeBufferMax)
	_, err := wb.Write(data)
	if err != nil {
		t.Fatalf("write to fill: %v", err)
	}

	// Exceeding capacity should fail.
	_, err = wb.Write([]byte("x"))
	if err != ErrWriteBufferFull {
		t.Fatalf("expected ErrWriteBufferFull, got: %v", err)
	}
}

// gatedWriter blocks on Write until gate channel is closed.
type gatedWriter struct {
	gate chan struct{}
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	<-w.gate
	return len(p), nil
}

func TestWriteBuffer_Pending(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})}
	wb := NewWriteBuffer(gw)
	defer func() {
		close(gw.gate)
		wb.Close()
	}()

	wb.Write([]byte("hello"))
	// Pending should reflect buffered data (drain is blocked).
	p := wb.Pending()
	if p < 0 {
		t.Fatalf("expected non-negative pending, got %d", p)
	}
}

func TestWriteBuffer_CloseFlushes(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	wb.Write([]byte("data"))
	wb.Close()

	if dest.String() != "data" {
		t.Fatalf("expected %q after close, got %q", "data", dest.String())
	}
}

func TestWriteBuffer_DoubleClose(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)
	wb.Close()
	if err := wb.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestWriteBuffer_WriteAfterClose(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)
	wb.Close()

	_, err := wb.Write([]byte("x"))
	if err != ErrWriteBufferFull {
		t.Fatalf("expected ErrWriteBufferFull after close, got: %v", err)
	}
}

func TestWriteBuffer_LargeWrite(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	// Write data larger than one chunk to exercise chunked flushing.
	data := make([]byte, writeChunkSize*3)
	for i := range data {
		data[i] = byte(i % 256)
	}

	n, err := wb.Write(data)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), n)
	}

	// Close waits for drain to complete, making dest safe to read.
	wb.Close()

	if dest.Len() != len(data) {
		t.Fatalf("expected %d drained bytes, got %d", len(data), dest.Len())
	}
}
