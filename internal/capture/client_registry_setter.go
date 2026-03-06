// Purpose: Sets and updates the optional client registry dependency on Capture.
// Why: Lets daemon bootstrap wire a concrete registry without creating import cycles in capture constructors.
// Docs: docs/features/feature/request-session-correlation/index.md

package capture

// SetClientRegistry wires the client registry used by /clients endpoints.
// Safe to call at startup before serving requests.
func (c *Capture) SetClientRegistry(reg ClientRegistry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientRegistry = reg
}
