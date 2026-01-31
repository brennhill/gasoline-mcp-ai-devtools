# Roadmap Reorganization Strategy

**Note:** This is the strategic analysis document. The **operational source of truth is [docs/roadmap.md](roadmap.md)** — it may have been updated since this analysis was written.

## Current State Analysis

**Thesis:** "AI closes the feedback loop autonomously" — observe, diagnose, repair, verify without human intervention.

**Competitive Landscape:**
- Chrome DevTools MCP: raw observation, no intelligence
- Standard CI tools: reactive (test fails, then debug), not proactive
- Other AI coding agents: brittle selectors, hallucinated CSS, can't verify fixes, no team workflow

**Strategic Problems Being Solved (from roadmap):**
- **A. Token Inefficiency** — Raw DOM bloats context window
- **B. Shallow Debugging** — Symptoms not causes
- **C. Weak Feedback Loops** — Fix → Verify → Done is manual
- **D. Brittle Selectors & Tests** — AI-generated tests fail on refactor
- **E. Raw Data Output** — Not developer-ready
- **F. Production Unsafety** — Can't use in prod

---

## Current v6+ Organization Issues

### Problem 1: Bloated v6.1 (13 features)
Mixes core observation (Visual-Semantic, Time-Travel, Causal Diffing) with specialized features (SSR Hydration, SEO Audit, Design Systems). Confuses prioritization.

### Problem 2: Massive v6.4 (15 features)
Conflates test automation (Healer Mode) with team workflows (Flight Recorder) with infrastructure (CLI, MCP harness). No coherent theme.

### Problem 3: Features Not Mapped to Competitive Value
- Zero-Trust Sandbox (enterprise differentiator) buried in v6.3
- Token Compression (cost reduction) scattered across v6.2-6.3
- Team workflows (adoption enabler) lumped with infrastructure

### Problem 4: Missing Tier Strategy
Current roadmap doesn't clearly show which features are:
- **Must-have for v6.0 thesis** (already defined: Self-Healing, CI, Context Streaming, etc.)
- **Enterprise unlock** (safety + verification)
- **Production readiness** (compliance + integration)
- **Polish & specialization** (domain-specific audits, advanced interactions)

---

## Proposed Reorganization: Tier-Based Strategy

### Tier 1: Core Moat (v6.0-6.1)
**Goal:** Validate thesis — AI autonomously closes the loop.
**Competitive Differentiator:** Smart observation + causal diagnosis + safe repair.

**v6.0 (Existing)**
- Self-Healing Tests
- Gasoline CI Infrastructure
- Context Streaming
- PR Preview Exploration
- Agentic E2E Repair
- Deployment Watchdog

**v6.1: Smart Observation**
Solves Problems A (tokens) + B (causality) + D (selector brittleness).
- Advanced Filtering (Signal-to-Noise) (reduces noise before AI sees it)
- Visual-Semantic Bridge (enables reliable clicking)
- State "Time Travel" (enables causal debugging)
- Causal Diffing (enables root-cause analysis)
- Reverse Engineering Engine (enables legacy code)
- Design System Injector (enables semantic code generation)
- Deep Framework Intelligence (enables component-level understanding)
- DOM Fingerprinting (enables stable selectors)
- Smart DOM Pruning (removes non-essential noise, fits DOM in <25% context)
- Hydration Doctor (enables SSR/Nuxt debugging)

**v6.2: Safe Repair & Verification**
Solves Problems C (feedback loops) + F (production safety).
- Prompt-Based Network Mocking (verify error handling)
- Shadow Mode (test destructive operations safely)
- Pixel-Perfect Guardian (visual regression detection)
- Healer Mode (auto-convert fixes to tests)

---

### Tier 2: Enterprise Unlock (v6.3)
**Goal:** Enable corporate adoption with safety guarantees.
**Competitive Differentiator:** Action gating, data masking, compliance-ready.

**v6.3: Zero-Trust Enterprise**
Solves Problem F (production safety).
- Zero-Trust Sandbox (action gating, data masking, prompt injection defense)
- Asynchronous Multiplayer Debugging (Flight Recorder links)
- Session Replay Exports (.gas files for hand-off)

---

### Tier 3: Production Ready (v6.4)
**Goal:** Enable production debugging at scale with governance.
**Competitive Differentiator:** Read-only mode, compliance audit, team integration.

**v6.4: Production Compliance**
- Read-Only Mode (non-mutating observation)
- Tool Allowlisting (restrict capabilities)
- Project Isolation (multi-tenant contexts)
- Configuration Profiles (paranoid/restricted bundles)
- Redaction Audit Log (compliance logging)
- GitHub/Jira Integration (bug reports)
- CI/CD Integration (GitHub Actions, SARIF, HAR)
- IDE Integration (VS Code, Claude Code)

---

### Tier 4: Optimization & Specialization (v6.5+)
**Goal:** Continuous shipping of incremental value.
**Competitive Differentiator:** Cost reduction, domain-specific expertise, rich interactions.

**v6.5: Token & Context Efficiency**
- Focus Mode (component-level debugging, 90% token reduction)
- Semantic Token Compression (lightweight references, JIT context)
- Smart DOM Pruning (removes non-essential nodes)

