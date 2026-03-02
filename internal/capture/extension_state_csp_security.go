package capture

// GetCSPStatus returns the last reported CSP restriction level for the tracked page.
func (c *Capture) GetCSPStatus() (restricted bool, level string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.cspRestricted, c.extensionState.cspLevel
}

// CSPBlockedActions returns the actions blocked by the given CSP level and a
// human-readable reason string. When the level is "none" or unrecognized, both
// return values are nil/"" — callers should omit them from the response entirely
// to avoid wasting tokens on normal pages.
func CSPBlockedActions(level string) (actions []string, reason string) {
	switch level {
	case "script_exec":
		return []string{"execute_js"},
			"Page CSP blocks dynamic script execution. Use dom, get_readable, or list_interactive instead."
	case "page_blocked":
		return []string{
				"execute_js", "click", "type", "select", "check", "scroll_to", "focus",
				"get_text", "get_value", "get_attribute", "set_attribute",
				"list_interactive", "get_readable", "get_markdown",
				"fill_form", "fill_form_and_submit",
			},
			"Page blocks all script injection. Only navigate, screenshot, and network observation available."
	default:
		return nil, ""
	}
}

// SetSecurityMode updates altered-environment mode reported to callers.
// mode values: normal (default), insecure_proxy.
//
// Invariants:
// - Any non-insecure mode value normalizes to SecurityModeNormal.
// - Rewrite slice is copied on write to avoid external aliasing.
func (c *Capture) SetSecurityMode(mode string, rewrites []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch mode {
	case SecurityModeInsecureProxy:
		c.extensionState.securityMode = SecurityModeInsecureProxy
		c.extensionState.insecureRewrites = append([]string(nil), rewrites...)
	default:
		c.extensionState.securityMode = SecurityModeNormal
		c.extensionState.insecureRewrites = nil
	}
}

// GetSecurityMode returns current altered-environment mode and rewrite set.
// production_parity is true only in normal mode.
//
// Invariants:
// - Returned rewrite slice is copied and safe for caller mutation.
func (c *Capture) GetSecurityMode() (mode string, productionParity bool, rewrites []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	mode = c.extensionState.securityMode
	if mode == "" {
		mode = SecurityModeNormal
	}
	productionParity = mode == SecurityModeNormal
	rewrites = append([]string(nil), c.extensionState.insecureRewrites...)
	return mode, productionParity, rewrites
}
