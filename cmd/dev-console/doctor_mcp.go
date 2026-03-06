// Purpose: Implements MCP-facing doctor response composition.
// Why: Keeps tool-level doctor formatting and status aggregation separate from raw checks.

package main

import (
	"fmt"
	"time"
)

// toolDoctor runs all live diagnostic checks and returns structured results.
// This is the MCP-facing doctor — the daemon is already running.
func (h *ToolHandler) toolDoctor(req JSONRPCRequest) JSONRPCResponse {
	checks := runDoctorChecks(h.capture)

	// Add server uptime (only available via ToolHandler).
	if h.healthMetrics != nil {
		uptime := h.healthMetrics.GetUptime()
		checks = append(checks, doctorCheck{
			Name:   "server_uptime",
			Status: "pass",
			Detail: fmt.Sprintf("Server running for %s (version %s)", uptime.Round(time.Second), version),
		})
	}

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

	return succeed(req, "Doctor: "+overallStatus, map[string]any{
		"status":                overallStatus,
		"ready_for_interaction": readyForInteraction,
		"checks":                checks,
		"hint":                  h.DiagnosticHintString(),
	})
}
