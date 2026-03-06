---
doc_type: guide
feature_id: feature-quality-gates
last_reviewed: 2026-03-06
---

# Quality Gates Setup Guide

Automated code quality enforcement via Claude Code hooks + Haiku.

## Quick Start

### 1. Create config files

```
configure(what="setup_quality_gates")
```

This creates:
- `.gasoline.json` — points to your standards doc
- `gasoline-code-standards.md` — starter coding conventions

### 2. Add a Claude Code hook

Add to `.claude/settings.json` in your project:

**Option A: Command hook (injects standards as context)**

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "cat gasoline-code-standards.md | head -200",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

The standards doc content appears in the AI's context after every edit. The AI self-reviews against the standards.

**Option B: Prompt hook (Haiku reviews automatically)**

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

Haiku reviews every edit (~$0.0001/edit) and blocks non-conforming changes with specific findings.

### 3. Edit your standards

Open `gasoline-code-standards.md` and add your project's patterns:

```markdown
### Command Pattern
Functions with 3+ sequential phases should implement the Command interface.
See internal/cmd/base.go for the canonical implementation.

### Validation Guards
Use validateAndRespond() from internal/util/guards.go.
Do not write inline if/else validation chains.
```

Write rules the way you'd explain them to a new team member. No special format — just markdown.

## How It Works

```
AI calls Edit/Write
  -> PostToolUse hook fires
  -> Standards doc injected into context (Option A)
     OR Haiku reviews the change (Option B)
  -> AI sees findings and fixes immediately
  -> No extra tool calls, no token cost for analysis
```

## Configuration

### `.gasoline.json`

```json
{
  "code_standards": "gasoline-code-standards.md",
  "file_size_limit": 800,
  "duplicate_threshold": 8
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `code_standards` | `gasoline-code-standards.md` | Path to your standards doc |
| `file_size_limit` | `800` | Warn when files exceed this LOC |
| `duplicate_threshold` | `8` | Min lines for duplicate detection |

### Pointing to an existing doc

If you already have a conventions doc:

```json
{
  "code_standards": "docs/core/common-patterns.md"
}
```

Update the hook command to `cat` that file instead.

## Token Cost

| Approach | Cost per edit |
|----------|--------------|
| Command hook (inject standards) | ~200 tokens (standards doc size) |
| Prompt hook (Haiku review) | ~$0.0001 (Haiku input + output) |
| Manual review pass | ~2,000-5,000 tokens |
| No quality gates | Free, but 3-6x review passes later |

## Troubleshooting

**Hook not firing?**
- Verify `.claude/settings.json` exists in the project root
- Check that `matcher` matches the tool name exactly: `Edit|Write`
- Run `claude --debug` to see hook execution

**Standards file not found?**
- The `command` path in the hook is relative to the project root
- Use absolute paths if the standards file is outside the project

**Too many false positives?**
- Make rules more specific in the standards doc
- Focus on patterns that cause real bugs, not style preferences
- Use the prompt hook with "Only flag clear violations" instruction
