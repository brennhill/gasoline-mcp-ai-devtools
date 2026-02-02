---
status: active
scope: documentation/cross-reference
ai-priority: medium
tags: [cross-reference, dependencies, relationships, navigation]
relates-to: [README.md, features/README.md, features/feature-navigation.md]
last-verified: 2026-01-31
---

# Documentation Cross-Reference Index

**For LLM Agents:** Map of document dependencies and relationships. Use this to understand how docs connect and find related information quickly.

---

## Document Dependency Graph

### Core Release & Status Documents

```
RELEASE.md (process)
├── KNOWN-ISSUES.md (blockers)
├── UAT-v5.3-CHECKLIST.md (testing)
└── VERSION-CHECKING.md (deployment)
    └── CHANGELOG.md (history)

KNOWN-ISSUES.md (blockers)
├── FEATURE-index.md (status)
└── features/*/product-spec.md (feature reqs)
```

### Architecture & System Design

```
.claude/refs/architecture.md (canonical system design)
├── .claude/docs/spec-review.md (spec process)
├── .claude/docs/testing.md (TDD workflow)
├── docs/core/mcp-correctness.md (MCP tool rules)
└── docs/adrs/ADR-*.md (feature decisions)
    ├── features/feature/*/tech-spec.md
    └── features/feature/*/product-spec.md
```

### Feature Documentation

```
FEATURE-index.md (canonical status table)
├── features/feature-navigation.md (folder structure)
├── features/README.md (guide for LLMs)
└── features/feature/<name>/
    ├── product-spec.md (requirements)
    │   └── User stories, API design
    ├── tech-spec.md (implementation)
    │   ├── Code references (file.go:line)
    │   └── Performance requirements
    ├── qa-plan.md (testing)
    │   ├── Test scenarios
    │   └── Acceptance criteria
    ├── <name>-review.md (issues & recommendations)
    │   └── Critical issues, implementation roadmap
    └── ADR-<name>.md (in /adrs/) (decisions)
        └── Why this design was chosen
```

### Testing & Quality

```
core/uat-v5.3-checklist.md (canonical UAT)
├── core/release.md (quality gates)
├── features/feature/*/qa-plan.md
└── .claude/docs/testing.md (TDD workflow)
    └── .claude/refs/testing-examples.md (patterns)

.claude/docs/spec-review.md (spec approval)
└── features/feature/*/product-spec.md (feature specs)
```

### Archive & Historical

```
archive/index.md (what's archived & why)
├── archive/stale-tracking/ (abandoned features)
├── archive/2026-01-31-UAT-TEST-PLAN-v*.md (superseded)
└── archive/v*-specification.md (old roadmaps)

ROADMAP-STRATEGY-ANALYSIS.md (v6+ planning)
└── archive/v6-specification.md (earlier draft)
```

---

## Feature Relationships

### Features That Build On Pagination

```
cursor-pagination (shipped)
├── advanced-filtering (proposed)
│   └── Uses pagination for filtered results
├── flow-recording (in-progress)
│   └── Paginate recorded interactions
└── behavioral-baselines (in-progress)
    └── Compare paginated performance metrics
```

### Features That Use Audit Tools

```
sarif-export (shipped)
├── Depends on: accessibility audit
├── security-hardening (shipped)
│   └── Uses SARIF for vulnerability tracking
└── design-audit-archival (proposed)
    └── Exports design compliance as SARIF
```

### Features That Require Security Review

```
api-key-auth (shipped)
├── data-privacy (shipped)
├── security-hardening (shipped)
└── rate-limiting (in-progress)
    └── DDoS/abuse prevention
```

---

## Documentation by Audience

### For Product Managers

**Read these first:**
1. `FEATURE-index.md` — What's shipped vs. proposed
2. `features/feature/<name>/product-spec.md` — Requirements
3. `KNOWN-ISSUES.md` — What's blocking

**Then read:**
- `ROADMAP-STRATEGY-ANALYSIS.md` — Roadmap for v6+
- `features/<name>-review.md` — Issues and risks

### For Developers

**Read these first:**
1. `features/feature/<name>/product-spec.md` — What to build
2. `features/feature/<name>/tech-spec.md` — How to build it
3. `.claude/docs/testing.md` — TDD workflow

**Then read:**
- `features/feature/<name>/qa-plan.md` — Test scenarios
- `.claude/refs/architecture.md` — Design patterns
- Code references in TECH_SPEC (file.go:line)

### For Architects

**Read these first:**
1. `.claude/refs/architecture.md` — System design
2. `docs/adrs/adrs.md` — Decisions index
3. `features/feature/<name>/tech-spec.md` — Implementation

