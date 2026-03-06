---
status: active
scope: process/release
ai-priority: high
tags: [release, process, quality-gates, deployment]
relates-to: [known-issues.md, docs/core/uat-v5.3-checklist.md]
last-verified: 2026-02-04
canonical: true
---

# Release Process

Gasoline MCP uses a `UNSTABLE` → `main` branching model with strict quality gates. Every release goes through automated and manual verification before reaching users.

## Branch Model

```
main    ─●───────────────────●────── (releases only)
          │                   ↑
          │             merge + tag
          ↓                   │
UNSTABLE ─●──●──●──●──●──●──●─● ──── (integration)
             ↑  ↑        ↑
feature/a ───●  │        │
feature/b ──────●        │
feature/c ───────────────●
```

- **`main`** — Published releases. What's on npm and the Chrome Web Store.
- **`UNSTABLE`** — Integration branch. All features merge here first.
- **Feature branches** — Branch from `UNSTABLE`, merge back to `UNSTABLE`.

## Quality Gates

Every feature must pass all gates before merging to `UNSTABLE`. All gates must be green before `UNSTABLE` merges to `main`.

### Gate 1: Tests Pass

```bash
make test                              # Go server tests
node --test tests/extension/*.test.js  # Extension tests
```

No code is merged with failing tests.

### Gate 2: Test Quality

Tests must:
- Import and test actual source code (no inline logic)
- Verify behavior, not mocks
- Cover edge cases, error paths, and boundaries
- Map to specification requirements in `docs/`

### Gate 3: Specification Coverage

Every requirement in the specification has corresponding tests:
- Buffer sizes, truncation limits, timeouts
- SLO targets with validation
- Protocol compliance (JSON-RPC 2.0, MCP)
- Error conditions (invalid input, overflow, timeout)

### Gate 4: Static Analysis

```bash
go vet ./cmd/dev-console/    # No warnings
make build                   # Cross-platform build succeeds
```

All platforms must build: darwin-arm64, darwin-x64, linux-arm64, linux-x64, windows-x64.

### Gate 5: Performance SLOs

| Metric | Target |
|--------|--------|
| `fetch()` wrapper overhead | < 0.5ms |
| WebSocket handler overhead | < 0.1ms per message |
| Page load impact | < 20ms |
| Server memory under load | < 30MB |
| MCP tool response time | < 200ms |

### Gate 6: Code Coverage

| Scope | Minimum |
|-------|---------|
| Overall (statements) | 95% |
| Per-file (statements) | 90% |

```bash
go test -coverprofile=coverage.out ./cmd/dev-console/
go tool cover -func=coverage.out | grep total
```

Coverage must not decrease between commits.

### Gate 7: Squash & Tag

Before pushing to `UNSTABLE`, all feature work is squashed into a single commit:

```bash
# Squash all commits since branching from UNSTABLE
/squash

# Tag for pre-UAT
git tag v{version}-pre-uat-{feature}

# Push
git push origin HEAD --follow-tags
```

### Gate 8: MCP Command Completeness (MANDATORY)

**This gate cannot be skipped.** Every command exposed via MCP MUST be fully implemented.

**Rule:** If an MCP tool/command is advertised in the tool schema, it MUST:

1. Be fully functional with all documented parameters working
2. Return proper results (not stubs, placeholders, or "not implemented" errors)
3. Have corresponding tests verifying the implementation
4. Have documentation matching the actual behavior

**If a command is not fully implemented:**

1. Remove it from the MCP tool definitions (do not expose it to clients)
2. Add a TODO in the code marking it for future implementation
3. Track in `docs/core/known-issues.md` under "Planned Features"

**Verification:**

```bash
# Review all MCP tool definitions
grep -r "tools\|inputSchema" cmd/dev-console/tools_*.go

# Ensure no stub implementations
grep -rn "TODO\|FIXME\|not implemented" cmd/dev-console/tools_*.go

# Cross-reference with test coverage
go test -v ./cmd/dev-console/ | grep -E "^--- (PASS|FAIL)"
```

**Why this matters:** Clients (Claude Code, IDEs, automation) rely on MCP tool schemas to understand capabilities. Advertising unimplemented commands breaks client expectations and causes confusing errors.

### Gate 9: Architecture Invariant Tests (MANDATORY)

**This gate cannot be skipped.** Critical architecture invariants must be verified before every release.

#### 9.1 MCP Stdio Silence

The server MUST NOT output anything to stdio except JSON-RPC messages. Any non-JSON-RPC output breaks LLM communication.

```bash
go test ./cmd/dev-console -run "TestToolHandler.*Stdout" -v
go test ./cmd/dev-console -run "TestStdioSilence" -v
```

See: `.claude/refs/mcp-stdio-invariant.md`

#### 9.2 Server Persistence

The HTTP server MUST stay alive as long as stdin remains open. This ensures browser extension connectivity throughout the MCP session.

