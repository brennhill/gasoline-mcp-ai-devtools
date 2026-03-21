# Analyze Tool Reference

Complete reference for all 27 analyze modes.

**Universal params (available on every mode):**
- `what` (string, required) — mode name
- `telemetry_mode` (string) — telemetry collection mode
- `background` (bool) — run asynchronously, returns `correlation_id`
- `selector` (string) — CSS selector scope
- `frame` (string) — target frame

---

## dom
DOM structure analysis.
**Params:** selector (string), frame (string), tab_id (number)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"dom","selector":"main"}'
```

## performance
Performance metrics.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"performance"}'
```

## accessibility
Accessibility audit.
**Params:** selector (string), frame (string), scope (string), tags (array), force_refresh (bool), summary (bool)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"accessibility","tags":["wcag2a"],"summary":true}'
```

## error_clusters
Error pattern clustering.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"error_clusters"}'
```

## navigation_patterns
Navigation pattern detection.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"navigation_patterns"}'
```

## security_audit
Security vulnerability audit.
**Params:** checks (array: credentials|pii|headers|cookies|transport|auth), severity_min (string: critical|high|medium|low|info), summary (bool)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"security_audit","checks":["credentials","pii"],"severity_min":"high"}'
```

## third_party_audit
Third-party script analysis.
**Params:** first_party_origins (array), include_static (bool), custom_lists (object), summary (bool)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"third_party_audit","first_party_origins":["example.com"],"summary":true}'
```

## link_health
Link health validation.
**Params:** domain (string), timeout_ms (number, default 10000), max_workers (number)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"link_health","domain":"example.com","timeout_ms":5000}'
```

## link_validation
Link validation.
**Params:** urls (array)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"link_validation","urls":["https://example.com/page1","https://example.com/page2"]}'
```

## page_summary
Page content summary.
**Params:** timeout_ms (number, default 5000), world (string: auto|main|isolated), tab_id (number)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"page_summary","timeout_ms":3000}'
```

## annotations
Retrieve annotations from draw sessions.
**Params:** operation (string: analyze|report|clear|flush), annot_session (string), url (string), url_pattern (string), timeout_ms (number, default 15000)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"annotations","operation":"report","url_pattern":"*.example.com/*"}'
```

## annotation_detail
Detail for specific annotation.
**Params:** correlation_id (string)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"annotation_detail","correlation_id":"abc-123"}'
```

## api_validation
API endpoint validation.
**Params:** operation (string: analyze|report|clear|flush), ignore_endpoints (array)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"api_validation","operation":"analyze","ignore_endpoints":["/health"]}'
```

## draw_history
Draw session history.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"draw_history"}'
```

## draw_session
Load specific draw session.
**Params:** file (string)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"draw_session","file":"session-2026-03-12.json"}'
```

## computed_styles
Computed CSS styles.
**Params:** selector (string)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"computed_styles","selector":".hero-banner"}'
```

## forms
Form element analysis.
**Params:** selector (string)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"forms","selector":"#login-form"}'
```

## form_state
Form field state capture.
**Params:** selector (string)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"form_state","selector":"#checkout-form"}'
```

## form_validation
Form validation rules.
**Params:** summary (bool)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"form_validation","summary":true}'
```

## data_table
Data table extraction.
**Params:** selector (string), max_rows (number), max_cols (number)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"data_table","selector":"table.results","max_rows":50}'
```

## visual_baseline
Visual baseline capture.
**Params:** name (string)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"visual_baseline","name":"homepage-v1"}'
```

## visual_diff
Visual diff comparison.
**Params:** name (string), baseline (string), threshold (number, 0-255, default 30)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"visual_diff","name":"homepage-current","baseline":"homepage-v1","threshold":25}'
```

## visual_baselines
List visual baselines.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"visual_baselines"}'
```

## navigation
Navigation structure analysis.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"navigation"}'
```

## page_structure
Page structural analysis.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"page_structure"}'
```

## audit
Comprehensive audit.
**Params:** categories (array: performance|accessibility|security|best_practices), summary (bool)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"audit","categories":["performance","accessibility"],"summary":true}'
```

## feature_gates
Feature flag detection.
**Params:** none (universal only)
**Example:**
```bash
bash scripts/gasoline-call.sh analyze '{"what":"feature_gates"}'
```
