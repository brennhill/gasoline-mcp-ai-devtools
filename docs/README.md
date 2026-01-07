# Gasoline Documentation

> **For LLM agents:** Start here to navigate all project documentation. Each section links to the canonical location for that topic.

## Quick Reference

| What you need | Where to look |
|---------------|---------------|
| Project rules & commands | [`/CLAUDE.md`](../CLAUDE.md) |
| System architecture | [`.claude/docs/architecture.md`](../.claude/docs/architecture.md) |
| Feature index (all features + status) | [`features/FEATURE-INDEX.md`](features/FEATURE-INDEX.md) |
| UAT test plan | [`core/UAT-TEST-PLAN.md`](core/UAT-TEST-PLAN.md) |
| Known issues | [`/KNOWN-ISSUES.md`](../KNOWN-ISSUES.md) |
| Changelog | [`/CHANGELOG.md`](../CHANGELOG.md) |
| Release process | [`/RELEASE.md`](../RELEASE.md) |

## Directory Structure

```
docs/
├── README.md                  ← You are here (master index)
├── core/                      ← Cross-product specs, API specs, UAT
│   ├── PRODUCT_SPEC.md        ← Product requirements
│   ├── TECH_SPEC.md           ← Technical specification
│   ├── UAT-TEST-PLAN.md       ← Canonical UAT checklist
│   ├── async-command-api.yaml ← OpenAPI 3.0 spec
│   └── in-progress/           ← Active tracking docs, issue trackers
├── features/                  ← All feature documentation
│   ├── FEATURE-INDEX.md       ← Machine-readable feature status table
│   └── feature/<name>/        ← Per-feature: PRODUCT_SPEC, TECH_SPEC, review
├── adrs/                      ← Architecture Decision Records
│   ├── ADRs.md                ← ADR index
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
├── privacy.md                 ← Privacy policy
├── troubleshooting.md         ← Common issues
└── pypi-distribution.md       ← PyPI packaging docs
```

## Documentation Lifecycle

```
Proposed → In-Progress → Shipped → (Deprecated → Archived)
```

1. **Proposed** — Feature has `PRODUCT_SPEC.md` and `TECH_SPEC.md` in `features/feature/<name>/`. Status: `proposed` in frontmatter.
2. **In-Progress** — Tracking docs live in `core/in-progress/`. Feature status updated to `in-progress`.
3. **Shipped** — Implementation complete, tests passing. Status updated to `shipped` with version number.
4. **Deprecated** — Feature sunset. Status updated to `deprecated`.
5. **Archived** — Docs moved to `archive/`. Removed from feature index.

## For LLM Agents

### Adding a new feature
1. Copy `templates/FEATURE-TEMPLATE.md` to `features/feature/<name>/PRODUCT_SPEC.md`
2. Copy and fill `TECH_SPEC.md` using the same template structure
3. Create `ADR-<name>.md` from `templates/ADR-TEMPLATE.md` in `adrs/`
4. Add entry to `features/FEATURE-INDEX.md`
5. **MANDATORY**: Get spec review before implementation (see `.claude/docs/spec-review.md`)

### Creating a release
1. Update `CHANGELOG.md` using the structured format
2. Update `KNOWN-ISSUES.md` if issues are resolved
3. Follow `/RELEASE.md` process

### Finding implementation details
- Go source: `cmd/dev-console/*.go`
- Extension source: `extension/*.js`
- MCP tool definitions: `cmd/dev-console/tools.go`
- Test files: `tests/`
