// Purpose: Implements redaction rules for sensitive data in captured telemetry.
// Why: Reduces secret leakage risk in logs, diagnostics, and captured payloads.
// Docs: docs/features/feature/redaction-patterns/index.md

// redaction.go — Configurable redaction patterns for MCP tool responses.
// Scrubs sensitive data from tool responses before they reach the AI client.
// Uses RE2 regex (Go's regexp package) for guaranteed linear-time matching.
// Thread-safe: the engine is initialized once at startup and reused across requests.
//
// Layout:
// - redaction_types.go: shared data models and engine struct
// - redaction_builtin_patterns.go: built-in detection patterns
// - redaction_engine.go: engine construction and text/JSON redaction
// - redaction_keys.go: sensitive-key normalization and matching
// - redaction_map.go: recursive structured-value redaction
// - redaction_luhn.go: credit-card validation helper
package redaction
