// bridge_adapter.go -- Wires the bridge sub-package to main-package dependencies.
// Purpose: Provides the dependency injection glue so the bridge package can call main-package functions.
// Why: Keeps the bridge package decoupled while allowing it to access logging, stdout, MCP identity, and daemon lifecycle helpers.

package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"

	bridgepkg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
)

// mcpStdoutMu serializes all writes to stdout so concurrent bridgeForwardRequest
// goroutines cannot interleave JSON-RPC responses.
var mcpStdoutMu sync.Mutex

// initBridge wires the bridge sub-package to main-package dependencies.
// Must be called before any bridge function is used.
func initBridge() {
	bridgepkg.Init(bridgepkg.Deps{
		Version:              version,
		MaxPostBodySize:      maxPostBodySize,
		MCPServerName:        mcpServerName,
		LegacyMCPServerNames: legacyMCPServerNames,
		ServerInstructions:   serverInstructions,

		// Logging
		Stderrf: stderrf,
		Debugf:  debugf,

		// Stdout transport
		WriteMCPPayload:      writeMCPPayload,
		SyncStdoutBestEffort: syncStdoutBestEffort,
		SetStderrSink: setStderrSink,

		// Push state
		GetBridgeFraming:      getBridgeFraming,
		StoreBridgeFraming:    storeBridgeFraming,
		SetPushClientCapabilities: func(caps push.ClientCapabilities) {
			setPushClientCapabilities(caps)
		},
		ExtractClientCapabilities: func(rawParams json.RawMessage) push.ClientCapabilities {
			return extractClientCapabilities(rawParams)
		},

		// MCP content
		NegotiateProtocolVersion: negotiateProtocolVersion,
		MCPResources: func() []mcp.MCPResource {
			return mcpResources()
		},
		MCPResourceTemplates: func() []any {
			return mcpResourceTemplates()
		},
		ResolveResourceContent: resolveResourceContent,

		// Daemon lifecycle
		DaemonProcessArgv0:  daemonProcessArgv0,
		StopServerForUpgrade: stopServerForUpgrade,
		FindProcessOnPort:    findProcessOnPort,
		IsProcessAlive:       isProcessAlive,
		VersionsMatch:        versionsMatch,
		DecodeHealthMetadata: func(body []byte) (bridgepkg.HealthMeta, bool) {
			meta, ok := decodeHealthMetadata(body)
			if !ok {
				return bridgepkg.HealthMeta{}, false
			}
			return bridgepkg.HealthMeta{
				Version:     meta.Version,
				ServiceName: meta.resolvedServiceName(),
			}, true
		},
		AppendExitDiagnostic: appendExitDiagnostic,
	})
}

// runBridgeMode delegates to the bridge sub-package.
func runBridgeMode(port int, logFile string, maxEntries int) {
	bridgepkg.RunMode(port, logFile, maxEntries)
}

// ensureBridgeIOIsolation delegates to the bridge sub-package.
func ensureBridgeIOIsolation(logFile string) error {
	return bridgepkg.EnsureIOIsolation(logFile)
}

// bridgeLaunchFingerprint delegates to the bridge sub-package.
func bridgeLaunchFingerprint() map[string]any {
	return bridgepkg.LaunchFingerprint()
}

// activeMCPTransportWriter delegates to the bridge sub-package.
func activeMCPTransportWriter() *os.File {
	return bridgepkg.ActiveMCPTransportWriter()
}

// isKaboomService delegates to the bridge sub-package.
func isKaboomService(name string) bool {
	return bridgepkg.IsKaboomService(name)
}

// flushStdout delegates to the bridge sub-package.
func flushStdout() {
	bridgepkg.FlushStdout()
}

// extractToolAction delegates to the bridge sub-package.
func extractToolAction(req JSONRPCRequest) (toolName, action string) {
	return bridgepkg.ExtractToolAction(req)
}

// -- Telemetry adapters for tests --

// resetFastPathCounters delegates to the bridge sub-package.
func resetFastPathCounters() {
	bridgepkg.ResetFastPathCounters()
}

// recordFastPathEvent delegates to the bridge sub-package.
func recordFastPathEvent(method string, success bool, errorCode int) {
	bridgepkg.RecordFastPathEvent(method, success, errorCode)
}

// recordFastPathResourceRead delegates to the bridge sub-package.
func recordFastPathResourceRead(uri string, success bool, errorCode int) {
	bridgepkg.RecordFastPathResourceRead(uri, success, errorCode)
}

// resetFastPathResourceReadCounters delegates to the bridge sub-package.
func resetFastPathResourceReadCounters() {
	bridgepkg.ResetFastPathResourceReadCounters()
}

// bridgeWrapperLogFileName is re-exported from the bridge sub-package.
const bridgeWrapperLogFileName = bridgepkg.BridgeWrapperLogFileName

// Note: writeMCPPayload and mcpStdoutMu remain in mcp_stdout.go.
// The bridge package calls deps.WriteMCPPayload which routes back to main's writeMCPPayload.

// isServerRunning delegates to the bridge sub-package.
func isServerRunning(port int) bool {
	return bridgepkg.IsServerRunning(port)
}

// waitForServer delegates to the bridge sub-package.
func waitForServer(port int, timeout time.Duration) bool {
	return bridgepkg.WaitForServer(port, timeout)
}

// snapshotFastPathResourceReadCounters delegates to the bridge sub-package.
func snapshotFastPathResourceReadCounters() (success int64, failure int64) {
	return bridgepkg.SnapshotFastPathResourceReadCounters()
}

// fastPathTelemetryLogPath delegates to the bridge sub-package.
func fastPathTelemetryLogPath() (string, error) {
	return bridgepkg.FastPathTelemetryLogPath()
}

// buildPushNotification delegates to the bridge sub-package.
func buildPushNotification(ev push.PushEvent) []byte {
	return bridgepkg.BuildPushNotification(ev)
}
