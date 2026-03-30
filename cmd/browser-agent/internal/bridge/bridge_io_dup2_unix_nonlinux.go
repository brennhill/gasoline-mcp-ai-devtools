//go:build !linux && !windows
// +build !linux,!windows

// bridge_io_dup2_unix_nonlinux.go -- Provides macOS/BSD dup2 syscall wrapper for file descriptor duplication in bridge IO isolation.
// Why: Non-Linux Unix platforms use the standard dup2 syscall rather than Linux dup3.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import "syscall"

func dup2Compat(oldfd, newfd int) error {
	if oldfd == newfd {
		return nil
	}
	return syscall.Dup2(oldfd, newfd)
}
