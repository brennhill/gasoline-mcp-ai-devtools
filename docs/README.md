# Gasoline MCP Documentation

> **For LLM agents:** Start here to navigate all project documentation. Each section links to the canonical location for that topic.

## Quick Reference

| What you need | Where to look |
|---------------|---------------|
| Project rules & commands | [`/claude.md`](../claude.md) |
| System architecture | [`.claude/refs/architecture.md`](../.claude/refs/architecture.md) |
| Feature index (all features + status) | [`features/FEATURE-index.md`](features/FEATURE-index.md) |
| UAT test plan (current) | [`core/uat-v5.3-checklist.md`](core/uat-v5.3-checklist.md) |
| Known issues | [`core/known-issues.md`](core/known-issues.md) |
| Changelog | [`/CHANGELOG.md`](../CHANGELOG.md) |
| Release process | [`core/release.md`](core/release.md) |

## Directory Structure

```
docs/
├── README.md                  ← You are here (master index)
├── core/                      ← Cross-product specs, API specs, UAT
│   ├── UAT-v5.3-CHECKLIST.md  ← Current UAT checklist (canonical)
│   ├── RELEASE.md             ← Release process & quality gates
│   ├── known-issues.md        ← Current blockers & issues
│   ├── async-command-api.yaml ← OpenAPI 3.0 spec
│   └── in-progress/           ← Active tracking docs, issue trackers
├── features/                  ← All feature documentation
│   ├── FEATURE-index.md       ← Machine-readable feature status table
│   └── feature/<name>/        ← Per-feature: PRODUCT_SPEC, TECH_SPEC, review
├── adrs/                      ← Architecture Decision Records
│   ├── adrs.md                ← ADR index
│   └── ADR-<feature>.md       ← One ADR per feature
├── templates/                 ← Templates for new docs
│   ├── FEATURE-TEMPLATE.md
│   ├── ADR-TEMPLATE.md
│   ├── RELEASE-NOTES-TEMPLATE.md
│   └── KNOWN-ISSUE-TEMPLATE.md
├── mcp-integration/           ← IDE-specific setup guides
│   ├── claude-code.md
│   ├── cursor.md
│   ├── windsurf.md
│   ├── zed.md
│   └── claude-desktop.md
├── benchmarks/                ← Performance data
├── assets/                    ← Images, CSS
├── archive/                   ← Deprecated/legacy docs
├── getting-started.md         ← User onboarding
├── roadmap.md                 ← Feature roadmap
├── gemini-feedback.md         ← Feedback from Gemini agent (2026-02-16)
├── privacy.md                 ← Privacy policy
├── troubleshooting.md         ← Common issues
└── pypi-distribution.md       ← PyPI packaging docs
```

## Documentation Lifecycle

```
Proposed → In-Progress → Shipped → (Deprecated → Archived)
```

1. **Proposed** — Feature has `product-spec.md` and `tech-spec.md` in `features/feature/<name>/`. Status: `proposed` in frontmatter.
2. **In-Progress** — Tracking docs live in `core/in-progress/`. Feature status updated to `in-progress`.
3. **Shipped** — Implementation complete, tests passing. Status updated to `shipped` with version number.
4. **Deprecated** — Feature sunset. Status updated to `deprecated`.
5. **Archived** — Docs moved to `archive/`. Removed from feature index.

## Quality & Verification

- [quality-standards.md](quality-standards.md) — Complete quality guide (20 sections)
- [quality-quick-reference.md](quality-quick-reference.md) — One-page quality checklist
- [post-refactor-verification.md](post-refactor-verification.md) — Verification after refactoring
- [5.4-todo.md](5.4-todo.md) — v5.4 refactoring notes and prevention rules

## For LLM Agents

### Adding a new feature
1. Copy `templates/FEATURE-TEMPLATE.md` to `features/feature/<name>/product-spec.md`
2. Copy and fill `tech-spec.md` using the same template structure
3. Create `ADR-<name>.md` from `templates/ADR-TEMPLATE.md` in `adrs/`
4. Add entry to `features/FEATURE-index.md`
5. **MANDATORY**: Get spec review before implementation (see `.claude/docs/spec-review.md`)

### Creating a release
1. Update `CHANGELOG.md` using the structured format
2. Update `known-issues.md` if issues are resolved
3. Follow `/release.md` process

### Finding implementation details
- Go source: `cmd/dev-console/*.go`
- Extension source: `extension/*.js`
- MCP tool definitions: `cmd/dev-console/tools.go`
- Test files: `tests/`
