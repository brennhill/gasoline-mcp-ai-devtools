// security_windows.go — Hard link detection stub for Windows.
//go:build windows

package upload

import "os"

// CheckHardlink is a no-op on Windows where Nlink detection is not reliably available.
func CheckHardlink(_ os.FileInfo) error {
	return nil
}
