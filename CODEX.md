# Gasoline Codex Ways Of Working

Use this with `CLAUDE.md` and `.claude/instructions.md`. If rules conflict, choose the stricter rule.

## Worktree Isolation (Required)

Before writing code:
- Confirm context with `pwd`.
- Confirm repo root with `git rev-parse --show-toplevel`.
- Confirm target branch with `git branch --show-current`.
- Stop and fix context if any check does not match the intended PR/task.

Before commit or push:
- Run `git status --short --branch` and verify only intended files are changed.
- Re-check branch/worktree context before `git commit` and `git push`.
- Do not push from the repo root when the task is assigned to a dedicated worktree.

When switching tasks/PRs:
- Explicitly switch to the target worktree path first.
- Repeat the full preflight checks before running any edit, commit, cherry-pick, or push command.

## Documentation Cross-Reference Contract (Required)

For EVERY feature and EVERY refactor, documentation updates are required in the same change:

1. Add or update the canonical flow map in `docs/architecture/flow-maps/`.
2. Add or update the feature-local `flow-map.md` pointer in `docs/features/feature/<feature>/` when a feature folder exists.
3. Update the feature `index.md`:
   - `last_reviewed`
   - `code_paths` and `test_paths`
   - link to `flow-map.md`
4. Update `docs/architecture/flow-maps/README.md` when adding a new canonical flow map.
5. Keep links bidirectional (feature -> canonical flow map, and canonical flow map -> concrete code/test paths).

No code-only refactor is complete until this documentation contract is satisfied.
