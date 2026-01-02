# Claude Code Project Instructions

## Project Overview

**Gasoline** is a browser extension + MCP server that captures real-time browser logs, network activity, WebSocket events, and DOM state, making them available to AI coding assistants via the Model Context Protocol.

**Tech stack:**
- **Server**: Go (zero dependencies, single binary)
- **Extension**: Chrome Extension (Manifest V3, vanilla JS)
- **Protocol**: MCP (JSON-RPC 2.0 over stdio)
- **Distribution**: Cross-platform NPM packages wrapping Go binaries

## Project Structure

```
cmd/dev-console/
├── main.go             # Server + MCP handler
├── main_test.go        # Server tests (v3)
├── v4.go               # v4 types and implementation
└── v4_test.go          # v4 tests

extension/
├── manifest.json       # Chrome Manifest V3
├── background.js       # Service worker (batching, polling)
├── content.js          # Content script (message bridge)
├── inject.js           # Page-injected script (capture logic)
├── popup.html/js       # Extension popup UI
└── options.html/js     # Settings page

extension-tests/
├── background.test.js  # Service worker tests
├── inject.test.js      # Inject script tests
├── websocket.test.js   # WebSocket capture tests (v4)
├── network-bodies.test.js  # Network body tests (v4)
└── on-demand.test.js   # DOM query + a11y tests (v4)

docs/
├── specification.md        # v3 technical spec
├── v4-specification.md     # v4 technical spec
└── product-description.md  # Product overview
```

## Quick Commands

```bash
# Go server
make test                 # Run Go tests
make dev                  # Build for current platform
make build                # Cross-platform build
go vet ./cmd/dev-console/ # Static analysis

# Extension tests
node --test extension-tests/*.test.js  # Run all extension tests

# Run specific test file
node --test extension-tests/websocket.test.js

# Build and run
make run                  # Run server locally
```

## Core Principles

1. **TDD is MANDATORY** - Write tests FIRST, then implementation (see `.claude/docs/tdd.md`)
2. **Zero dependencies** - Server uses only Go stdlib; extension uses no frameworks
3. **Performance first** - Extension MUST NOT degrade browsing (see v4-specification.md SLOs)
4. **Privacy by default** - Sensitive data never leaves localhost
5. **Specification-driven** - All features derive from `docs/` specs
6. **Forward-looking** - Never fall back to legacy patterns

## TDD Workflow (NON-NEGOTIABLE)

**STOP if you find yourself writing implementation code without tests first.**

```
1. Read specification (docs/specification.md or docs/v4-specification.md)
2. Write test cases from specification
3. Run tests → Confirm they FAIL (red)
4. Write implementation
5. Run tests → Confirm they PASS (green)
6. Refactor if needed
7. Commit with tests
```

Every function, endpoint, and capture handler MUST have tests written BEFORE implementation.

## Go Server Constraints

- Zero external dependencies (stdlib only)
- Single binary output
- Must run on darwin-arm64, darwin-x64, linux-arm64, linux-x64, windows-x64
- Concurrent-safe with sync.RWMutex
- Memory-bounded buffers (see v4-specification.md)
- JSON-RPC 2.0 for MCP protocol

## Extension Constraints

- Manifest V3 (no Manifest V2 patterns)
- Vanilla JavaScript (no build tools, no transpilation)
- Zero impact on page load (v4 intercepts deferred to after `load` event)
- WebSocket handler budget: < 0.1ms per message
- Memory cap: 20MB soft limit, 50MB hard limit
- Never blocks the main thread

## Testing

### Go Tests
- Use `testing` package (stdlib)
- Test helpers: `setupTestServer(t)`, `httptest.NewRecorder()`
- Table-driven tests where appropriate
- Cover: happy path, error cases, edge cases, protocol compliance

### Extension Tests
- Use `node:test` (built-in Node.js test runner)
- Use `node:assert` for assertions
- Mock Chrome APIs via `globalThis.chrome`
- Mock `window`, `document`, `crypto` as needed
- Test message flow: inject → content → background → server

## Code Style

- Go: standard `gofmt`, no linter config needed
- JS: ES modules, no semicolons optional (follow existing)
- Files: `kebab-case` for JS, Go convention for Go
- Functions: `camelCase` (JS), `PascalCase` exported / `camelCase` unexported (Go)

## Autonomous Quality Checks

Run these automatically without asking:

1. **After Go changes**: `go vet ./cmd/dev-console/`
2. **After Go changes**: `make test`
3. **After JS changes**: `node --test extension-tests/*.test.js`
4. **Before commits**: Verify tests pass

## Detailed Documentation

See `.claude/docs/` for detailed policies:

| Document | Topic |
|----------|-------|
| [tdd.md](.claude/docs/tdd.md) | Test-Driven Development workflow |
| [architecture.md](.claude/docs/architecture.md) | System architecture |
| [quality-gates.md](.claude/docs/quality-gates.md) | Quality gates and test standards |