```bash
go test ./cmd/dev-console -run "TestServerPersistence" -v
```

**Key invariants tested:**

- Server survives 10+ seconds with open stdin (no data)
- Health endpoint responds within 100ms at all times
- Server survives stdin close (waits for SIGTERM)
- Server handles rapid health checks under load

See: `.claude/refs/mcp-stdio-invariant.md#server-persistence-invariant---critical`

#### 9.3 Behavioral Audit Tests

All MCP tools must have comprehensive behavioral tests verifying actual functionality, not just "doesn't crash".

```bash
go test ./cmd/dev-console -run "Test.*Audit" -v
```

**Test coverage required:**

| Test File | Tools Covered | Minimum Tests |
|-----------|---------------|---------------|
| `tools_observe_audit_test.go` | observe (29 modes) | 41 tests |
| `tools_configure_audit_test.go` | configure (19 actions) | 46 tests |
| `tools_generate_audit_test.go` | generate (10 formats) | 28 tests |
| `tools_interact_audit_test.go` | interact (11 actions) | 31 tests |

## Release Checklist

When `UNSTABLE` is stable and ready for release:

### 1. Final Verification on `UNSTABLE`

```bash
# Full test suite
make test
node --test tests/extension/*.test.js

# Static analysis
go vet ./cmd/dev-console/

# Cross-platform build
make build

# Coverage check
go test -coverprofile=coverage.out ./cmd/dev-console/
go tool cover -func=coverage.out | grep total
```

### 2. Version Bump

**CRITICAL:** Use `/bump-version {version}` to update all locations, then **MUST run validation**:

```bash
bash scripts/validate-versions.sh
```

This validates all 17+ version locations match, including:
- All package.json files (npm, extension, server)
- Go main.go version constant
- MCP golden test file
- README badge
- **optionalDependencies in npm/gasoline-mcp/package.json** (CRITICAL - must match main version)

**If validation fails, STOP. Do not proceed with release.**

All locations updated by bump-version:

| File | Field |
|------|-------|
| `Makefile` | `VERSION :=` |
| `cmd/dev-console/main.go` | `version` constant |
| `extension/manifest.json` | `"version"` |
| `extension/package.json` | `"version"` |
| `server/package.json` | `"version"` |
| `server/scripts/install.js` | `VERSION` constant |
| `npm/gasoline-mcp/package.json` | `"version"` + `optionalDependencies` ⚠️ |
| `npm/darwin-arm64/package.json` | `"version"` |
| `npm/darwin-x64/package.json` | `"version"` |
| `npm/linux-arm64/package.json` | `"version"` |
| `npm/linux-x64/package.json` | `"version"` |
| `npm/win32-x64/package.json` | `"version"` |
| `cmd/dev-console/testdata/mcp-initialize.golden.json` | `"version"` |
| `README.md` | Version badge |
| `tests/extension/background.test.js` | Test assertions (2 locations) |
| `extension/background/index.test.js` | Mock manifest version |

**⚠️ CRITICAL:** `optionalDependencies` in `npm/gasoline-mcp/package.json` MUST point to the same version as the wrapper package itself. If these are mismatched, npx will install old binaries.

### 3. Merge to `main`

```bash
git checkout main
git merge UNSTABLE
```

### 4. Tag the Release

```bash
git tag v{version}
git push origin main --follow-tags
```

### 5. Build & Publish

```bash
# Cross-platform binaries
make build
```

**NPM:**
```bash
cd npm && ./publish.sh
```

**PyPI:**
```bash
# Build all PyPI packages
make pypi-build

# Test PyPI first (recommended)
make pypi-test-publish

# Production PyPI
make pypi-publish
```

See `docs/pypi-distribution.md` for detailed PyPI publishing instructions.

**Chrome Web Store:**
```bash
# Upload extension/ directory via Chrome Developer Dashboard
```

### 6. Sync `UNSTABLE`

```bash
git checkout UNSTABLE
git merge main
git push origin UNSTABLE
```

### 7. Update Marketing Site

The marketing site is a separate repo at `~/dev/gasoline-site` (Astro).
Blog posts go in `src/content/docs/blog/`. Update version numbers and
add release blog post there after tagging.

## Hotfix Process

For critical fixes that can't wait for the next release:

```bash
git checkout -b hotfix/fix-name main
# Fix, test, commit
git checkout main && git merge hotfix/fix-name
git tag v{version}
git push origin main --follow-tags

# Sync back
git checkout UNSTABLE && git merge hotfix/fix-name
git push origin UNSTABLE
git branch -d hotfix/fix-name
```

## Pre-UAT Tags

Every feature entering UAT gets a tagged, squashed commit:

```
v4.7.0-pre-uat-websocket-monitoring
v4.7.0-pre-uat-network-bodies
v4.7.0-pre-uat-checkpoint-diffs
```

If UAT fails, the single commit can be reverted atomically.
