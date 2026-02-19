// upload_security_windows.go — Hard link detection delegate for Windows.
//go:build windows

package main

import (
	"os"

	"github.com/dev-console/dev-console/internal/upload"
)

func checkHardlink(info os.FileInfo) error {
	return upload.CheckHardlink(info)
}
