---
status: active
scope: documentation/navigation
ai-priority: high
tags: [features, navigation, structure, index]
last-verified: 2026-01-31
---

# Features Documentation Guide

**For LLM Agents:** This guide explains how to navigate feature documentation and find the information you need for development tasks.

---

## Quick Navigation

- **[FEATURE-index.md](FEATURE-index.md)** — Machine-readable status table of all features (shipped, in-progress, proposed)
- **[feature/](feature/)** — All feature documentation folders

---

## Feature Folder Structure

Each feature has its own folder at `docs/features/feature/<feature-name>/` with the following standard files:

### Standard Files per Feature

| File | Purpose | Audience | Status |
|------|---------|----------|--------|
| **product-spec.md** | What the feature does (user stories, requirements, API spec) | Product managers, LLMs | Required |
| **tech-spec.md** | How it's implemented (architecture, data structures, algorithms) | Developers, LLMs | Required |
| **qa-plan.md** | How to test it (test scenarios, acceptance criteria, regression tests) | QA, developers, LLMs | Required |
| **<feature>-review.md** | Principal engineer review (issues found, recommendations, implementation roadmap) | Architects, senior devs | Optional |
| **ADR-<feature>.md** | Architecture decision record (in `/docs/adrs/`) | Architects, historians | Optional |

### File Naming Conventions

- **Lowercase, hyphenated:** `feature-name-review.md` (NOT `FEATURE_NAME_review.md`)
- **Descriptive names:** Use full words (`security-review.md`, not `sec-review.md`)
- **No redundant prefixes:** File is already in `feature/<name>/`, so prefix with feature name only if ambiguous

---

## How to Find Documentation

### As an LLM, I Need to Know...

#### "What are all the features?"
→ Read **[FEATURE-index.md](FEATURE-index.md)**
- Status column shows: `shipped`, `in-progress`, `proposed`
- Status `deprecated` means old feature, check archive

#### "What does feature X do?"
→ Read `docs/features/feature/<feature-name>/product-spec.md`
- User stories explain the "why"
- API examples show the "how to use"
- Acceptance criteria define "done"

#### "How is feature X implemented?"
→ Read `docs/features/feature/<feature-name>/tech-spec.md`
- Architecture section explains design decisions
- Code references (filename:line_number) point to implementation
- Performance requirements define acceptable behavior

#### "How do I test feature X?"
→ Read `docs/features/feature/<feature-name>/qa-plan.md`
- Test categories organize scenarios by area
- Each scenario has clear inputs, outputs, acceptance criteria
- Regression tests show what existing behavior to protect
- Success criteria define release readiness

#### "What issues were found in feature X?"
→ Read `docs/features/feature/<feature-name>/<feature>-review.md`
- Executive summary gives high-level assessment
- Critical issues are flagged with priority
- Recommendations show priority order for fixes
- Implementation roadmap shows phase ordering

#### "What architectural decisions were made?"
→ Read `docs/adrs/ADR-<feature>.md` or tech-spec.md "Architectural Decisions" section
- ADRs explain the "why" behind major choices
- Trade-offs are documented
- Alternatives considered are listed

---

## Metadata & AI Discovery

### YAML Frontmatter

Every feature doc includes metadata for AI discovery:

```yaml
---
status: [proposed|in-progress|shipped|deprecated]
version-applies-to: v5.3, v6.0+
scope: feature/<feature-name>
ai-priority: [high|medium|low]
tags: [tag1, tag2, tag3]
relates-to: [related-doc.md, other-doc.md]
supersedes: [old-doc.md]
last-verified: YYYY-MM-DD
incomplete: true  # Optional, marks placeholder docs
---
```

#### What this enables:
- Find all HIGH priority docs: grep `ai-priority: high`
- Find related docs: check `relates-to` field
- Verify freshness: check `last-verified` date
- Identify placeholders: check `incomplete: true`

---

## Feature Status Legend

### Status: `shipped`
Feature is implemented and released in a version.
- PRODUCT_SPEC complete
- TECH_SPEC complete
- QA_PLAN complete
- In active use

**What I (the LLM) do:** Read PRODUCT_SPEC to understand behavior, TECH_SPEC for implementation details, QA_PLAN for regression tests.

### Status: `in-progress`
Feature is being actively developed.
- Some or all specs may be incomplete
- Code may be partial or under review
- Tests may be failing

**What I (the LLM) do:** Check `incomplete: true` flag, read docs as reference, assume implementation details may change.

### Status: `proposed`
Feature is approved by stakeholders but not started.
- PRODUCT_SPEC complete
- TECH_SPEC may be placeholder
- QA_PLAN may be placeholder

**What I (the LLM) do:** Read PRODUCT_SPEC to understand requirements, use TECH_SPEC as starting reference for implementation.

### Status: `deprecated`
Feature is no longer supported or scheduled for removal.
- Check `supersedes:` field to find replacement
- May be moved to `/docs/archive/`

**What I (the LLM) do:** Skip unless specifically asked about historical behavior. Use replacement feature if available.

---

## Common Patterns

### Pattern: "Add a new feature"

