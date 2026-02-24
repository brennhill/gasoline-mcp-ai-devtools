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
