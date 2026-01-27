---
> **[MIGRATED: 2024-xx-xx | Source: /docs/specs/workflow-integration-review.md]**
> This file was migrated as part of the documentation reorganization. Please update links and references accordingly.
---

# Review: Workflow Integration (tech-spec-workflow-integration.md)

## Executive Summary

This spec defines three integration levels: automatic session summaries, an MCP tool for PR annotations, and optional git hook integration. It is the simplest spec in the batch and the most practical -- developers get immediate value from PR-level visibility into AI-driven performance changes. However, the session summary relies on a clean shutdown that often does not happen, the PR summary format is too opinionated (markdown table layout), and the git hook approach is fragile and under-specified.

## Critical Issues (Must Fix Before Implementation)

### 1. Session Summary Generation on Shutdown Is Unreliable

**Section:** "Session Summary Generation" / "Server Integration" -- Shutdown Hook

The spec says: "When the server shuts down (after flushing persistent memory), the server generates the session summary and writes it."

**Problem:** The Gasoline server is typically killed in one of these ways:
1. **Ctrl+C in terminal** (SIGINT) -- the server gets a signal and can run cleanup. This works.
2. **AI agent exits** -- the MCP client closes stdin. The server detects EOF and exits. Cleanup runs if the shutdown handler catches this.
3. **Terminal closes** (SIGHUP) -- depends on whether the server traps SIGHUP.
4. **System sleep/hibernate** -- no signal. Server process is frozen, then killed if the laptop is closed for too long.
5. **Process killed** (SIGKILL, OOM killer) -- no cleanup possible.
6. **Server crashes** (panic, segfault) -- no cleanup possible.

Cases 4-6 are common in development workflows (developer closes laptop, goes to lunch). The session summary is never generated. The spec acknowledges this under "Edge Cases" ("Server crash: Summary isn't generated") but does not provide mitigation.

**Fix:** Do not rely on shutdown for summary generation. Instead:
1. Maintain a running session summary in memory, updated incrementally on each performance snapshot, error observation, and a11y audit.
2. Flush the running summary to `.gasoline/sessions/latest.json` periodically (every 60 seconds, as part of the existing persistent memory flush goroutine).
3. On clean shutdown, do a final flush. On unclean shutdown, the periodic flush ensures at most 60 seconds of data is lost.

The `generate_pr_summary` tool reads the running in-memory summary, so it always has current data regardless of shutdown behavior.

### 2. PR Summary Markdown Format Is Too Rigid

**Section:** "PR Summary Tool"

The spec produces a fixed markdown template:

```markdown
## Performance Impact
| Metric | Before | After | Delta |
...
### Resource Changes
- **Added**: ...
### Errors
...
### Accessibility
...
```