**v6.6: Specialized Audits & Analytics**
- Performance Audit (root-cause perf issues)
- Best Practices Audit (structural/deprecated/security)
- SEO Audit (metadata/heading/structured data)
- A11y Tree Snapshots (accessibility compression)
- Enhanced WCAG Audit (deep accessibility)
- Annotated Screenshots (visual context for vision models)
- Design Audit & Archival (screenshot archival + queryable design regression testing across responsive variants)

**v6.7: Advanced Interactions**
- Form Filling (complex form automation)
- Dialog Handling (alerts/confirms/prompts)
- Drag & Drop (advanced UI interactions)
- CPU/Network Emulation (throttling/load testing)
- Local Web Scraping (authenticated multi-step extraction)

**v6.8: Infrastructure & Quality**
- Fuzz Tests (5 types: parser, HTTP, security, WebSocket, network)
- Async Command Execution (prevent MCP hangs)
- Multi-Client Architecture (multiple AI clients)
- Test Generation v2 (DOM assertions, fixtures, visual snapshots)
- Performance Budget Monitor (regression detection)

---

## Why This Reorganization Works

### 1. **Thesis Clarity**
- v6.0-6.2 directly prove "AI closes the feedback loop autonomously"
- v6.3+ expand to production-scale and specialization
- Each tier answers: "How does this advance the thesis?"

### 2. **Competitive Positioning**
- **Tier 1** = Moat (what Chrome DevTools can't do)
- **Tier 2** = Enterprise adoption unlock (safety-first)
- **Tier 3** = Production readiness (compliance + governance)
- **Tier 4** = Polish (continuous value)

### 3. **Prioritization Clarity**
- v6.0-6.2 = Critical path for product differentiation
- v6.3 = Critical path for enterprise sales
- v6.4+ = Continuous shipping (parallel track)

### 4. **Feature Grouping Makes Sense**
- v6.1 = All observation features (not scattered)
- v6.2 = All repair/verify features (not scattered)
- v6.3 = Safety as a unified theme (not spread across v6.3-v6.4)
- v6.5-6.8 = Specialization (performance, a11y, interactions, infra)

### 5. **Time-to-Value Alignment**
- v6.0 = Thesis validated (market validation)
- v6.1-6.2 = Feature moat in place (competitive advantage)
- v6.3 = Enterprise sales ready (Tier 1 + safety = TAM unlock)
- v6.4 = Production ready (compliance + integration)
- v6.5+ = Continuous shipping (quarterly updates)

### 6. **Problem-to-Solution Mapping**
| Problem | Tier | Feature | Version |
|---------|------|---------|---------|
| A (tokens) | 1 | Smart DOM Pruning | v6.1 |
| A (tokens) | 4 | Focus Mode + Token Compression | v6.5 |
| B (causality) | 1 | State Time-Travel + Causal Diffing | v6.1 |
| C (feedback loop) | 1-2 | Network Mocking + Healer Mode | v6.1-6.2 |
| D (brittle tests) | 1 | Visual-Semantic Bridge + Fingerprinting | v6.1 |
| E (raw output) | 3 | GitHub/Jira + Session Export | v6.3-6.4 |
| F (production safety) | 2-3 | Zero-Trust + Read-Only + Audit | v6.2-6.4 |

---

## Implementation Sequence

**Sequential (blocking dependencies):**
1. v6.0 → v6.1 → v6.2 → v6.3 → v6.4 (each tier unlocks next)

**Parallel (no dependencies):**
- v6.5, v6.6, v6.7, v6.8 can start after v6.4 completes (1 agent each)
- Or after v6.2, depending on team bandwidth

**Marketing Milestones:**
- v6.0 release: "AI closes the feedback loop autonomously"
- v6.2 release: "Enterprise-safe autonomous debugging" (Zero-Trust Sandbox, Healer Mode)
- v6.4 release: "Production-ready with compliance" (Audit, Integration)
- v6.5+ release: "Specialized debugging superpowers"

---

## Validation Against Thesis

**Thesis: "AI will be the driving force in development."**

✅ **v6.0-6.2 enables this:**
- Observe autonomously (Visual-Semantic, Time-Travel, Framework Intelligence)
- Diagnose autonomously (Causal Diffing, Reverse Engineering)
- Repair autonomously (Network Mocking, Shadow Mode, Healer Mode)
- Verify autonomously (Pixel-Perfect Guardian, Self-Healing Tests)

✅ **v6.3 enables enterprise adoption of this:**
- Safety for production (Zero-Trust, Read-Only, Audit)
- Trust through transparency (Session Export, Flight Recorder)
- Compliance requirements (Redaction, Project Isolation)

✅ **v6.4 enables team workflows around this:**
- Integration with existing tools (GitHub, Jira, CI/CD, VS Code)
- Knowledge capture and sharing (Flight Recorder, Session Export)
- Governance and compliance (Audit Log, Redaction)

✅ **v6.5+ enables specialization and optimization:**
- Cost reduction through efficiency (token compression, focus mode)
- Domain-specific expertise (performance, a11y, SEO audits)
- Rich interactions for complex scenarios
