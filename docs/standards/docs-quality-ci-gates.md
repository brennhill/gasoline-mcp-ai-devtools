---
doc_type: process_standard
status: active
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
owners:
  - Brenn
---

# Docs Quality CI Gates

This project enforces docs quality in three phases.

## Phase 1: Trust-Breaking Integrity

Command:

```bash
npm run docs:gate:phase1
```

Blocks merge on:

- Missing YAML frontmatter
- Broken links and unresolved references
- Frontmatter parse errors
- Missing required review-date fields

## Phase 2: Freshness

Command:

```bash
npm run docs:gate:phase2
```

Includes Phase 1 checks, plus:

- Stale review dates (`last_reviewed` / `last-verified` older than policy window)

## Phase 3: Reference Executability

Command:

```bash
npm run docs:gate:phase3
```

Includes Phase 2 checks, plus:

- Every tool mode/action has a corresponding executable example section
- Each section includes:
  - Minimal call
  - Expected response shape
  - Failure example and fix

## Additional Determinism Gates

```bash
npm run docs:lint:site-content-ids
```

Blocks duplicate content IDs/slugs in `gokaboom.dev/src/content/docs`.