**Problem:** This format assumes:
- The developer wants all sections (they might only care about performance, not a11y)
- The PR platform renders markdown tables well (GitHub does; GitLab does; Bitbucket's markdown table support is poor)
- The metrics shown are the right ones (what if the developer's team uses TTFB, not FCP?)
- The section order is correct

More importantly, the AI agent calling `generate_pr_summary` cannot customize the output. If the agent is writing a PR description, it needs to integrate the performance summary with its own explanation of the code changes. A pre-formatted markdown block is harder to integrate than structured data.

**Fix:** Return structured JSON from `generate_pr_summary` and let the AI (or a template) format it:

```json
{
  "performance": {
    "metrics": [
      { "name": "Load Time", "before_ms": 1200, "after_ms": 1400, "delta_ms": 200, "delta_pct": 16.7 }
    ],
    "resources": { "added": [...], "removed": [...], "net_bytes": 45000 }
  },
  "errors": { "fixed": [...], "new": [...], "persistent": [...] },
  "a11y": { "new_violations": [], "existing_violations": 2 },
  "metadata": { "samples": 12, "page_loads": 8, "duration_s": 2700 }
}
```

Additionally, provide a `format` parameter: `"json"` (default, structured), `"markdown"` (the current template), `"one_liner"` (for git hooks). This gives the agent flexibility without requiring it to format everything from scratch.

### 3. First Snapshot / Last Snapshot Delta Is a Poor Performance Comparison

**Section:** "Session Summary Generation" -- item 1

The spec computes performance delta by comparing the first snapshot of the session against the last. This is misleading when:

- **The first snapshot is a cold load** (empty cache) and the last is a warm load (primed cache). The delta shows "improvement" that is just caching, not code changes.
- **The developer loaded different pages.** First snapshot: homepage. Last snapshot: settings page. The delta compares two completely different pages.
- **The developer made multiple changes.** The delta shows the net result of all changes, but the PR might only include the last change. The delta attributes earlier changes to this PR.

The existing implementation (`types.go` lines 507-545, `SessionTracker` in types.go lines 645-649) tracks `firstSnapshots` per URL, which is better than a single first/last comparison. But the spec does not use per-URL deltas -- it describes a single session-wide delta.

**Fix:** Compute deltas per-URL. For each URL that has both a first and last snapshot in the session, compute the delta. Report the top 5 URLs by reload count (already suggested in the spec's edge cases). This prevents cross-page comparisons and gives per-route performance impact.

Additionally, exclude the very first snapshot per URL (likely a cold load) from the delta. Start the comparison from the second snapshot (first warm load) against the last snapshot. This eliminates the cold-cache bias.

### 4. Git Hook Approach Is Fragile and Under-Specified

**Section:** "Git Hook Format"

```bash
curl -s http://localhost:3663/v4/session-summary | jq -r '.one_liner' >> .git/COMMIT_EDITMSG
```

**Problems:**
1. **Wrong port.** The example uses port 3663; the default Gasoline port is 7890. If the developer customized the port, the hook breaks silently.
2. **Server might not be running.** If the developer commits without a Gasoline session active, `curl` fails and the hook either errors (breaking the commit) or silently appends nothing (if `curl -s` is used with `|| true`).
3. **`COMMIT_EDITMSG` is not the right file.** Post-commit hooks run after the commit is already created. `COMMIT_EDITMSG` is the message of the just-completed commit. Appending to it does nothing -- the commit is already written. The spec needs a `prepare-commit-msg` hook (pre-commit) or `commit-msg` hook instead.
4. **`jq` dependency.** Not all developer machines have `jq` installed. The hook should work with just `curl` and standard tools.

**Fix:** Ship a proper hook script (`scripts/gasoline-commit-hook.sh`) that:
1. Checks if Gasoline is running (try `curl -s http://localhost:${GASOLINE_PORT:-7890}/health`)
2. If running, fetches the one-liner from `/v4/session-summary`
3. If not running, exits 0 silently (do not break the commit)
4. Appends to `$1` (the commit message file, passed as argument to `prepare-commit-msg`)
5. Uses only `curl` and `sed` -- no `jq` dependency
6. Is installed via `gasoline install-hook` CLI command that creates a symlink or copies the script

Document the correct hook type (`prepare-commit-msg`, not `post-commit`).

### 5. Archive Deduplication by Timestamp Is Insufficient

**Section:** "Edge Cases" -- "Concurrent sessions (two server instances): Each generates its own summary. Archive deduplicates by timestamp."

Timestamp-based deduplication requires millisecond-precision filenames, and two sessions ending within the same second would still collide. More importantly, two concurrent sessions produce two different summaries -- deduplication by timestamp would discard one.

**Fix:** Use a session ID (generated at server startup, e.g., `session_{unix_ms}_{random4}`) as the archive filename. Never deduplicate -- both summaries are valid records. Cap the archive at 20 entries by deletion of oldest, which the spec already specifies.

## Recommendations (Should Consider)

### 1. Session Summary Should Include Git Context

The summary says what happened to performance metrics but not what code changes were made. If the summary captures the current git branch, latest commit hash, and uncommitted file count at session start and end, the AI can correlate "bundle size grew by 45KB" with "between commit abc123 and def456, which added chart-library as a dependency."

Add to `SessionMetadata`:
```json
{
  "git_branch": "feature/dashboard-charts",
  "git_commit_start": "abc1234",
  "git_commit_end": "def5678",
  "uncommitted_files_start": 3,
  "uncommitted_files_end": 0
}
```

This requires shelling out to `git` -- acceptable for metadata gathered at session start/end (not on every snapshot).

### 2. The `GET /v4/session-summary` Endpoint Should Be Documented as Unstable

The endpoint serves a running summary that changes with every observation. A git hook or CI script that calls it at different times during a commit flow will get different results. Document this explicitly: "The summary reflects observations up to the moment of the request. For deterministic results, use `generate_pr_summary` which snapshots the data."

### 3. Consider a `generate_commit_message` Tool Instead of Git Hooks

Git hooks are notoriously fragile (installation, permissions, `.husky` vs. `.git/hooks` vs. `lefthook`, etc.). A more Gasoline-native approach: provide a `generate_commit_message` MCP tool that the AI calls before committing. The AI integrates the performance context into its commit message naturally.

This is how the AI actually works -- it calls `git commit -m "..."` with a message it composes. A tool that provides the performance one-liner as structured data is more useful than a git hook that appends to the message file.

### 4. Bundle Size Delta Needs Clarification

**Section:** "PR Summary Tool" markdown example

The example shows "Bundle Size: 340KB -> 385KB, +45KB (+13%)." But "bundle size" is ambiguous -- is this `transferSize` (compressed, over-the-wire) or `decodedSize` (uncompressed)? The `PerformanceDelta` type uses `BundleSizeBefore`/`After` without specifying which.

For PR summaries, `transferSize` is more actionable (it affects load time directly). For debugging, `decodedSize` reveals actual code growth. Report both, or at minimum, label which one is used.

### 5. The One-Liner Format Needs an Escape Hatch

**Section:** "Git Hook Format"

The one-liner format: `[perf: +200ms load, +45KB bundle | errors: -1 fixed | a11y: clean]`

If there are no performance changes, no errors, and no a11y changes, this becomes: `[perf: - | errors: - | a11y: -]` -- pure noise in a commit message. The hook should suppress output entirely when there is nothing meaningful to report.

## Implementation Roadmap

1. **Incremental session summary** (1 day): Refactor session summary to update incrementally on each performance snapshot and error observation. Add periodic flush (60s) to `.gasoline/sessions/latest.json`. Remove reliance on clean shutdown.

2. **Per-URL delta computation** (0.5 days): Compute deltas per URL instead of session-wide. Exclude first (cold) snapshot per URL. Report top 5 by reload count.

3. **`generate_pr_summary` tool** (1 day): Return structured JSON by default. Add `format` parameter for `json`, `markdown`, and `one_liner`. Ensure markdown format works in GitHub, GitLab, and Bitbucket. Add git context (branch, commit range).

4. **`GET /v4/session-summary` endpoint** (0.5 days): Serve the running summary. Add one-liner variant via `?format=one_liner` query param.

5. **Git hook script** (0.5 days): Write `scripts/gasoline-commit-hook.sh` as a `prepare-commit-msg` hook. Handle missing server gracefully. No `jq` dependency. Add `gasoline install-hook` CLI command (future work, document manual installation for now).

6. **Archive management** (0.5 days): Use session ID filenames. Cap at 20 entries. No deduplication.

Total: ~4 days of implementation work. The SessionTracker infrastructure already exists in `types.go`. Focus on incremental summary updates and the structured PR summary tool.
