// health.go — Health & SLA Metrics (Tier 3.4).
// Exposes server uptime, buffer utilization, memory usage, request counts,
// and error rates via the get_health MCP tool.
// Design: Thread-safe counters using sync.RWMutex. Memory stats from runtime.
// All metrics are computed on-demand when the tool is called.
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
	Server       ServerInfo       `json:"server"`
	Memory       MemoryInfo       `json:"memory"`
	Buffers      BuffersInfo      `json:"buffers"`
	RateLimiting RateLimitingInfo `json:"rate_limiting"`
	Audit        AuditInfo        `json:"audit"`
	Pilot        PilotInfo        `json:"pilot"`
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
	CurrentMB     float64 `json:"current_mb"`
	AllocMB       float64 `json:"alloc_mb"`
	SysMB         float64 `json:"sys_mb"`
	HardLimitMB   float64 `json:"hard_limit_mb"`
	UsedPct       float64 `json:"used_pct"`
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
}

// RateLimitingInfo contains rate limiting state.
type RateLimitingInfo struct {
	CurrentRate    int    `json:"current_rate"`
	Threshold      int    `json:"threshold"`
	CircuitOpen    bool   `json:"circuit_open"`
	CircuitReason  string `json:"circuit_reason,omitempty"`
	Total429s      int    `json:"total_429s"`
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
	// Server info
	serverInfo := ServerInfo{
		Version:       ver,
		UptimeSeconds: hm.GetUptime().Seconds(),
		PID:           os.Getpid(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:     runtime.Version(),
	}

	// Memory info from runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	hardLimitMB := float64(capture.MemoryHardLimit) / (1024 * 1024)
	currentMB := float64(memStats.Alloc) / (1024 * 1024)
	usedPct := (currentMB / hardLimitMB) * 100
	if usedPct > 100 {
		usedPct = 100
	}

	// Read all capture state under a single lock for atomic snapshot
	var wsMem, nbMem, actionMem int64
	var networkEntries, wsEntries, actionEntries int
	var consoleEntries int
	var currentRate int
	var circuitOpen bool
	var circuitReason string
	if server != nil {
		server.mu.RLock()
		consoleEntries = len(server.entries)
		server.mu.RUnlock()
	}
	if cap != nil {
		// Use getter methods instead of direct field access
		health := cap.GetHealthSnapshot()
		networkEntries = health.NetworkBodyCount
		wsEntries = health.WebSocketCount
		actionEntries = health.ActionCount
		currentRate = health.WindowEventCount
		circuitOpen = health.CircuitOpen
		circuitReason = health.CircuitReason
		// TODO: Get memory calculations through accessor if needed
		// For now, using 0 as placeholder
		wsMem = 0
		nbMem = 0
		actionMem = 0
	}

	memInfo := MemoryInfo{
		CurrentMB:   currentMB,
		AllocMB:     float64(memStats.Alloc) / (1024 * 1024),
		SysMB:       float64(memStats.Sys) / (1024 * 1024),
		HardLimitMB: hardLimitMB,
		UsedPct:     usedPct,
		BufferBreakdown: BufferMemoryBreakdown{
			WebSocketBytes: wsMem,
			NetworkBytes:   nbMem,
			ActionsBytes:   actionMem,
		},
	}

	consoleCapacity := defaultMaxEntries
	if server != nil {
		consoleCapacity = server.maxEntries
	}
	buffersInfo := BuffersInfo{
		Console: BufferStats{
			Entries:        consoleEntries,
			Capacity:       consoleCapacity,
			UtilizationPct: calcUtilization(consoleEntries, consoleCapacity),
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

	rateLimitInfo := RateLimitingInfo{
		CurrentRate:   currentRate,
		Threshold:     capture.RateLimitThreshold,
		CircuitOpen:   circuitOpen,
		CircuitReason: circuitReason,
		Total429s:     0, // Could be tracked separately if needed
	}

	// Audit info
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

	auditInfo := AuditInfo{
		TotalCalls:   totalCalls,
		TotalErrors:  totalErrors,
		ErrorRatePct: errorRate,
		CallsPerTool: callsPerTool,
	}

	// Pilot info
	pilotStatus := PilotInfo{
		Enabled:            false,
		Source:             "never_connected",
		ExtensionConnected: false,
	}
	if cap != nil {
		statusMap := cap.GetPilotStatus()
		if m, ok := statusMap.(map[string]any); ok {
			enabled := false
			if e, ok := m["enabled"].(bool); ok {
				enabled = e
			}
			source := ""
			if s, ok := m["source"].(string); ok {
				source = s
			}
			extConn := false
			if ec, ok := m["extension_connected"].(bool); ok {
				extConn = ec
			}
			pilotStatus = PilotInfo{
				Enabled:            enabled,
				Source:             source,
				ExtensionConnected: extConn,
				LastPollAgo:        "",
			}
		}
	}

	return MCPHealthResponse{
		Server:       serverInfo,
		Memory:       memInfo,
		Buffers:      buffersInfo,
		RateLimiting: rateLimitInfo,
		Audit:        auditInfo,
		Pilot:        pilotStatus,
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

	hm, ok := h.healthMetrics.(*HealthMetrics)
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Health metrics type mismatch", "Internal server error — do not retry")}
	}
	response := hm.GetHealth(h.capture, h.server, version)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Server health", response)}
}
