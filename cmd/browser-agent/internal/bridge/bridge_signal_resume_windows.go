//go:build windows
// +build windows

// bridge_signal_resume_windows.go -- No-op Windows stub for daemon resume signaling since Windows lacks SIGCONT.
// Why: Provides a compile-time shim so bridge signal resume works cross-platform.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import "os"

func signalResumeProcess(_ *os.Process) {}
