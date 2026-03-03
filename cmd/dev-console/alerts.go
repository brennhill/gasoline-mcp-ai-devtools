// Purpose: Adapts streaming alert/CI primitives into ToolHandler-facing alert management methods.
// Why: Keeps alert buffering/dedup logic centralized in streaming while preserving legacy cmd package call sites.
// Docs: docs/features/feature/push-alerts/index.md

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/streaming"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"
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


// drainAlerts delegates to the AlertBuffer.
func (h *ToolHandler) drainAlerts() []Alert {
	return h.alertBuffer.DrainAlerts()
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
		return CIResult{}, fmt.Errorf("ci_alert: request body too large. Reduce payload size to under 1 MB")
	}

	var ciResult CIResult
	if err := json.Unmarshal(body, &ciResult); err != nil {
		return CIResult{}, fmt.Errorf("ci_alert: invalid JSON in request body. Send a valid JSON object with status and source fields")
	}

	if ciResult.Status == "" {
		return CIResult{}, fmt.Errorf("ci_alert: missing required field 'status'. Include a status field (e.g. pass, fail) in the request body")
	}
	if ciResult.Source == "" {
		return CIResult{}, fmt.Errorf("ci_alert: missing required field 'source'. Include a source field identifying the CI system")
	}

	return ciResult, nil
}
