# Gasoline MCP — Quick Links

**Browser extension + MCP server** for real-time browser telemetry (logs, network, WebSocket, DOM)

**Stack:** Go (zero deps) | Chrome Extension (MV3) | MCP (JSON-RPC 2.0) | NPM distribution

---

## 🚀 START HERE (Every Session)

1. [quick-reference.md](.claude/docs/quick-reference.md) — One-page cheat sheet (~2 min)
2. [startup-checklist.md](.claude/docs/startup-checklist.md) — Essential rules (~5 min)
3. [context-on-demand.md](.claude/docs/context-on-demand.md) — Smart context loading (~3 min)

---

## 🎯 BY TASK

**Requesting a new feature?**
→ [docs/examples/feature-request-template.md](docs/examples/feature-request-template.md) — How to maintain quality

**Implementing a feature?**
→ [docs/features/README.md](docs/features/README.md) → Feature folder

**Fixing a bug?**
→ [docs/core/known-issues.md](docs/core/known-issues.md) → TECH_SPEC

**Finding something?**
→ [docs/how-to-use-new-system.md](docs/how-to-use-new-system.md) → [docs/cross-reference.md](docs/cross-reference.md)

**Understanding the system?**
→ [.claude/refs/architecture.md](.claude/refs/architecture.md)

---

## 📚 All Resources

**Navigation & Discovery:**
- [docs/README.md](docs/README.md) — Master index
- [docs/cross-reference.md](docs/cross-reference.md) — Doc relationships
- [docs/features/feature-navigation.md](docs/features/feature-navigation.md) — All 71 features
- [docs/features/README.md](docs/features/README.md) — LLM guide
- [docs/how-to-use-new-system.md](docs/how-to-use-new-system.md) — How-to guide

**Mandatory Reading:**
- [.claude/docs/feature-workflow.md](.claude/docs/feature-workflow.md) — 5-gate process
- [.claude/docs/documentation-maintenance.md](.claude/docs/documentation-maintenance.md) — Doc updates (required)
- [.claude/docs/testing.md](.claude/docs/testing.md) — TDD workflow
- [.claude/docs/spec-review.md](.claude/docs/spec-review.md) — Spec approval

**Core Reference:**
- [.claude/refs/architecture.md](.claude/refs/architecture.md) — System design
- [.claude/docs/git-and-concurrency.md](.claude/docs/git-and-concurrency.md) — Git workflow
- [docs/core/release.md](docs/core/release.md) — Release process
- [docs/quality-standards.md](docs/quality-standards.md) — Quality maintenance guide
- [docs/quality-quick-reference.md](docs/quality-quick-reference.md) — Quick quality checklist
- [docs/standards/](docs/standards/) — Implementation standards (6 focused guides: data, validation, memory, errors, APIs, code quality)
- [docs/core/code-index.md](docs/core/code-index.md) — Code → feature map (when implementing)
- [docs/core/feature-to-strategy.md](docs/core/feature-to-strategy.md) — Feature → strategy map (when designing)

---

## ⚡ The 8 Rules

1. **Spec Review** — All specs need review
2. **TDD** — Tests first, always
3. **TypeScript** — No `any`, strict mode
4. **Zero Deps** — No production dependencies
5. **5 Tools Max** — observe|generate|configure|interact|analyze
6. **Performance** — WS < 0.1ms, HTTP < 0.5ms
7. **Privacy** — Data stays local
8. **Docs** — Update before committing

---

## 🛠️ Commands

```bash
npm run typecheck                      # TypeScript check
npm run lint                          # ESLint
make test                             # All tests
make ci-local                         # Full CI locally
make compile-ts                       # Compile TS (REQUIRED)
python3 scripts/lint-documentation.py # Check docs
```

---

## ➡️ Next Steps

**Read** [.claude/docs/quick-reference.md](.claude/docs/quick-reference.md) (~2 min)
**Then** start your task (context loads on-demand)
**Always** update `last-verified` before committing