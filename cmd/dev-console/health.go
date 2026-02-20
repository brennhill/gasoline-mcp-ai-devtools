// Purpose: Owns health.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// health.go — Health & SLA Metrics (Tier 3.4).
// Exposes server uptime, buffer utilization, memory usage, request counts,
// and error rates via the get_health MCP tool.
// Design: Thread-safe counters using sync.RWMutex. Memory stats from runtime.
// All metrics are computed on-demand when the tool is called.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package main

import (
	"github.com/dev-console/dev-console/internal/capture"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	defaultMaxEntries = 1000
)

// ============================================
// HealthMetrics
// ============================================

// HealthMetrics tracks server operational metrics for the get_health tool.
// All counters are thread-safe for concurrent access.
type HealthMetrics struct {
	mu            sync.RWMutex
	startTime     time.Time
	requestCounts map[string]int64 // per-tool request counts
	errorCounts   map[string]int64 // per-tool error counts
}

// NewHealthMetrics creates a new HealthMetrics instance with initialized counters.
func NewHealthMetrics() *HealthMetrics {
	return &HealthMetrics{
		startTime:     time.Now(),
		requestCounts: make(map[string]int64),
		errorCounts:   make(map[string]int64),
	}
}

// IncrementRequest increments the request count for the given tool.
func (hm *HealthMetrics) IncrementRequest(toolName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.requestCounts[toolName]++
}

// IncrementError increments the error count for the given tool.
func (hm *HealthMetrics) IncrementError(toolName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.errorCounts[toolName]++
}

// GetRequestCount returns the request count for the given tool.
func (hm *HealthMetrics) GetRequestCount(toolName string) int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.requestCounts[toolName]
}

// GetErrorCount returns the error count for the given tool.
func (hm *HealthMetrics) GetErrorCount(toolName string) int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.errorCounts[toolName]
}

// GetTotalRequests returns the total request count across all tools.
func (hm *HealthMetrics) GetTotalRequests() int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	var total int64
	for _, count := range hm.requestCounts {
		total += count
	}
	return total
}

// GetTotalErrors returns the total error count across all tools.
func (hm *HealthMetrics) GetTotalErrors() int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	var total int64
	for _, count := range hm.errorCounts {
		total += count
	}
	return total
}

// GetUptime returns the server uptime duration.
func (hm *HealthMetrics) GetUptime() time.Duration {
	return time.Since(hm.startTime)
}

// ============================================
// Health Response Types
// ============================================

// MCPHealthResponse is the response structure for the get_health MCP tool.
// Named to distinguish from the simpler HealthResponse used by /health HTTP endpoint.
type MCPHealthResponse struct {
	Server           ServerInfo           `json:"server"`
	Memory           MemoryInfo           `json:"memory"`
	Buffers          BuffersInfo          `json:"buffers"`
	RateLimiting     RateLimitingInfo     `json:"rate_limiting"`
	Audit            AuditInfo            `json:"audit"`
	Pilot            PilotInfo            `json:"pilot"`
	CommandExecution CommandExecutionInfo `json:"command_execution"`
	Upgrade          *UpgradeInfo         `json:"upgrade,omitempty"`
}

// UpgradeInfo contains binary upgrade detection state.
type UpgradeInfo struct {
	Pending    bool   `json:"pending"`
	NewVersion string `json:"new_version"`
	DetectedAt string `json:"detected_at"`
}

// ServerInfo contains server identification and uptime.
type ServerInfo struct {
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	PID           int     `json:"pid"`
	Platform      string  `json:"platform"`
	GoVersion     string  `json:"go_version"`
}

// MemoryInfo contains memory usage statistics.
type MemoryInfo struct {
	CurrentMB       float64               `json:"current_mb"`
	AllocMB         float64               `json:"alloc_mb"`
	SysMB           float64               `json:"sys_mb"`
	BufferBreakdown BufferMemoryBreakdown `json:"buffer_breakdown"`
}

