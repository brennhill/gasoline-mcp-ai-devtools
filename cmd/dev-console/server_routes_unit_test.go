package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%q", err, string(body))
	}
	return out
}

func localRequest(method, path string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, "http://localhost"+path, body)
}

func TestSetupHTTPRoutesBasicEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux := setupHTTPRoutes(srv, cap)

	rootReq := localRequest(http.MethodGet, "/", nil)
	rootRR := httptest.NewRecorder()
	mux.ServeHTTP(rootRR, rootReq)
	if rootRR.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rootRR.Code, http.StatusOK)
	}
	rootBody := decodeJSONMap(t, rootRR.Body.Bytes())
	if rootBody["name"] != "gasoline" {
		t.Fatalf("root name = %v, want gasoline", rootBody["name"])
	}

	notFoundReq := localRequest(http.MethodGet, "/missing", nil)
	notFoundRR := httptest.NewRecorder()
	mux.ServeHTTP(notFoundRR, notFoundReq)
	if notFoundRR.Code != http.StatusNotFound {
		t.Fatalf("GET /missing status = %d, want %d", notFoundRR.Code, http.StatusNotFound)
	}

	healthReq := localRequest(http.MethodGet, "/health", nil)
	healthRR := httptest.NewRecorder()
	mux.ServeHTTP(healthRR, healthReq)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", healthRR.Code, http.StatusOK)
	}
	healthBody := decodeJSONMap(t, healthRR.Body.Bytes())
	if healthBody["status"] != "ok" {
		t.Fatalf("health status = %v, want ok", healthBody["status"])
	}
	if healthBody["service-name"] != "gasoline" {
		t.Fatalf("health service-name = %v, want gasoline", healthBody["service-name"])
	}

	healthBadReq := localRequest(http.MethodPost, "/health", nil)
	healthBadRR := httptest.NewRecorder()
	mux.ServeHTTP(healthBadRR, healthBadReq)
	if healthBadRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status = %d, want %d", healthBadRR.Code, http.StatusMethodNotAllowed)
	}

	const rawSecret = "Bearer tokenValue1234567890abcdef"
	cap.LogHTTPDebugEntry(capture.HTTPDebugEntry{
		Timestamp:    time.Now(),
		Endpoint:     "/mcp",
		Method:       http.MethodPost,
		RequestBody:  `{"auth":"` + rawSecret + `"}`,
		ResponseBody: `{"ok":true}`,
		DurationMs:   5,
	})

	diagReq := localRequest(http.MethodGet, "/diagnostics", nil)
	diagRR := httptest.NewRecorder()
	mux.ServeHTTP(diagRR, diagReq)
	if diagRR.Code != http.StatusOK {
		t.Fatalf("GET /diagnostics status = %d, want %d", diagRR.Code, http.StatusOK)
	}
	diagBody := decodeJSONMap(t, diagRR.Body.Bytes())
	if _, ok := diagBody["generated_at"]; !ok {
		t.Fatalf("diagnostics missing generated_at: %v", diagBody)
	}
	httpDebug, ok := diagBody["http_debug_log"].(map[string]any)
	if !ok {
		t.Fatalf("diagnostics missing http_debug_log payload: %v", diagBody)
	}
	entries, ok := httpDebug["entries"].([]any)
	if !ok {
		t.Fatalf("diagnostics http_debug_log.entries missing: %v", httpDebug)
	}
	redactedFound := false
	for _, entryAny := range entries {
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		if entry["endpoint"] != "/mcp" || entry["method"] != http.MethodPost {
			continue
		}
		bodyText, _ := entry["request_body"].(string)
		if strings.Contains(bodyText, rawSecret) {
			t.Fatalf("diagnostics leaked secret in request_body: %q", bodyText)
		}
		if strings.Contains(bodyText, "[REDACTED:bearer-token]") {
			redactedFound = true
		}
	}
	if !redactedFound {
		t.Fatal("diagnostics did not include redacted http debug request body")
	}

	diagBadReq := localRequest(http.MethodPost, "/diagnostics", nil)
	diagBadRR := httptest.NewRecorder()
	mux.ServeHTTP(diagBadRR, diagBadReq)
	if diagBadRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /diagnostics status = %d, want %d", diagBadRR.Code, http.StatusMethodNotAllowed)
	}

	shutdownBadReq := localRequest(http.MethodGet, "/shutdown", nil)
	shutdownBadReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	shutdownBadRR := httptest.NewRecorder()
	mux.ServeHTTP(shutdownBadRR, shutdownBadReq)
	if shutdownBadRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /shutdown status = %d, want %d", shutdownBadRR.Code, http.StatusMethodNotAllowed)
	}
}

