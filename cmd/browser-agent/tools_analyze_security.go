// Purpose: Handles analyze modes for security_audit and third_party_audit, delegating to internal/security and internal/analysis.
// Why: Isolates security-focused analysis from general analyze dispatch to keep audit logic focused.
// Docs: docs/features/feature/security-hardening/index.md
package main

import (
	"encoding/json"
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security"
)

// ============================================
// Security Tool Implementations
// ============================================

func (h *ToolHandler) toolAnalyzeSecurityAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SeverityMin string   `json:"severity_min"`
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		Summary     bool     `json:"summary"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Ensure security scanner is initialized
	if h.securityScannerImpl == nil {
		return fail(req, ErrNotInitialized, "Security scanner not initialized", "Internal error — do not retry")
	}

	// Gather data from capture buffers
	networkBodies := h.capture.GetNetworkBodies()
	waterfallEntries := h.capture.GetNetworkWaterfallEntries()

	// Convert console entries to security.LogEntry
	h.server.logs.mu.RLock()
	consoleEntries := make([]security.LogEntry, len(h.server.logs.entries))
	for i, e := range h.server.logs.entries {
		consoleEntries[i] = security.LogEntry(e)
	}
	h.server.logs.mu.RUnlock()

	// Get page URLs from the tracked tab
	var pageURLs []string
	_, _, tabURL := h.capture.GetTrackingStatus()
	if tabURL != "" {
		pageURLs = append(pageURLs, tabURL)
	}

	// Run the security scan
	result, err := h.securityScannerImpl.HandleSecurityAudit(args, networkBodies, consoleEntries, pageURLs, waterfallEntries)
	if err != nil {
		return fail(req, ErrInternal, err.Error(), "Internal error — do not retry")
	}

	if params.Summary {
		if scanResult, ok := result.(security.ScanResult); ok {
			return succeed(req, "Security audit summary", buildSecurityAuditSummary(scanResult))
		}
	}

	return succeed(req, "Security audit complete", result)
}

func (h *ToolHandler) toolAuditThirdParties(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Summary bool `json:"summary"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &params)
	}

	// Gather data from capture buffers
	networkBodies := h.capture.GetNetworkBodies()

	// Get page URLs from the tracked tab
	var pageURLs []string
	_, _, tabURL := h.capture.GetTrackingStatus()
	if tabURL != "" {
		pageURLs = append(pageURLs, tabURL)
	}

	// Use the package-level handler function
	result, err := analysis.HandleAuditThirdParties(args, networkBodies, pageURLs)
	if err != nil {
		return fail(req, ErrInvalidJSON, err.Error(), "Fix JSON arguments and try again")
	}

	if params.Summary {
		if tpResult, ok := result.(analysis.ThirdPartyResult); ok {
			return succeed(req, "Third-party audit summary", buildThirdPartySummary(tpResult))
		}
	}

	return succeed(req, "Third-party audit complete", result)
}

// ============================================
// Summary Builders
// ============================================

var severityOrder = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

func buildSecurityAuditSummary(result security.ScanResult) map[string]any {
	bySeverity := make(map[string]int)
	for _, f := range result.Findings {
		bySeverity[f.Severity]++
	}

	topN := 5
	if len(result.Findings) < topN {
		topN = len(result.Findings)
	}

	// Sort findings by severity (critical first)
	sorted := make([]security.Finding, len(result.Findings))
	copy(sorted, result.Findings)
	sort.Slice(sorted, func(i, j int) bool {
		return severityOrder[sorted[i].Severity] < severityOrder[sorted[j].Severity]
	})

	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"check":    sorted[i].Check,
			"severity": sorted[i].Severity,
			"title":    sorted[i].Title,
		}
	}

	return map[string]any{
		"total":       len(result.Findings),
		"by_severity": bySeverity,
		"top_issues":  topIssues,
	}
}

func buildThirdPartySummary(result analysis.ThirdPartyResult) map[string]any {
	byRisk := map[string]int{
		"critical": result.Summary.CriticalRisk,
		"high":     result.Summary.HighRisk,
		"medium":   result.Summary.MediumRisk,
		"low":      result.Summary.LowRisk,
	}

	topN := 5
	if len(result.ThirdParties) < topN {
		topN = len(result.ThirdParties)
	}

	// Sort by risk (critical first)
	sorted := make([]analysis.ThirdPartyEntry, len(result.ThirdParties))
	copy(sorted, result.ThirdParties)
	sort.Slice(sorted, func(i, j int) bool {
		return severityOrder[sorted[i].RiskLevel] < severityOrder[sorted[j].RiskLevel]
	})

	top := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		top[i] = map[string]any{
			"origin": sorted[i].Origin,
			"risk":   sorted[i].RiskLevel,
			"reason": sorted[i].RiskReason,
		}
	}

	return map[string]any{
		"total_origins": len(result.ThirdParties),
		"by_risk":       byRisk,
		"top":           top,
	}
}
