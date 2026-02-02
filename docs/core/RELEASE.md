---
status: active
scope: process/release
ai-priority: high
tags: [release, process, quality-gates, deployment]
relates-to: [KNOWN-ISSUES.md, docs/core/uat-v5.3-checklist.md]
last-verified: 2026-01-30
canonical: true
---

# Release Process

Gasoline MCP uses a `next` → `main` branching model with strict quality gates. Every release goes through automated and manual verification before reaching users.

## Branch Model

```
main    ─●───────────────────●────── (releases only)
          │                   ↑
          │             merge + tag
          ↓                   │
next    ─●──●──●──●──●──●──●─● ──── (integration)
             ↑  ↑        ↑
feature/a ───●  │        │
feature/b ──────●        │
feature/c ───────────────●
```

- **`main`** — Published releases. What's on npm and the Chrome Web Store.
- **`next`** — Integration branch. All features merge here first.
- **Feature branches** — Branch from `next`, merge back to `next`.

## Quality Gates

Every feature must pass all gates before merging to `next`. All gates must be green before `next` merges to `main`.

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

Before pushing to `next`, all feature work is squashed into a single commit:

```bash
# Squash all commits since branching from next
/squash

# Tag for pre-UAT
git tag v{version}-pre-uat-{feature}

# Push
git push origin HEAD --follow-tags
```

## Release Checklist

When `next` is stable and ready for release:

### 1. Final Verification on `next`

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

All locations must be updated together (use `/bump-version`):

| File | Field |
|------|-------|
| `extension/manifest.json` | `"version"` |
| `extension/inject.js` | `version:` in `window.__gasoline` |
| `extension/background.js` | `version:` in debug export |
| `extension/package.json` | `"version"` |
| `cmd/dev-console/main.go` | `version` constant |
| `server/package.json` | `"version"` |
| `npm/gasoline-mcp/package.json` | `"version"` + `optionalDependencies` |
| `npm/darwin-arm64/package.json` | `"version"` |
| `npm/darwin-x64/package.json` | `"version"` |
| `npm/linux-arm64/package.json` | `"version"` |
| `npm/linux-x64/package.json` | `"version"` |
| `npm/win32-x64/package.json` | `"version"` |
| `pypi/gasoline-mcp/pyproject.toml` | `version` + optional deps |
| `pypi/gasoline-mcp-*/pyproject.toml` | `version` (5 platform packages) |
| `pypi/gasoline-mcp*/__init__.py` | `__version__` (6 packages) |
| `README.md` | Version badge |

Verify: `grep -r "OLD_VERSION" --include="*.json" --include="*.js" --include="*.go" --include="*.md" .` should return zero results.

### 3. Merge to `main`

```bash
git checkout main
git merge next
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

### 6. Sync `next`

```bash
git checkout next
git merge main
git push origin next
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
git checkout next && git merge hotfix/fix-name
git push origin next
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
