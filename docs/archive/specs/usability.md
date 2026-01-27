# Usability Improvements Specification

**Status:** Planning
**Priority:** High (Critical for adoption)
**Effort:** 1-2 days

## Overview

New users face significant friction getting Gasoline running. This spec catalogs all usability barriers and proposes fixes to achieve a "5-minute setup" goal.

## Success Criteria

- New user can go from zero to working setup in under 5 minutes
- Clear verification that setup succeeded
- Helpful error messages with actionable fixes
- Single recommended setup path (with alternatives documented)

---

## Issues by Priority

### Critical (Blocks Setup)

#### 1. NPM Package Naming Mismatch

#### 2. Binary Download Fails Without Guidance

| Aspect | Detail |
|--------|--------|
| **Problem** | `server/scripts/install.js:91-121` downloads from GitHub releases — if no release exists, users see "Download failed" with no recovery path |
| **Impact** | `npm install` fails; users don't know what to do |
| **Fix** | Better error: "No pre-built binary. Build from source: `go build ./cmd/dev-console`" |
| **File** | `server/scripts/install.js` |

#### 3. MCP Config Uses Relative Path

| Aspect | Detail |
|--------|--------|
| **Problem** | README shows `"args": ["run", "./cmd/dev-console"]` which only works from repo root |
| **Impact** | MCP spawn fails silently if launched from different directory |
| **Fix** | Use npx: `"command": "npx", "args": ["-y", "gasoline-mcp"]` |
| **File** | `README.md` |

---

### High (Significant Friction)

#### 4. Chrome Extension Not in Web Store

| Aspect | Detail |
|--------|--------|
| **Problem** | Manual "Load unpacked" requires 4 steps through chrome://extensions |
| **Impact** | High friction; many users abandon setup here |
| **Fix** | (a) Expedite Web Store approval, (b) Provide install script that opens chrome://extensions, (c) Create downloadable CRX |
| **Status** | Web Store submission pending |

#### 5. Three Different Setup Methods (Confusing)

| Aspect | Detail |
|--------|--------|
| **Problem** | README shows: (1) `go run`, (2) `npx gasoline-mcp`, (3) MCP config — users don't know which to use |
| **Impact** | Analysis paralysis; users try wrong method |
| **Fix** | Single "Quick Start" with clear branching based on use case |
| **File** | `README.md` |

#### 6. No Verification That Setup Worked

| Aspect | Detail |
|--------|--------|
| **Problem** | After setup, no way to verify success |
| **Impact** | Users don't know if server running, extension connected, or MCP tools available |
| **Fix** | Add `npx gasoline-mcp --check` command and print `curl localhost:7890/health` in startup |
| **Files** | `cmd/dev-console/main.go`, `server/bin/gasoline-mcp` |

---

### Medium (Poor Experience)

#### 7. Extension "Disconnected" Without Explanation

| Aspect | Detail |
|--------|--------|
| **Problem** | Popup shows red "Disconnected" but troubleshooting is hidden in separate panel |
| **Impact** | Users don't know WHY or what to do |
| **Fix** | Show troubleshooting inline when disconnected |
| **File** | `extension/popup.html` |

#### 8. Port Conflict Error Is Cryptic

| Aspect | Detail |
|--------|--------|
| **Problem** | `Fatal: cannot bind port 7890` doesn't tell users how to fix |
| **Impact** | Users stuck with no guidance |
| **Fix** | Add: "Port in use. Kill existing: `lsof -ti :7890 \| xargs kill` or use `--port 7891`" |
| **File** | `cmd/dev-console/main.go:913-915` |

#### 9. No First-Run Experience

| Aspect | Detail |
|--------|--------|
| **Problem** | Server starts, shows banner, then... silence |
| **Impact** | Users don't know next steps |
| **Fix** | Print: "Next: (1) Install extension, (2) Open browser, (3) Check popup shows Connected" |
| **File** | `cmd/dev-console/main.go` |

#### 10. Go Requirement Not Stated

| Aspect | Detail |
|--------|--------|
| **Problem** | Quick Start uses `go run` without mentioning Go is required |
| **Impact** | Users without Go get "command not found" |
| **Fix** | Add prerequisite OR make npx the primary path |
| **File** | `README.md` |

---

### Low (Polish)

#### 11. MCP Config Varies by Tool

| Aspect | Detail |
|--------|--------|
| **Problem** | README vs .mcp.json show different configs |
| **Fix** | Standardize on ONE config format |

#### 12. Extension Host Permissions Confusing

| Aspect | Detail |
|--------|--------|
| **Problem** | Users don't understand extension captures from ANY page but sends to localhost |
| **Fix** | Add inline help in popup |

#### 13. MCP Server Exits When Claude Code Closes

| Aspect | Detail |
|--------|--------|
| **Problem** | Server exits 2s after MCP stdin closes |
| **Fix** | Document behavior OR add `--persist` flag |

#### 14. bin/ Directory Missing

| Aspect | Detail |
|--------|--------|
| **Problem** | install.js expects binaries in `server/bin/` but doesn't exist |
| **Fix** | Create during build OR skip postinstall for dev |

#### 15. No Version Check in Extension

| Aspect | Detail |
|--------|--------|
| **Problem** | Extension doesn't verify server version compatibility |
| **Fix** | Call `/health` on connect, warn on mismatch |

---

## Implementation Plan

### Phase 1: Critical Fixes (Day 1) ✅

1. [x] Rename package to `gasoline-mcp` — commit 033cc0b
2. [x] Fix install.js error messages — commit 64e4f90
3. [x] Update README with npx-based MCP config — commit 033cc0b
4. [x] Add `--check` flag to verify setup — commit 64e4f90

### Phase 2: UX Improvements (Day 2) ✅

5. [x] Add first-run message with next steps — commit 64e4f90
6. [x] Improve port conflict error message — commit 033cc0b
7. [x] Show troubleshooting inline in extension popup — commit 64e4f90
8. [x] Add prerequisites to README — commit 033cc0b

### Phase 3: Polish ✅

9. [x] Extension version compatibility check — commit e554109
10. [x] `--persist` flag for server — commit e554109
11. [ ] Streamlined extension install (CRX or Web Store) — pending Web Store approval

---

## Verification Checklist

After implementation, verify with fresh environment:

- [ ] `npx gasoline-mcp` works (no Go required)
- [ ] `npx gasoline-mcp --check` reports status
- [ ] Extension shows helpful message when disconnected
- [ ] Port conflict shows kill command
- [ ] First-run shows next steps
- [ ] README Quick Start works in under 5 minutes
