# v5.3.0 Release Preparation

**Target Release:** v5.3.0
**Branch:** next → main
**Status:** ✅ Ready for UAT

---

## ✅ Completed Work

### Features Implemented
1. ✅ **Cursor-Based Pagination** (6 commits)
   - Cursor pagination for logs (with sequence numbers)
   - Cursor pagination for actions
   - Cursor pagination for websocket_events
   - Errors handler migrated to JSON format with pagination
   - tracked_tab_id metadata added to all JSON responses
   - Observe tool description condensed to <800 chars

2. ✅ **Buffer-Specific Clearing** (1 commit)
   - Granular buffer clearing with `buffer` parameter
   - Support for: network, websocket, actions, logs, all
   - Backward compatible (defaults to logs)
   - Returns JSON with counts

3. ✅ **GitHub Version Checking** (1 commit)
   - Server checks GitHub releases API (daily, cached 6h)
   - /health endpoint includes availableVersion field
   - Extension badge shows ⬆ when update available
   - Eliminates periodic polling overhead

### Tests & Quality
- ✅ All Go tests passing (7s runtime)
- ✅ All extension tests passing
- ✅ `go vet` passes
- ✅ TypeScript compiles successfully
- ✅ Pre-UAT quality gates complete

---

## 📋 UAT Requirements

**BEFORE PROCEEDING, USER MUST:**

1. **Run UAT Checklist**
   - Follow [docs/core/UAT-v5.3-CHECKLIST.md](UAT-v5.3-CHECKLIST.md)
   - Test all pagination features
   - Test buffer clearing (all modes)
   - Test version checking (server + extension)
   - Verify backward compatibility
   - Check edge cases

2. **Sign Off on UAT**
   - All critical tests must PASS
   - No regressions
   - Performance acceptable
   - Document in UAT checklist

---

## 🚀 Release Steps (After UAT Passes)

### Step 1: Version Bump

**Update version in 2 places:**

1. **cmd/dev-console/main.go** - Line ~50:
   ```go
   const (
       version         = "5.3.0"  // Change from 5.2.5
       ServerVersion   = version
   )
   ```

2. **extension/manifest.json** - Line 3:
   ```json
   {
     "name": "Gasoline",
     "version": "5.3.0",  // Change from 5.2.5
     ...
   }
   ```

**Verify:**
```bash
grep -n "5.3.0" cmd/dev-console/main.go extension/manifest.json
# Should show both files
```

### Step 2: Update CHANGELOG

**Add to CHANGELOG.md:**
```markdown
## [5.3.0] - 2026-01-30

### Added
- **Cursor-based pagination** for logs, actions, websocket_events, errors
  - Stable pagination over live data with timestamp:sequence cursors
  - Support for after_cursor, before_cursor, since_cursor parameters
  - Automatic restart on buffer eviction with restart_on_eviction flag
  - Metadata includes cursor, count, total, has_more, oldest/newest timestamps
- **tracked_tab_id metadata** in all JSON observe responses
  - Helps AI understand which browser tab generated the data
  - Only present when extension is actively tracking a tab
- **Buffer-specific clearing** with granular control
  - configure({action: "clear", buffer: "network"}) - clear network buffers
  - configure({action: "clear", buffer: "websocket"}) - clear WebSocket buffers
  - configure({action: "clear", buffer: "actions"}) - clear user actions
  - configure({action: "clear", buffer: "logs"}) - clear console logs
  - configure({action: "clear", buffer: "all"}) - clear everything
  - Returns JSON with counts of cleared items
- **GitHub version checking** for update notifications
  - Server checks GitHub releases API (daily, cached 6 hours)
  - /health endpoint includes availableVersion field
  - Extension shows ⬆ badge when newer version available
  - Eliminates periodic polling overhead

### Changed
- **Errors handler now returns JSON** instead of markdown table
  - BREAKING: Old clients expecting markdown will need updates
  - New format includes cursor pagination support
  - Better for programmatic parsing
- Observe tool description condensed to <800 characters for MCP compliance

### Fixed
- Extension module import test now accepts .js extensions

### Performance
- Cursor pagination enables efficient querying of large datasets
- Version checking reduced from every 30 minutes to daily
- Buffer clearing provides instant memory reclamation

[5.3.0]: https://github.com/brennhill/gasoline-mcp-ai-devtools/compare/v5.2.5...v5.3.0
```

### Step 3: Final Quality Check

```bash
# Ensure all tests still pass
go vet ./cmd/dev-console/
make test
node --test tests/extension/*.test.cjs

# Ensure builds succeed
make dev
# OR
make compile-ts && go build ./cmd/dev-console/

# Check git status
git status
# Should show only version bump and CHANGELOG changes
```

