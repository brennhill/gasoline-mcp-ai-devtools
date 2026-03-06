---
doc_type: qa-plan
feature_id: feature-interact-explore
status: shipped
owners: []
last_reviewed: 2026-02-17
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
---

# Interact QA Plan (TARGET)

## Automated Coverage
- `cmd/dev-console/tools_interact_rich_test.go`
- `cmd/dev-console/tools_interact_upload_test.go`
- `cmd/dev-console/tools_interact_state_test.go`
- `cmd/dev-console/tools_interact_draw_test.go`

## Required Scenarios
1. Action dispatch
- Every schema action reaches a valid handler path.

2. Pilot dependency
- Disabled pilot returns structured error for browser-control actions.

3. Async/sync control
- Default sync completion.
- Background/async queued behavior with correlation IDs.

4. DOM primitives
- Required parameter validation (`selector`, `text`, `value`, `name`).
- Command result contains enrichment fields and error propagation.

5. Special actions
- Upload queueing and completion.
- Draw mode start + downstream annotation retrieval.
- Screenshot alias behavior.

## Manual UAT
1. `interact(action:"navigate", url:"https://example.com")`
2. `interact(action:"click", selector:"text=Submit")`
3. `interact(action:"execute_js", script:"document.title")`
4. `interact(action:"upload", selector:"input[type=file]", file_path:"<abs path>")`
5. `interact(action:"draw_mode_start")` then retrieve annotations with analyze.
