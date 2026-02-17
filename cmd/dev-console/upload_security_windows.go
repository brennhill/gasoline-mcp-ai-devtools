// Purpose: Owns upload_security_windows.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// upload_security_windows.go — Hard link detection stub for Windows.
//go:build windows

package main

import "os"

// checkHardlink is a no-op on Windows where Nlink detection is not reliably available.
func checkHardlink(_ os.FileInfo) error {
	return nil
}
