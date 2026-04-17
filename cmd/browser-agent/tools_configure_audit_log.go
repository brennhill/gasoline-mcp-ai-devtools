// Purpose: Handles configure audit_log report/analyze/clear operations.
// Why: Isolates audit-trail filtering and session cleanup logic from other configure handlers.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.auditTrail == nil {
		return fail(req, ErrNotInitialized, "Audit trail not initialized", "Internal error — do not retry")
	}

	var params struct {
		Operation      string `json:"operation"`
		AuditSessionID string `json:"audit_session_id"`
		ToolName       string `json:"tool_name"`
		Limit          int    `json:"limit"`
		Since          string `json:"since"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation == "" {
		operation = "report"
	}
	if operation != "analyze" && operation != "report" && operation != "clear" {
		return fail(req, ErrInvalidParam, "Invalid audit_log operation: "+params.Operation, "Use operation: analyze, report, or clear", withParam("operation"))
	}
	if operation == "clear" {
		cleared := h.auditTrail.Clear()
		func() {
			h.auditMu.Lock()
			defer h.auditMu.Unlock()
			h.auditSessionMap = make(map[string]string)
		}()
		return succeed(req, "Audit log cleared", map[string]any{
			"status":    "ok",
			"operation": "clear",
			"cleared":   cleared,
		})
	}

	filter := audit.Filter{
		AuditSessionID: params.AuditSessionID,
		ToolName:       params.ToolName,
		Limit:          params.Limit,
	}
	if params.Since != "" {
		since, err := time.Parse(time.RFC3339, params.Since)
		if err != nil {
			return fail(req, ErrInvalidParam, "Invalid 'since' timestamp: "+err.Error(), "Use RFC3339 format, for example 2026-02-17T15:04:05Z", withParam("since"))
		}
		filter.Since = &since
	}

	entries := h.auditTrail.Query(filter)
	if operation == "analyze" {
		return succeed(req, "Audit log analysis", map[string]any{
			"status":    "ok",
			"operation": "analyze",
			"summary":   cfg.SummarizeAuditEntries(entries),
		})
	}

	return succeed(req, "Audit log entries", map[string]any{
		"status":    "ok",
		"operation": "report",
		"entries":   entries,
		"count":     len(entries),
	})
}
