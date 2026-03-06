// Purpose: Handles security scanner configuration loading, defaults, and policy persistence boundaries.
// Why: Ensures scanners run with explicit, auditable config rather than scattered implicit constants.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"os"
	"sync"
)

var (
	isMCPMode     bool
	isInteractive bool
	modeMu        sync.RWMutex
)

func InitMode() {
	modeMu.Lock()
	defer modeMu.Unlock()

	if os.Getenv("MCP_MODE") == "1" {
		isMCPMode = true
		isInteractive = false
		return
	}

	isMCPMode = false
	isInteractive = true
}

func IsMCPMode() bool {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return isMCPMode
}

func IsInteractiveTerminal() bool {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return isInteractive
}
