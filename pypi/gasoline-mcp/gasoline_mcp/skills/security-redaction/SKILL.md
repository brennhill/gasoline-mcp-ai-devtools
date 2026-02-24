---
name: security-redaction
description: Audit redaction coverage to ensure secrets and tokens are never exposed in logs, diagnostics, errors, or debug output.
---

# Gasoline Security Redaction

Use this skill to validate that sensitive data is redacted everywhere.

## Inputs To Request

- Secret classes to test (API keys, bearer tokens, cookies, PATs)
- Surfaces to inspect (logs, diagnostics, command_result, debug output)

## Workflow

1. Seed synthetic secrets:
Use clearly fake but realistic token patterns.

2. Trigger representative flows:
Run network, error, and debug-producing actions.

3. Inspect all outputs:
Check raw and transformed outputs for leaks.

4. Verify policy behavior:
Ensure expected redaction markers are present.

5. Report gaps with exact surface and key path.

## Output Contract

- `tested_surfaces`
- `leaks_found`
- `redaction_pass_rate`
- `required_fixes`
