// deps.go — Declares the Deps interface for configure-local handlers.
// Why: Narrow interface decouples configure handlers from the full ToolHandler.

package toolconfigure

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Deps provides all dependencies the configure-local handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	// NoiseConfig returns the noise configuration, or nil if not initialized.
	NoiseConfig() *noise.NoiseConfig

	// ConsoleEntries returns a copy of the current console log entries.
	ConsoleEntries() []noise.LogEntry

	// NetworkBodies returns captured network bodies for auto-detect.
	NetworkBodies() []types.NetworkBody

	// AllWebSocketEvents returns captured WebSocket events for auto-detect.
	AllWebSocketEvents() []types.WebSocketEvent

	// GetTrackingStatus returns (enabled, tabID, tabURL) for the tracked tab.
	GetTrackingStatus() (bool, int, string)

	// IsExtensionConnected reports whether the browser extension is connected.
	IsExtensionConnected() bool

	// GetPilotStatus returns the pilot status from capture.
	GetPilotStatus() any

	// ToolsList returns the list of MCP tools for capabilities introspection.
	ToolsList() []mcp.MCPTool

	// GetToolModuleExamples returns examples for a tool module by name, if available.
	GetToolModuleExamples(toolName string) any

	// GetSecurityMode returns (mode, productionParity, rewrites) from the capture subsystem.
	GetSecurityMode() (string, bool, []string)

	// SetSecurityMode sets the security mode on the capture subsystem.
	SetSecurityMode(mode string, rewrites []string)

	// GetTelemetryMode returns the current telemetry mode.
	GetTelemetryMode() string

	// SetTelemetryMode sets the telemetry mode.
	SetTelemetryMode(mode string)

	// InteractActionSetJitter sets the action jitter in milliseconds.
	InteractActionSetJitter(ms int)

	// InteractActionGetJitter returns the current action jitter in milliseconds.
	InteractActionGetJitter() int

	// HasCapture reports whether the capture subsystem is initialized.
	HasCapture() bool
}
