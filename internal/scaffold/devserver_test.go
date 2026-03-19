// devserver_test.go — Tests for dev server port detection from Vite stdout.

package scaffold

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// ============================================
// Vite stdout port parsing
// ============================================

func TestParseVitePort_StandardOutput(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  int
		found bool
	}{
		{
			name:  "standard vite output",
			line:  "  ➜  Local:   http://localhost:5173/",
			want:  5173,
			found: true,
		},
		{
			name:  "alternate port",
			line:  "  ➜  Local:   http://localhost:5174/",
			want:  5174,
			found: true,
		},
		{
			name:  "without trailing slash",
			line:  "  ➜  Local:   http://localhost:5173",
			want:  5173,
			found: true,
		},
		{
			name:  "with spaces",
			line:  "    Local:   http://localhost:3000/",
			want:  3000,
			found: true,
		},
		{
			name:  "no match - network line",
			line:  "  ➜  Network: http://192.168.1.1:5173/",
			want:  0,
			found: false,
		},
		{
			name:  "no match - random text",
			line:  "vite v5.0.0 building for development...",
			want:  0,
			found: false,
		},
		{
			name:  "no match - empty",
			line:  "",
			want:  0,
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, found := ParseVitePort(tt.line)
			if found != tt.found {
				t.Errorf("ParseVitePort(%q): found=%v, want %v", tt.line, found, tt.found)
			}
			if port != tt.want {
				t.Errorf("ParseVitePort(%q): port=%d, want %d", tt.line, port, tt.want)
			}
		})
	}
}

// ============================================
// Port polling fallback
// ============================================

func TestPollForDevServer_FindsListeningPort(t *testing.T) {
	// Start a temporary HTTP server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	found, err := PollForDevServer(ctx, port, port)
	if err != nil {
		t.Fatalf("PollForDevServer: %v", err)
	}
	if found != port {
		t.Errorf("PollForDevServer: want port %d, got %d", port, found)
	}
}

func TestPollForDevServer_TimeoutWhenNoServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Use an unlikely port range.
	_, err := PollForDevServer(ctx, 59990, 59991)
	if err == nil {
		t.Error("PollForDevServer should return error on timeout")
	}
}

func TestPollForDevServer_FindsPortInRange(t *testing.T) {
	// Start server on a specific port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Search a range that includes the port.
	found, err := PollForDevServer(ctx, port-2, port+2)
	if err != nil {
		t.Fatalf("PollForDevServer: %v", err)
	}
	if found != port {
		t.Errorf("PollForDevServer: want port %d, got %d", port, found)
	}
}

// ============================================
// DevServerDetector integration
// ============================================

func TestDevServerDetector_ParsesLineAndReturnsPort(t *testing.T) {
	d := NewDevServerDetector()

	// Feed Vite output lines.
	d.FeedLine("vite v5.0.0 building for development...")
	d.FeedLine("  ➜  Local:   http://localhost:5173/")

	port, ok := d.DetectedPort()
	if !ok {
		t.Fatal("expected port to be detected")
	}
	if port != 5173 {
		t.Errorf("want port 5173, got %d", port)
	}
}

func TestDevServerDetector_NoPortBeforeReady(t *testing.T) {
	d := NewDevServerDetector()

	d.FeedLine("vite v5.0.0 building for development...")

	_, ok := d.DetectedPort()
	if ok {
		t.Error("port should not be detected before Local line appears")
	}
}

func TestDevServerDetector_FallbackPolling(t *testing.T) {
	// Start a temporary HTTP server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	d := NewDevServerDetector()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := d.WaitForReady(ctx, port, port)
	if err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
	if got != port {
		t.Errorf("WaitForReady: want port %d, got %d", port, got)
	}
}

func TestDevServerDetector_StdoutTakesPrecedence(t *testing.T) {
	// Start a temporary HTTP server on some port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	d := NewDevServerDetector()
	d.FeedLine(fmt.Sprintf("  ➜  Local:   http://localhost:%d/", port))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := d.WaitForReady(ctx, 59990, 59991) // Wrong range, but stdout should win.
	if err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
	if got != port {
		t.Errorf("WaitForReady: want port %d (from stdout), got %d", port, got)
	}
}
