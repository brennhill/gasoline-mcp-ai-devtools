// Purpose: Re-exports SSRF-safe HTTP transport, dial context, and IP validation from internal/upload.
// Why: Provides package-level aliases so upload handlers can use SSRF protection without direct internal imports.
// Docs: docs/features/feature/security-hardening/index.md

package main

import "github.com/dev-console/dev-console/internal/upload"

var (
	resolvePublicIP      = upload.ResolvePublicIP
	ssrfSafeDialContext  = upload.SSRFSafeDialContext
	newSSRFSafeTransport = upload.NewSSRFSafeTransport
	isPrivateIP          = upload.IsPrivateIP
	isSSRFAllowedHost    = upload.IsSSRFAllowedHost
)

const ssrfLookupTimeout = upload.SSRFLookupTimeout
