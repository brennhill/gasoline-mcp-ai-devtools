---
doc_type: documentation_standard
scope: docs
status: active
last_reviewed: 2026-03-02
---

# LLM Flow Map Placement Best Practice

## Goal

Maximize retrieval quality while minimizing documentation drift.

## Canonical Pattern (Hybrid)

1. Keep one canonical flow map per subsystem in `docs/architecture/flow-maps/`.
2. Add a feature-local pointer file (`flow-map.md`) in each relevant feature directory.
3. Link feature `index.md` to local `flow-map.md`.
4. Link each pointer file to exactly one canonical flow map.

## Why This Works for LLMs

- Improves discoverability from both architecture-centric and feature-centric queries.
- Keeps semantic chunks small (feature pages stay concise).
- Avoids conflicting duplicates that reduce answer reliability.

## Required Sections for Canonical Flow Maps

- Scope
- Entrypoints
- Primary Flow
- Error and Recovery Paths
- State and Contracts
- Code Paths
- Test Paths
- Edit Guardrails

## Anti-Patterns

- Duplicating full flow content in multiple feature folders.
- Maintaining separate, divergent copies of the same flow map.
- Linking directly to deep code without entrypoint context.
