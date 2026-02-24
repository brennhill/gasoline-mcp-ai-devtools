//go:build !linux && !windows
// +build !linux,!windows

// Purpose: Implements bridge transport lifecycle, forwarding, and reconnect behavior.
// Why: Keeps client tool calls resilient across daemon restarts and transport disruptions.
// Docs: docs/features/feature/bridge-restart/index.md

package main

import "syscall"

func dup2Compat(oldfd, newfd int) error {
	if oldfd == newfd {
		return nil
	}
	return syscall.Dup2(oldfd, newfd)
}
