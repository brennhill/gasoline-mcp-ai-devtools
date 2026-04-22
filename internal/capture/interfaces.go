// Purpose: Defines cross-package interfaces consumed by capture for schema, CSP, and client-registry integrations.
// Why: Decouples capture from concrete implementations to avoid import cycles and tighten test seams.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "encoding/json"

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
	// HandleGenerateCSP is the MCP tool handler for CSP generation.
	// params is a JSON-encoded security.CSPParams; returns *security.CSPResponse.
	HandleGenerateCSP(params json.RawMessage) (any, error)
}

// ClientRegistry defines the interface for managing connected MCP clients.
// Implemented by *session.ClientRegistry. Called by HTTP handlers.
// Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.
//
// Return types use any to avoid an import cycle (session imports capture):
//   - List() returns []session.ClientInfo
//   - Register() returns *session.ClientState
//   - Get() returns *session.ClientState (nil if not found)
//
// CONTRACT: any-returning methods MUST normalize absent/missing results to
// *untyped* nil, not a typed-nil pointer wrapped in an interface. A caller
// checking `if v == nil` on a typed-nil pointer (e.g., (*session.ClientState)(nil)
// passed through any) will get false and proceed as if the value were present.
// This produced a real bug where GET /clients/{id} returned `200 null` instead
// of 404. See cmd/browser-agent/client_registry_adapter_test.go for the
// regression test and cmd/browser-agent/client_registry_adapter.go for the
// reference implementation.
type ClientRegistry interface {
	// Count returns the number of registered clients.
	Count() int
	// List returns all registered clients as []session.ClientInfo.
	List() any
	// Register creates or updates a client registration, returning *session.ClientState.
	Register(cwd string) any
	// Get returns a specific client by ID as *session.ClientState, or UNTYPED nil
	// if not found — see the CONTRACT note on this interface.
	Get(id string) any
	// Unregister removes a client by ID and reports whether the client existed.
	Unregister(id string) bool
}
