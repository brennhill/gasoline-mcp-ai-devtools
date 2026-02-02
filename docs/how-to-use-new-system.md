---
status: active
scope: documentation/guide
ai-priority: high
tags: [documentation, how-to, workflow, automation]
last-verified: 2026-01-31
---

# How to Use the New Documentation System

**Date:** 2026-01-31
**For:** Developers and AI agents
**Purpose:** Quick start guide to new docs system

---

## TL;DR

**Before every commit:**
```bash
python3 scripts/lint-documentation.py  # Fix errors
python3 scripts/generate-feature-navigation.py  # Update index
git add docs/ && git commit -m "feat: ... docs: ..."
```

**Remember:** Update `last-verified: YYYY-MM-DD` in frontmatter (today's date)

---

## Navigation

### I'm an LLM Agent Looking for Information

**Start here:** `docs/features/README.md` (optimized for AI discovery)

**Then use:**
- `docs/features/feature-navigation.md` — Find any feature by status
- `docs/cross-reference.md` — Find related documents
- `docs/features/FEATURE-index.md` — Status table of all features

**For system design:**
- `.claude/refs/architecture.md` (canonical)
- `docs/core/codebase-canon-v5.3.md` (baseline reference)

**For processes:**
- `.claude/docs/documentation-maintenance.md` (mandatory workflow)
- `.claude/docs/startup-checklist.md` (before you start)

---

### I'm a Developer Working on a Feature

**Step 1: Understand the scope**
```bash
# Read feature requirements
cat docs/features/feature/<name>/product-spec.md

# Read implementation guide
cat docs/features/feature/<name>/tech-spec.md

# Understand tests needed
cat docs/features/feature/<name>/qa-plan.md
```

**Step 2: While coding**
- Update tech-spec.md as you implement
- Add code references: `cmd/dev-console/tools.go:observe()`
- Document APIs and data structures

**Step 3: Before committing**
```bash
# Update last-verified date
vim docs/features/feature/<name>/tech-spec.md  # Change date to today

# Check for lint errors
python3 scripts/lint-documentation.py

# Regenerate navigation (if you added/changed features)
python3 scripts/generate-feature-navigation.py

# Commit
git add docs/
git commit -m "feat: ... docs: Updated TECH_SPEC, QA_PLAN, etc."
```

---

### I'm Fixing a Bug

**Step 1: Find the issue**
```bash
grep -r "Issue #123" docs/core/known-issues.md
```

**Step 2: Read the context**
```bash
cat docs/features/feature/<name>/product-spec.md  # Expected behavior
cat docs/features/feature/<name>/tech-spec.md     # Current implementation
```

**Step 3: After fixing**
- Update known-issues.md (mark as fixed)
- Update qa-plan.md (add regression test)
- Update `last-verified` date
- Commit with docs

---

### I'm Adding a New Feature

**Step 1: Create the structure**
```bash
mkdir docs/features/feature/my-feature
cp docs/templates/FEATURE-TEMPLATE.md docs/features/feature/my-feature/product-spec.md
```

**Step 2: Write specs**
- Fill product-spec.md (user stories, APIs)
- Fill tech-spec.md (design, code references)
- Fill qa-plan.md (test scenarios)
- Verify YAML frontmatter on all files

**Step 3: Get approval**
- Follow spec review process (`.claude/docs/spec-review.md`)
- Get principal engineer review

**Step 4: Implement**
- Follow TDD workflow (tests first)
- Update docs as you code
- Add code references

**Step 5: Before shipping**
```bash
# Update status to shipped
vim docs/features/feature/my-feature/product-spec.md
# Change: status: proposed → status: shipped

# Update navigation
python3 scripts/generate-feature-navigation.py

# Lint check
python3 scripts/lint-documentation.py

# Commit
git add docs/features/feature/my-feature/
git commit -m "feat: Add my-feature

- docs: PRODUCT_SPEC, TECH_SPEC, QA_PLAN (status: shipped)
- Updated FEATURE-index.md
- All docs marked last-verified: $(date +%Y-%m-%d)
"
```

---

## Tools Reference

### Lint Checker
```bash
python3 scripts/lint-documentation.py
```

**Checks:**
- ✅ All markdown links point to existing files
- ✅ Code references (file.go:function) exist
- ✅ YAML frontmatter is valid
- ✅ `last-verified` dates are recent (< 30 days)
- ❌ Blocks commits if errors found

**Errors:** Must fix before committing
**Warnings:** Should fix (stale docs, incomplete frontmatter)

### Feature Navigation Generator
```bash
python3 scripts/generate-feature-navigation.py
```

**Generates:**
- `docs/features/feature-navigation.md` (auto-index)
- Scans all feature folders
- Groups by status (shipped/in-progress/proposed)
- Lists all files per feature

**When to run:** After adding/removing features or changing status

---

## Metadata Explained

Every feature doc has YAML frontmatter:

```yaml
---
status: shipped
  # shipped: implemented and released
  # in-progress: being developed
  # proposed: designed but not started
  # deprecated: no longer supported

scope: feature/<name>/implementation
  # Where in the hierarchy does this doc live?
  # Examples:
  # - feature/<name> (the feature)
  # - feature/<name>/implementation (how to build)
  # - feature/<name>/qa (how to test)

ai-priority: high
  # high: essential for understanding the feature
  # medium: useful context
  # low: reference material

tags: [feature, security, shipped]
  # Searchable keywords
  # Examples: security, performance, testing, shipped, proposed

relates-to: [tech-spec.md, qa-plan.md]
  # What other docs are related?
  # Helps navigate between connected documents

last-verified: 2026-01-31
  # MUST update this whenever you change the doc
  # Format: YYYY-MM-DD (today's date)
  # Lint checker warns if > 30 days old

incomplete: true  # (optional)
  # Marks this as a placeholder/TODO
  # Tells AI agents to be skeptical

canonical: true  # (optional)
  # Is this the authoritative source for this topic?
  # Example: architecture.md is canonical
```

---

## Common Tasks

### Find all shipped features
```bash
grep -r "status: shipped" docs/features/feature/*/product-spec.md | wc -l
```

### Find stale docs (not updated in 30+ days)
```bash
python3 scripts/lint-documentation.py | grep "stale"
```

### Find docs related to a feature
```bash
grep -r "relates-to:.*my-feature" docs/features/
```

### Update all last-verified dates
```bash
# When you've updated multiple docs, refresh the dates:
for file in docs/features/feature/*/tech-spec.md; do
  sed -i '' "s/last-verified: .*/last-verified: $(date +%Y-%m-%d)/" "$file"
done
```

### Check for broken links
```bash
python3 scripts/lint-documentation.py | grep "broken link"
```

---

## Quality Gates (Pre-Commit)

These checks happen BEFORE you commit:

1. **Lint Check** — Run `python3 scripts/lint-documentation.py`
   - ❌ Fails if broken links found
   - ❌ Fails if frontmatter invalid
   - ⚠️  Warns if docs > 30 days old

2. **Metadata Check** — Every touched file must have:
   - `status` field
   - `last-verified` updated to today
   - `ai-priority` set
   - Valid YAML

3. **Code Reference Check** — All `file.go:function()` references must exist

**If any check fails:** Fix before committing

---

## File Structure Quick Reference

```
docs/
├── README.md (master index)
├── CROSS-REFERENCE.md (doc relationships)
├── OPTIMIZATION-SUMMARY-2026-01-31.md (what was done)
├── HOW-TO-USE-NEW-SYSTEM.md (this file)
├── features/
│   ├── README.md (LLM-optimized guide)
│   ├── FEATURE-index.md (status table)
│   ├── feature-navigation.md (auto-generated index)
│   └── feature/<name>/ (per-feature docs)
│       ├── product-spec.md (requirements)
│       ├── tech-spec.md (implementation)
│       ├── qa-plan.md (tests)
│       └── <name>-review.md (issues found)
├── core/
│   ├── release.md (process)
│   ├── known-issues.md (blockers)
│   ├── UAT-v5.3-CHECKLIST.md (canonical UAT)
│   └── CODEBASE-CANON-V5.3.md (baseline reference)
├── adrs/
│   ├── adrs.md (index)
│   └── ADR-<name>.md (per-feature decisions)
└── archive/
    ├── index.md (what's archived & why)
    ├── stale-tracking/ (abandoned features)
    └── 2026-01-31-*.md (dated archives)

scripts/
├── generate-feature-navigation.py (auto-gen)
└── lint-documentation.py (quality check)

.claude/docs/
├── DOCUMENTATION-MAINTENANCE.md (mandatory workflow)
├── STARTUP-CHECKLIST.md (session startup)
└── ... (other process docs)
```

---

## Next Time You Start

**Before you start working:**

1. Read `.claude/docs/startup-checklist.md`
2. Read `.claude/docs/documentation-maintenance.md`
3. Read the relevant feature docs (PRODUCT → TECH → QA)
4. Start coding

**Before you commit:**

1. Update `last-verified` in all touched docs (today's date)
2. Run `python3 scripts/lint-documentation.py` (fix errors)
3. Run `python3 scripts/generate-feature-navigation.py` (if needed)
4. Commit docs with code: `git add docs/` + `git commit`

---

## Related Documents

- `.claude/docs/documentation-maintenance.md` — Complete workflow (mandatory reading)
- `.claude/docs/startup-checklist.md` — Session startup (read first)
- `docs/features/README.md` — Feature docs guide (for LLMs)
- `docs/cross-reference.md` — Doc relationships
- `docs/optimization-summary-2026-01-31.md` — What changed

---

**Remember: Documentation is not optional. Every commit updates docs.** ✨
