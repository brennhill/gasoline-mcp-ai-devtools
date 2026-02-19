// Purpose: Owns server_routes.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

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
	"github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
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

// screenshotsDir returns the runtime screenshots directory, creating it if needed.
func screenshotsDir() (string, error) {
	dir, err := state.ScreenshotsDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine screenshots directory: %w", err)
	}
	// #nosec G301 -- directory: owner rwx, group rx for traversal
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create screenshots directory: %w", err)
	}
	return dir, nil
}

// checkScreenshotRateLimit enforces per-client screenshot rate limiting.
// Returns an HTTP status code (0 means allowed) and an error message.
func checkScreenshotRateLimit(clientID string) (int, string) {
	if clientID == "" {
		return 0, ""
	}
	screenshotRateMu.Lock()
	defer screenshotRateMu.Unlock()

	lastUpload, exists := screenshotRateLimiter[clientID]
	if exists && time.Since(lastUpload) < screenshotMinInterval {
		return http.StatusTooManyRequests, "Rate limit exceeded: max 1 screenshot per second"
	}
	if len(screenshotRateLimiter) >= 10000 && !exists {
		// Inline eviction: purge stale entries before rejecting
		for id, ts := range screenshotRateLimiter {
			if time.Since(ts) > screenshotMinInterval {
				delete(screenshotRateLimiter, id)
			}
		}
		if len(screenshotRateLimiter) >= 10000 {
			return http.StatusServiceUnavailable, "Rate limiter capacity exceeded"
		}
	}
	screenshotRateLimiter[clientID] = time.Now()
	return 0, ""
}

// decodeDataURL extracts binary data from a data URL (e.g. "data:image/jpeg;base64,...").
// Returns the decoded bytes, or an error string suitable for HTTP responses.
func decodeDataURL(dataURL string) ([]byte, string) {
	if dataURL == "" {
		return nil, "Missing dataUrl"
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, "Invalid dataUrl format"
	}
	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "Invalid base64 data"
	}
	return imageData, ""
}

// buildScreenshotFilename constructs a sanitized filename from URL hostname,
// timestamp, and optional correlation ID.
func buildScreenshotFilename(pageURL, correlationID string) string {
	hostname := "unknown"
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}
	timestamp := time.Now().Format("20060102-150405")
	if correlationID != "" {
		return fmt.Sprintf("%s-%s-%s.jpg",
			sanitizeForFilename(hostname),
			timestamp,
			sanitizeForFilename(correlationID))
	}
	return fmt.Sprintf("%s-%s.jpg", sanitizeForFilename(hostname), timestamp)
}

// saveImageToScreenshotsDir writes image data to the screenshots directory.
// Returns the full path on success, or an HTTP status and error message on failure.
func saveImageToScreenshotsDir(filename string, imageData []byte) (string, int, string) {
	dir, dirErr := screenshotsDir()
	if dirErr != nil {
		return "", http.StatusInternalServerError, "Failed to resolve screenshots directory"
	}
	savePath := filepath.Join(dir, filename)
	if !isWithinDir(savePath, dir) {
		return "", http.StatusBadRequest, "Invalid screenshot path"
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	if err := os.WriteFile(savePath, imageData, 0o600); err != nil {
		return "", http.StatusInternalServerError, "Failed to save screenshot"
	}
	return savePath, 0, ""
}

// handleScreenshot saves a screenshot JPEG to disk and returns the filename.
// If query_id is provided, resolves the pending query directly (on-demand screenshot flow).
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if status, msg := checkScreenshotRateLimit(r.Header.Get("X-Gasoline-Client")); status != 0 {
		jsonResponse(w, status, map[string]string{"error": msg})
		return
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

	imageData, errMsg := decodeDataURL(body.DataURL)
	if errMsg != "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}

	filename := buildScreenshotFilename(body.URL, body.CorrelationID)
	savePath, status, saveErr := saveImageToScreenshotsDir(filename, imageData)
	if status != 0 {
		jsonResponse(w, status, map[string]string{"error": saveErr})
		return
	}

	result := map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	}
	if body.QueryID != "" && cap != nil {
		// Error impossible: map contains only primitive types from input
		resultJSON, _ := json.Marshal(result)
		cap.SetQueryResult(body.QueryID, resultJSON)
	}
	jsonResponse(w, http.StatusOK, result)
}

