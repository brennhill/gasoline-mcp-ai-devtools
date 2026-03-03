// Purpose: Defines stable schemas returned by get_health.
// Why: Keeps response contracts centralized and separate from runtime collection logic.
// Docs: docs/features/feature/observe/index.md

package main

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
	Version          string  `json:"version"`
	UptimeSeconds    float64 `json:"uptime_seconds"`
	PID              int     `json:"pid"`
	Platform         string  `json:"platform"`
	GoVersion        string  `json:"go_version"`
	LaunchMode       string  `json:"launch_mode,omitempty"`
	LaunchModeReason string  `json:"launch_mode_reason,omitempty"`
	ParentProcess    string  `json:"parent_process,omitempty"`
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
