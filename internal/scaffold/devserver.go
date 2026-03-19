// devserver.go — Dev server port detection from Vite stdout with fallback polling.

package scaffold

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// vitePortRegex matches Vite's "Local: http://localhost:PORT" output.
var vitePortRegex = regexp.MustCompile(`Local:\s+http://localhost:(\d+)`)

// ParseVitePort extracts the port number from a Vite stdout line.
// Returns the port and true if found, or 0 and false otherwise.
func ParseVitePort(line string) (int, bool) {
	matches := vitePortRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return 0, false
	}
	port, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return port, true
}

// PollForDevServer polls ports in the given range for an HTTP 200 response.
// Returns the first port that responds, or an error if the context expires.
func PollForDevServer(ctx context.Context, startPort, endPort int) (int, error) {
	client := &http.Client{Timeout: 500 * time.Millisecond}

	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("dev server not found on ports %d-%d: %w", startPort, endPort, ctx.Err())
		default:
		}

		for port := startPort; port <= endPort; port++ {
			url := fmt.Sprintf("http://localhost:%d", port)
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return port, nil
			}
		}

		// Brief pause before next poll cycle.
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("dev server not found on ports %d-%d: %w", startPort, endPort, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// DevServerDetector detects the dev server port via stdout parsing and fallback polling.
type DevServerDetector struct {
	mu   sync.Mutex
	port int
	ok   bool
}

// NewDevServerDetector creates a new dev server detector.
func NewDevServerDetector() *DevServerDetector {
	return &DevServerDetector{}
}

// FeedLine feeds a line of stdout output for port detection.
func (d *DevServerDetector) FeedLine(line string) {
	port, found := ParseVitePort(line)
	if found {
		d.mu.Lock()
		d.port = port
		d.ok = true
		d.mu.Unlock()
	}
}

// DetectedPort returns the detected port if available.
func (d *DevServerDetector) DetectedPort() (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.port, d.ok
}

// WaitForReady waits for the dev server to be ready.
// First checks if stdout parsing found a port, then falls back to polling.
func (d *DevServerDetector) WaitForReady(ctx context.Context, startPort, endPort int) (int, error) {
	// Check if stdout already detected the port.
	if port, ok := d.DetectedPort(); ok {
		// Verify the port is actually responding.
		client := &http.Client{Timeout: 2 * time.Second}
		url := fmt.Sprintf("http://localhost:%d", port)
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return port, nil
			}
		}
	}

	// Fall back to polling.
	return PollForDevServer(ctx, startPort, endPort)
}
