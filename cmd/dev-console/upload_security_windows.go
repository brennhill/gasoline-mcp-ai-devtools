//go:build windows
// +build windows

// Purpose: Windows-specific upload security aliases — path validation with Windows-native ownership semantics.
// Why: Provides platform-appropriate file security checks for upload path validation on Windows.
// Docs: docs/features/feature/file-upload/index.md

package main

import (
	"os"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/upload"
)

func checkHardlink(info os.FileInfo) error {
	return upload.CheckHardlink(info)
}