// saveDrawScreenshot decodes a data URL and writes the screenshot to disk.
// Returns the saved path (empty string on any failure, with a non-nil error
// only for directory resolution failures that should abort the request).
func saveDrawScreenshot(dataURL string, tabID int) (string, error) {
	imageData, errMsg := decodeDataURL(dataURL)
	if errMsg != "" {
		return "", nil
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("draw_%s_tab%d_%d.png", sanitizeForFilename(timestamp), tabID, randomInt63()%10000)

	dir, dirErr := screenshotsDir()
	if dirErr != nil {
		return "", dirErr
	}
	path := filepath.Join(dir, filename)
	if !isWithinDir(path, dir) {
		return "", nil
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	if err := os.WriteFile(path, imageData, 0o600); err != nil {
		return "", nil
	}
	return path, nil
}

// parseAnnotations unmarshals raw annotation JSON, collecting warnings for
// entries that fail to parse.
func parseAnnotations(rawAnnotations []json.RawMessage) ([]Annotation, []string) {
	parsed := make([]Annotation, 0, len(rawAnnotations))
	var warnings []string
	for i, raw := range rawAnnotations {
		var ann Annotation
		if err := json.Unmarshal(raw, &ann); err != nil {
			warnings = append(warnings, fmt.Sprintf("annotation[%d]: %v", i, err))
		} else {
			parsed = append(parsed, ann)
		}
	}
	return parsed, warnings
}

// storeElementDetails persists annotation element details into the global store.
func storeElementDetails(details map[string]json.RawMessage) {
	for correlationID, rawDetail := range details {
		var detail AnnotationDetail
		if err := json.Unmarshal(rawDetail, &detail); err == nil {
			if detail.Selector == "" && detail.Tag == "" {
				rawStr := string(rawDetail)
				if len(rawStr) > 200 {
					rawStr = rawStr[:200] + "..."
				}
				fmt.Fprintf(os.Stderr, "[gasoline] draw detail %s: empty (raw=%s)\n", correlationID, rawStr)
			}
			detail.CorrelationID = correlationID
			globalAnnotationStore.StoreDetail(correlationID, detail)
		} else {
			rawStr := string(rawDetail)
			if len(rawStr) > 200 {
				rawStr = rawStr[:200] + "..."
			}
			fmt.Fprintf(os.Stderr, "[gasoline] draw detail %s: unmarshal error: %v (raw=%s)\n", correlationID, err, rawStr)
		}
	}
}

// drawModeRequest holds the parsed and validated fields from a draw mode POST body.
type drawModeRequest struct {
	ScreenshotDataURL string                     `json:"screenshot_data_url"`
	Annotations       []json.RawMessage          `json:"annotations"`
	ElementDetails    map[string]json.RawMessage `json:"element_details"`
	PageURL           string                     `json:"page_url"`
	TabID             int                        `json:"tab_id"`
	AnnotSessionName  string                     `json:"annot_session_name"`
	CorrelationID     string                     `json:"correlation_id"`
}

// persistDrawSession writes the full draw session (annotations + element details) to disk
// as a JSON file alongside the screenshot. Files are retained until manually cleared.
func persistDrawSession(body *drawModeRequest, screenshotPath string, annotations []Annotation) {
	dir, err := screenshotsDir()
	if err != nil {
		return
	}
	ts := time.Now().UnixMilli()
	filename := fmt.Sprintf("draw-session-%d-%d.json", body.TabID, ts)
	path := filepath.Join(dir, filename)

	session := map[string]any{
		"annotations":     annotations,
		"element_details": body.ElementDetails,
		"page_url":        body.PageURL,
		"tab_id":          body.TabID,
		"screenshot":      screenshotPath,
		"timestamp":       ts,
		"correlation_id":  body.CorrelationID,
	}
	if body.AnnotSessionName != "" {
		session["annot_session_name"] = body.AnnotSessionName
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	_ = os.WriteFile(path, data, 0o600)
}

// storeAnnotationSession creates and persists an annotation session, returning
// the stored session for response building.
func storeAnnotationSession(body *drawModeRequest, screenshotPath string, annotations []Annotation) {
	session := &AnnotationSession{
		Annotations:    annotations,
		ScreenshotPath: screenshotPath,
		PageURL:        body.PageURL,
		TabID:          body.TabID,
		Timestamp:      time.Now().UnixMilli(),
	}
	globalAnnotationStore.StoreSession(body.TabID, session)
	if body.AnnotSessionName != "" {
		globalAnnotationStore.AppendToNamedSession(body.AnnotSessionName, session)
	}
}

// handleDrawModeComplete receives annotation data and screenshot from the extension
// when the user finishes a draw mode session.
func (s *Server) handleDrawModeComplete(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body drawModeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.TabID <= 0 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "tab_id is required and must be > 0"})
		return
	}

	var screenshotPath string
	if body.ScreenshotDataURL != "" {
		path, err := saveDrawScreenshot(body.ScreenshotDataURL, body.TabID)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to resolve screenshots directory"})
			return
		}
		screenshotPath = path
	}

	parsedAnnotations, parseWarnings := parseAnnotations(body.Annotations)
	storeAnnotationSession(&body, screenshotPath, parsedAnnotations)
	storeElementDetails(body.ElementDetails)

	// Persist full session to disk so the LLM can compare/contrast across restarts
	persistDrawSession(&body, screenshotPath, parsedAnnotations)

	result := map[string]any{
		"status":           "stored",
		"annotation_count": len(parsedAnnotations),
		"screenshot":       screenshotPath,
	}
	if len(parseWarnings) > 0 {
		result["warnings"] = parseWarnings
	}

	// Complete the pending command — unblocks WaitForCommand in tools_async.go
	// so the LLM can retrieve results via correlation_id.
	if body.CorrelationID != "" && cap != nil {
		resultJSON, _ := json.Marshal(result)
		cap.CompleteCommand(body.CorrelationID, resultJSON, "")
	}

	jsonResponse(w, http.StatusOK, result)
}

