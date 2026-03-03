// Purpose: Connection retry and daemon-version compatibility checks for bridge startup.
// Why: Keeps retry/version policy separate from high-level connection orchestration.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// connectWithRetries attempts to connect to an existing server's health endpoint
// with up to maxRetries. Returns nil on success, or the last error on failure.
func connectWithRetries(server *Server, healthURL string, mcpEndpoint string, dw *debugWriter) error {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			server.logLifecycle("connection_retry", 0, map[string]any{
				"attempt": attempt,
				"error":   fmt.Sprintf("%v", lastErr),
			})
			dw.write(fmt.Sprintf("connection_attempt_%d", attempt), lastErr, map[string]any{"health_url": healthURL})
			time.Sleep(1 * time.Second)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		resp, err := http.DefaultClient.Do(req) // #nosec G704 -- healthURL is localhost-only from trusted port
		cancel()
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
			_ = resp.Body.Close() // lint:body-close-ok immediate close after bounded read
			meta, ok := decodeHealthMetadata(body)
			if !ok {
				return &nonGasolineServiceError{serviceName: ""}
			}
			serviceName := meta.resolvedServiceName()
			if !isGasolineService(serviceName) {
				return &nonGasolineServiceError{serviceName: serviceName}
			}
			runningVersion := strings.TrimSpace(meta.Version)
			if runningVersion == "" {
				return &serverVersionMismatchError{
					expected: version,
					actual:   "<missing>",
				}
			}
			if !versionsMatch(runningVersion, version) {
				return &serverVersionMismatchError{
					expected: version,
					actual:   runningVersion,
				}
			}
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "[gasoline] Connection successful after %d retries\n", attempt)
			}
			bridgeStdioToHTTP(mcpEndpoint)
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close() // lint:body-close-ok immediate close in retry loop
		}
		if err != nil {
			lastErr = err
		} else if resp != nil {
			lastErr = fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
		} else {
			lastErr = errors.New("health request failed with empty response")
		}
	}
	return lastErr
}

func normalizeVersionString(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func versionsMatch(a string, b string) bool {
	return normalizeVersionString(a) == normalizeVersionString(b)
}
