// Purpose: Re-exports SSRF-safe IP validation from the uploadhandler sub-package.
// Why: Provides package-level alias so upload tests can use SSRF protection without direct internal imports.
// Docs: docs/features/feature/security-hardening/index.md

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

var isPrivateIP = uploadhandler.IsPrivateIP
