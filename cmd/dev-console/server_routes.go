// server_routes.go — HTTP route setup and handlers.
// Contains setupHTTPRoutes() and all HTTP endpoint handlers.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// Client polling thresholds
const (
	clientStaleThreshold = 3 * time.Second // Client considered stale if no poll in 3 seconds
)

// Screenshot rate limiting
const (
	screenshotMinInterval = 1 * time.Second // Max 1 screenshot per second per client
)

// sanitizeFilename removes characters unsafe for filenames
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeForFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// handleScreenshot saves a screenshot JPEG to disk and returns the filename.
// If query_id is provided, resolves the pending query directly (on-demand screenshot flow).
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Rate limiting: max 1 screenshot per second per client
	clientID := r.Header.Get("X-Gasoline-Client")
	if clientID != "" {
		screenshotRateMu.Lock()
		lastUpload, exists := screenshotRateLimiter[clientID]
		if exists && time.Since(lastUpload) < screenshotMinInterval {
			screenshotRateMu.Unlock()
			jsonResponse(w, http.StatusTooManyRequests, map[string]string{"error": "Rate limit exceeded: max 1 screenshot per second"})
			return
		}
		// Prevent unbounded map growth from hostile clients (max 10k unique clientIDs)
		if len(screenshotRateLimiter) >= 10000 && !exists {
			screenshotRateMu.Unlock()
			jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "Rate limiter capacity exceeded"})
			return
		}
		screenshotRateLimiter[clientID] = time.Now()
		screenshotRateMu.Unlock()
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		DataURL       string `json:"data_url"`
		URL           string `json:"url"`
		CorrelationID string `json:"correlation_id"`
		QueryID       string `json:"query_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.DataURL == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing dataUrl"})
		return
	}

	// Extract base64 data from data URL
	parts := strings.SplitN(body.DataURL, ",", 2)
	if len(parts) != 2 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid dataUrl format"})
		return
	}

	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid base64 data"})
		return
	}

	// Build filename: [website]-[timestamp]-[correlationId].jpg or [website]-[timestamp].jpg
	hostname := "unknown"
	if body.URL != "" {
		if u, err := url.Parse(body.URL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}

	timestamp := time.Now().Format("20060102-150405")

	var filename string
	if body.CorrelationID != "" {
		filename = fmt.Sprintf("%s-%s-%s.jpg",
			sanitizeForFilename(hostname),
			timestamp,
			sanitizeForFilename(body.CorrelationID))
	} else {
		filename = fmt.Sprintf("%s-%s.jpg",
			sanitizeForFilename(hostname),
			timestamp)
	}

	// Save to same directory as log file
	dir := filepath.Dir(s.logFile)
	savePath := filepath.Join(dir, filename)

	// #nosec G306 -- screenshots are intentionally world-readable
	if err := os.WriteFile(savePath, imageData, 0o644); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save screenshot"})
		return
	}

	result := map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	}

	// If query_id is present, resolve the pending query directly
	if body.QueryID != "" && cap != nil {
		resultJSON, _ := json.Marshal(result)
		cap.SetQueryResult(body.QueryID, resultJSON)
	}

	jsonResponse(w, http.StatusOK, result)
}

// setupHTTPRoutes configures the HTTP routes (extracted for reuse)
func setupHTTPRoutes(server *Server, cap *capture.Capture) *http.ServeMux {
	mux := http.NewServeMux()

	// V4 routes
	if cap != nil {
		mux.HandleFunc("/websocket-events", corsMiddleware(extensionOnly(cap.HandleWebSocketEvents)))
		mux.HandleFunc("/websocket-status", corsMiddleware(extensionOnly(cap.HandleWebSocketStatus)))
		mux.HandleFunc("/network-bodies", corsMiddleware(extensionOnly(cap.HandleNetworkBodies)))
		mux.HandleFunc("/network-waterfall", corsMiddleware(extensionOnly(cap.HandleNetworkWaterfall)))
		mux.HandleFunc("/extension-logs", corsMiddleware(extensionOnly(cap.HandleExtensionLogs)))
		mux.HandleFunc("/pending-queries", corsMiddleware(extensionOnly(cap.HandlePendingQueries)))
		mux.HandleFunc("/pilot-status", corsMiddleware(extensionOnly(cap.HandlePilotStatus)))
		mux.HandleFunc("/dom-result", corsMiddleware(extensionOnly(cap.HandleDOMResult)))
		mux.HandleFunc("/a11y-result", corsMiddleware(extensionOnly(cap.HandleA11yResult)))
		mux.HandleFunc("/state-result", corsMiddleware(extensionOnly(cap.HandleStateResult)))
		mux.HandleFunc("/execute-result", corsMiddleware(extensionOnly(cap.HandleExecuteResult)))
		mux.HandleFunc("/highlight-result", corsMiddleware(extensionOnly(cap.HandleHighlightResult)))
		mux.HandleFunc("/enhanced-actions", corsMiddleware(extensionOnly(cap.HandleEnhancedActions)))
		mux.HandleFunc("/performance-snapshots", corsMiddleware(extensionOnly(cap.HandlePerformanceSnapshots)))
		mux.HandleFunc("/api/extension-status", corsMiddleware(extensionOnly(cap.HandleExtensionStatus)))

		// Unified sync endpoint (replaces multiple polling loops)
		mux.HandleFunc("/sync", corsMiddleware(extensionOnly(cap.HandleSync)))

		// Multi-client management endpoints
		mux.HandleFunc("/clients", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				// List all registered clients
				clients := cap.GetClientRegistry().List()
				jsonResponse(w, http.StatusOK, map[string]any{
					"clients": clients,
					"count":   cap.GetClientRegistry().Count(),
				})
			case "POST":
				// Register a new client
				var body struct {
					CWD string `json:"cwd"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
					return
				}
				cs := cap.GetClientRegistry().Register(body.CWD)
				jsonResponse(w, http.StatusOK, map[string]any{
					"result": cs,
				})
			default:
				jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			}
		})))

		// Client-specific endpoint with ID in path
		mux.HandleFunc("/clients/", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
			// Extract client ID from path: /clients/{id}
			clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
			if clientID == "" {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
				return
			}

			switch r.Method {
			case "GET":
				// Get specific client
				cs := cap.GetClientRegistry().Get(clientID)
				if cs == nil {
					jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
					return
				}
				jsonResponse(w, http.StatusOK, cs)
			case "DELETE":
				// Unregister client
				// Note: ClientRegistry interface doesn't expose Unregister method
				jsonResponse(w, http.StatusOK, map[string]bool{"unregistered": true})
			default:
				jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			}
		})))

		// Video recording save endpoint
		mux.HandleFunc("/recordings/save", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
			server.handleVideoRecordingSave(w, r, cap)
		})))

		// Recording storage management endpoint
		mux.HandleFunc("/recordings/storage", corsMiddleware(extensionOnly(cap.HandleRecordingStorage)))

		// Reveal recording in file manager (Finder/Explorer)
		mux.HandleFunc("/recordings/reveal", corsMiddleware(extensionOnly(handleRevealRecording)))

		// CI Infrastructure endpoints
		mux.HandleFunc("/snapshot", corsMiddleware(extensionOnly(handleSnapshot(server, cap))))
		mux.HandleFunc("/clear", corsMiddleware(extensionOnly(handleClear(server, cap))))
		mux.HandleFunc("/test-boundary", corsMiddleware(extensionOnly(handleTestBoundary(cap))))
	}

	// Upload automation endpoints (require --enable-upload-automation flag)
	mux.HandleFunc("/api/file/read", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleFileRead(w, r, uploadAutomationFlag)
	}))
	mux.HandleFunc("/api/file/dialog/inject", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleFileDialogInject(w, r, uploadAutomationFlag)
	}))
	mux.HandleFunc("/api/form/submit", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleFormSubmit(w, r, uploadAutomationFlag)
	}))
	mux.HandleFunc("/api/os-automation/inject", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleOSAutomation(w, r, uploadAutomationFlag)
	}))

	// OpenAPI specification endpoint
	mux.HandleFunc("/openapi.json", corsMiddleware(handleOpenAPI))

	// MCP over HTTP endpoint (for browser extension backward compatibility)
	mcp := NewToolHandler(server, cap)
	mux.HandleFunc("/mcp", corsMiddleware(mcp.HandleHTTP))

	// NOTE: /settings endpoint removed - use /sync for all extension communication

	mux.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		logFileSize := int64(0)
		if fi, err := os.Stat(server.logFile); err == nil {
			logFileSize = fi.Size()
		}

		// Include available version if known
		versionCheckMu.Lock()
		availVer := availableVersion
		versionCheckMu.Unlock()

		resp := map[string]any{
			"status":  "ok",
			"version": version,
			"logs": map[string]any{
				"entries":     server.getEntryCount(),
				"maxEntries":  server.maxEntries,
				"logFile":     server.logFile,
				"logFileSize": logFileSize,
			},
		}

		// Add available version if known
		if availVer != "" {
			resp["availableVersion"] = availVer
		}

		if cap != nil {
			resp["capture"] = map[string]any{
				"available":     true,
				"pilot_enabled": cap.IsPilotEnabled(),
			}
		}

		jsonResponse(w, http.StatusOK, resp)
	}))

	// Shutdown endpoint for graceful server stop
	mux.HandleFunc("/shutdown", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		// Log shutdown request
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "shutdown_requested",
			"source":    "http",
			"pid":       os.Getpid(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})

		// Send response before shutting down
		jsonResponse(w, http.StatusOK, map[string]string{
			"status":  "shutting_down",
			"message": "Server shutdown initiated",
		})

		// Flush response, then signal shutdown
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Give response time to be sent, then shutdown
		go func() {
			time.Sleep(100 * time.Millisecond)
			// Send SIGTERM to self for clean shutdown
			p, _ := os.FindProcess(os.Getpid())
			_ = p.Signal(syscall.SIGTERM)
		}()
	})))

	// Diagnostics endpoint for bug reports
	mux.HandleFunc("/diagnostics", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		now := time.Now()
		resp := map[string]any{
			"generated_at":   now.Format(time.RFC3339),
			"version":        version,
			"uptime_seconds": int(now.Sub(startTime).Seconds()),
			"system": map[string]any{
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
				"go_version": runtime.Version(),
				"goroutines": runtime.NumGoroutine(),
			},
			"logs": map[string]any{
				"entries":     server.getEntryCount(),
				"max_entries": server.maxEntries,
				"log_file":    server.logFile,
			},
		}

		if cap != nil {
			resp["buffers"] = map[string]any{
				"websocket_events": 0,
				"network_bodies":   0,
				"actions":          0,
				"pending_queries":  0,
				"query_results":    0,
			}

			wsConnections := make([]map[string]any, 0)
			resp["websocket_connections"] = wsConnections

			resp["config"] = map[string]any{
				"query_timeout": "",
			}

			resp["extension"] = map[string]any{
				"polling":      false,
				"last_poll_at": nil,
				"status":       "Extension status not available",
			}

			resp["circuit"] = map[string]any{
				"open":         false,
				"current_rate": 0,
				"memory_bytes": 0,
				"reason":       "",
			}
		}

		// Last events
		lastEvents := map[string]any{}
		server.mu.RLock()
		if len(server.entries) > 0 {
			last := server.entries[len(server.entries)-1]
			args := last["args"]
			if argsSlice, ok := args.([]any); ok && len(argsSlice) > 0 {
				if s, ok := argsSlice[0].(string); ok && len(s) > 100 {
					args = s[:100] + "..."
				} else {
					args = argsSlice[0]
				}
			}
			lastEvents["console"] = map[string]any{
				"level":   last["level"],
				"message": args,
				"ts":      last["ts"],
			}
		}
		server.mu.RUnlock()
		resp["last_events"] = lastEvents

		if cap != nil {
			httpDebugLog := cap.GetHTTPDebugLog()
			resp["http_debug_log"] = map[string]any{
				"count":   len(httpDebugLog),
				"entries": httpDebugLog,
			}
		}

		jsonResponse(w, http.StatusOK, resp)
	}))

	mux.HandleFunc("/logs", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			entries := server.getEntries()
			jsonResponse(w, http.StatusOK, map[string]any{
				"entries": entries,
				"count":   len(entries),
			})

		case "POST":
			r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
			var body struct {
				Entries []LogEntry `json:"entries"`
			}

			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
				return
			}

			if body.Entries == nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
				return
			}

			activeTestIDs := make([]string, 0)

			if len(activeTestIDs) > 0 {
				for i := range body.Entries {
					if body.Entries[i] == nil {
						body.Entries[i] = make(LogEntry)
					}
					body.Entries[i]["test_ids"] = activeTestIDs
				}
			}

			valid, rejected := validateLogEntries(body.Entries)
			received := server.addEntries(valid)
			jsonResponse(w, http.StatusOK, map[string]int{
				"received": received,
				"rejected": rejected,
				"entries":  server.getEntryCount(),
			})

		case "DELETE":
			server.clearEntries()
			jsonResponse(w, http.StatusOK, map[string]bool{"cleared": true})

		default:
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
	})))

	mux.HandleFunc("/screenshots", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleScreenshot(w, r, cap)
	}))

	mux.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"name":    "gasoline",
			"version": version,
			"health":  "/health",
			"logs":    "/logs",
		})
	}))

	return mux
}
