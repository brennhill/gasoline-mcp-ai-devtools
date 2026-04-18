package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWSAcceptKeyKnownVector(t *testing.T) {
	t.Parallel()

	got := wsAcceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("wsAcceptKey mismatch: got %q want %q", got, want)
	}
}

func TestWSWriteFrameLengthEncoding(t *testing.T) {
	t.Parallel()

	t.Run("short payload", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		rw := bufio.NewReadWriter(bufio.NewReader(&b), bufio.NewWriter(&b))
		if err := wsWriteFrame(rw, 0x1, []byte("hi")); err != nil {
			t.Fatalf("wsWriteFrame failed: %v", err)
		}
		got := b.Bytes()
		if len(got) != 4 {
			t.Fatalf("unexpected frame length: got %d", len(got))
		}
		if got[0] != 0x81 || got[1] != 0x02 || got[2] != 'h' || got[3] != 'i' {
			t.Fatalf("unexpected frame bytes: %v", got)
		}
	})

	t.Run("extended 16-bit payload", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte{'a'}, 126)
		var b bytes.Buffer
		rw := bufio.NewReadWriter(bufio.NewReader(&b), bufio.NewWriter(&b))
		if err := wsWriteFrame(rw, 0x2, payload); err != nil {
			t.Fatalf("wsWriteFrame failed: %v", err)
		}
		got := b.Bytes()
		if len(got) != 4+len(payload) {
			t.Fatalf("unexpected frame length: got %d want %d", len(got), 4+len(payload))
		}
		if got[0] != 0x82 || got[1] != 126 || got[2] != 0x00 || got[3] != 126 {
			t.Fatalf("unexpected frame header: %v", got[:4])
		}
	})

	t.Run("extended 64-bit payload", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte{'b'}, 65536)
		var b bytes.Buffer
		rw := bufio.NewReadWriter(bufio.NewReader(&b), bufio.NewWriter(&b))
		if err := wsWriteFrame(rw, 0x2, payload); err != nil {
			t.Fatalf("wsWriteFrame failed: %v", err)
		}
		got := b.Bytes()
		if len(got) != 10+len(payload) {
			t.Fatalf("unexpected frame length: got %d want %d", len(got), 10+len(payload))
		}
		if got[0] != 0x82 || got[1] != 127 {
			t.Fatalf("unexpected first header bytes: %v", got[:2])
		}
		if n := binary.BigEndian.Uint64(got[2:10]); n != uint64(len(payload)) {
			t.Fatalf("unexpected encoded payload length: got %d want %d", n, len(payload))
		}
	})
}

func TestWSReadFrame(t *testing.T) {
	t.Parallel()

	t.Run("reads unmasked short frame", func(t *testing.T) {
		t.Parallel()

		r := bytes.NewReader([]byte{0x81, 0x02, 'h', 'i'})
		fin, opcode, payload, err := wsReadFrame(r)
		if err != nil {
			t.Fatalf("wsReadFrame failed: %v", err)
		}
		if !fin {
			t.Fatal("expected FIN=true")
		}
		if opcode != 0x1 {
			t.Fatalf("unexpected opcode: got %d want %d", opcode, 0x1)
		}
		if string(payload) != "hi" {
			t.Fatalf("unexpected payload: got %q", string(payload))
		}
	})

	t.Run("reads masked short frame", func(t *testing.T) {
		t.Parallel()

		mask := [4]byte{0x01, 0x02, 0x03, 0x04}
		payload := []byte("hello")
		masked := make([]byte, len(payload))
		for i := range payload {
			masked[i] = payload[i] ^ mask[i%4]
		}

		frame := []byte{0x81, 0x80 | byte(len(payload))}
		frame = append(frame, mask[:]...)
		frame = append(frame, masked...)

		fin, opcode, gotPayload, err := wsReadFrame(bytes.NewReader(frame))
		if err != nil {
			t.Fatalf("wsReadFrame failed: %v", err)
		}
		if !fin {
			t.Fatal("expected FIN=true")
		}
		if opcode != 0x1 {
			t.Fatalf("unexpected opcode: got %d want %d", opcode, 0x1)
		}
		if string(gotPayload) != "hello" {
			t.Fatalf("unexpected payload: got %q", string(gotPayload))
		}
	})

	t.Run("rejects oversize payload", func(t *testing.T) {
		t.Parallel()

		tooLarge := uint64(maxWSPayload + 1)
		frame := []byte{0x81, 127, 0, 0, 0, 0}
		frame = append(frame, byte(tooLarge>>24), byte(tooLarge>>16), byte(tooLarge>>8), byte(tooLarge))
		_, _, _, err := wsReadFrame(bytes.NewReader(frame))
		if err == nil {
			t.Fatal("expected payload-size error")
		}
		if !strings.Contains(err.Error(), "exceeds limit") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestHandleTestPagesSpecialEndpoints(t *testing.T) {
	t.Parallel()

	h := handleTestPages()
	tests := []struct {
		path        string
		wantStatus  int
		wantBodySub string
	}{
		{path: "/tests/", wantStatus: http.StatusOK, wantBodySub: "Kaboom"},
		{path: "/tests/404", wantStatus: http.StatusNotFound, wantBodySub: "network_error"},
		{path: "/tests/500", wantStatus: http.StatusInternalServerError, wantBodySub: "network_error"},
		{path: "/tests/cors-test", wantStatus: http.StatusOK, wantBodySub: "cors_block"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			h.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status mismatch: got %d want %d", rr.Code, tt.wantStatus)
			}
			if !strings.Contains(rr.Body.String(), tt.wantBodySub) {
				t.Fatalf("body missing %q: %q", tt.wantBodySub, rr.Body.String())
			}
		})
	}
}

func TestHandleTestPagesMethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := handleTestPages()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tests/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusMethodNotAllowed)
	}
	if !strings.Contains(rr.Body.String(), "Method not allowed") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleTestHarnessWSValidation(t *testing.T) {
	t.Parallel()

	t.Run("missing upgrade headers", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tests/ws", nil)
		handleTestHarnessWS(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusBadRequest)
		}
		if !strings.Contains(rr.Body.String(), "websocket upgrade required") {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})

	t.Run("upgrade requested but writer is not hijacker", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tests/ws", nil)
		req.Header.Set("Sec-WebSocket-Key", "abc")
		req.Header.Set("Upgrade", "websocket")
		handleTestHarnessWS(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(rr.Body.String(), "does not support hijacking") {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})
}