**Then read:**
- `features/<name>-review.md` — Known issues
- `ROADMAP-STRATEGY-ANALYSIS.md` — Future direction
- `docs/core/mcp-correctness.md` — Tool constraints

### For QA/Testers

**Read these first:**
1. `features/feature/<name>/product-spec.md` — Expected behavior
2. `features/feature/<name>/qa-plan.md` — Test scenarios
3. `core/uat-v5.3-checklist.md` — UAT patterns

**Then read:**
- `features/<name>-review.md` — Known issues (regression prevention)
- `KNOWN-ISSUES.md` — Current blockers
- `.claude/refs/testing-examples.md` — Test patterns

---

## Finding Documents

### By Topic

**Release & Versioning:**
- `docs/core/release.md` — Process
- `docs/core/version-checking.md` — Implementation
- `docs/core/known-issues.md` — Blockers
- `CHANGELOG.md` — History

**Feature Status:**
- `FEATURE-index.md` — Complete list with status
- `features/feature-navigation.md` — Folder structure
- `features/README.md` — Navigation guide for LLMs

**Architecture & Design:**
- `.claude/refs/architecture.md` — System architecture
- `docs/adrs/` — Architectural decision records
- `features/feature/*/tech-spec.md` — Component design

**Testing:**
- `core/uat-v5.3-checklist.md` — UAT checklist
- `features/feature/*/qa-plan.md` — Feature tests
- `.claude/docs/testing.md` — TDD workflow
- `.claude/refs/testing-examples.md` — Test patterns

**Security:**
- `features/feature/security-hardening/tech-spec.md`
- `features/feature/api-key-auth/tech-spec.md`
- `features/feature/data-privacy/tech-spec.md`

**Performance:**
- `features/feature/behavioral-baselines/product-spec.md`
- `features/feature/performance-profiling/tech-spec.md`
- `docs/core/uat-v5.3-checklist.md` (performance section)

---

## How to Navigate Document Changes

When a document is updated, trace the impact:

1. **If PRODUCT_SPEC changes** → Update TECH_SPEC, QA_PLAN, and review doc
2. **If TECH_SPEC changes** → Update QA_PLAN, and potentially ADR
3. **If QA_PLAN changes** → Check RELEASE.md quality gates
4. **If status changes** → Update FEATURE-index.md
5. **If feature supersedes another** → Mark old feature deprecated, update `supersedes:` field

---

## Metadata Field Reference

Every doc includes YAML frontmatter with these fields:

```yaml
status: [proposed|in-progress|shipped|deprecated|active]
scope: [feature|process|testing|architecture|documentation]
ai-priority: [high|medium|low]
tags: [tag1, tag2]
version-applies-to: v5.3+, v6.0
relates-to: [doc1.md, doc2.md]
supersedes: [old-doc.md]
incomplete: true  # Placeholder/TODO
last-verified: 2026-01-31
canonical: true   # Authoritative source
```

**Use these fields to:**
- Find related docs: grep `relates-to:`
- Find replacements: grep `supersedes:`
- Find placeholders: grep `incomplete: true`
- Verify freshness: check `last-verified` date
- Find canonical source: grep `canonical: true`

---

## Document Health Checks

### For LLMs: Validate Before Using

- ✅ Has YAML frontmatter
- ✅ `last-verified` is recent (< 30 days)
- ✅ No `incomplete: true` flag (unless placeholder)
- ✅ Code references use `file.go:line` format
- ✅ No broken links (check `relates-to`, `supersedes`)
- ✅ Metadata `status` matches content

### Red Flags (Be Suspicious)

- ❌ No frontmatter
- ❌ `last-verified` > 30 days old
- ❌ References deleted files
- ❌ Contradicts codebase
- ❌ Status says "shipped" but has TODO comments

---

## Quick Links Reference

**Master indexes:**
- `README.md` — Master documentation index
- `features/FEATURE-index.md` — Feature status table
- `features/feature-navigation.md` — Feature folder guide
- `features/README.md` — LLM-optimized navigation guide

**Core processes:**
- `core/release.md` — Release process
- `core/known-issues.md` — Current blockers
- `core/uat-v5.3-checklist.md` — UAT test plan

**Architecture:**
- `.claude/refs/architecture.md` — System design
- `docs/adrs/adrs.md` — Decision records index

**Getting started:**
- `.claude/docs/startup-checklist.md` — Session startup rules
- `.claude/docs/testing.md` — TDD workflow
- `.claude/docs/spec-review.md` — Spec approval process

---

## Related Documents

- `README.md` — Master documentation index
- `features/README.md` — Features guide for LLMs
- `features/feature-navigation.md` — Feature structure guide
- `.claude/docs/documentation-policy.md` — Documentation standards
