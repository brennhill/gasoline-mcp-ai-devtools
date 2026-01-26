# Gasoline — Project Instructions

Browser extension + MCP server: captures real-time browser telemetry (logs, network, WebSocket, DOM) for AI coding assistants via Model Context Protocol.

**Stack:** Go server (zero deps) + Chrome Extension (MV3, vanilla JS) + MCP (JSON-RPC 2.0 over stdio) + NPM distribution

## Commands

```bash
make test                              # Go tests
go vet ./cmd/dev-console/              # Static analysis
node --test extension-tests/*.test.js  # Extension tests
make dev                               # Build current platform
```

## Rules

1. **Spec Review** — MANDATORY: Every feature spec must be reviewed by a principal engineer agent before implementation. See [spec-review.md](.claude/docs/spec-review.md)
2. **TDD** — Write tests FIRST. Read spec → tests → confirm fail → implement → confirm pass → commit
3. **Zero deps** — Go server: stdlib only. Extension: no frameworks, no build tools
4. **Performance** — Extension must not degrade browsing. WS < 0.1ms, fetch < 0.5ms, never block main thread
5. **Privacy** — Sensitive data never leaves localhost. Strip auth headers, body capture opt-in
6. **Quality gates** — `go vet` + `make test` + `node --test` must pass before every commit

## Git

- **`main`** — stable releases only. **`next`** — active development
- **Subrepos** (DO NOT delete/recreate): `.claude/` and `docs/marketing/` are independent repos
- Before push to `next`: `/squash` to combine commits

## Docs

| Document | Topic |
|----------|-------|
| [spec-review.md](.claude/docs/spec-review.md) | **MANDATORY** spec review process before implementation |
| [testing.md](.claude/docs/testing.md) | TDD workflow, test requirements, quality gates |
| [architecture.md](.claude/docs/architecture.md) | System diagram, data flows, memory, security |
| [code-style.md](.claude/docs/code-style.md) | File headers, comment hygiene, Go/JS patterns |
| [version-management.md](.claude/docs/version-management.md) | Version sync across 14 locations |
| [branching.md](.claude/docs/branching.md) | Branch model, parallel agents, worktrees |
| [product-philosophy.md](.claude/docs/product-philosophy.md) | Feature evaluation: capture vs interpret |
