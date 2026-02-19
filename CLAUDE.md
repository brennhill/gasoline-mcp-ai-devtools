# Gasoline MCP — Core Rules

Browser extension + MCP server for real-time browser telemetry.
**Stack:** Go (zero deps) | Chrome Extension (MV3) | MCP (JSON-RPC 2.0)

---

## 🔴 Mandatory Rules

1. **TDD** — Write tests first, then implementation
2. **No `any`** — TypeScript strict mode, no implicit any
3. **Zero Deps** — No production dependencies in Go or extension
4. **Compile TS** — Run `make compile-ts` after ANY src/ change
5. **5 Tools** — observe, generate, configure, interact, analyze
6. **Performance** — WebSocket < 0.1ms, HTTP < 0.5ms
7. **Privacy** — All data stays local, no external transmission
8. **Wire Types** — `wire_*.go` and `wire-*.ts` are the source of truth for HTTP payloads. Changes to either side MUST update the counterpart. Run `make check-wire-drift`

## Git Workflow

- Branch from `next`, PR to `next`
- Never push directly to `main`
- Squash commits before merge

---

## Commands

```bash
make compile-ts    # REQUIRED after src/ changes
make test          # All tests
make ci-local      # Full CI locally
npm run typecheck  # TypeScript check
npm run lint       # ESLint
```

## Testing

**Primary UAT Script:** [`scripts/test-all-tools-comprehensive.sh`](scripts/test-all-tools-comprehensive.sh)

Tests: cold start, tool calls, concurrent clients, stdout purity, persistence, graceful shutdown.

```bash
./scripts/test-all-tools-comprehensive.sh  # Run full UAT
```

**UAT Rules:**

- **NEVER modify tests during UAT** — run tests as-is, report results
- If tests have issues, note them and propose changes AFTER UAT completes
- UAT validates the npm-installed version (`gasoline-mcp` from PATH)
- Extension must be connected for data flow tests to pass

## Code Standards

**JSON API fields:** ALL JSON fields use `snake_case`. No exceptions. External spec fields (MCP protocol, SARIF) are tagged with `// SPEC:<name>` comments.

**TypeScript:**

- No dynamic imports in service worker (background/)
- No circular dependencies
- Content scripts must be bundled (MV3 limitation)
- All fetch() needs try/catch + response.ok check

**Go:**

- Append-only I/O on hot paths
- Single-pass eviction (never loop-remove-recheck)
- File headers required: `// filename.go — Purpose summary.`

**File size:** Max 800 LOC. Refactor if larger.

## Finding Things

| Need                  | Location                                         |
| --------------------- | ------------------------------------------------ |
| Feature specs         | `docs/features/<name>/`                          |
| Test plans            | `docs/features/<name>/{name}-test-plan.md`       |
| Test plan template    | `docs/features/_template/template-test-plan.md`  |
| Architecture          | `.claude/refs/architecture.md`                   |
| Known issues          | `docs/core/known-issues.md`                      |
| All features          | `docs/features/feature-navigation.md`            |
