# Claude Code Project Instructions

## Project Overview

**Gasoline** is a browser extension + MCP server that captures real-time browser logs, network activity, WebSocket events, and DOM state, making them available to AI coding assistants via the Model Context Protocol.

**Tech stack:** Go server (zero deps) + Chrome Extension (MV3, vanilla JS) + MCP (JSON-RPC 2.0 over stdio) + NPM distribution

## Git Subrepos (DO NOT DELETE)

| Path | Repository | Purpose |
|------|-----------|---------|
| `.claude/` | gasoline-claude | Claude Code skills, docs, and quality gates |
| `docs/marketing/` | gasoline-marketing | Marketing site (Jekyll) |

- NEVER delete or recreate these directories — they are independent repos with their own history
- To update: `cd` into the subrepo, commit, push to its own remote, then `git add <path> && git commit` in parent

## Quick Commands

```bash
make test                              # Go tests
make dev                               # Build current platform
go vet ./cmd/dev-console/              # Static analysis
node --test extension-tests/*.test.js  # Extension tests
make run                               # Run server
```

## Core Principles

1. **TDD is MANDATORY** — Write tests FIRST, then implementation
2. **Zero dependencies** — Server uses only Go stdlib; extension uses no frameworks
3. **Performance first** — Extension MUST NOT degrade browsing
4. **Privacy by default** — Sensitive data never leaves localhost
5. **Specification-driven** — Features derive from `docs/` specs

## TDD Workflow (NON-NEGOTIABLE)

**STOP if you find yourself writing implementation code without tests first.**

Read spec → Write tests → Confirm FAIL → Implement → Confirm PASS → Refactor → Commit

## Constraints

**Go server:** Zero deps, single binary, 5 platforms, concurrent-safe (sync.RWMutex), memory-bounded buffers, JSON-RPC 2.0

**Extension:** MV3 only, vanilla JS, zero page-load impact, WS handler < 0.1ms, memory cap 20MB soft / 50MB hard, never blocks main thread

## Testing

**Go:** `testing` package, `setupTestServer(t)`, table-driven, cover happy/error/edge/protocol
**JS:** `node:test` + `node:assert`, mock Chrome APIs via `globalThis.chrome`, test inject→content→background→server flow

## Autonomous Quality Checks

Run automatically without asking:
1. After Go changes: `go vet ./cmd/dev-console/` && `make test`
2. After JS changes: `node --test extension-tests/*.test.js`
3. Before commits: Verify tests pass
4. Before push to `next`: `/squash` to combine commits, tag, push

## Don't Do This

- Write implementation before tests
- Add external dependencies to the Go server
- Use Node.js build tools for the extension
- Block the main thread in inject.js
- Log sensitive data (auth tokens, cookies, passwords)
- Commit code without corresponding tests
- Delete or recreate git subrepos

## Do This

- Write tests FIRST (TDD) — ALWAYS
- Derive tests from specifications
- Keep the server zero-dependency
- Follow existing patterns in the codebase
- Run quality checks before committing

## Detailed Documentation

| Document | Topic |
|----------|-------|
| [testing.md](.claude/docs/testing.md) | TDD workflow, test requirements, quality gates |
| [architecture.md](.claude/docs/architecture.md) | System diagram, data flows, memory, security |
| [code-style.md](.claude/docs/code-style.md) | Code style, file headers, tech spec rules |
| [version-management.md](.claude/docs/version-management.md) | Version sync across 14 locations |
| [branching.md](.claude/docs/branching.md) | Branch model, parallel agents, worktrees |
