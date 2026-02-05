# Gasoline MCP

Browser extension + MCP server for real-time browser telemetry.
**Stack:** Go (zero deps) | Chrome Extension (MV3) | MCP (JSON-RPC 2.0)

## Core Rules (Mandatory)

1. **TDD** — Write tests first, then implementation
2. **No `any`** — TypeScript strict mode, no implicit any
3. **Zero Deps** — No production dependencies in Go or extension
4. **Compile TS** — Run `make compile-ts` after ANY src/ change
5. **5 Tools Only** — observe, generate, configure, interact, analyze
6. **Performance** — WebSocket < 0.1ms, HTTP < 0.5ms
7. **Privacy** — All data stays local, no external transmission

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

**JSON API fields:** Use `snake_case` for MCP responses. Exception: browser API pass-through fields (PerformanceResourceTiming, etc.) keep camelCase.

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

## Feature Workflow

```text
product-spec.md → tech-spec.md → Review → qa-plan.md → Implementation
```

Don't skip gates. Tests before code.

### Tech Spec Requirements

Every tech spec must include:

1. **Sequence Diagram** — Visual flow of the feature using mermaid
   - Cold start (initial state)
   - Warm start (subsequent uses)
   - Concurrent operations (if applicable)

2. **Edge Case Analysis** — Enumerate and answer all edge cases
   - What happens when X fails?
   - What if Y happens simultaneously?
   - How do we recover from Z?
   - Document resolution for each edge case

3. **State Machine** — Describe state transitions (or mark N/A)
   - Valid states
   - Transition triggers
   - Invalid transitions
   - Terminal states

4. **Network Communication** — For any service-to-service or network calls
   - Protocol specification (HTTP, WebSocket, stdio, etc.)
   - Request/response schemas
   - Failure modes (timeout, refused, crash, etc.)
   - Recovery strategies (retry, fallback, graceful degradation)
   - Race condition protection

**Example:** See [`docs/features/mcp-persistent-server/architecture.md`](docs/features/mcp-persistent-server/architecture.md) for reference implementation with 3 sequence diagrams and 14 edge cases.

## Finding Things

| Need           | Location                              |
| -------------- | ------------------------------------- |
| Feature specs  | `docs/features/<name>/`               |
| Architecture   | `.claude/refs/architecture.md`        |
| Known issues   | `docs/core/known-issues.md`           |
| All features   | `docs/features/feature-navigation.md` |

## Git

- Branch from `next`, PR to `next`
- Never push directly to `main`
- Squash commits before merge
