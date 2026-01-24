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
├── main.go             # HTTP server, routes, v3 MCP handler
├── main_test.go        # Server tests (v3)
├── types.go            # Types, constants, Capture struct
├── websocket.go        # WebSocket buffer, connections, MCP tools
├── network.go          # Network body storage, MCP tool
├── queries.go          # Pending queries, DOM/A11y, a11y cache
├── actions.go          # Enhanced actions buffer, MCP tool
├── performance.go      # Snapshots, baselines, regression detection
├── codegen.go          # Reproduction scripts, timeline, test gen
├── tools.go            # MCP tool dispatcher, schemas, memory/rate-limit
├── ai_checkpoint.go    # Checkpoint/diff system
├── rate_limit.go       # Rate limiting, circuit breaker, health endpoint
├── memory.go           # Memory enforcement, eviction, periodic checks
└── *_test.go           # Matching test files per domain

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
├── on-demand.test.js   # DOM query + a11y tests (v4)
├── rate-limit.test.js  # Rate limiting / circuit breaker tests
├── memory.test.js      # Memory enforcement tests
├── interception-deferral.test.js  # Phase 1/Phase 2 deferral tests
└── content.test.js     # Content script tests

docs/
├── specification.md        # v3 technical spec
├── v4-specification.md     # v4 technical spec
└── product-description.md  # Product overview
```

## Git Subrepos (DO NOT DELETE)

This project uses git submodules for shared configuration. These are **separate repositories** embedded in the project tree:

| Path | Repository | Purpose |
|------|-----------|---------|
| `.claude/` | gasoline-claude | Claude Code skills, docs, and quality gates |
| `docs/marketing/` | gasoline-marketing | Marketing site (Jekyll) |

**Rules:**
- NEVER delete or recreate these directories — they are independent repos with their own history
- To update: `cd` into the subrepo, commit, and push to its own remote
- After committing inside a subrepo, update the parent repo's submodule reference: `git add <path> && git commit`
- Both the subrepo push and the parent ref update are required for changes to propagate

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

### File Headers (Mandatory)

Every Go source file in `cmd/dev-console/` MUST have a descriptive header comment before `package main`. The header declares the file's purpose and key design decisions.

```go
// filename.go — One-line purpose summary.
// 1-2 sentences expanding on what the file contains and how it relates
// to the rest of the system.
// Design: Key architectural decisions (data structure choices, limits, patterns).
package main
```

**Requirements:**
- First line: `// filename.go — Purpose` (em-dash, not hyphen)
- 2-4 additional lines explaining scope and design decisions
- No version labels (v4, v5) or phase labels (Phase 1, Phase 2) in comments — these become stale
- Comments must describe current behavior, not historical development phases

### Comment Hygiene

- Comments must describe **current** behavior, not historical phases
- Never use internal version labels (v4, v5) or phase labels (Phase 1, Phase 2, Phase 3) in source comments
- Endpoint references in tests must match actual registered routes
- Section markers (`// ====`) should use descriptive names, not version-prefixed names

## Autonomous Quality Checks

Run these automatically without asking:

1. **After Go changes**: `go vet ./cmd/dev-console/`
2. **After Go changes**: `make test`
3. **After JS changes**: `node --test extension-tests/*.test.js`
4. **Before commits**: Verify tests pass
5. **Before push/merge to `next`**: Use `/squash` to combine all commits into one with a generated summary, then tag and push (see `.claude/docs/quality-gates.md` Gate 6)

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
| `cmd/dev-console/main.go` | HTTP server, routes, v3 MCP handler |
| `cmd/dev-console/types.go` | All types, constants, `Capture` struct |
| `cmd/dev-console/websocket.go` | WebSocket buffer, connection tracking |
| `cmd/dev-console/network.go` | Network body storage and retrieval |
| `cmd/dev-console/queries.go` | DOM/A11y queries, a11y cache |
| `cmd/dev-console/actions.go` | Enhanced actions buffer |
| `cmd/dev-console/performance.go` | Performance snapshots, baselines |
| `cmd/dev-console/codegen.go` | Playwright scripts, timeline, test gen |
| `cmd/dev-console/tools.go` | MCP tool dispatcher, schemas |
| `cmd/dev-console/ai_checkpoint.go` | Checkpoint/diff system |
| `extension/inject.js` | Page capture (console, network, WS, DOM) |
| `extension/background.js` | Service worker (batching, server comm) |
| `extension/content.js` | Message bridge between inject and background |
| `docs/v4-specification.md` | Feature specification and SLOs |

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

## Branching Strategy

### Branch Model: `main` + `next`

- **`main`** — Last stable release. What's published on npm and the Chrome Web Store. Only touched by release merges and hotfixes.
- **`next`** — Active development. All feature branches merge here. Always buildable, always tests passing.
- **Feature branches** — Branch from `next`, merge back to `next`.
- **Hotfixes** — Branch from `main`, merge to both `main` and `next`.

```
main    ─●───────────────────●────────── (releases only)
          │                   ↑
          │             merge + tag
          ↓                   │
next    ─●──●──●──●──●──●──●─● ──────── (integration)
             ↑  ↑        ↑
feature/a ───●  │        │
feature/b ──────●        │
feature/c ───────────────●
```

### Releasing