func TestHealthEndpointExposesDroppedCount(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux := setupHTTPRoutes(srv, cap)

	// Create a server with a channel of size 1 and NO async worker,
	// so the channel stays full when we manually fill it.
	tinyLogSrv := &Server{
		logFile:    filepath.Join(t.TempDir(), "drop.jsonl"),
		maxEntries: 100,
		entries:    make([]LogEntry, 0),
		logChan:    make(chan []LogEntry, 1),
		logDone:    make(chan struct{}),
	}

	tinyMux := setupHTTPRoutes(tinyLogSrv, cap)

	// Fill channel (no worker draining it), then trigger a drop
	tinyLogSrv.logChan <- []LogEntry{{"level": "info", "message": "fill"}}
	_ = tinyLogSrv.appendToFile([]LogEntry{{"level": "info", "message": "drop"}})

	healthReq := localRequest(http.MethodGet, "/health", nil)
	healthRR := httptest.NewRecorder()
	tinyMux.ServeHTTP(healthRR, healthReq)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", healthRR.Code, http.StatusOK)
	}

	healthBody := decodeJSONMap(t, healthRR.Body.Bytes())
	logs, ok := healthBody["logs"].(map[string]any)
	if !ok {
		t.Fatalf("health response missing logs object: %v", healthBody)
	}

	droppedCount, ok := logs["dropped_count"]
	if !ok {
		t.Fatal("health logs missing dropped_count field")
	}
	if droppedCount.(float64) != 1 {
		t.Fatalf("dropped_count = %v, want 1", droppedCount)
	}

	// Drain the channel and shut down cleanly
	<-tinyLogSrv.logChan
	close(tinyLogSrv.logDone)

	// Verify zero-state too: fresh server should have 0 dropped_count
	freshReq := localRequest(http.MethodGet, "/health", nil)
	freshRR := httptest.NewRecorder()
	mux.ServeHTTP(freshRR, freshReq)
	freshBody := decodeJSONMap(t, freshRR.Body.Bytes())
	freshLogs := freshBody["logs"].(map[string]any)
	if freshLogs["dropped_count"].(float64) != 0 {
		t.Fatalf("fresh server dropped_count = %v, want 0", freshLogs["dropped_count"])
	}
}

