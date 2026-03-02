// Purpose: Generates Subresource Integrity hashes and related metadata from observed script/style resources.
// Why: Enables integrity pinning workflows that reduce third-party tampering and supply-chain risk.
// Docs: docs/features/feature/security-hardening/index.md
//
// Layout:
// - sri_types.go: input/output and internal pipeline models
// - sri_generate.go: filtering and generation pipeline
// - sri_helpers.go: hashing/content-type/url/tag helpers
// - sri_tooling.go: MCP/tool adapter
package security
