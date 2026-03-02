<!-- gitnexus:start -->
# GitNexus MCP

This project is indexed by GitNexus as **gasoline** (16459 symbols, 45825 relationships, 300 execution flows).

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
