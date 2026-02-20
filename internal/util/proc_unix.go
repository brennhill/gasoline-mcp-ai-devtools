// proc_unix.go — Unix-specific detached process configuration.

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
