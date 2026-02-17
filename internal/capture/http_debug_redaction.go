// Purpose: Owns http_debug_redaction.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// http_debug_redaction.go — Sensitive data redaction for HTTP debug log entries.
package capture

// redactHTTPDebugEntry scrubs sensitive data from HTTP debug entry fields before storage.
func (c *Capture) redactHTTPDebugEntry(entry HTTPDebugEntry) HTTPDebugEntry {
	if c.logRedactor == nil {
		return entry
	}

	if len(entry.Headers) > 0 {
		redactedHeaders := make(map[string]string, len(entry.Headers))
		for key, value := range entry.Headers {
			redactedHeaders[key] = c.logRedactor.Redact(value)
		}
		entry.Headers = redactedHeaders
	}

	if entry.RequestBody != "" {
		entry.RequestBody = c.logRedactor.Redact(entry.RequestBody)
	}
	if entry.ResponseBody != "" {
		entry.ResponseBody = c.logRedactor.Redact(entry.ResponseBody)
	}
	if entry.Error != "" {
		entry.Error = c.logRedactor.Redact(entry.Error)
	}

	return entry
}
