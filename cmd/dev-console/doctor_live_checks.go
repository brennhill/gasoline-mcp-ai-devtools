// Purpose: Implements live doctor checks for HTTP and MCP surfaces.
// Why: Centralizes runtime readiness evaluation separate from setup preflight checks.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// doctorCheck represents a single diagnostic check result.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "warn", "fail"
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// handleDoctorHTTP serves the /doctor HTTP endpoint with JSON readiness checks.
func handleDoctorHTTP(w http.ResponseWriter, cap *capture.Capture) {
	checks := runDoctorChecks(cap)

	overallStatus := "healthy"
	readyForInteraction := true
	for _, c := range checks {
		if c.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		}
		if c.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":                overallStatus,
		"ready_for_interaction": readyForInteraction,
		"version":               version,
		"checks":                checks,
	})
}

// runDoctorChecks runs all live diagnostic checks against the capture instance.
func runDoctorChecks(cap *capture.Capture) []doctorCheck {
	checks := make([]doctorCheck, 0, 9)
	snap := cap.GetHealthSnapshot()

	// 1. Extension connectivity.
	if cap.IsExtensionConnected() {
		lastSeen := "unknown"
		if !snap.LastPollTime.IsZero() {
			lastSeen = fmt.Sprintf("%.1fs ago", time.Since(snap.LastPollTime).Seconds())
		}
		checks = append(checks, doctorCheck{
			Name: "extension_connected", Status: "pass",
			Detail: "Extension connected (last seen: " + lastSeen + ")",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "extension_connected", Status: "fail",
			Detail: "Extension is not connected",
			Fix:    "Open the Gasoline extension popup and verify it shows 'Connected'. If not, click the extension icon or reload the page.",
		})
	}

	// 2. Pilot enabled/assumed/disabled.
	pilotState := ""
	if status, ok := cap.GetPilotStatus().(map[string]any); ok {
		pilotState, _ = status["state"].(string)
	}
	switch pilotState {
	case "explicitly_disabled":
		checks = append(checks, doctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot is explicitly disabled — interact actions will fail",
			Fix:    "Enable AI Web Pilot in the extension popup",
		})
	case "assumed_enabled":
		checks = append(checks, doctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot status not yet confirmed; assuming enabled until first sync",
			Fix:    "Open the extension once to confirm pilot settings, then rerun doctor",
		})
	default:
		if cap.IsPilotActionAllowed() {
			checks = append(checks, doctorCheck{
				Name: "pilot_enabled", Status: "pass",
				Detail: "AI Web Pilot is enabled",
			})
		} else {
			checks = append(checks, doctorCheck{
				Name: "pilot_enabled", Status: "warn",
				Detail: "AI Web Pilot is disabled — interact actions will fail",
				Fix:    "Enable AI Web Pilot in the extension popup",
			})
		}
	}

	// 3. Tracked tab.
	tracking, tabID, tabURL := cap.GetTrackingStatus()
	if tracking && tabID != 0 {
		checks = append(checks, doctorCheck{
			Name: "tracked_tab", Status: "pass",
			Detail: fmt.Sprintf("Tracking tab %d: %s", tabID, tabURL),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "tracked_tab", Status: "warn",
			Detail: "No tab is being tracked — observe and interact may return empty results",
			Fix:    "Navigate to a page in Chrome. The extension auto-tracks the active tab.",
		})
	}

	// 4. Circuit breaker.
	if !snap.CircuitOpen {
		checks = append(checks, doctorCheck{
			Name: "circuit_breaker", Status: "pass",
			Detail: "Circuit breaker closed (healthy)",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "circuit_breaker", Status: "fail",
			Detail: "Circuit breaker OPEN: " + snap.CircuitReason,
			Fix:    "Extension is sending too many errors. Check observe(errors) for root cause, then use configure(action:'clear',what:'circuit') to reset.",
		})
	}

	// 5. Command queue.
	queueDepth := cap.QueueDepth()
	if queueDepth < 5 {
		detail := "Command queue empty"
		if queueDepth > 0 {
			detail = fmt.Sprintf("Command queue: %d pending", queueDepth)
		}
		checks = append(checks, doctorCheck{
			Name: "command_queue", Status: "pass", Detail: detail,
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "command_queue", Status: "warn",
			Detail: fmt.Sprintf("Command queue has %d pending commands — extension may be falling behind", queueDepth),
			Fix:    "Wait for commands to complete, or check extension connectivity.",
		})
	}

	// 6. Command execution reliability.
	cmdExec := buildCommandExecutionInfo(cap)
	cmdExecCheck := doctorCheck{
		Name:   "command_execution",
		Status: cmdExec.Status,
		Detail: cmdExec.Detail,
	}
	if cmdExec.Status != "pass" {
		cmdExecCheck.Fix = "Inspect observe(what:\"failed_commands\") for recent expiry/timeout/error events and verify extension polling (/sync). If degradation persists, reload the extension or run configure(action:\"restart\")."
	}
	checks = append(checks, cmdExecCheck)

	return checks
}
