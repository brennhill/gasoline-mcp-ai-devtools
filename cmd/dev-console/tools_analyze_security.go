// tools_analyze_security.go — Analyze sub-handlers for security audit and third-party audit.
package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/analysis"
	"github.com/dev-console/dev-console/internal/security"
)

// ============================================
// Security Tool Implementations
// ============================================

func (h *ToolHandler) toolSecurityAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SeverityMin string   `json:"severity_min"`
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Ensure security scanner is initialized
	if h.securityScannerImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Security scanner not initialized", "Internal error — do not retry")}
	}

	// Gather data from capture buffers
	networkBodies := h.capture.GetNetworkBodies()
	waterfallEntries := h.capture.GetNetworkWaterfallEntries()

	// Convert console entries to security.LogEntry
	h.server.mu.RLock()
	consoleEntries := make([]security.LogEntry, len(h.server.entries))
	for i, e := range h.server.entries {
		consoleEntries[i] = security.LogEntry(e)
	}
	h.server.mu.RUnlock()

	// Get page URLs from the tracked tab
	var pageURLs []string
	_, _, tabURL := h.capture.GetTrackingStatus()
	if tabURL != "" {
		pageURLs = append(pageURLs, tabURL)
	}

	// Run the security scan
	result, err := h.securityScannerImpl.HandleSecurityAudit(args, networkBodies, consoleEntries, pageURLs, waterfallEntries)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Internal error — do not retry")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security audit complete", result)}
}

func (h *ToolHandler) toolAuditThirdParties(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, err.Error(), "Fix JSON arguments and try again")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Third-party audit complete", result)}
}
