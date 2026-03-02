// Purpose: Applies cross-cutting tool response post-processing (redaction, warnings, telemetry).
// Why: Keeps tools/call execution flow focused while encapsulating response-guard policies.

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// applyToolResponsePostProcessing applies redaction and operator warnings to tool output.
func (h *MCPHandler) applyToolResponsePostProcessing(resp JSONRPCResponse, clientID, toolName, telemetryModeOverride string) JSONRPCResponse {
	redactor := h.toolHandler.GetRedactionEngine()
	if redactor != nil && resp.Result != nil {
		resp.Result = redactor.RedactJSON(resp.Result)
	}
	if h.server != nil {
		resp = appendWarningsToResponse(resp, h.server.TakeWarnings())
	}
	resp = h.maybeAddSecurityModeWarning(resp)
	resp = h.maybeAddVersionWarning(resp)
	resp = maybeAddUpdateAvailableWarning(resp)
	resp = maybeAddUpgradeWarning(resp)
	return h.maybeAddTelemetrySummary(resp, clientID, toolName, telemetryModeOverride)
}

// prependWarningToResponse prepends a warning string to the first text block of an MCP response.
func prependWarningToResponse(resp JSONRPCResponse, warning string) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		result.Content[0].Text = warning + result.Content[0].Text
	} else {
		result.Content = append([]MCPContentBlock{{Type: "text", Text: warning}}, result.Content...)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

func (h *MCPHandler) maybeAddSecurityModeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}

	mode, productionParity, rewrites := cap.GetSecurityMode()
	if mode == capture.SecurityModeNormal {
		return resp
	}

	warning := "[ALTERED ENVIRONMENT] security_mode=insecure_proxy; production_parity=false. CSP headers are rewritten for debugging.\n\n"
	resp = prependWarningToResponse(resp, warning)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["security_mode"] = mode
	result.Metadata["production_parity"] = productionParity
	result.Metadata["insecure_rewrites_applied"] = rewrites

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// maybeAddVersionWarning prepends a version mismatch warning when extension/server major.minor differ.
func (h *MCPHandler) maybeAddVersionWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}
	extVer, srvVer, mismatch := cap.GetVersionMismatch()
	if !mismatch {
		return resp
	}

	warning := fmt.Sprintf("WARNING: Version mismatch detected — server v%s, extension v%s. Update your extension to avoid issues.\n\n", srvVer, extVer)
	return prependWarningToResponse(resp, warning)
}

// updateNotifyLastShown tracks when the update-available warning was last shown.
var updateNotifyLastShown time.Time

// maybeAddUpdateAvailableWarning prepends a notice when a newer release is available.
func maybeAddUpdateAvailableWarning(resp JSONRPCResponse) JSONRPCResponse {
	if resp.Result == nil {
		return resp
	}

	if binaryUpgradeState != nil {
		if pending, _, _ := binaryUpgradeState.UpgradeInfo(); pending {
			return resp
		}
	}

	availVer := getAvailableVersion()

	if availVer == "" || !isNewerVersion(availVer, version) {
		return resp
	}
	if !updateNotifyLastShown.IsZero() && time.Since(updateNotifyLastShown) < 24*time.Hour {
		return resp
	}
	updateNotifyLastShown = time.Now()

	warning := fmt.Sprintf("UPDATE AVAILABLE: Gasoline v%s is available (current: v%s). Run: npm install -g gasoline-mcp@latest\n\n", availVer, version)
	return prependWarningToResponse(resp, warning)
}

// maybeAddUpgradeWarning prepends a binary-upgrade notice when auto-restart is pending.
func maybeAddUpgradeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if binaryUpgradeState == nil || resp.Result == nil {
		return resp
	}
	pending, newVer, detectedAt := binaryUpgradeState.UpgradeInfo()
	if !pending {
		return resp
	}

	elapsed := time.Since(detectedAt).Truncate(time.Second)
	warning := fmt.Sprintf("NOTICE: Gasoline v%s detected on disk (current: v%s, detected %s ago). Auto-restart imminent. Your next tool call will use the new version.\n\n", newVer, version, elapsed)
	return prependWarningToResponse(resp, warning)
}
