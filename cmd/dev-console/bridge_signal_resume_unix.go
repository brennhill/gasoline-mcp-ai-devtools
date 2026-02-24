// Purpose: Implements bridge transport lifecycle, forwarding, and reconnect behavior.
// Why: Keeps client tool calls resilient across daemon restarts and transport disruptions.
// Docs: docs/features/feature/bridge-restart/index.md

package main

import (
	"os"
	"syscall"
)

func signalResumeProcess(p *os.Process) {
	_ = p.Signal(syscall.SIGCONT)
}
