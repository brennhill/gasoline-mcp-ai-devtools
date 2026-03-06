// Purpose: Implements CSP origin accumulation and policy generation from observed runtime resource usage.
// Why: Produces enforceable security policies grounded in real traffic instead of static guesswork.
// Docs: docs/features/feature/security-hardening/index.md
//
// Layout:
// - csp_types.go: core models and constants
// - csp_store.go: observation accumulation and bounded storage
// - csp_generate.go: policy generation pipeline
// - csp_tooling.go: MCP/tool-facing adapters and override hooks
package security
