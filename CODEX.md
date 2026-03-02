# Gasoline Codex Ways Of Working

Use this with `CLAUDE.md` and `.claude/instructions.md`. If rules conflict, choose the stricter rule.

## GitNexus Policy (Non-Blocking)

- Use GitNexus when it helps with architecture/context, but treat it as optional.
- If GitNexus is unavailable, stale, slow, or errors, continue immediately using local repo inspection and tests.
- Never pause task execution waiting for GitNexus index refresh.

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

## Engineering Best Practices Contract (Required)

1. Instruction precedence is strict: system > repo policy > task request > style preference.
2. If requirements are ambiguous, state assumptions explicitly before implementation.
3. Definition of done includes code + tests + docs + flow maps in the same change.
4. Lint/type/test must pass, or known failures must be documented with issue links.
5. Keep modules single-purpose; avoid god objects and hidden shared state.
6. Keep public interfaces minimal and explicit; cross-feature calls go through clear boundaries.
7. Refactors must preserve behavior unless a behavior change is explicitly requested.
8. Every bug fix must include a regression test that fails before and passes after.
9. Prefer deterministic tests (mocks/fakes/controlled clocks) over sleep-based timing.
10. Enforce startup and request latency budgets with explicit timeout/retry/backoff policies.
11. Use structured logs with correlation IDs; avoid protocol-breaking stdout/stderr noise.
12. Version public contracts and keep wire schemas synchronized across Go/TS boundaries.
13. Redact secrets from logs/errors/diagnostics and never commit credentials.
14. New dependencies require explicit justification; remove unused dependencies promptly.
15. Reviews and handoffs must cover correctness, modularity, performance, testability, docs quality, and DRY adherence.
16. CI must block merges on broken docs links, missing required docs, or failing quality gates.