func TestLogsEndpointValidationAndMethods(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	mux := setupHTTPRoutes(srv, nil)

	// GET /logs returns 405 (reads go through /telemetry?type=logs)
	getReq := localRequest(http.MethodGet, "/logs", nil)
	getReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /logs status = %d, want %d", getRR.Code, http.StatusMethodNotAllowed)
	}

	badJSONReq := localRequest(http.MethodPost, "/logs", bytes.NewBufferString("{"))
	badJSONReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	badJSONRR := httptest.NewRecorder()
	mux.ServeHTTP(badJSONRR, badJSONReq)
	if badJSONRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /logs invalid json status = %d, want %d", badJSONRR.Code, http.StatusBadRequest)
	}

	missingEntriesReq := localRequest(http.MethodPost, "/logs", bytes.NewBufferString(`{"foo":"bar"}`))
	missingEntriesReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	missingEntriesRR := httptest.NewRecorder()
	mux.ServeHTTP(missingEntriesRR, missingEntriesReq)
	if missingEntriesRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /logs missing entries status = %d, want %d", missingEntriesRR.Code, http.StatusBadRequest)
	}

	validReq := localRequest(http.MethodPost, "/logs", bytes.NewBufferString(`{"entries":[{"level":"error","message":"boom"},{"level":"invalid","message":"skip"}]}`))
	validReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	validRR := httptest.NewRecorder()
	mux.ServeHTTP(validRR, validReq)
	if validRR.Code != http.StatusOK {
		t.Fatalf("POST /logs valid payload status = %d, want %d", validRR.Code, http.StatusOK)
	}
	validBody := decodeJSONMap(t, validRR.Body.Bytes())
	if validBody["received"].(float64) != 1 || validBody["rejected"].(float64) != 1 {
		t.Fatalf("POST /logs counts unexpected: %v", validBody)
	}

	deleteReq := localRequest(http.MethodDelete, "/logs", nil)
	deleteReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	deleteRR := httptest.NewRecorder()
	mux.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("DELETE /logs status = %d, want %d", deleteRR.Code, http.StatusOK)
	}

	putReq := localRequest(http.MethodPut, "/logs", nil)
	putReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	putRR := httptest.NewRecorder()
	mux.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT /logs status = %d, want %d", putRR.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleScreenshotRoutes(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux := setupHTTPRoutes(srv, cap)

	// Reset global rate limiter map for deterministic tests.
	screenshotRateMu.Lock()
	screenshotRateLimiter = make(map[string]time.Time)
	screenshotRateMu.Unlock()
	t.Cleanup(func() {
		screenshotRateMu.Lock()
		screenshotRateLimiter = make(map[string]time.Time)
		screenshotRateMu.Unlock()
	})

	methodReq := localRequest(http.MethodGet, "/screenshots", nil)
	methodReq.Header.Set("X-Gasoline-Client", "gasoline-extension")
	methodRR := httptest.NewRecorder()
	mux.ServeHTTP(methodRR, methodReq)
	if methodRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /screenshots status = %d, want %d", methodRR.Code, http.StatusMethodNotAllowed)
	}

	// Each POST uses a unique versioned client ID to avoid rate limiting (1 screenshot/sec/client).
	invalidJSONReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString("{"))
	invalidJSONReq.Header.Set("X-Gasoline-Client", "gasoline-extension/test-1")
	invalidJSONRR := httptest.NewRecorder()
	mux.ServeHTTP(invalidJSONRR, invalidJSONReq)
	if invalidJSONRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots invalid json status = %d, want %d", invalidJSONRR.Code, http.StatusBadRequest)
	}

	missingDataReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(`{"url":"https://example.test"}`))
	missingDataReq.Header.Set("X-Gasoline-Client", "gasoline-extension/test-2")
	missingDataRR := httptest.NewRecorder()
	mux.ServeHTTP(missingDataRR, missingDataReq)
	if missingDataRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots missing data_url status = %d, want %d", missingDataRR.Code, http.StatusBadRequest)
	}

	badFormatReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(`{"data_url":"not-a-data-url"}`))
	badFormatReq.Header.Set("X-Gasoline-Client", "gasoline-extension/test-3")
	badFormatRR := httptest.NewRecorder()
	mux.ServeHTTP(badFormatRR, badFormatReq)
	if badFormatRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots bad data_url format status = %d, want %d", badFormatRR.Code, http.StatusBadRequest)
	}

	badBase64Req := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(`{"data_url":"data:image/jpeg;base64,%%%INVALID%%%"}`))
	badBase64Req.Header.Set("X-Gasoline-Client", "gasoline-extension/test-4")
	badBase64RR := httptest.NewRecorder()
	mux.ServeHTTP(badBase64RR, badBase64Req)
	if badBase64RR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots invalid base64 status = %d, want %d", badBase64RR.Code, http.StatusBadRequest)
	}

	rawImage := []byte("abc123")
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(rawImage)
	validBody := `{"data_url":"` + dataURL + `","url":"https://example.test/page","correlation_id":"corr-1","query_id":"query-1"}`
	validReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(validBody))
	validReq.Header.Set("X-Gasoline-Client", "gasoline-extension/test-5")
	validRR := httptest.NewRecorder()
	mux.ServeHTTP(validRR, validReq)
	if validRR.Code != http.StatusOK {
		t.Fatalf("POST /screenshots valid status = %d, want %d body=%q", validRR.Code, http.StatusOK, validRR.Body.String())
	}
	resp := decodeJSONMap(t, validRR.Body.Bytes())
	savePath := resp["path"].(string)
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("saved screenshot path %q stat error = %v", savePath, err)
	}
	if !strings.Contains(resp["filename"].(string), "example.test") {
		t.Fatalf("filename = %q, expected sanitized hostname", resp["filename"])
	}

	if result, ok := cap.GetQueryResult("query-1"); !ok || len(result) == 0 {
		t.Fatalf("expected query result for query-1 to be set, got ok=%v result=%q", ok, string(result))
	}

	rlReq1 := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(validBody))
	rlReq1.Header.Set("X-Gasoline-Client", "gasoline-extension/rl-client")
	rlRR1 := httptest.NewRecorder()
	mux.ServeHTTP(rlRR1, rlReq1)
	if rlRR1.Code != http.StatusOK {
		t.Fatalf("rate-limit first request status = %d, want %d", rlRR1.Code, http.StatusOK)
	}

	rlReq2 := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(validBody))
	rlReq2.Header.Set("X-Gasoline-Client", "gasoline-extension/rl-client")
	rlRR2 := httptest.NewRecorder()
	mux.ServeHTTP(rlRR2, rlReq2)
	if rlRR2.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limit second request status = %d, want %d", rlRR2.Code, http.StatusTooManyRequests)
	}

	// Capacity branch.
	screenshotRateMu.Lock()
	screenshotRateLimiter = make(map[string]time.Time, 10000)
	for i := 0; i < 10000; i++ {
		screenshotRateLimiter["client-"+strconv.Itoa(i)] = time.Now()
	}
	screenshotRateMu.Unlock()

	capReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(validBody))
	capReq.Header.Set("X-Gasoline-Client", "gasoline-extension/brand-new-client")
	capRR := httptest.NewRecorder()
	mux.ServeHTTP(capRR, capReq)
	if capRR.Code != http.StatusServiceUnavailable {
		t.Fatalf("rate-limiter capacity status = %d, want %d", capRR.Code, http.StatusServiceUnavailable)
	}
}
