//go:build windows
// +build windows

// Purpose: Implements upload validation, security checks, and automation support paths.
// Why: Enforces upload safety boundaries against path traversal and SSRF-style abuse.
// Docs: docs/features/feature/file-upload/index.md

package upload

import "os"

// CheckHardlink is a no-op on Windows where Nlink detection is not reliably available.
func CheckHardlink(_ os.FileInfo) error {
	return nil
}
