// security_unix.go — Hard link detection for Unix platforms.
//go:build !windows

package upload

import (
	"fmt"
	"os"
	"syscall"
)

// CheckHardlink returns an error if the file has multiple hard links, which could
// indicate a hardlink to a sensitive file that bypasses path-based security checks.
func CheckHardlink(info os.FileInfo) error {
	sys := info.Sys()
	if sys == nil {
		return nil
	}
	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if stat.Nlink > 1 {
		return fmt.Errorf("file has %d hard links — hardlinks to sensitive files are not allowed. Copy the file instead", stat.Nlink)
	}
	return nil
}
