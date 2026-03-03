---
doc_type: qa-plan
feature_id: feature-page-structure-detection
status: proposed
last_reviewed: 2026-03-03
---

# Page Structure Detection QA Plan

## Automated Coverage
- `cmd/dev-console/tools_analyze_page_structure_test.go`

## Required Scenarios
1. Framework/global detection success path.
2. CSP-restricted fallback path with reduced confidence.
3. Routing mode classification for hash/history/static pages.
4. Response shape and required fields remain stable.