```bash
git checkout main
git merge next
git tag v0.X.0
# Bump versions (see Version Management section)
# Build + publish npm packages
# Upload extension to Chrome Web Store
git checkout next
git merge main  # Keep next in sync with the tag commit
```

### Hotfixes

```bash
git checkout -b hotfix/fix-name main
# Fix, test, commit
git checkout main && git merge hotfix/fix-name
git checkout next && git merge hotfix/fix-name
git branch -d hotfix/fix-name
```

### Parallel Agent Strategy

Multiple agents work on features in parallel using `git worktree`. Each agent gets its own directory and branch — no conflicts, no branch switching.

### Setup (run from main repo)

```bash
# Create a worktree for each feature (branch from next)
git worktree add ../gasoline-<feature-name> -b feature/<feature-name> next

# Example: 4 agents in parallel
git worktree add ../gasoline-feature-a -b feature/feature-a next
git worktree add ../gasoline-feature-b -b feature/feature-b next
git worktree add ../gasoline-feature-c -b feature/feature-c next
```

### Rules for Each Agent

1. **One feature per branch** — Never commit multiple features to the same branch
2. **Branch from `next`** — All feature branches start from current `next`
3. **Own your files** — Avoid touching the same files as another parallel feature. If overlap is unavoidable, keep changes minimal and isolated to reduce merge conflicts
4. **TDD in your worktree** — Write tests, run them, implement, run again. All tests must pass before merge
5. **Commit frequently** — Small, focused commits with descriptive messages
6. **Never touch `main`** — Only release merges touch `main`

### File Ownership (Conflict Prevention)

Each domain file can be worked on by one agent at a time:

| File | Parallel-safe? |
|------|---------------|
| `types.go` | Shared (append-only for new types) |
| `websocket.go` | One agent at a time |
| `network.go` | One agent at a time |
| `queries.go` | One agent at a time |
| `actions.go` | One agent at a time |
| `performance.go` | One agent at a time |
| `codegen.go` | One agent at a time |
| `tools.go` | Shared (dispatcher — add case + tool schema) |
| `ai_checkpoint.go` | One agent at a time |
| `extension/inject.js` | One agent at a time |
| `extension/background.js` | Can be shared if changes are in different functions |
| `extension-tests/*` | Each agent owns their test file |
| `docs/*` | Low conflict risk, multiple agents OK |

### Agent Lock Protocol (Advisory)

A `.agent-locks.json` file (gitignored) tracks which agent owns which file. Before modifying a domain file:

1. **Check locks**: Read `.agent-locks.json` to see if the file is locked
2. **Acquire lock**: Add an entry with your feature name, branch, and expiry (16h default)
3. **Release lock**: Remove your entry when done (or let it expire)

Lock entry format:
```json
{
  "file": "cmd/dev-console/websocket.go",
  "agent": "feature-name",
  "branch": "feature/ws-binary",
  "acquired": "2026-01-23T10:00:00Z",
  "expires": "2026-01-24T02:00:00Z"
}
```

This is advisory — agents should respect locks but the system won't block commits. The real protection comes from file-level isolation (each domain file is independent) and pre-commit hooks catching compilation errors.

### Merging Back

```bash
# From main repo — merge to next, not main
git checkout next
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
1. Spec exists in docs/ (e.g., docs/ai-first/feature-name.md)
2. Create worktree + branch from next
3. Agent reads spec → writes tests (TDD red)
4. Agent implements → tests pass (TDD green)
5. Agent runs full test suite in worktree
6. Merge to next, resolve any conflicts
7. Remove worktree
```

### Current Feature Roadmap

See `docs/v5-roadmap.md` for the prioritized list. Pick unchecked items.

## Technical Specifications (NON-NEGOTIABLE)

Tech specs MUST be written in natural language. They are human-readable documents that describe **what** the system does and **how** it behaves — not code dumps with comments.

**Rules:**
1. Describe behavior in plain English sentences and paragraphs
2. Use prose to explain data flow, decisions, and constraints
3. Code snippets are allowed ONLY as brief illustrative examples (a few lines max), never as the primary content
4. Structure with sections like: Purpose, How It Works, Data Model (described in words), Tool Interface, Behavior, Edge Cases, Performance Constraints, Test Scenarios
5. A non-engineer should be able to read a tech spec and understand what the feature does
6. The spec acts as a cross-reference for target behavior — if an implementer reads it, they know exactly what to build without needing to reverse-engineer code blocks

**Don't:** Paste full Go structs, function implementations, or test function signatures as the spec content.
**Do:** Write "The server maintains a map of checkpoint IDs to captured state. Each checkpoint stores the buffer positions at the time it was created, so diffs can be computed by comparing current positions against the checkpoint."

## Don't Do This

- Write implementation before tests (**TDD VIOLATION**)
- Add external dependencies to the Go server
- Use Node.js build tools for the extension
- Block the main thread in inject.js
- Log sensitive data (auth tokens, cookies, passwords)
- Skip performance budget checks
- Commit code without corresponding tests
- Delete or recreate git subrepos (`.claude/`, `docs/marketing/`) — update them in place

## Do This

- Write tests FIRST (TDD) - **ALWAYS**
- Derive tests from specifications
- Keep the server zero-dependency
- Follow existing patterns in the codebase
- Respect SLO budgets (see v4-specification.md)
- Run quality checks before committing
