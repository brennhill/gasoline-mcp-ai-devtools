// Purpose: Owns interfaces.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// interfaces.go — Abstracted component interfaces.
// These interfaces are implemented by other packages (analysis, security, session).
// They define the contracts for components that Capture depends on.
package capture

// SchemaStore defines the interface for API schema detection and tracking.
// Implemented by *analysis.SchemaStore. Methods called by HTTP handlers and observers.
// Has its own lock; safe to call outside Capture.mu.
type SchemaStore interface {
	// EndpointCount returns the number of unique endpoints observed
	EndpointCount() int
}

// CSPGenerator defines the interface for Content-Security-Policy generation.
// Implemented by *security.CSPGenerator. Called by HTTP handlers.
// Has its own lock; safe to call outside Capture.mu.
type CSPGenerator interface {
	// GenerateCSP produces a CSP policy from observed origins (stub or full).
	// Signature matches security.CSPGenerator.GenerateCSP(params any) any
	// For type safety in capture, callers will use type assertions.
}

// ClientRegistry defines the interface for managing connected MCP clients.
// Implemented by *session.ClientRegistry. Called by HTTP handlers.
// Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.
type ClientRegistry interface {
	// Count returns the number of registered clients
	Count() int
	// List returns all registered clients (returns []session.ClientInfo)
	List() any
	// Register creates a new client registration (returns *session.ClientState)
	Register(cwd string) any
	// Get returns a specific client by ID (returns *session.ClientState)
	Get(id string) any
}
