//go:build !windows
// +build !windows

// Purpose: Implements upload command handling, validation, and OS automation wiring.
// Why: Reduces upload flake by centralizing validation and secure browser-to-OS handoff behavior.
// Docs: docs/features/feature/file-upload/index.md

package main

import (
	"os"

	"github.com/dev-console/dev-console/internal/upload"
)

func checkHardlink(info os.FileInfo) error {
	return upload.CheckHardlink(info)
}
