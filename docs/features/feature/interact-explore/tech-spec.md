---
doc_type: tech-spec
feature_id: feature-interact-explore
status: shipped
owners: []
last_reviewed: 2026-03-03
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
---

# Interact Tech Spec (TARGET)

## Dispatcher
- Entry: `toolInteract` in `cmd/dev-console/tools_interact_entrypoint.go`
- Action routing:
- named action handlers (`interactActionHandler.interactDispatch` map in `cmd/dev-console/tools_interact_dispatch.go`)
- DOM primitives (`domPrimitiveActions` -> `handleDOMPrimitive`)

### Handler Decomposition (Issue #402)
- `ToolHandler` remains the MCP entrypoint for `interact`, then delegates to `interactActionHandler` for:
- dispatch map construction/caching
- action list generation for schema/error hints
- jitter application policy
- named vs DOM-primitive action routing
- `list_interactive` orchestration + index metadata/truncation post-processing
- DOM primitive selector resolution (`index`/`index_generation`) before queueing `dom_action`
- browser action implementation internals (navigate/refresh/back/forward/new_tab/switch_tab/activate_tab/close_tab, highlight, execute_js, subtitle/screenshot aliases)
- URL rewrite (`gasoline-insecure://`) and perf snapshot staging for `perf_diff`
- interact dispatch map now points directly to `interactActionHandler` methods (ToolHandler browser wrappers removed)

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
- `cmd/dev-console/tools_interact_action_handler.go`
- `cmd/dev-console/tools_interact_entrypoint.go`
- `cmd/dev-console/tools_interact_dispatch.go`
- `cmd/dev-console/tools_interact_draw.go`
- `cmd/dev-console/tools_interact_upload.go`
- `src/background/pending-queries.ts`
- `src/background/query-execution.ts`
