package capture

// LogHTTPDebugEntry logs an HTTP debug entry. Delegates to DebugLogger (own lock).
func (c *Capture) LogHTTPDebugEntry(entry HTTPDebugEntry) {
	c.debug.LogHTTPDebugEntry(c.redactHTTPDebugEntry(entry))
}

// GetHTTPDebugLog returns a copy of the HTTP debug log. Delegates to DebugLogger (own lock).
func (c *Capture) GetHTTPDebugLog() []HTTPDebugEntry {
	return c.debug.GetHTTPDebugLog()
}
