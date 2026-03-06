---
name: config-doctor
description: Diagnose and fix Gasoline MCP configuration across Claude, Codex, and Gemini, including command paths and connection health.
---

# Gasoline Config Doctor

Use this skill when setup is failing or behavior differs by client.

## Inputs To Request

- Target agent(s)
- Install method (global or project)
- Current error messages

## Workflow

1. Inspect config files:
Detect missing, malformed, or conflicting MCP entries.

2. Validate binary and args:
Ensure command, version, and flags are valid for the environment.

3. Validate runtime connectivity:
Check health endpoint, MCP handshake, and extension connectivity.

4. Apply minimal safe fixes:
Patch only needed fields and preserve unrelated config.

5. Re-verify with diagnostics.

## Output Contract

- `issues_found`
- `changes_applied`
- `post_fix_validation`
- `next_steps`
