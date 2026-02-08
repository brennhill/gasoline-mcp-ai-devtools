// types.go — Core capture types and the Capture struct.
// WebSocket events, network bodies, user actions, and the main Capture buffer.
// Design: Capture-specific types remain here; domain types moved to their packages.
package capture

// This file now serves as a package-level import hub.
// All types have been refactored into focused files:
// - interfaces.go: Abstracted component interfaces
// - type-aliases.go: Type aliases for imported packages
// - session-types.go: Session tracking types
// - security-types.go: Security threat flagging
// - network-types.go: Network waterfall and body types
// - websocket-types.go: WebSocket event and connection tracking types
// - extension-logging-types.go: Extension logging types
// - enhanced-actions-types.go: Enhanced actions types
// - internal-types.go: Internal types used by Capture struct
// - constants.go: Buffer capacity and configuration constants
// - buffer-types.go: Ring buffer types for Capture composition
// - capture-struct.go: Main Capture struct and factory function
