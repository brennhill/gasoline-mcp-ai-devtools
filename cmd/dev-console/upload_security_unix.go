//go:build !windows
// +build !windows

// Purpose: Unix-specific upload security aliases — symlink resolution and ownership checks via internal/upload.
// Why: Prevents symlink traversal attacks on Unix by validating real paths before file operations.
// Docs: docs/features/feature/file-upload/index.md

package main

import (
	"os"

	"github.com/dev-console/dev-console/internal/upload"
)

func checkHardlink(info os.FileInfo) error {
	return upload.CheckHardlink(info)
}