// BufferMemoryBreakdown shows memory usage per buffer type.
type BufferMemoryBreakdown struct {
	WebSocketBytes int64 `json:"websocket_bytes"`
	NetworkBytes   int64 `json:"network_bytes"`
	ActionsBytes   int64 `json:"actions_bytes"`
}

// BuffersInfo contains buffer utilization statistics.
type BuffersInfo struct {
	Console   BufferStats `json:"console"`
	Network   BufferStats `json:"network"`
	WebSocket BufferStats `json:"websocket"`
	Actions   BufferStats `json:"actions"`
}

// BufferStats contains statistics for a single buffer.
type BufferStats struct {
	Entries        int     `json:"entries"`
	Capacity       int     `json:"capacity"`
	UtilizationPct float64 `json:"utilization_pct"`
	DroppedCount   int64   `json:"dropped_count"`
}

// RateLimitingInfo contains rate limiting state.
type RateLimitingInfo struct {
	CurrentRate   int    `json:"current_rate"`
	Threshold     int    `json:"threshold"`
	CircuitOpen   bool   `json:"circuit_open"`
	CircuitReason string `json:"circuit_reason,omitempty"`
	Total429s     int    `json:"total_429s"`
}

// AuditInfo contains tool invocation statistics.
type AuditInfo struct {
	TotalCalls   int64            `json:"total_calls"`
	TotalErrors  int64            `json:"total_errors"`
	ErrorRatePct float64          `json:"error_rate_pct"`
	CallsPerTool map[string]int64 `json:"calls_per_tool"`
}

// PilotInfo contains AI Web Pilot toggle state and connection status.
type PilotInfo struct {
	Enabled            bool   `json:"enabled"`
	Source             string `json:"source"` // "extension_poll", "stale", or "never_connected"
	ExtensionConnected bool   `json:"extension_connected"`
	LastPollAgo        string `json:"last_poll_ago,omitempty"`
}

// ============================================
// GetHealth
// ============================================

// GetHealth computes and returns the current health metrics.
// This is called on-demand when the get_health tool is invoked.
func (hm *HealthMetrics) GetHealth(cap *capture.Capture, server *Server, ver string) MCPHealthResponse {
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
	return ServerInfo{
		Version:       ver,
		UptimeSeconds: hm.GetUptime().Seconds(),
		PID:           os.Getpid(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:     runtime.Version(),
	}
}

// buildMemoryInfo returns runtime memory statistics.
func buildMemoryInfo(cap *capture.Capture) MemoryInfo {
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
func buildBuffersInfo(cap *capture.Capture, server *Server) BuffersInfo {
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
			Entries: consoleEntries, Capacity: consoleCapacity,
			UtilizationPct: calcUtilization(consoleEntries, consoleCapacity), DroppedCount: consoleDropped,
		},
		Network: BufferStats{
			Entries: networkEntries, Capacity: capture.MaxNetworkBodies,
			UtilizationPct: calcUtilization(networkEntries, capture.MaxNetworkBodies),
		},
		WebSocket: BufferStats{
			Entries: wsEntries, Capacity: capture.MaxWSEvents,
			UtilizationPct: calcUtilization(wsEntries, capture.MaxWSEvents),
		},
		Actions: BufferStats{
			Entries: actionEntries, Capacity: capture.MaxEnhancedActions,
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
func buildRateLimitInfo(cap *capture.Capture) RateLimitingInfo {
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
		TotalCalls: totalCalls, TotalErrors: totalErrors,
		ErrorRatePct: errorRate, CallsPerTool: callsPerTool,
	}
}

// buildPilotInfo returns AI Web Pilot status from capture.
func buildPilotInfo(cap *capture.Capture) PilotInfo {
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

// ============================================
// MCP Tool Handler
// ============================================

// toolGetHealth is the MCP tool handler for get_health.
// It returns comprehensive server health metrics.
func (h *ToolHandler) toolGetHealth(req JSONRPCRequest) JSONRPCResponse {
	if h.healthMetrics == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")}
	}

	response := h.healthMetrics.GetHealth(h.capture, h.server, version)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Server health", response)}
}
