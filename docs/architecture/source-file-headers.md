---
doc_type: standard
status: active
last_reviewed: 2026-02-17
owners: []
---

# Source File Headers

All non-test implementation files under `src/`, `cmd/`, and `internal/` must include a short opening header with:

- `Purpose`: One sentence explaining what the file owns.
- `Docs`: One or more links to relevant feature docs in `docs/features/feature/...`.

## TypeScript Format

```ts
/**
 * Purpose: Handles pending extension queries and routes results.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 */
```

## Go Format

```go
// Purpose: Implements observe tool response shaping for captured buffers.
// Docs: docs/features/feature/observe/index.md
// Docs: docs/features/feature/pagination/index.md
package main
```

## Rules

- Header must appear near the top of file.
- `Purpose` and at least one `Docs` line are required.
- Link to feature index docs (`.../index.md`) instead of deep links when possible.
- Test files (`*_test.go`, `*.test.ts`) are excluded.
