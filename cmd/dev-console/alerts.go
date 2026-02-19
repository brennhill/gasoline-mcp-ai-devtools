// alerts.go — Alert system integration: type aliases, ToolHandler delegation, HTTP handlers.
// Pure logic lives in internal/streaming; this file owns the HTTP/MCP glue.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dev-console/dev-console/internal/streaming"
	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// Type Aliases (backward compatibility)
// ============================================

// Alert is a type alias for the canonical alert type from types package
type Alert = types.Alert

// CIResult is a type alias for the canonical CI result type from types package
type CIResult = types.CIResult

// CIFailure is a type alias for the canonical CI failure type from types package
type CIFailure = types.CIFailure

// ============================================
// Constant Aliases
// ============================================

const (
	alertBufferCap       = streaming.AlertBufferCap
	ciResultsCap         = streaming.CIResultsCap
	correlationWindow    = streaming.CorrelationWindow
	anomalyWindowSeconds = streaming.AnomalyWindowSeconds
	anomalyBucketSeconds = streaming.AnomalyBucketSeconds
)

// ============================================
// Function Aliases
// ============================================

var (
	deduplicateAlerts    = streaming.DeduplicateAlerts
	correlateAlerts      = streaming.CorrelateAlerts
	canCorrelate         = streaming.CanCorrelate
	mergeAlerts          = streaming.MergeAlerts
	sortAlertsByPriority = streaming.SortAlertsByPriority
	severityRank         = streaming.SeverityRank
	formatAlertsBlock    = streaming.FormatAlertsBlock
	buildAlertSummary    = streaming.BuildAlertSummary
)

// ============================================
// ToolHandler Delegation
// ============================================

// addAlert delegates to the AlertBuffer.
func (h *ToolHandler) addAlert(a Alert) {
	h.alertBuffer.AddAlert(a)
}

// drainAlerts delegates to the AlertBuffer.
func (h *ToolHandler) drainAlerts() []Alert {
	return h.alertBuffer.DrainAlerts()
}

// recordErrorForAnomaly delegates to the AlertBuffer.
func (h *ToolHandler) recordErrorForAnomaly(t time.Time) {
	h.alertBuffer.RecordErrorForAnomaly(t)
}

// buildCIAlert delegates to the streaming package.
func (h *ToolHandler) buildCIAlert(ci CIResult) Alert {
	return streaming.BuildCIAlert(ci)
}

// ============================================
// CI/CD Webhook Handler
// ============================================

func (h *ToolHandler) handleCIWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ciResult, err := parseCIWebhookBody(r)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ciResult.ReceivedAt = time.Now().UTC()

	newAlert := h.alertBuffer.ProcessCIResult(ciResult)

	// Emit to streaming outside the lock
	if newAlert != nil && h.alertBuffer.Stream != nil {
		h.alertBuffer.Stream.EmitAlert(*newAlert)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// parseCIWebhookBody reads and validates the CI webhook request body.
func parseCIWebhookBody(r *http.Request) (CIResult, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 1024*1024)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return CIResult{}, fmt.Errorf(`{"error":"request body too large"}`)
	}

	var ciResult CIResult
	if err := json.Unmarshal(body, &ciResult); err != nil {
		return CIResult{}, fmt.Errorf(`{"error":"invalid JSON"}`)
	}

	if ciResult.Status == "" {
		return CIResult{}, fmt.Errorf(`{"error":"missing required field: status"}`)
	}
	if ciResult.Source == "" {
		return CIResult{}, fmt.Errorf(`{"error":"missing required field: source"}`)
	}

	return ciResult, nil
}
