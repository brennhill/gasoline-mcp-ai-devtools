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

## Git Workflow

- Branch from `next`, PR to `next`
- Never push directly to `main`
- Squash commits before merge

---

## 📚 Reference Sections

For detailed guidance on specific topics, see the sections below.

### Quick Commands

**See [CLAUDE.md#commands](#commands) for:**

- `make compile-ts`, `make test`, `make ci-local`
- TypeScript check & linting

### Testing & UAT

**See [CLAUDE.md#testing](#testing) for:**

- Primary UAT script: `scripts/test-all-tools-comprehensive.sh`
- UAT rules and test coverage areas

### Code Standards

**See [CLAUDE.md#code-standards](#code-standards) for:**

- JSON API field naming (snake_case vs camelCase)
- TypeScript restrictions (dynamic imports, bundling, fetch handling)
- Go standards (append-only I/O, eviction, file headers)
- Max 800 LOC per file

### Feature Development

**See [CLAUDE.md#feature-development](#feature-development) for:**

- Workflow: product spec → tech spec → review → QA → implementation
- Tech spec template (sequence diagrams, edge cases, state machines, network communication)
- Reference implementation example

### File Locations

**See [CLAUDE.md#finding-things](#finding-things) for:**

- Feature specs: `docs/features/<name>/`
- Architecture: `.claude/refs/architecture.md`
- Known issues: `docs/core/known-issues.md`

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

## Feature Development

### Feature Workflow

```text
product-spec.md → tech-spec.md → test-plan.md → Review → Implementation
```

Don't skip gates. Tests before code.

**Critical Gates:**

- After product-spec.md and tech-spec.md, create `{feature}-test-plan.md` that describes:
  - **Product Tests** — High-level test scenarios proving the feature works across all edge cases (negative and positive) or state machine transitions
  - **Technical Tests** — How tests will be implemented (unit, integration, UAT, automation, manual steps)
  - **Test Status** — Links to generated test files after implementation begins

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

### Test Plan Requirements

After product-spec.md and tech-spec.md are approved, create `{feature}-test-plan.md`:

1. **Product Tests** — What must be verified to prove the feature works
   - Test scenarios covering all valid states
   - Negative test cases for each edge case
   - Concurrent/race condition scenarios
   - Failure and recovery paths
   - Format: plain language, executable checklists

2. **Technical Tests** — Implementation details of how testing will happen
   - Unit test coverage areas
   - Integration/UAT test scenarios
   - Manual testing steps (if applicable)
   - Automation tools and frameworks
   - Pass/fail criteria for each test

3. **Test Status** — Living document updated as tests are generated
   - Links to unit test files (e.g., `test/features/feature-name.test.ts`)
   - Links to UAT/integration test files
   - Links to automation scripts
   - Pass/fail status before merge

## Finding Things

| Need                  | Location                                         |
| --------------------- | ------------------------------------------------ |
| Feature specs         | `docs/features/<name>/`                          |
| Test plans            | `docs/features/<name>/{name}-test-plan.md`       |
| Test plan template    | `docs/features/_template/template-test-plan.md`  |
| Architecture          | `.claude/refs/architecture.md`                   |
| Known issues          | `docs/core/known-issues.md`                      |
| All features          | `docs/features/feature-navigation.md`            |
