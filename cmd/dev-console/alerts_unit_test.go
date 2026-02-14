package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSeverityRank(t *testing.T) {
	t.Parallel()

	if got := severityRank("error"); got != 3 {
		t.Fatalf("severityRank(error) = %d, want 3", got)
	}
	if got := severityRank("warning"); got != 2 {
		t.Fatalf("severityRank(warning) = %d, want 2", got)
	}
	if got := severityRank("info"); got != 1 {
		t.Fatalf("severityRank(info) = %d, want 1", got)
	}
	if got := severityRank("unknown"); got != 0 {
		t.Fatalf("severityRank(unknown) = %d, want 0", got)
	}
}

func TestDeduplicateCorrelateAndSortAlerts(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 11, 8, 0, 0, 0, time.UTC)
	raw := []Alert{
		{
			Severity:  "error",
			Category:  "regression",
			Title:     "Request failed",
			Detail:    "500 on /api",
			Timestamp: base.Format(time.RFC3339),
			Source:    "network",
		},
		{
			Severity:  "warning",
			Category:  "anomaly",
			Title:     "Spike",
			Detail:    "error burst",
			Timestamp: base.Add(2 * time.Second).Format(time.RFC3339),
			Source:    "anomaly_detector",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI failure",
			Detail:    "pipeline failed",
			Timestamp: base.Add(1 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI failure",
			Detail:    "pipeline failed again",
			Timestamp: base.Add(3 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
	}

	deduped := deduplicateAlerts(raw)
	if len(deduped) != 3 {
		t.Fatalf("deduplicateAlerts len = %d, want 3", len(deduped))
	}

	var ci Alert
	for _, a := range deduped {
		if a.Category == "ci" {
			ci = a
			break
		}
	}
	if ci.Count != 2 {
		t.Fatalf("deduped ci count = %d, want 2", ci.Count)
	}

	correlated := correlateAlerts(deduped)
	if len(correlated) != 2 {
		t.Fatalf("correlateAlerts len = %d, want 2", len(correlated))
	}

	sortAlertsByPriority(correlated)
	if correlated[0].Category != "regression" {
		t.Fatalf("highest priority category = %q, want regression", correlated[0].Category)
	}
	if !strings.Contains(correlated[0].Title, "Correlated:") {
		t.Fatalf("expected merged correlation title, got %q", correlated[0].Title)
	}
}

func TestCanCorrelateAndMergeAlerts(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 11, 8, 5, 0, 0, time.UTC)
	reg := Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "R",
		Detail:    "d1",
		Timestamp: base.Format(time.RFC3339),
		Source:    "src1",
	}
	anom := Alert{
		Severity:  "error",
		Category:  "anomaly",
		Title:     "A",
		Detail:    "d2",
		Timestamp: base.Add(3 * time.Second).Format(time.RFC3339),
		Source:    "src2",
	}

	if !canCorrelate(reg, anom) {
		t.Fatal("expected regression+anomaly within window to correlate")
	}

	merged := mergeAlerts(reg, anom)
	if merged.Severity != "error" {
		t.Fatalf("merged severity = %q, want error", merged.Severity)
	}
	if !strings.Contains(merged.Title, "Correlated:") {
		t.Fatalf("merged title = %q, want correlated prefix", merged.Title)
	}
	if merged.Source != reg.Source {
		t.Fatalf("merged source = %q, want %q", merged.Source, reg.Source)
	}

	other := Alert{Category: "ci", Timestamp: base.Format(time.RFC3339)}
	if canCorrelate(reg, other) {
		t.Fatal("regression+ci should not correlate")
	}

	badTS := Alert{Category: "anomaly", Timestamp: "not-a-time"}
	if canCorrelate(reg, badTS) {
		t.Fatal("invalid timestamp should not correlate")
	}
}