1. Create folder: `docs/features/feature/<feature-name>/`
2. Copy template: `cp docs/templates/FEATURE-TEMPLATE.md docs/features/feature/<feature-name>/product-spec.md`
3. Fill PRODUCT_SPEC with user stories and requirements
4. Create TECH_SPEC (can start as skeleton)
5. Create QA_PLAN (can start as skeleton)
6. Add entry to FEATURE-index.md with status `proposed`
7. Create ADR at `docs/adrs/ADR-<feature-name>.md`
8. **REQUIRED:** Get spec review before implementation (see `.claude/docs/spec-review.md`)

### Pattern: "Implement a proposed feature"

1. Read PRODUCT_SPEC to understand requirements
2. Read TECH_SPEC for architectural guidance
3. Check TECH_SPEC for "TODO" sections (incomplete parts)
4. Use QA_PLAN to understand acceptance criteria
5. Search for code references (filename:line_number) in specs
6. If TECH_SPEC incomplete, update it as you implement
7. Write code following TECH_SPEC design
8. Run tests from QA_PLAN
9. Get principal engineer review (see QA_PLAN for checklist)

### Pattern: "Fix a bug in shipped feature"

1. Read PRODUCT_SPEC to understand expected behavior
2. Read TECH_SPEC, specifically "Known Limitations" section
3. Check feature review doc for known issues that might be related
4. Check QA_PLAN for regression tests related to bug
5. Run related tests before fixing
6. Update QA_PLAN with new regression test
7. Update TECH_SPEC "Known Limitations" if bug is unfixable edge case

### Pattern: "Review a feature implementation"

1. Read PRODUCT_SPEC — what should it do?
2. Read TECH_SPEC — how should it do it?
3. Compare code to TECH_SPEC — does it match design?
4. Read QA_PLAN — are tests comprehensive?
5. Read feature review doc — what issues were flagged?
6. Check git history — are all TODOs resolved?
7. Verify all ADR items are addressed

---

## Document Quality Checklist

### For Me (LLM) to Trust a Doc

- ✅ Has YAML frontmatter with `status` and `last-verified` date
- ✅ Recent `last-verified` (not more than 30 days old)
- ✅ No `incomplete: true` flag (unless clearly marked as placeholder)
- ✅ Code references use `file.go:line_number` format
- ✅ Related docs are listed in `relates-to` field
- ✅ Specs reference each other (PRODUCT_SPEC → TECH_SPEC → QA_PLAN)

### Red Flags (Be Suspicious)

- ❌ No frontmatter metadata
- ❌ `last-verified` more than 30 days old
- ❌ `incomplete: true` but used as reference
- ❌ References to deleted or archived files
- ❌ Code references that don't exist (file.go:line_number)
- ❌ Contradicts information in codebase

---

## Codebase Truth

### The codebase is the ground truth.

If documentation contradicts code:
1. Assume code is current (docs may be stale)
2. Update doc to match code
3. Add `last-verified: YYYY-MM-DD` with today's date
4. Check if code should be updated instead (rare)

---

## Related Documents

### For humans:
- `FEATURE-index.md` — Status table with brief descriptions

### For architects:
- `.claude/refs/architecture.md` — System design and 5-tool constraint
- `docs/adrs/adrs.md` — Architecture decision record index

### For developers:
- `.claude/docs/testing.md` — TDD workflow
- `.claude/docs/spec-review.md` — Spec review process

### For releases:
- `docs/core/release.md` — Release process and quality gates
- `docs/core/known-issues.md` — Current blockers

---

## For AI Agents (Extended Reference)

### When Searching Docs

#### Search strategy:
1. Always start with FEATURE-index.md (quick status lookup)
2. If status is `shipped`, read PRODUCT_SPEC + TECH_SPEC + QA_PLAN
3. If status is `proposed`, read PRODUCT_SPEC + placeholder specs
4. If status is `in-progress`, check `incomplete: true` flag
5. Always check `relates-to` field for context

#### Cursor navigation:
```javascript
// All shipped features and their tech specs:
grep -r "status: shipped" docs/features/feature/*/
grep -l "tech-spec.md" docs/features/feature/*/

// All high-priority docs:
grep -r "ai-priority: high" docs/features/

// All docs updated in last 7 days:
find docs -name "*.md" -mtime -7
```

### API Contract Reference

When implementing features:
1. Read PRODUCT_SPEC "API" section
2. Check TECH_SPEC for data structure definitions
3. Cross-reference with `docs/core/async-command-api.yaml` (OpenAPI spec)
4. Compare to implementation in `cmd/dev-console/tools.go`

### Testing Reference

When writing tests:
1. Read QA_PLAN "Test Categories" section
2. Look for similar features' QA_PLAN for patterns
3. Check `docs/core/uat-v5.3-checklist.md` for UAT examples
4. Reference `.claude/refs/testing-examples.md` for TDD patterns

---

## Questions?

**For developers:** Check spec-related docs or ask principal engineer
**For architects:** Check ADRs or `.claude/refs/architecture.md`
**For LLMs (me):** Use `relates-to` field to find connected docs, check `last-verified` for staleness
