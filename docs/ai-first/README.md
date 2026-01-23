# AI-First Feature Set

Features designed for a world where AI agents are the primary coders.

## Specs

| # | Feature | Tool | Spec |
|---|---------|------|------|
| 1 | [Compressed State Diffs](compressed-state-diffs.md) | `get_changes_since` | Token-efficient delta reporting for tight agent feedback loops |
| 2 | [Noise Filtering](noise-filtering.md) | `configure_noise` / `dismiss_noise` | Auto-classify browser noise; only report real signals |
| 3 | [Behavioral Baselines](behavioral-baselines.md) | `save_baseline` / `compare_baseline` | Persistent "what does correct look like?" for regression detection |
| 4 | [Persistent Memory](persistent-memory.md) | `session_store` / `load_session_context` | Cross-session learning (noise, schemas, baselines, errors) |
| 5 | [API Schema Inference](api-schema-inference.md) | `get_api_schema` | Learn API contracts from observed traffic |
| 6 | [DOM Fingerprinting](dom-fingerprinting.md) | `get_dom_fingerprint` / `compare_dom_fingerprint` | Structural UI verification without vision models |

## Economic Justification

See [financial-analysis.md](financial-analysis.md) for detailed ROI calculations.

**Summary:** $24,888-27,010/developer/year in combined token savings + productivity gains. $0 cost (OSS). Replaces $65-90K/year commercial alternatives.

## Priority Order

1. Compressed Diffs — unblocks tight feedback loop (token efficiency)
2. Noise Filtering — makes all other signals useful (reduces false positives)
3. Behavioral Baselines — enables regression detection without tests
4. Persistent Memory — agent accumulates understanding over time
5. API Schema Inference — agent understands the system without docs
6. DOM Fingerprinting — structural UI verification without vision models

## Dependencies

```
                    ┌─────────────────┐
                    │ Persistent      │
                    │ Memory (4)      │
                    └────────┬────────┘
                             │ persists
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │ Noise       │  │ Baselines   │  │ API Schema  │
    │ Rules (2)   │  │ (3)         │  │ (5)         │
    └──────┬──────┘  └──────┬──────┘  └─────────────┘
           │                 │
           ▼                 ▼
    ┌─────────────────────────────────┐
    │ Compressed Diffs (1)            │
    │ (uses noise rules, references   │
    │  baselines for comparison)      │
    └─────────────────────────────────┘
              │
              ▼
    ┌─────────────────────────────────┐
    │ DOM Fingerprinting (6)          │
    │ (optional component of diffs    │
    │  and baselines)                 │
    └─────────────────────────────────┘
```

Features 1-2 can ship independently. Features 3-5 benefit from Feature 4 (persistence) but work without it (in-memory only). Feature 6 requires extension changes and can ship independently.