// handleHealth serves the /health endpoint with server status and version info.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	logFileSize := int64(0)
	if fi, err := os.Stat(s.logFile); err == nil {
		logFileSize = fi.Size()
	}

	versionCheckMu.Lock()
	availVer := availableVersion
	versionCheckMu.Unlock()

	resp := map[string]any{
		"status":       "ok",
		"service-name": "gasoline",
		"name":         "gasoline", // legacy compatibility
		"version":      version,
		"logs": map[string]any{
			"entries":       s.getEntryCount(),
			"max_entries":   s.maxEntries,
			"log_file":      s.logFile,
			"log_file_size": logFileSize,
			"dropped_count": s.getLogDropCount(),
		},
	}
	successReads, failedReads := snapshotFastPathResourceReadCounters()
	resp["bridge_fastpath"] = map[string]any{
		"resources_read_success": successReads,
		"resources_read_failure": failedReads,
	}
	if availVer != "" {
		resp["available_version"] = availVer
	}
	if cap != nil {
		extStatus := cap.GetExtensionStatus()
		resp["capture"] = map[string]any{
			"available":           true,
			"pilot_enabled":       cap.IsPilotEnabled(),
			"extension_connected": cap.IsExtensionConnected(),
			"extension_last_seen": extStatus["last_seen"],
			"extension_client_id": extStatus["client_id"],
		}
	}
	jsonResponse(w, http.StatusOK, resp)
}

