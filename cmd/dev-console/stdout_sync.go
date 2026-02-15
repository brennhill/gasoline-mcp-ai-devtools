package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// syncStdoutBestEffort flushes stdout and only logs actionable sync failures.
// Sync on pipes/ptys can return EINVAL/EBADF even when output is already delivered.
func syncStdoutBestEffort() {
	if err := os.Stdout.Sync(); err != nil && !isIgnorableStdoutSyncError(err) {
		fmt.Fprintf(os.Stderr, "[gasoline] warning: stdout.Sync failed: %v\n", err)
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
