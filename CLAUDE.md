# Gasoline — Project Instructions

Browser extension + MCP server: captures real-time browser telemetry (logs, network, WebSocket, DOM) for AI coding assistants via Model Context Protocol.

**Stack:** Go server (zero deps) + Chrome Extension (MV3, vanilla JS) + MCP (JSON-RPC 2.0 over stdio) + NPM distribution

## Commands

```bash
make compile-ts                        # REQUIRED: Compile TypeScript before ANY extension changes
make test                              # Go tests (full suite)
go test -short ./cmd/dev-console/      # Go tests (fast iteration, skips slow)
go vet ./cmd/dev-console/              # Static analysis
golangci-lint run ./cmd/dev-console/   # Code quality checks (complexity, maintainability, etc)
node --test tests/extension/*.test.js  # Extension tests
make dev                               # Build current platform
.claude/check-token-budget.sh          # Check doc token budget (keep < 1000 lines)
```

## Rules

1. **Spec Review** — MANDATORY: Every feature spec must be reviewed by a principal engineer agent before implementation. See [spec-review.md](.claude/docs/spec-review.md)
2. **TDD** — Write tests FIRST. Read spec → tests → confirm fail → implement → confirm pass → commit
3. **TypeScript Compilation** — CRITICAL: After ANY change to `src/**/*.ts`, you MUST run `make compile-ts` and verify it succeeds. NEVER commit without compilation. See [typescript-workflow.md](.claude/docs/typescript-workflow.md)
4. **Zero deps** — Production runtime only: Go server uses stdlib only; Extension uses no frameworks. Dev tooling (test runners, linters, code generators) may use external packages.
5. **No remote code** — Chrome Web Store PROHIBITS loading remotely hosted code. All third-party libraries (e.g., axe-core) MUST be bundled locally in the extension package. NEVER load scripts from CDNs or external URLs
6. **5-Tool Maximum** — Gasoline exposes exactly 5 MCP tools: `observe`, `analyze`, `generate`, `configure`, `interact`. Creating a 6th tool requires architecture review. New features MUST be added as a mode/action under one of these 5. See [architecture.md](.claude/docs/architecture.md)
7. **Performance** — Extension must not degrade browsing. WS < 0.1ms, fetch < 0.5ms, never block main thread
8. **Privacy** — Sensitive data never leaves localhost. Strip auth headers, body capture opt-in
9. **Quality gates** — `make compile-ts` + `go vet` + `make test` + `node --test` + smoke test must ALL pass before every commit

## Pre-Commit Checklist

BEFORE EVERY COMMIT involving extension code (`src/` changes), you MUST:

- [ ] **Run `make compile-ts`** - Verify TypeScript compiles without errors
- [ ] **Check compilation output** - Verify `extension/background/index.js` exists and is recent
- [ ] **Run `make test`** - All Go tests pass
- [ ] **Run `node --test tests/extension/*.test.js`** - All extension tests pass
- [ ] **Smoke test** - If you modified TypeScript, reload extension in Chrome and check console for errors
- [ ] **Verify manifest** - If you changed background entry point, verify `manifest.json` points to correct file

**If ANY step fails, DO NOT COMMIT. Fix the issue first.**

## Git

- **`main`** — stable releases only. **`next`** — active development
- **Subrepos** (DO NOT delete/recreate): `.claude/` is an independent repo
- **Marketing site**: Separate repo at `~/dev/gasoline-site` (Astro, blog posts in `src/content/docs/blog/`)
- Before push to `next`: `/squash` to combine commits

## Docs

See `.claude/docs/` for:
- **Spec Standards** — Where specs live, what they must contain. See [spec-standards.md](.claude/docs/spec-standards.md) — **START HERE** before creating any spec
- **Spec Review** — MANDATORY principal engineer review before implementation
- **Architecture** — 5-tool constraint, data flows, security, concurrency
- **Development** — TDD workflow, testing, code style
- **Git & Releases** — Branch model, worktrees, parallel agents, version sync, release process
- **Product** — Feature evaluation: capture vs interpret

## UAT

**FOR UAT, USE THIS CHECKLIST:** [docs/core/UAT-TEST-PLAN.md](docs/core/UAT-TEST-PLAN.md)
- Use this whenever the user asks for doing UAT or asking for next steps in doing UAT.