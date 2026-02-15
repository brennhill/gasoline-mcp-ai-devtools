// upload_security_windows.go — Hard link detection stub for Windows.
//go:build windows

package main

import "os"

// checkHardlink is a no-op on Windows where Nlink detection is not reliably available.
func checkHardlink(_ os.FileInfo) error {
	return nil
}
