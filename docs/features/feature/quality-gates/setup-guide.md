---
doc_type: guide
feature_id: feature-quality-gates
last_reviewed: 2026-03-07
---

# Quality Gates Setup Guide

Automated code quality enforcement via Claude Code hooks.

## Quick Start

### 1. Create config files

```
configure(what="setup_quality_gates")
```

This creates:
- `.gasoline.json` — points to your standards doc, configures thresholds
- `gasoline-code-standards.md` — starter coding conventions

### 2. Add Claude Code hooks

Add to `.claude/settings.json` in your project:

**Recommended: Both quality gates + output compression**

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "gasoline hook quality-gate",
            "timeout": 10
          }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "gasoline hook compress-output",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

What gets injected:
- **On Edit/Write:** Your code standards doc, file size warnings, review instructions
- **On Bash:** Compressed test/build output (91-99% token savings on verbose output)

**Optional: Add Haiku review (belt and suspenders)**

Chain the quality gate hook with a Haiku prompt hook for automated code review:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "gasoline hook quality-gate",
            "timeout": 10
          },
          {
            "type": "prompt",
            "prompt": "Review this code change against the project standards and any quality gate findings above. Only flag clear violations. Respond {\"ok\": true} if acceptable, or {\"ok\": false, \"reason\": \"specific findings\"} if not.",
            "model": "haiku",
            "timeout": 30
          }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "gasoline hook compress-output",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

### 3. Edit your standards

Open `gasoline-code-standards.md` and add your project's patterns. Be specific — vague rules cause false positives.

Good rule: "Request validation must use `validateAndRespond()` from `internal/util/guards.go`. Do not write inline if/else validation chains."

Bad rule: "Use good patterns." (flags everything)

## What Gets Checked

### By `gasoline hook quality-gate`

| Check | Trigger | Data injected |
|-------|---------|---------------|
| **Standards doc** | Every Edit/Write | Full standards content (first 150 lines) |
| **File size** | File > 90% of limit | Warning with line count and limit |

### By `gasoline hook compress-output`

| Pattern | Detection | Compression |
|---------|-----------|-------------|
| go test | Command or `--- PASS/FAIL` markers | Summary + failed tests only |
| jest/vitest | Command or `Test Suites:` markers | Summary + failure files |
| pytest | Command or `N passed` markers | Summary + failures |
| cargo test | Command or `test result:` markers | Summary + failures |
| go build/vet | Command | Error lines only |
| make | Command | Error/warning lines only |
| tsc | Command | `error TS` lines only |
| npm/webpack | Command | ERROR lines only |
| cargo build | Command | `error[E` lines only |
| Generic | >100 lines, no pattern | First 30 + last 20 lines |

### By the standards doc (AI self-review or Haiku)

| Default rule | Trigger |
|-------------|---------|
| Max 800 LOC per file | File exceeds limit |
| No orphan/dead code | Commented-out blocks, unused imports |
| Semantic naming | Non-descriptive names, abbreviations |
| Error handling | Ignored error returns, unstructured messages |
| Extract helpers | 3+ similar lines repeated |
| Structural patterns | 3+ switch branches, 4+ nesting levels, 50+ line functions |
| Test coverage | New functions without tests, bug fixes without regression tests |
| Security | Logged secrets, unvalidated external input |

## How It Works

```
AI calls Edit/Write
  -> PostToolUse hook fires
  -> `gasoline hook quality-gate` runs:
     1. Finds .gasoline.json (walks up from changed file)
     2. Reads code_standards doc
     3. Checks file line count vs limit
     4. Outputs findings as additionalContext (JSON)
  -> AI sees standards + findings in its context
  -> Fixes violations before proceeding

AI calls Bash (test/build)
  -> PostToolUse hook fires
  -> `gasoline hook compress-output` runs:
     1. Detects output pattern (go test, jest, tsc, etc.)
     2. Compresses to summary + errors only
     3. Posts savings to daemon for tracking
     4. Outputs compressed result as additionalContext (JSON)
  -> AI sees compressed output instead of verbose logs
```

## Configuration

### `.gasoline.json`

```json
{
  "code_standards": "gasoline-code-standards.md",
  "file_size_limit": 800,
  "duplicate_threshold": 3
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `code_standards` | `gasoline-code-standards.md` | Path to your standards doc |
| `file_size_limit` | `800` | Warn when files exceed this LOC |
| `duplicate_threshold` | `3` | Min lines for duplicate detection |

### Pointing to an existing doc

If you already have a conventions doc:

```json
{
  "code_standards": "docs/core/common-patterns.md"
}
```

## Token Savings

Token savings are tracked per session and across lifetime:
- **Session summary** logged to stderr on daemon shutdown
- **Lifetime stats** persisted to `~/.gasoline/stats/lifetime.json`

| Approach | Cost per edit |
|----------|--------------|
| Quality gate hook (inject standards) | ~200-400 tokens |
| Compression hook (Bash output) | Saves 91-99% on test/build output |
| Prompt hook (Haiku review) | ~$0.0001 |
| Manual review pass | ~2,000-5,000 tokens |

## Troubleshooting

**Hook not firing?**
- Verify `.claude/settings.json` exists under the project root
- Check that `matcher` matches the tool name exactly: `Edit|Write` or `Bash`
- Run `claude --debug` to see hook execution

**gasoline not found?**
- Ensure gasoline is installed: `npm install -g gasoline-agentic-devtools`
- Or use the full path: `gasoline-mcp hook quality-gate`

**Too many false positives?**
- Make rules more specific in the standards doc
- Focus on patterns that cause real bugs, not style preferences
- Remove generic rules and add project-specific ones