// handleShutdown initiates a graceful server shutdown via SIGTERM.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	_ = s.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "shutdown_requested",
		"source":    "http",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "shutting_down",
		"message": "Server shutdown initiated",
	})

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	util.SafeGo(func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})
}

// lastConsoleEvent returns a summary of the most recent console log entry,
// truncating long argument strings to 100 characters.
func (s *Server) lastConsoleEvent() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil
	}
	last := s.entries[len(s.entries)-1]
	args := last["args"]
	if argsSlice, ok := args.([]any); ok && len(argsSlice) > 0 {
		if str, ok := argsSlice[0].(string); ok && len(str) > 100 {
			args = str[:100] + "..."
		} else {
			args = argsSlice[0]
		}
	}
	return map[string]any{
		"level":   last["level"],
		"message": args,
		"ts":      last["ts"],
	}
}

// handleDiagnostics serves the /diagnostics endpoint with debug information for bug reports.
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
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
			"entries":     s.getEntryCount(),
			"max_entries": s.maxEntries,
			"log_file":    s.logFile,
		},
	}

	if cap != nil {
		appendCaptureDiagnostics(resp, cap)
	}

	lastEvents := map[string]any{}
	if evt := s.lastConsoleEvent(); evt != nil {
		lastEvents["console"] = evt
	}
	resp["last_events"] = lastEvents

	if cap != nil {
		httpDebugLog := cap.GetHTTPDebugLog()
		resp["http_debug_log"] = map[string]any{
			"count":   len(httpDebugLog),
			"entries": httpDebugLog,
		}
	}

	jsonResponse(w, http.StatusOK, resp)
}

// appendCaptureDiagnostics adds capture-related diagnostic fields to the response map.
func appendCaptureDiagnostics(resp map[string]any, cap *capture.Capture) {
	snap := cap.GetHealthSnapshot()
	health := cap.GetHealthStatus()

	resp["buffers"] = map[string]any{
		"websocket_events": snap.WebSocketCount,
		"network_bodies":   snap.NetworkBodyCount,
		"actions":          snap.ActionCount,
		"pending_queries":  snap.PendingQueryCount,
		"query_results":    snap.QueryResultCount,
	}

	wsStatus := cap.GetWebSocketStatus(capture.WebSocketStatusFilter{})
	conns := make([]map[string]any, 0, len(wsStatus.Connections))
	for _, c := range wsStatus.Connections {
		conns = append(conns, map[string]any{
			"id":  c.ID,
			"url": c.URL,
		})
	}
	resp["websocket_connections"] = conns

	resp["config"] = map[string]any{
		"query_timeout": snap.QueryTimeout.String(),
	}

	lastPoll := any(nil)
	if !snap.LastPollTime.IsZero() {
		lastPoll = snap.LastPollTime.Format(time.RFC3339)
	}
	resp["extension"] = map[string]any{
		"polling":       !snap.LastPollTime.IsZero(),
		"last_poll_at":  lastPoll,
		"ext_session":   snap.ExtSessionID,
		"pilot_enabled": snap.PilotEnabled,
	}

	resp["circuit"] = map[string]any{
		"open":         snap.CircuitOpen,
		"current_rate": health.CurrentRate,
		"reason":       snap.CircuitReason,
	}
}

// handleLogs serves the /logs endpoint for ingesting and clearing log entries.
// Reads go through GET /telemetry?type=logs.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleLogsPost(w, r)

	case "DELETE":
		s.clearEntries()
		jsonResponse(w, http.StatusOK, map[string]bool{"cleared": true})

	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleLogsPost processes POST /logs requests to ingest new log entries.
func (s *Server) handleLogsPost(w http.ResponseWriter, r *http.Request) {
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

	valid, rejected := validateLogEntries(body.Entries)
	received := s.addEntries(valid)
	jsonResponse(w, http.StatusOK, map[string]int{
		"received": received,
		"rejected": rejected,
		"entries":  s.getEntryCount(),
	})
}

