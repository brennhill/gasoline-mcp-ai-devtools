// Purpose: Adapts streaming alert/CI primitives into ToolHandler-facing alert management methods.
// Why: Keeps alert buffering/dedup logic centralized in streaming while preserving legacy cmd package call sites.
// Docs: docs/features/feature/push-alerts/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
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
