---
name: version-bump-all-files
description: Bump release version across all tracked files, then block completion until stale old-version references are fully removed.
---
<!-- gasoline-managed-skill id:version-bump-all-files version:1 -->

# Gasoline Version Bump (All Files)

Use this skill for any release bump where missed version references are unacceptable.

## Inputs

- `OLD_VERSION` (for stale-string sweep)
- `NEW_VERSION` (strict semver, e.g. `0.7.11`)

## Workflow

1. Run deterministic bump:
`node scripts/bump-version.js "$NEW_VERSION"`

2. Sync generated/versioned files:
`make sync-version`

3. Run version-gate validation:
`bash scripts/validate-versions.sh`

4. Run hard stale-reference sweep for `OLD_VERSION`:
`rg -n "$OLD_VERSION" --glob '!dist/**' --glob '!node_modules/**' --glob '!.git/**'`

5. If any stale matches remain, update them and rerun steps 3-4 until zero matches.

6. Run targeted regression guard:
`node --test tests/extension/install-script-extension-source.test.js`

## One-Command Runner

Use the bundled helper when possible:

`bash .codex/skills/version-bump-all-files/scripts/run.sh "$OLD_VERSION" "$NEW_VERSION"`

## Done Criteria

- `VERSION` file equals `NEW_VERSION`
- `bash scripts/validate-versions.sh` passes
- stale sweep returns zero matches for `OLD_VERSION`
- touched version files are committed together
