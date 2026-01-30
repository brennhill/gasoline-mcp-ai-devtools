# Gasoline — Project Overview

Browser extension + MCP server: captures real-time browser telemetry (logs, network, WebSocket, DOM) for AI coding assistants via Model Context Protocol.

**Stack:** Go server (zero deps) + Chrome Extension (MV3, vanilla JS) + MCP (JSON-RPC 2.0 over stdio) + NPM distribution

## Essential Links

**Start Here (Every Session):**
- [.claude/docs/STARTUP-CHECKLIST.md](.claude/docs/STARTUP-CHECKLIST.md) — Essential rules for this session
- [.claude/docs/typescript-workflow.md](.claude/docs/typescript-workflow.md) — TypeScript compilation workflow

**Detailed References (On-Demand):**
- [.claude/docs/spec-standards.md](.claude/docs/spec-standards.md) — How to write feature specs
- [.claude/docs/spec-review.md](.claude/docs/spec-review.md) — Spec review process
- [.claude/docs/testing.md](.claude/docs/testing.md) — TDD workflow
- [.claude/docs/javascript-typescript-rules.md](.claude/docs/javascript-typescript-rules.md) — TypeScript/JS essentials
- [.claude/refs/javascript-typescript-rules-detailed.md](.claude/refs/javascript-typescript-rules-detailed.md) — Complete rules reference
- [.claude/refs/typescript-workflow-detailed.md](.claude/refs/typescript-workflow-detailed.md) — Full workflow details

**Architecture & Policy:**
- [.claude/refs/architecture.md](.claude/refs/architecture.md) — System architecture (5-tool constraint, data flows)
- [.claude/docs/git-and-concurrency.md](.claude/docs/git-and-concurrency.md) — Git branching, releases

**Testing & Examples:**
- [.claude/refs/testing-examples.md](.claude/refs/testing-examples.md) — TDD examples
- [.claude/refs/spec-templates.md](.claude/refs/spec-templates.md) — Spec templates

**UAT:**
- [docs/core/UAT-TEST-PLAN.md](docs/core/UAT-TEST-PLAN.md) — UAT test checklist

## Commands

```bash
make compile-ts                        # Compile TypeScript (REQUIRED after src/ changes)
make test                              # All tests (Go + extension)
make dev                               # Build for current platform
.claude/check-token-budget.sh          # Check doc token budget
```

## Core Rules (Brief)

1. **Spec Review** — Feature specs need principal engineer review
2. **TDD** — Tests first, then implementation
3. **TypeScript** — Compile immediately: `make compile-ts` after any `src/` change
4. **Zero Deps** — Production runtime (Go + extension) has no external dependencies
5. **No Remote Code** — All code must be bundled locally (Chrome Web Store requirement)
6. **5-Tool Constraint** — Exactly 5 MCP tools (observe, generate, configure, interact, analyze)
7. **Performance** — Extension must not degrade browsing (WS < 0.1ms, fetch < 0.5ms)
8. **Privacy** — Sensitive data stays on localhost
9. **Quality Gates** — All must pass: compile + lint + test + smoke test

## Git Workflow

- **`main`** — stable releases only
- **`next`** — active development (default branch)
- **`.claude/`** — independent subrepo (DO NOT delete/recreate)
- **Pre-push to `next`:** Use `/squash` to combine commits

See [.claude/docs/git-and-concurrency.md](.claude/docs/git-and-concurrency.md) for details.