---
doc_type: tech-spec
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

# Interact Tech Spec (TARGET)

## Dispatcher
- Entry: `toolInteract` in `cmd/dev-console/tools_interact.go`
- Action routing:
- named action handlers (`interactDispatch` map)
- DOM primitives (`domPrimitiveActions` -> `handleDOMPrimitive`)

## Query-Type Mapping
- `navigate/refresh/back/forward/new_tab` -> `browser_action`
- `execute_js` -> `execute`
- DOM primitive actions -> `dom_action`
- `highlight` -> `highlight`
- `subtitle` -> `subtitle`
- `upload` -> `upload`
- `draw_mode_start` -> `draw_mode`
- `record_start` / `record_stop` -> `record_start` / `record_stop`

## Execution Pipeline
1. Go queues command with correlation ID.
2. Extension receives command through `/sync`.
3. `pending-queries.ts` resolves target tab and executes.
4. Result is posted back through `/sync` command results.
5. Server completes command and returns inline or via observe polling.

## Additional Mechanics
- `query-execution.ts` handles world-aware `execute_js` fallback logic.
- DOM primitives run through extension-side DOM dispatch utilities.
- Performance diff enrichment is attached at command-result formatting time.

## Code Anchors
- `cmd/dev-console/tools_interact.go`
- `cmd/dev-console/tools_interact_draw.go`
- `cmd/dev-console/tools_interact_upload.go`
- `src/background/pending-queries.ts`
- `src/background/query-execution.ts`
