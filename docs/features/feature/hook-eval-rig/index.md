---
doc_type: feature_index
feature_id: feature-hook-eval-rig
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-07
code_paths:
  - cmd/hooks/eval_test.go
  - internal/hook/eval/
  - scripts/eval/
test_paths:
  - cmd/hooks/eval_test.go
  - internal/hook/eval/eval_test.go
  - scripts/eval/
---

# Hook Eval Rig

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | proposed                                |
| **Binary**    | gasoline-hooks                          |
| **Command**   | `gasoline-hooks eval`                   |
| **Purpose**   | Measure token savings, accuracy, and redundancy elimination |
| **Parent**    | [Quality Gates](../quality-gates/index.md) |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)

## Summary

A deterministic evaluation framework for measuring the real-world impact of gasoline-hooks on AI coding sessions. Three tiers of testing: unit-level hook evals (synthetic inputs, known-good outputs), integration evals (controlled codebases with known dependency graphs), and live session metrics (real usage data aggregated across sessions).

The rig answers: "Do these hooks actually make AI coding better, and by how much?"
