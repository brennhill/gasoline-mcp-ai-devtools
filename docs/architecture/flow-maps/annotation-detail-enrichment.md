---
doc_type: flow_map
scope: annotation_detail_enrichment
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
---

# Annotation Detail Enrichment

## Scope

Enrichment of annotation detail responses with parent/sibling DOM context, CSS framework detection, error correlation, and LLM action hints. Covers the full path from extension DOM capture through Go handler response.

## Entrypoints

1. **Extension:** `buildElementDetail()` in `extension/content/draw-mode.js` — captures DOM context when user draws annotations
2. **Go Handler:** `toolGetAnnotationDetail()` in `cmd/dev-console/tools_analyze_annotations_handlers.go` — returns enriched detail to LLM
3. **Session Hints:** `buildAnnotationSessionResult()` / `buildNamedAnnotationSessionResult()` — adds LLM checklist hints

## Primary Flow

1. User draws annotation rectangle in draw mode
2. `captureElementsUnderRect()` identifies DOM elements under the rectangle
3. `buildElementDetail(el)` captures:
   - Existing: selector, tag, classes, computed styles, a11y flags, shadow DOM, outer HTML
   - **New:** `parent_context` (parent + grandparent tag/classes/id/role)
   - **New:** `siblings` (up to 2 before + 2 after with tag/classes/text/position)
   - **New:** `css_framework` via `detectCSSFramework(el)` — heuristic detection of Tailwind/Bootstrap/CSS Modules/styled-components
   - **New:** `selector_candidates` — deduplicated fallback locator strategies (`css`, `testid`, `role`, `label`, `placeholder`, `text`)
   - **New:** `js_framework` + `component` metadata from runtime component inspection
4. Extension sends detail data to Go server via `storeElementDetails()` route
5. Go `Detail` struct stores new fields as `json.RawMessage` (parent_context, siblings) and `string` (css_framework)
6. LLM calls `analyze({what:'annotation_detail', correlation_id:'...'})`:
   a. Handler retrieves detail from annotation store
   b. Handler returns `selector_candidates`, `js_framework`, and `component` when present
   b. **New:** `findAnnotationTimestamp()` locates the annotation's timestamp
   c. **New:** `findErrorsNearTimestamp()` scans log entries for errors within ±5 seconds
   d. **New:** `buildDetailHints()` generates context-aware LLM guidance based on CSS/runtime frameworks, a11y flags, and correlated errors
7. Response includes all enriched fields + hints
8. `generate({what:'visual_test'})` consumes `selector_candidates` via `resolveAnnotationLocator(...)` and emits resilient Playwright locators with fallback order.

## Session-Level Hints

When LLM calls `analyze({what:'annotations'})`:
- `buildSessionHints()` adds a `hints` object with:
  - `checklist`: ordered action steps for processing annotations
  - `screenshot_baseline`: path to pre-alteration screenshot (when available)

## Error and Recovery Paths

- DOM traversal errors in extension are caught silently — fields default to null/empty
- CSS framework detection returns empty string on no match or error
- Error correlation returns no results if annotation timestamp not found or no errors in window
- Detail hints return nil (omitted from response) when no special data is present
- Invalid/broken selector candidates in generated tests are skipped at runtime and fallback continues

## State and Contracts

- Session TTL: **2 hours** (increased from 30 minutes)
- Detail TTL: 10 minutes (unchanged)
- Error correlation window: ±5 seconds, up to 5 errors
- CSS framework detection thresholds: Tailwind ≥3, Bootstrap ≥2, CSS Modules ≥1, styled-components ≥2

## Code Paths

| Component | File |
|-----------|------|
| Extension DOM capture | `extension/content/draw-mode.js` — `buildElementDetail()`, `detectCSSFramework()`, `collectSelectorCandidates()`, `detectComponentSource()` |
| Go Detail struct | `internal/annotation/store.go` — `Detail` struct |
| Go handler + enrichment | `cmd/dev-console/tools_analyze_annotations_handlers.go` |
| Detail/session hints | `cmd/dev-console/tools_analyze_annotations_hints.go` — `buildSessionHints()`, `buildDetailHints()` |
| Visual test generation | `cmd/dev-console/tools_generate_annotations_visual.go` — `resolveAnnotationLocator` output + selector fallback |
| Error correlation | `cmd/dev-console/tools_analyze_annotations_handlers.go` — `findAnnotationTimestamp()`, `findErrorsNearTimestamp()` |

## Test Paths

| Component | File |
|-----------|------|
| Extension enrichments | `tests/extension/draw-mode.test.js` — "Element Detail Enrichment" describe block (`selector_candidates`, `js_framework`, `component`) |
| Go TTL change | `internal/annotation/store_test.go` — `TestStore_SessionTTL_Is2Hours` |
| Go handler fields | `cmd/dev-console/tools_analyze_annotations_test.go` — `TestToolGetAnnotationDetail_NewEnrichmentFields`, `*_NewFieldsAbsentWhenEmpty`, `*_Hints_RuntimeFramework` |
| Generated locator fallback | `cmd/dev-console/tools_generate_annotations_test.go` — `TestGenerate_VisualTest_UsesSelectorCandidates` |
| Error correlation | `cmd/dev-console/tools_analyze_annotations_test.go` — `TestToolGetAnnotationDetail_ErrorCorrelation`, `*_ErrorCorrelation_NoErrors` |
| LLM hints | `cmd/dev-console/tools_analyze_annotations_test.go` — `TestToolGetAnnotations_SessionHints_*`, `TestToolGetAnnotationDetail_Hints_*`, `*_NoHints_*`, `*_NamedSessionHints` |

## Edit Guardrails

- `detectCSSFramework()` is heuristic-based — adjust thresholds carefully and add tests for new patterns
- Error correlation scans all log entries — keep the window small (5s) and cap results (5) for performance
- `json.RawMessage` fields in `Detail` struct must match extension JSON shape — no Go-side parsing needed
- Session-level hints are conditional on annotation count > 0 (suppressed for empty sessions to avoid confusing LLMs)
- Detail-level hints are conditional — only appear when relevant data exists
