# Feature Documentation Index

Each feature in `/docs/features/feature/` should include:
- Product spec (user-facing, requirements, deprecations)
- Technical spec (implementation, code mapping)
- ADRs (if any)

## Example Structure for a Feature

```
/docs/features/feature/noise-filtering/
  - PRODUCT_SPEC.md
  - TECH_SPEC.md
  - ADRS.md (if any)
```

## Cross-links
- Each feature spec should link to its ADR(s) and core specs as appropriate.
- Core specs should reference features where relevant.