// setupHTTPRoutes configures the HTTP routes (extracted for reuse)
func setupHTTPRoutes(server *Server, cap *capture.Capture) *http.ServeMux {
	mux := http.NewServeMux()

	if cap != nil {
		registerCaptureRoutes(mux, server, cap)
	}

	registerUploadRoutes(mux, server)
	registerCoreRoutes(mux, server, cap)

	return mux
}

// registerCaptureRoutes adds capture-dependent routes to the mux.
// NOT MCP — These are extension-to-daemon HTTP endpoints for telemetry ingestion
// and internal state management. AI agents use the 5 MCP tools instead.
func registerCaptureRoutes(mux *http.ServeMux, server *Server, cap *capture.Capture) {
	// NOT MCP — Extension telemetry ingestion (extension → daemon data pipeline)
	mux.HandleFunc("/websocket-events", corsMiddleware(extensionOnly(cap.HandleWebSocketEvents)))
	mux.HandleFunc("/websocket-status", corsMiddleware(extensionOnly(cap.HandleWebSocketStatus)))
	mux.HandleFunc("/network-bodies", corsMiddleware(extensionOnly(cap.HandleNetworkBodies)))
	mux.HandleFunc("/network-waterfall", corsMiddleware(extensionOnly(cap.HandleNetworkWaterfall)))
	mux.HandleFunc("/query-result", corsMiddleware(extensionOnly(cap.HandleQueryResult)))
	mux.HandleFunc("/enhanced-actions", corsMiddleware(extensionOnly(cap.HandleEnhancedActions)))
	mux.HandleFunc("/performance-snapshots", corsMiddleware(extensionOnly(cap.HandlePerformanceSnapshots)))

	// NOT MCP — Unified sync endpoint (extension polls this instead of individual routes above)
	mux.HandleFunc("/sync", corsMiddleware(extensionOnly(cap.HandleSync)))

	// NOT MCP — Multi-client registry (extension bookkeeping, not AI-facing)
	mux.HandleFunc("/clients", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientsList(w, r, cap)
	})))
	mux.HandleFunc("/clients/", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientByID(w, r, cap)
	})))

	// NOT MCP — Video recording binary upload (extension → daemon file storage)
	mux.HandleFunc("/recordings/save", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleVideoRecordingSave(w, r, cap)
	})))

	// NOT MCP — Recording storage management (extension UI)
	mux.HandleFunc("/recordings/storage", corsMiddleware(extensionOnly(cap.HandleRecordingStorage)))

	// NOT MCP — OS file manager integration (opens Finder/Explorer)
	mux.HandleFunc("/recordings/reveal", corsMiddleware(extensionOnly(handleRevealRecording)))

	// NOT MCP — Unified telemetry read (extension and legacy HTTP clients)
	mux.HandleFunc("/telemetry", corsMiddleware(handleTelemetry(server, cap)))

	// NOT MCP — CI infrastructure (test harness boundaries, not AI-facing)
	mux.HandleFunc("/snapshot", corsMiddleware(extensionOnly(handleSnapshot(server, cap))))
	mux.HandleFunc("/clear", corsMiddleware(extensionOnly(handleClear(server, cap))))
	mux.HandleFunc("/test-boundary", corsMiddleware(extensionOnly(handleTestBoundary(cap))))
}

