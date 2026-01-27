---
feature: noise-filtering
status: shipped
tool: configure
mode: noise_rule, dismiss
version: 5.0.0
---
# Product Spec: Noise Filtering

Describes the user-facing requirements, rationale, and deprecations for the Noise Filtering feature.

- **Purpose:** Suppress irrelevant browser noise for AI agents and developers.
- **Requirements:**
  - Filter out extension, analytics, and framework noise from logs.
  - Allow user/agent to add custom noise rules.
  - Built-in rules always active; user rules session-scoped.
- **Deprecations:**
  - Any legacy noise filtering logic is replaced by this system.

---

**See also:**
- [Noise Filtering Tech Spec](TECH_SPEC.md)
- [Noise Filtering ADRs](ADRS.md)
- [Core Product Spec](../../../core/PRODUCT_SPEC.md)
