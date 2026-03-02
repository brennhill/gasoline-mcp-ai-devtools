package capture

// Close shuts down capture-owned background goroutines.
//
// Failure semantics:
// - Idempotent for query cleanup lifecycle; no panic on repeated calls.
// - Does not clear in-memory buffers.
func (c *Capture) Close() {
	if c.queryDispatcher != nil {
		c.queryDispatcher.Close()
	}
}

// SetNavigationCallback sets a callback function that fires after a navigation
// action is ingested. The callback is invoked outside of the Capture lock in a
// separate goroutine (via util.SafeGo) so it is safe to call Capture methods.
// Used for automatic noise detection after page navigations.
func (c *Capture) SetNavigationCallback(cb func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.navigationCallback = cb
}

// SetLifecycleCallback sets a callback function for lifecycle events.
// The callback receives an event name and data map with event-specific fields.
// Events: "circuit_opened", "circuit_closed", "extension_connected", "extension_disconnected",
// "buffer_eviction", "rate_limit_triggered"
func (c *Capture) SetLifecycleCallback(cb func(event string, data map[string]any)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lifecycleCallback = cb
}

// AddLifecycleCallback appends a callback to the lifecycle event chain.
// Unlike SetLifecycleCallback, this preserves any previously registered callback
// and calls both in order. Thread-safe.
func (c *Capture) AddLifecycleCallback(cb func(event string, data map[string]any)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	existing := c.lifecycleCallback
	if existing == nil {
		c.lifecycleCallback = cb
		return
	}
	c.lifecycleCallback = func(event string, data map[string]any) {
		existing(event, data)
		cb(event, data)
	}
}

// emitLifecycleEvent dispatches lifecycle callbacks outside lock-heavy paths.
//
// Invariants:
// - Callback pointer is captured under c.mu and invoked after unlock.
//
// Failure semantics:
// - Missing callback is a silent no-op.
func (c *Capture) emitLifecycleEvent(event string, data map[string]any) {
	c.mu.RLock()
	cb := c.lifecycleCallback
	c.mu.RUnlock()
	if cb != nil {
		cb(event, data)
	}
}

// SetServerVersion sets server version for compatibility checking.
// Called once at startup with version from main.go.
func (c *Capture) SetServerVersion(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverVersion = v
}

// GetServerVersion returns server version.
func (c *Capture) GetServerVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverVersion
}
