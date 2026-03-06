// Purpose: Re-exports SSRF-safe IP validation from internal/upload.
// Why: Provides package-level alias so upload tests can use SSRF protection without direct internal imports.
// Docs: docs/features/feature/security-hardening/index.md

package main

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/upload"

var isPrivateIP = upload.IsPrivateIP
