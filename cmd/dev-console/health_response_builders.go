// Purpose: Assembles get_health payloads from runtime state, capture snapshots, and process metadata.
// Why: Keeps response composition logic cohesive and independent from mutation/handler paths.
// Docs: docs/features/feature/observe/index.md

package main

import (
	"os"
	"runtime"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// GetHealth computes and returns the current health metrics.
// This is called on-demand when the get_health tool is invoked.
func (hm *HealthMetrics) GetHealth(cap *capture.Store, server *Server, ver string) MCPHealthResponse {
	resp := MCPHealthResponse{
		Server:           hm.buildServerInfo(ver),
		Memory:           buildMemoryInfo(cap),
		Buffers:          buildBuffersInfo(cap, server),
		RateLimiting:     buildRateLimitInfo(cap),
		Audit:            hm.buildAuditInfo(),
		Pilot:            buildPilotInfo(cap),
		CommandExecution: buildCommandExecutionInfo(cap),
	}
	if info := buildUpgradeInfo(); info != nil {
		resp.Upgrade = info
	}
	return resp
}

// buildUpgradeInfo returns upgrade detection state, or nil if no upgrade is pending.
func buildUpgradeInfo() *UpgradeInfo {
	if binaryUpgradeState == nil {
		return nil
	}
	pending, newVer, detectedAt := binaryUpgradeState.UpgradeInfo()
	if !pending {
		return nil
	}
	return &UpgradeInfo{
		Pending:    true,
		NewVersion: newVer,
		DetectedAt: detectedAt.UTC().Format(time.RFC3339),
	}
}

// buildServerInfo returns server identification and uptime.
func (hm *HealthMetrics) buildServerInfo(ver string) ServerInfo {
	launch := getCurrentLaunchMode()
	return ServerInfo{
		Version:          ver,
		UptimeSeconds:    hm.GetUptime().Seconds(),
		PID:              os.Getpid(),
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:        runtime.Version(),
		LaunchMode:       launch.Mode,
		LaunchModeReason: launch.Reason,
		ParentProcess:    launch.ParentProcess,
	}
}

// buildMemoryInfo returns runtime memory statistics.
func buildMemoryInfo(cap *capture.Store) MemoryInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return MemoryInfo{
		CurrentMB: float64(memStats.Alloc) / (1024 * 1024),
		AllocMB:   float64(memStats.Alloc) / (1024 * 1024),
		SysMB:     float64(memStats.Sys) / (1024 * 1024),
		BufferBreakdown: BufferMemoryBreakdown{
			WebSocketBytes: 0,
			NetworkBytes:   0,
			ActionsBytes:   0,
		},
	}
}

// buildBuffersInfo returns buffer utilization stats from capture and server.
func buildBuffersInfo(cap *capture.Store, server *Server) BuffersInfo {
	var networkEntries, wsEntries, actionEntries int
	if cap != nil {
		health := cap.GetHealthSnapshot()
		networkEntries = health.NetworkBodyCount
		wsEntries = health.WebSocketCount
		actionEntries = health.ActionCount
	}

	consoleEntries, consoleCapacity, consoleDropped := getConsoleStats(server)

	return BuffersInfo{
		Console: BufferStats{
			Entries:        consoleEntries,
			Capacity:       consoleCapacity,
			UtilizationPct: calcUtilization(consoleEntries, consoleCapacity),
			DroppedCount:   consoleDropped,
		},
		Network: BufferStats{
			Entries:        networkEntries,
			Capacity:       capture.MaxNetworkBodies,
			UtilizationPct: calcUtilization(networkEntries, capture.MaxNetworkBodies),
		},
		WebSocket: BufferStats{
			Entries:        wsEntries,
			Capacity:       capture.MaxWSEvents,
			UtilizationPct: calcUtilization(wsEntries, capture.MaxWSEvents),
		},
		Actions: BufferStats{
			Entries:        actionEntries,
			Capacity:       capture.MaxEnhancedActions,
			UtilizationPct: calcUtilization(actionEntries, capture.MaxEnhancedActions),
		},
	}
}

// getConsoleStats returns console buffer entries, capacity, and drop count from the server.
func getConsoleStats(server *Server) (int, int, int64) {
	if server == nil {
		return 0, defaultMaxEntries, 0
	}
	server.mu.RLock()
	entries := len(server.entries)
	server.mu.RUnlock()
	return entries, server.maxEntries, server.getLogDropCount()
}

// buildRateLimitInfo returns rate limiting state from capture.
func buildRateLimitInfo(cap *capture.Store) RateLimitingInfo {
	info := RateLimitingInfo{Threshold: capture.RateLimitThreshold}
	if cap != nil {
		health := cap.GetHealthSnapshot()
		info.CurrentRate = health.WindowEventCount
		info.CircuitOpen = health.CircuitOpen
		info.CircuitReason = health.CircuitReason
	}
	return info
}

// buildAuditInfo returns tool invocation statistics.
func (hm *HealthMetrics) buildAuditInfo() AuditInfo {
	hm.mu.RLock()
	callsPerTool := make(map[string]int64, len(hm.requestCounts))
	var totalCalls, totalErrors int64
	for tool, count := range hm.requestCounts {
		callsPerTool[tool] = count
		totalCalls += count
	}
	for _, count := range hm.errorCounts {
		totalErrors += count
	}
	hm.mu.RUnlock()

	var errorRate float64
	if totalCalls > 0 {
		errorRate = float64(totalErrors) / float64(totalCalls) * 100
	}

	return AuditInfo{
		TotalCalls:   totalCalls,
		TotalErrors:  totalErrors,
		ErrorRatePct: errorRate,
		CallsPerTool: callsPerTool,
	}
}

// buildPilotInfo returns AI Web Pilot status from capture.
func buildPilotInfo(cap *capture.Store) PilotInfo {
	defaultStatus := PilotInfo{Source: "never_connected"}
	if cap == nil {
		return defaultStatus
	}

	statusMap := cap.GetPilotStatus()
	m, ok := statusMap.(map[string]any)
	if !ok {
		return defaultStatus
	}

	enabled, _ := m["enabled"].(bool)
	source, _ := m["source"].(string)
	extConn, _ := m["extension_connected"].(bool)

	return PilotInfo{
		Enabled:            enabled,
		Source:             source,
		ExtensionConnected: extConn,
	}
}

// calcUtilization calculates buffer utilization percentage.
func calcUtilization(entries, capacity int) float64 {
	if capacity <= 0 {
		return 0
	}
	return float64(entries) / float64(capacity) * 100
}