// handleClientsList handles GET/POST on /clients for multi-client management.
func handleClientsList(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	switch r.Method {
	case "GET":
		clients := cap.GetClientRegistry().List()
		jsonResponse(w, http.StatusOK, map[string]any{
			"clients": clients,
			"count":   cap.GetClientRegistry().Count(),
		})
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
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
}

// handleClientByID handles GET/DELETE on /clients/{id} for specific client operations.
func handleClientByID(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
	if clientID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
		return
	}

	switch r.Method {
	case "GET":
		cs := cap.GetClientRegistry().Get(clientID)
		if cs == nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		jsonResponse(w, http.StatusOK, cs)
	case "DELETE":
		// Note: ClientRegistry interface doesn't expose Unregister method
		jsonResponse(w, http.StatusOK, map[string]bool{"unregistered": true})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// registerUploadRoutes adds upload automation endpoints to the mux.
// NOT MCP — These are extension-to-daemon escalation stages for file upload automation.
// AI agents use interact(action: "upload") via MCP instead.
// Stages 1-3 are always available; Stage 4 requires --enable-os-upload-automation.
func registerUploadRoutes(mux *http.ServeMux, server *Server) {
	// NOT MCP — File read metadata (upload escalation stage 1, always available)
	mux.HandleFunc("/api/file/read", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleFileRead(w, r)
	})))
	// NOT MCP — File dialog injection (upload escalation stage 2, always available)
	mux.HandleFunc("/api/file/dialog/inject", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleFileDialogInject(w, r)
	})))
	// NOT MCP — Form submit helper (upload escalation stage 3, always available)
	mux.HandleFunc("/api/form/submit", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleFormSubmit(w, r)
	})))
	// NOT MCP — OS-level file dialog automation (upload escalation stage 4, requires --enable-os-upload-automation)
	mux.HandleFunc("/api/os-automation/inject", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleOSAutomation(w, r, osUploadAutomationFlag)
	})))
	// NOT MCP — Dismiss dangling file dialog via Escape key (cleanup after failed Stage 4)
	mux.HandleFunc("/api/os-automation/dismiss", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleOSAutomationDismiss(w, r, osUploadAutomationFlag)
	})))
}

// registerCoreRoutes adds non-capture-dependent routes to the mux.
func registerCoreRoutes(mux *http.ServeMux, server *Server, cap *capture.Capture) {
	// NOT MCP — OpenAPI spec for HTTP API documentation
	mux.HandleFunc("/openapi.json", corsMiddleware(handleOpenAPI))

	// MCP — The single MCP JSON-RPC endpoint. All AI agent tool calls go through here.
	mcp := NewToolHandler(server, cap)
	mux.HandleFunc("/mcp", corsMiddleware(mcp.HandleHTTP))

	// NOT MCP — Dashboard status API (JSON feed for the HTML dashboard)
	mux.HandleFunc("/api/status", corsMiddleware(handleStatusAPI(server, cap, mcp)))

	// NOT MCP — Health check for extension and monitoring (MCP uses configure(action: "health"))
	mux.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleHealth(w, r, cap)
	}))

	// NOT MCP — Graceful shutdown (use CLI --stop flag, not MCP)
	mux.HandleFunc("/shutdown", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleShutdown(w, r)
	})))

	// NOT MCP — Debug diagnostics: HTML for browsers, JSON for programmatic access
	mux.HandleFunc("/diagnostics", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json") {
			serveEmbeddedHTML(w, r, diagnosticsHTML, "diagnostics")
			return
		}
		server.handleDiagnostics(w, r, cap)
	}))
	mux.HandleFunc("/diagnostics.json", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleDiagnostics(w, r, cap)
	}))

	// NOT MCP — Log ingestion from extension (MCP reads logs via observe(what: "logs"))
	mux.HandleFunc("/logs", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleLogs(w, r)
	})))

	// NOT MCP — HTML pages for human navigation
	mux.HandleFunc("/logs.html", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, logsHTML, "logs")
	}))
	mux.HandleFunc("/setup", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, setupHTML, "setup")
	}))
	mux.HandleFunc("/docs", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		serveEmbeddedHTML(w, r, docsHTML, "docs")
	}))

	// NOT MCP — Screenshot binary upload from extension (MCP reads via observe(what: "screenshot"))
	mux.HandleFunc("/screenshots", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleScreenshot(w, r, cap)
	})))

	// NOT MCP — Draw mode completion callback from extension (MCP uses analyze(what: "annotations"))
	mux.HandleFunc("/draw-mode/complete", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		server.handleDrawModeComplete(w, r, cap)
	})))

	// NOT MCP — HTML dashboard (browser) with JSON fallback (Accept: application/json)
	mux.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		server.handleDashboard(w, r)
	}))
}
