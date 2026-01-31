---
status: archived
scope: stale-tracking
ai-priority: low
tags: [archived, stale, tab-tracking, uat]
archived-date: 2026-01-31
reason: Tab tracking feature appears abandoned after Jan 27 UAT. Docs archived for historical reference.
---

# Stale Tab Tracking Documentation

This folder contains documentation for the **tab tracking feature** that appears to have been abandoned or completed without final closure (Jan 27, 2026).

## What Happened?

The tab tracking feature had extensive planning docs (PRODUCT_SPEC, TECH_SPEC, QA_PLAN) and UAT tracking, but was moved to archive because:

1. **No recent updates** — Last modified Jan 28 (3+ days stale)
2. **Incomplete closure** — uat-complete.md exists but contains TODOs ("// TODO: dom queries need proper implementation")
3. **No commit closure** — No git commits referencing completion
4. **Unclear status** — Feature appears in-progress but not documented as shipped or abandoned

## Contents

- **SPEC-TRACK-TAB-*.md** (4 files) — Product/technical/edge-case specs for tab tracking feature
- **tracking-analysis.md** — Analysis of tracking mechanics
- **uat-track-tab.md** — UAT test plan for tab tracking
- **uat-results-2026-01-27.md** — UAT test results from Jan 27
- **uat-schema-improvements.md** — Proposed schema improvements
- **uat-complete.md** — Completion marker (but with incomplete TODOs)
- **UAT-ISSUES-TRACKER.md** — Issues found during UAT

## Recovery Path (If Needed)

If tab tracking feature needs to resume:

1. Review spec-track-tab-summary.md for feature overview
2. Check git history for last commits related to tab tracking
3. Update FEATURE-index.md to reflect current status
4. Move relevant docs back to `/docs/features/tracked-tabs/` or appropriate location
5. Reopen as active feature with proper status tracking

## Related Active Docs

- [docs/features/FEATURE-index.md](../../features/FEATURE-index.md) — Current feature status
- [docs/core/known-issues.md](../../core/known-issues.md) — Current blockers
