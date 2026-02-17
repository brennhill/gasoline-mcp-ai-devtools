// Purpose: Owns proc_unix.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

//go:build !windows

package util

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcess configures the command to run as a detached process
func SetDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
