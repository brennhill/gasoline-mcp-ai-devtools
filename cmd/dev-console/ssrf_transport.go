// ssrf_transport.go — SSRF protection aliases delegating to internal/upload.
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
