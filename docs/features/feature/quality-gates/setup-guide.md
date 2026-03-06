---
doc_type: guide
feature_id: feature-quality-gates
last_reviewed: 2026-03-06
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

### 2. Add a Claude Code hook

Add to `.claude/settings.json` in your project:

**Option A: Quality gate script (recommended)**

The script reads your standards, checks file size, runs jscpd for duplicates, and injects all findings into the AI's context.

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "scripts/quality-gate-hook.sh",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

What gets injected on every Edit/Write:
- Your full code standards doc (from `.gasoline.json` -> `code_standards`)
- File size warning if approaching or exceeding the limit
- jscpd duplicate detection results (if duplicates found in the same directory)
- Review instruction telling the AI to fix violations before proceeding

**Option B: Prompt hook (Haiku reviews automatically)**

Haiku reviews every edit (~$0.0001/edit) and blocks non-conforming changes.

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Review this code change against the project standards. Only flag clear violations, not style preferences. If the change looks good, respond {\"ok\": true}. If there are issues, respond {\"ok\": false, \"reason\": \"specific findings\"}.",
            "model": "haiku",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

**Option C: Both (belt and suspenders)**

Chain both hooks — the script injects evidence (file size, duplicates, standards), then Haiku reviews the change with that evidence in context.

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "scripts/quality-gate-hook.sh",
            "timeout": 30
          },
          {
            "type": "prompt",
            "prompt": "Review this code change against the project standards and any quality gate findings above. Only flag clear violations. Respond {\"ok\": true} if acceptable, or {\"ok\": false, \"reason\": \"specific findings\"} if not.",
            "model": "haiku",
            "timeout": 30
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

### By the hook script (`quality-gate-hook.sh`)

| Check | Trigger | Data injected |
|-------|---------|---------------|
| **Standards doc** | Every Edit/Write | Full standards content (first 150 lines) |
| **File size** | File > 90% of limit | Warning with line count and limit |
| **Duplicates** | jscpd finds clones | Clone locations and similarity % |

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
  -> quality-gate-hook.sh runs:
     1. Finds .gasoline.json (walks up from changed file)
     2. Reads code_standards doc
     3. Checks file line count vs limit
     4. Runs jscpd on the directory (if npx available)
     5. Outputs findings as additionalContext (JSON)
  -> AI sees standards + findings in its context
  -> Fixes violations before proceeding
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

## Token Cost

| Approach | Cost per edit |
|----------|--------------|
| Command hook (inject standards + evidence) | ~200-400 tokens |
| Prompt hook (Haiku review) | ~$0.0001 |
| Both hooks combined | ~200-400 tokens + ~$0.0001 |
| Manual review pass | ~2,000-5,000 tokens |
| No quality gates | Free, but 3-6x review passes later |

## Troubleshooting

**Hook not firing?**
- Verify `.claude/settings.json` exists in `.claude/` under the project root
- Check that `matcher` matches the tool name exactly: `Edit|Write`
- Run `claude --debug` to see hook execution

**jscpd not running?**
- Requires `npx` in PATH (Node.js). The hook skips jscpd silently if unavailable.
- First run downloads jscpd (~5s). Subsequent runs are instant.

**Too many false positives?**
- Make rules more specific in the standards doc
- Focus on patterns that cause real bugs, not style preferences
- Remove generic rules and add project-specific ones
