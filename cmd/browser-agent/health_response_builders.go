// Purpose: Thin adapter bridging the health sub-package to main-package globals.
// Why: Keeps backward compatibility for callers while implementation lives in the health sub-package.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// serverDepsAdapter implements health.ServerDeps for the main package's *Server.
type serverDepsAdapter struct {
	s *Server
}

func (a *serverDepsAdapter) GetTerminalPort() int {
	if a.s == nil {
		return 0
	}
	return a.s.getTerminalPort()
}

func (a *serverDepsAdapter) GetConsoleStats() (int, int, int64) {
	if a.s == nil || a.s.logs == nil {
		return 0, defaultMaxEntries, 0
	}
	a.s.logs.mu.RLock()
	entries := len(a.s.logs.entries)
	a.s.logs.mu.RUnlock()
	return entries, a.s.logs.maxEntries, a.s.logs.getLogDropCount()
}

// defaultMaxEntries mirrors the health package's default for nil servers.
// Also used by config.go for the --max-entries flag default.
const defaultMaxEntries = 1000

// getHealthResponse computes health metrics, injecting main-package globals.
func getHealthResponse(hm *health.Metrics, cap *capture.Store, server *Server, ver string) health.MCPHealthResponse {
	var serverDeps health.ServerDeps
	if server != nil {
		serverDeps = &serverDepsAdapter{s: server}
	}
	var upgrade health.UpgradeProvider
	if binaryUpgradeState != nil {
		upgrade = binaryUpgradeState
	}
	return hm.GetHealth(cap, serverDeps, upgrade, getLaunchModeInfo, ver)
}

// getLaunchModeInfo adapts getCurrentLaunchMode to health.LaunchModeInfo.
func getLaunchModeInfo() health.LaunchModeInfo {
	lm := getCurrentLaunchMode()
	return health.LaunchModeInfo{
		Mode:          lm.Mode,
		Reason:        lm.Reason,
		ParentProcess: lm.ParentProcess,
	}
}

// buildUpgradeInfo delegates to the health sub-package.
func buildUpgradeInfo() *health.UpgradeInfo {
	if binaryUpgradeState == nil {
		return nil
	}
	return health.BuildUpgradeInfo(binaryUpgradeState)
}

