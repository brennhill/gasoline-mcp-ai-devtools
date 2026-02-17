---
doc_type: architecture
status: active
scope: features/core/target-architecture
ai-priority: high
tags: [core, architecture, target, canonical]
relates-to: [../../core/product-spec.md, ../../core/tech-spec.md, ../../core/qa-spec.md]
last-verified: 2026-02-17
canonical: true
---

# Target Architecture (Canonical Pointers)

This page is the architecture pointer for target behavior docs.

Canonical documents:
- Product contract: `docs/core/product-spec.md`
- Technical architecture and flows: `docs/core/tech-spec.md`
- QA contract and release gates: `docs/core/qa-spec.md`
- Command/option traceability: `docs/core/mcp-command-option-matrix.md`

Implementation anchors:
- MCP handler: `cmd/dev-console/handler.go`
- Tool schemas: `cmd/dev-console/tools_schema.go`
- Tool dispatch: `cmd/dev-console/tools_core.go`
- Query lifecycle: `internal/capture/queries.go`
- Unified sync endpoint: `internal/capture/sync.go`
- Extension sync executor: `src/background/sync-client.ts`, `src/background/pending-queries.ts`