## Key Files Reference

| File | Purpose |
|------|---------|
| `cmd/dev-console/main.go` | Server, HTTP routes, MCP handler |
| `cmd/dev-console/v4.go` | v4 types and implementations |
| `extension/inject.js` | Page capture (console, network, WS, DOM) |
| `extension/background.js` | Service worker (batching, server comm) |
| `extension/content.js` | Message bridge between inject and background |
| `docs/v4-specification.md` | v4 feature specification and SLOs |

## Version Management (KEEP IN SYNC)

When bumping the version, **ALL** of the following locations must be updated together. Missing any will cause build or publish failures.

| File | Field/Location |
|------|----------------|
| `extension/manifest.json` | `"version"` |
| `extension/inject.js` | `version:` in `window.__gasoline` object |
| `extension/background.js` | `version:` in debug export JSON |
| `extension/package.json` | `"version"` |
| `cmd/dev-console/main.go` | `version` constant |
| `server/package.json` | `"version"` |
| `npm/gasoline-cli/package.json` | `"version"` + all `optionalDependencies` versions |
| `npm/darwin-arm64/package.json` | `"version"` |
| `npm/darwin-x64/package.json` | `"version"` |
| `npm/linux-arm64/package.json` | `"version"` |
| `npm/linux-x64/package.json` | `"version"` |
| `npm/win32-x64/package.json` | `"version"` |
| `extension-tests/background.test.js` | Version assertion in debug export test |
| `README.md` | Version badge + `version` in Developer API table |

**Verification command:** `grep -r "OLD_VERSION" --include="*.json" --include="*.js" --include="*.go" --include="*.md" .` should return zero results after a bump.

## Branching & Parallel Agent Strategy

Multiple agents work on features in parallel using `git worktree`. Each agent gets its own directory and branch — no conflicts, no branch switching.

### Setup (run from main repo)

```bash
# Create a worktree for each feature
git worktree add ../gasoline-<feature-name> -b feature/<feature-name>

# Example: 4 agents in parallel
git worktree add ../gasoline-generate-test-v2 -b feature/generate-test-v2
git worktree add ../gasoline-circuit-breaker -b feature/circuit-breaker
git worktree add ../gasoline-memory-enforcement -b feature/memory-enforcement
git worktree add ../gasoline-a11y-caching -b feature/a11y-caching
```

### Rules for Each Agent

1. **One feature per branch** — Never commit multiple features to the same branch
2. **Branch from main** — All feature branches start from current `main`
3. **Own your files** — Avoid touching the same files as another parallel feature. If overlap is unavoidable, keep changes minimal and isolated to reduce merge conflicts
4. **TDD in your worktree** — Write tests, run them, implement, run again. All tests must pass before merge
5. **Commit frequently** — Small, focused commits with descriptive messages
6. **Don't touch main** — Only merge to main when the feature is complete and all tests pass

### File Ownership (Conflict Prevention)

When assigning parallel features, prefer features that don't overlap on primary files:

| File | Owner should be... |
|------|-------------------|
| `cmd/dev-console/v4.go` | Only one agent at a time (large file, high conflict risk) |
| `extension/inject.js` | Only one agent at a time (same reason) |
| `extension/background.js` | Can be shared if changes are in different functions |
| `extension-tests/*` | Each agent owns their test file |
| `e2e-tests/*` | Each agent owns their test file |
| `docs/*` | Low conflict risk, multiple agents OK |

If two features must touch `v4.go`, sequence them (finish one before starting the other) or isolate changes to clearly separate functions.

### Merging Back

```bash
# From main repo
git checkout main
git merge feature/<feature-name>

# If conflicts: resolve, run full test suite, then commit
make test
node --test extension-tests/*.test.js

# Cleanup
git worktree remove ../gasoline-<feature-name>
git branch -d feature/<feature-name>
```

### Feature Lifecycle

```
1. Spec exists in docs/ (e.g., docs/generate-test-v2.md)
2. Create worktree + branch
3. Agent reads spec → writes tests (TDD red)
4. Agent implements → tests pass (TDD green)
5. Agent runs full test suite in worktree
6. Merge to main, resolve any conflicts
7. Remove worktree
```

### Current Feature Roadmap

See `docs/v5-roadmap.md` for the prioritized list. Pick unchecked items.

## Don't Do This

- Write implementation before tests (**TDD VIOLATION**)
- Add external dependencies to the Go server
- Use Node.js build tools for the extension
- Block the main thread in inject.js
- Log sensitive data (auth tokens, cookies, passwords)
- Skip performance budget checks
- Commit code without corresponding tests

## Do This

- Write tests FIRST (TDD) - **ALWAYS**
- Derive tests from specifications
- Keep the server zero-dependency
- Follow existing patterns in the codebase
- Respect SLO budgets (see v4-specification.md)
- Run quality checks before committing
