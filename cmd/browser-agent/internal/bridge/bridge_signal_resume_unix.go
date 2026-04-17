//go:build !windows
// +build !windows

// bridge_signal_resume_unix.go -- Sends SIGCONT to a suspended daemon process on Unix to resume it after bridge reconnection.
// Why: Allows the bridge to wake a stopped daemon without requiring a full restart.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"os"
	"syscall"
)

func signalResumeProcess(p *os.Process) {
	_ = p.Signal(syscall.SIGCONT)
}