### Step 4: Commit Version Bump

```bash
git add cmd/dev-console/main.go extension/manifest.json CHANGELOG.md
git commit -m "chore: Bump version to v5.3.0

Release v5.3.0 with pagination, buffer clearing, and version checking.

See CHANGELOG.md for full release notes."
git push origin next
```

### Step 5: Merge to Main

```bash
# Switch to main branch
git checkout main

# Pull latest
git pull origin main

# Merge next into main
git merge next --no-ff -m "Release v5.3.0

Merge next branch for v5.3.0 release.

Features:
- Cursor-based pagination for large datasets
- Buffer-specific clearing (network, websocket, actions, logs)
- GitHub version checking with extension badge
- tracked_tab_id metadata in JSON responses

All UAT tests passed. Ready for release."

# Push to main
git push origin main
```

### Step 6: Create Git Tag

```bash
# Create annotated tag
git tag -a v5.3.0 -m "Release v5.3.0 - Pagination & Buffer Management

Major Features:
- Cursor-based pagination for logs, actions, websocket_events, errors
- Buffer-specific clearing with granular control
- GitHub version checking with extension update badges
- tracked_tab_id metadata in all JSON responses

Breaking Changes:
- Errors handler returns JSON instead of markdown

See CHANGELOG.md for full details."

# Push tag
git push origin v5.3.0
```

### Step 7: Create GitHub Release

1. Go to: https://github.com/brennhill/gasoline-mcp-ai-devtools/releases/new
2. Tag: v5.3.0
3. Title: **v5.3.0 - Pagination & Buffer Management**
4. Description (copy from CHANGELOG + add):
   ```markdown
   ## What's New in v5.3.0

   This release focuses on **usability improvements for AI workflows** - solving token limit issues and providing granular buffer management.

   ### 🎯 Cursor-Based Pagination
   Query large datasets in chunks without losing position. Stable pagination over live data with automatic eviction handling.

   ### 🧹 Buffer-Specific Clearing
   Clear individual buffers (network, websocket, actions, logs) or all at once. Better memory management for long-running sessions.

   ### 📦 GitHub Version Checking
   Automatic update notifications via extension badge. Server checks GitHub releases daily.

   ### Full Changelog
   [See detailed changelog](https://github.com/brennhill/gasoline-mcp-ai-devtools/blob/main/CHANGELOG.md#530---2026-01-30)

   ### Installation
   - **Extension:** Install from Chrome Web Store (updates automatically)
   - **Server:** Download binary for your platform below
   - **NPM:** `npm install -g gasoline-mcp` (coming soon)
   ```
5. Attach binaries (if built for release)
6. ✅ Set as latest release
7. Publish

### Step 8: Update NPM Package (If applicable)

```bash
cd npm/gasoline-mcp
npm version 5.3.0
npm publish
```

### Step 9: Announce Release

- Post on GitHub Discussions
- Update documentation site (if applicable)
- Notify users in relevant channels

---

## 🔍 Post-Release Verification

**After release, verify:**

1. **GitHub Release Page**
   - Tag v5.3.0 exists
   - Release notes accurate
   - Binaries attached (if applicable)

2. **Version Numbers Match**
   ```bash
   # Server version
   ./dist/gasoline --version
   # → Should show v5.3.0

   # Extension version
   # → Check chrome://extensions, should show 5.3.0
   ```

3. **GitHub API Returns v5.3.0**
   ```bash
   curl -s https://api.github.com/repos/brennhill/gasoline-mcp-ai-devtools/releases/latest | jq .tag_name
   # → Should show "v5.3.0"
   ```

4. **Version Checking Works**
   - Start server
   - Check logs for version check
   - Extension should NOT show update badge (already on latest)

---

## ⚠️ Rollback Plan (If Needed)

If critical issues found post-release:

1. **Revert tag:**
   ```bash
   git tag -d v5.3.0
   git push origin :refs/tags/v5.3.0
   ```

2. **Revert main branch:**
   ```bash
   git checkout main
   git revert -m 1 <merge-commit-sha>
   git push origin main
   ```

3. **Mark GitHub release as pre-release** or delete

4. **Fix issue, increment to v5.3.1**, repeat release process

---

## 📊 Success Metrics

Track after 1 week:
- GitHub stars/watchers increase
- Extension install count
- GitHub issues related to v5.3 features
- User feedback on pagination/buffer clearing

---

## Next Steps After v5.3

**Immediately:**
- Monitor for critical bugs
- Respond to user feedback
- Plan v5.4 or v6.0 (depending on roadmap)

**Future:**
- Continue v6.0 planning (AI feedback loop)
- Evaluate success of v5.3 features
- Gather metrics on pagination usage
