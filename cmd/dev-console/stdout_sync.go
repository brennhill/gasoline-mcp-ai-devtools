// Purpose: Owns stdout_sync.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

package main

import (
	"errors"
	"os"
	"syscall"
)

// syncStdoutBestEffort flushes stdout and only logs actionable sync failures.
// Sync on pipes/ptys can return EINVAL/EBADF even when output is already delivered.
func syncStdoutBestEffort() {
	if err := os.Stdout.Sync(); err != nil && !isIgnorableStdoutSyncError(err) {
		stderrf("[gasoline] warning: stdout.Sync failed: %v\n", err)
	}
}

func isIgnorableStdoutSyncError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EBADF) {
		return true
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, syscall.EINVAL) || errors.Is(pathErr.Err, syscall.EBADF)
	}
	return false
}
