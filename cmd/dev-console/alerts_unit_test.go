// alerts_unit_test.go — Integration tests for CI webhook HTTP handlers.
// Pure alert logic tests live in internal/streaming/alerts_test.go.
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/streaming"
)

func TestHandleCIWebhook(t *testing.T) {
	h := &ToolHandler{alertBuffer: streaming.NewAlertBuffer()}

	// Method guard.
	getReq := httptest.NewRequest(http.MethodGet, "/ci/webhook", nil)
	getRR := httptest.NewRecorder()
	h.handleCIWebhook(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", getRR.Code)
	}

	// Invalid JSON.
	badReq := httptest.NewRequest(http.MethodPost, "/ci/webhook", strings.NewReader("{bad"))
	badRR := httptest.NewRecorder()
	h.handleCIWebhook(badRR, badReq)
	if badRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, want 400", badRR.Code)
	}

	// Missing required fields.
	missingReq := httptest.NewRequest(http.MethodPost, "/ci/webhook", strings.NewReader(`{"status":"failure"}`))
	missingRR := httptest.NewRecorder()
	h.handleCIWebhook(missingRR, missingReq)
	if missingRR.Code != http.StatusBadRequest {
		t.Fatalf("missing source status = %d, want 400", missingRR.Code)
	}

	// Valid insert.
	payload := `{
		"status":"failure",
		"source":"github-actions",
		"commit":"abc123",
		"summary":"test failures",
		"failures":[{"name":"auth test","message":"boom"}]
	}`
	okReq := httptest.NewRequest(http.MethodPost, "/ci/webhook", strings.NewReader(payload))
	okRR := httptest.NewRecorder()
	h.handleCIWebhook(okRR, okReq)
	if okRR.Code != http.StatusOK {
		t.Fatalf("valid webhook status = %d, want 200", okRR.Code)
	}

	h.alertBuffer.Mu.Lock()
	if len(h.alertBuffer.CIResults) != 1 || len(h.alertBuffer.Alerts) != 1 {
		h.alertBuffer.Mu.Unlock()
		t.Fatalf("expected ciResults=1 alerts=1, got ciResults=%d alerts=%d", len(h.alertBuffer.CIResults), len(h.alertBuffer.Alerts))
	}
	initialReceived := h.alertBuffer.CIResults[0].ReceivedAt
	h.alertBuffer.Mu.Unlock()

	// Idempotent update: same commit+status should update, not append.
	time.Sleep(10 * time.Millisecond)
	updatePayload := `{
		"status":"failure",
		"source":"github-actions",
		"commit":"abc123",
		"summary":"updated summary"
	}`
	updateReq := httptest.NewRequest(http.MethodPost, "/ci/webhook", strings.NewReader(updatePayload))
	updateRR := httptest.NewRecorder()
	h.handleCIWebhook(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("update webhook status = %d, want 200", updateRR.Code)
	}

	h.alertBuffer.Mu.Lock()
	if len(h.alertBuffer.CIResults) != 1 || len(h.alertBuffer.Alerts) != 1 {
		h.alertBuffer.Mu.Unlock()
		t.Fatalf("idempotent update should not append, got ciResults=%d alerts=%d", len(h.alertBuffer.CIResults), len(h.alertBuffer.Alerts))
	}
	if h.alertBuffer.CIResults[0].Summary != "updated summary" {
		h.alertBuffer.Mu.Unlock()
		t.Fatalf("ci result summary not updated: %+v", h.alertBuffer.CIResults[0])
	}
	if !h.alertBuffer.CIResults[0].ReceivedAt.After(initialReceived) {
		h.alertBuffer.Mu.Unlock()
		t.Fatalf("ReceivedAt should be refreshed on update; before=%v after=%v", initialReceived, h.alertBuffer.CIResults[0].ReceivedAt)
	}
	h.alertBuffer.Mu.Unlock()

	// Body too large.
	huge := bytes.Repeat([]byte("x"), 1024*1024+1)
	hugeReq := httptest.NewRequest(http.MethodPost, "/ci/webhook", bytes.NewReader(huge))
	hugeRR := httptest.NewRecorder()
	h.handleCIWebhook(hugeRR, hugeReq)
	if hugeRR.Code != http.StatusBadRequest {
		t.Fatalf("huge body status = %d, want 400", hugeRR.Code)
	}
}

func TestHandleCIWebhookCapsBuffers(t *testing.T) {
	h := &ToolHandler{alertBuffer: streaming.NewAlertBuffer()}
	h.alertBuffer.Mu.Lock()
	for i := 0; i < ciResultsCap; i++ {
		h.alertBuffer.CIResults = append(h.alertBuffer.CIResults, CIResult{
			Status: "failure",
			Source: "gha",
			Commit: "commit-" + string(rune('a'+i)),
		})
	}
	for i := 0; i < alertBufferCap; i++ {
		h.alertBuffer.Alerts = append(h.alertBuffer.Alerts, Alert{
			Category: "ci",
			Detail:   "detail [commit-x]",
		})
	}
	h.alertBuffer.Mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/ci/webhook", strings.NewReader(`{
		"status":"failure",
		"source":"gha",
		"commit":"commit-new",
		"summary":"new failure"
	}`))
	rr := httptest.NewRecorder()
	h.handleCIWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, want 200", rr.Code)
	}

	h.alertBuffer.Mu.Lock()
	defer h.alertBuffer.Mu.Unlock()
	if len(h.alertBuffer.CIResults) != ciResultsCap {
		t.Fatalf("ciResults len = %d, want %d", len(h.alertBuffer.CIResults), ciResultsCap)
	}
	if len(h.alertBuffer.Alerts) != alertBufferCap {
		t.Fatalf("alerts len = %d, want %d", len(h.alertBuffer.Alerts), alertBufferCap)
	}
}
