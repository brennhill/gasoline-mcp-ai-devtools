---
feature: analyze-tool
status: shipped
version: v7.0
doc_type: tech-spec
feature_id: feature-analyze-tool
last_reviewed: 2026-02-17
---

# Analyze Tech Spec (TARGET)

## Dispatcher and Handlers
- Primary dispatch map: `analyzeHandlers` in `cmd/dev-console/tools_analyze.go`
- Annotation/session handlers: `cmd/dev-console/tools_analyze_annotations.go`

## Query-Type Mapping
- `dom` -> pending query type `dom`
- `accessibility` -> pending query type `a11y`
- `page_summary` -> pending query type `execute`
- `link_health` -> pending query type `link_health`

## Server-Only Flows
- `link_validation` uses server-side HTTP verification with SSRF-safe transport.
- `api_validation` runs incremental contract analysis over captured network bodies.
- `security_audit` and `third_party_audit` consume server capture buffers.

## Async/Synchronous Control
- Uses `maybeWaitForCommand` to implement sync-by-default behavior.
- Wait timeout is bounded; prolonged commands return `still_processing` and require polling.

## Annotation Flow
- Draw mode completion persists sessions.
- `annotations(wait=true)` uses waiter registration and command completion callbacks.
- `annotation_detail` resolves from annotation detail store.

## Code Anchors
- `cmd/dev-console/tools_analyze.go`
- `cmd/dev-console/tools_analyze_annotations.go`
- `cmd/dev-console/tools_security.go`
- `internal/capture/queries.go`
- `src/background/pending-queries.ts`
