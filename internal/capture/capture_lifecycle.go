// Purpose: Manages capture store lifecycle including shutdown and callback registration.
// Why: Separates lifecycle concerns (Close, SetNavigationCallback) from data ingestion and access.
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

// SetFeaturesCallback sets a callback for extension feature usage reports.
// Called from HandleSync when features_used is present. Invoked outside Capture lock.
func (c *Capture) SetFeaturesCallback(cb func(map[string]bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.featuresCallback = cb
}

// SubscribeLifecycle registers a typed lifecycle event listener and returns a
// subscription ID for later removal via UnsubscribeLifecycle.
// Thread-safe; the observer has its own lock independent of Capture.mu.
func (c *Capture) SubscribeLifecycle(fn LifecycleListener) int {
	return c.lifecycle.Subscribe(fn)
}

// UnsubscribeLifecycle removes a lifecycle listener by its subscription ID.
// No-op if the ID is not found.
func (c *Capture) UnsubscribeLifecycle(id int) {
	c.lifecycle.Unsubscribe(id)
}

// SetLifecycleCallback registers a string-based lifecycle callback.
// Backward-compatible: wraps the callback as a LifecycleListener on the observer.
// Note: does NOT clear previous listeners. Callers that need exclusive ownership
// should use SubscribeLifecycle/UnsubscribeLifecycle directly.
func (c *Capture) SetLifecycleCallback(cb func(event string, data map[string]any)) {
	c.lifecycle.Subscribe(func(event LifecycleEvent, data map[string]any) {
		cb(event.String(), data)
	})
}

// AddLifecycleCallback appends a string-based lifecycle callback.
// Backward-compatible: wraps the callback as a LifecycleListener on the observer.
// Deprecated: prefer SubscribeLifecycle for new code.
func (c *Capture) AddLifecycleCallback(cb func(event string, data map[string]any)) {
	c.lifecycle.Subscribe(func(event LifecycleEvent, data map[string]any) {
		cb(event.String(), data)
	})
}

// emitLifecycleEvent dispatches a lifecycle event via the observer.
// Backward-compatible bridge: converts string event name to typed event.
//
// Failure semantics:
// - No listeners is a silent no-op.
// - Individual listener panics are recovered (error isolation).
func (c *Capture) emitLifecycleEvent(event string, data map[string]any) {
	c.lifecycle.EmitString(event, data)
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