func TestBuildAlertSummaryAndFormatBlock(t *testing.T) {
	t.Parallel()

	alerts := []Alert{
		{Category: "regression"},
		{Category: "anomaly"},
		{Category: "ci"},
		{Category: "ci"},
	}

	summary := buildAlertSummary(alerts)
	if !strings.Contains(summary, "4 alerts:") {
		t.Fatalf("summary = %q, want count prefix", summary)
	}
	if !strings.Contains(summary, "1 regression") ||
		!strings.Contains(summary, "1 anomaly") ||
		!strings.Contains(summary, "2 ci") {
		t.Fatalf("summary categories missing: %q", summary)
	}

	block := formatAlertsBlock(alerts)
	if !strings.Contains(block, "--- ALERTS (4) ---") {
		t.Fatalf("alerts block missing header: %q", block)
	}
	if !strings.Contains(block, "4 alerts:") {
		t.Fatalf("alerts block should include summary for >=4 alerts: %q", block)
	}

	shortBlock := formatAlertsBlock(alerts[:2])
	if strings.Contains(shortBlock, "alerts:") {
		t.Fatalf("alerts block should not include summary for <=3 alerts: %q", shortBlock)
	}
}

func TestAddAlertAndDrainAlerts(t *testing.T) {
	base := time.Date(2026, 2, 11, 8, 10, 0, 0, time.UTC)
	h := &ToolHandler{}

	for i := 0; i < alertBufferCap+1; i++ {
		h.addAlert(Alert{
			Severity:  "info",
			Category:  "ci",
			Title:     "alert-" + string(rune('a'+(i%26))),
			Timestamp: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Source:    "test",
		})
	}

	h.alertMu.Lock()
	if len(h.alerts) != alertBufferCap {
		h.alertMu.Unlock()
		t.Fatalf("alerts len after eviction = %d, want %d", len(h.alerts), alertBufferCap)
	}
	h.alertMu.Unlock()

	// Replace with deterministic set to exercise dedupe/correlation.
	h.alertMu.Lock()
	h.alerts = []Alert{
		{
			Severity:  "error",
			Category:  "regression",
			Title:     "R1",
			Detail:    "d1",
			Timestamp: base.Format(time.RFC3339),
			Source:    "network",
		},
		{
			Severity:  "warning",
			Category:  "anomaly",
			Title:     "A1",
			Detail:    "d2",
			Timestamp: base.Add(1 * time.Second).Format(time.RFC3339),
			Source:    "anomaly",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI",
			Detail:    "pipeline [abc123]",
			Timestamp: base.Add(2 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI",
			Detail:    "pipeline [abc123]",
			Timestamp: base.Add(3 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
	}
	h.alertMu.Unlock()

	drained := h.drainAlerts()
	if len(drained) != 2 {
		t.Fatalf("drainAlerts len = %d, want 2", len(drained))
	}
	if drained[0].Category != "regression" {
		t.Fatalf("drained[0].Category = %q, want regression", drained[0].Category)
	}
	if drained[1].Category != "ci" || drained[1].Count != 2 {
		t.Fatalf("expected merged CI alert with count=2, got %+v", drained[1])
	}

	if second := h.drainAlerts(); second != nil {
		t.Fatalf("second drain should be nil after clear, got %+v", second)
	}
}

func TestBuildCIAlert(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}
	now := time.Date(2026, 2, 11, 8, 15, 0, 0, time.UTC)
	alert := h.buildCIAlert(CIResult{
		Status:     "failure",
		Source:     "github-actions",
		Commit:     "abc123",
		Summary:    "2 tests failed",
		ReceivedAt: now,
		Failures: []CIFailure{
			{Name: "auth flow"},
			{Name: "checkout flow"},
		},
	})

	if alert.Severity != "error" {
		t.Fatalf("failure status should map to error severity, got %q", alert.Severity)
	}
	if alert.Category != "ci" || alert.Source != "ci_webhook" {
		t.Fatalf("unexpected CI alert fields: %+v", alert)
	}
	if !strings.Contains(alert.Detail, "auth flow") || !strings.Contains(alert.Detail, "[abc123]") {
		t.Fatalf("CI alert detail missing failures/commit: %q", alert.Detail)
	}
	if alert.Timestamp != now.Format(time.RFC3339) {
		t.Fatalf("CI alert timestamp = %q, want %q", alert.Timestamp, now.Format(time.RFC3339))
	}
}

func TestHandleCIWebhook(t *testing.T) {
	h := &ToolHandler{}

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

	h.alertMu.Lock()
	if len(h.ciResults) != 1 || len(h.alerts) != 1 {
		h.alertMu.Unlock()
		t.Fatalf("expected ciResults=1 alerts=1, got ciResults=%d alerts=%d", len(h.ciResults), len(h.alerts))
	}
	initialReceived := h.ciResults[0].ReceivedAt
	h.alertMu.Unlock()

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

	h.alertMu.Lock()
	if len(h.ciResults) != 1 || len(h.alerts) != 1 {
		h.alertMu.Unlock()
		t.Fatalf("idempotent update should not append, got ciResults=%d alerts=%d", len(h.ciResults), len(h.alerts))
	}
	if h.ciResults[0].Summary != "updated summary" {
		h.alertMu.Unlock()
		t.Fatalf("ci result summary not updated: %+v", h.ciResults[0])
	}
	if !h.ciResults[0].ReceivedAt.After(initialReceived) {
		h.alertMu.Unlock()
		t.Fatalf("ReceivedAt should be refreshed on update; before=%v after=%v", initialReceived, h.ciResults[0].ReceivedAt)
	}
	h.alertMu.Unlock()

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
	h := &ToolHandler{}
	h.alertMu.Lock()
	for i := 0; i < ciResultsCap; i++ {
		h.ciResults = append(h.ciResults, CIResult{
			Status: "failure",
			Source: "gha",
			Commit: "commit-" + string(rune('a'+i)),
		})
	}
	for i := 0; i < alertBufferCap; i++ {
		h.alerts = append(h.alerts, Alert{
			Category: "ci",
			Detail:   "detail [commit-x]",
		})
	}
	h.alertMu.Unlock()

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

	h.alertMu.Lock()
	defer h.alertMu.Unlock()
	if len(h.ciResults) != ciResultsCap {
		t.Fatalf("ciResults len = %d, want %d", len(h.ciResults), ciResultsCap)
	}
	if len(h.alerts) != alertBufferCap {
		t.Fatalf("alerts len = %d, want %d", len(h.alerts), alertBufferCap)
	}
}

func TestRecordErrorForAnomaly(t *testing.T) {
	h := &ToolHandler{}
	base := time.Date(2026, 2, 11, 8, 20, 0, 0, time.UTC)

	// Build a burst where the second sample creates the first anomaly,
	// then ensure additional samples inside the bucket do not duplicate alerts.
	h.recordErrorForAnomaly(base)
	h.recordErrorForAnomaly(base.Add(1 * time.Second))
	h.recordErrorForAnomaly(base.Add(2 * time.Second))
	h.recordErrorForAnomaly(base.Add(3 * time.Second))

	h.alertMu.Lock()
	anomalyCount := 0
	for _, a := range h.alerts {
		if a.Category == "anomaly" && a.Source == "anomaly_detector" {
			anomalyCount++
		}
	}
	h.alertMu.Unlock()

	if anomalyCount != 1 {
		t.Fatalf("expected exactly one anomaly alert in the same bucket, got %d", anomalyCount)
	}

	// Additional sample inside the 10s dedupe bucket should not add a new anomaly.
	h.recordErrorForAnomaly(base.Add(9 * time.Second))

	h.alertMu.Lock()
	defer h.alertMu.Unlock()
	finalAnomalyCount := 0
	for _, a := range h.alerts {
		if a.Category == "anomaly" && a.Source == "anomaly_detector" {
			finalAnomalyCount++
		}
	}
	if finalAnomalyCount != 1 {
		t.Fatalf("expected one anomaly alert total, got %d", finalAnomalyCount)
	}
}

func TestFormatAlertsBlockProducesJSONPayload(t *testing.T) {
	t.Parallel()

	alerts := []Alert{
		{
			Severity:  "error",
			Category:  "regression",
			Title:     "boom",
			Timestamp: time.Date(2026, 2, 11, 8, 30, 0, 0, time.UTC).Format(time.RFC3339),
			Source:    "test",
		},
	}
	block := formatAlertsBlock(alerts)

	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		t.Fatalf("formatAlertsBlock returned too few lines: %q", block)
	}
	var parsed []Alert
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &parsed); err != nil {
		t.Fatalf("alerts JSON line not parseable: %v; block=%q", err, block)
	}
	if len(parsed) != 1 || parsed[0].Title != "boom" {
		t.Fatalf("unexpected parsed alerts JSON: %+v", parsed)
	}
}
