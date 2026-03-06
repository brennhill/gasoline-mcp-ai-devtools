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
