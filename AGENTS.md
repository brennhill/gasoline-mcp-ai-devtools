<!-- gitnexus:start -->
# GitNexus MCP

This project is indexed by GitNexus as **gasoline** (17519 symbols, 49337 relationships, 300 execution flows).

## Always Start Here

1. **Read `gitnexus://repo/{name}/context`** — codebase overview + check index freshness
2. **Match your task to a skill below** and **read that skill file**
3. **Follow the skill's workflow and checklist**

> If step 1 warns the index is stale, run `npx gitnexus analyze` in the terminal first.

## Skills

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->

## Documentation Cross-Reference Contract (Required)

For EVERY feature and EVERY refactor, documentation updates are mandatory in the same change:

1. Add or update the canonical flow map in `docs/architecture/flow-maps/`.
2. Add or update the feature-local `flow-map.md` pointer in `docs/features/feature/<feature>/` when a feature folder exists.
3. Update the feature `index.md`:
   - `last_reviewed`
   - `code_paths` and `test_paths`
   - link to `flow-map.md`
4. Update `docs/architecture/flow-maps/README.md` when adding a new canonical flow map.
5. Keep links bidirectional (feature -> canonical flow map, and canonical flow map -> concrete code/test paths).

No code-only refactor is considered complete until this documentation contract is satisfied.

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
17. ToolHandler naming convention is strict: `tool*` for top-level MCP mode/action entry points, `handle*` for sub-action handlers/helpers.
