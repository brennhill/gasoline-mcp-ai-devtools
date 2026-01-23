# Test-Driven Development (TDD) Requirements

**CRITICAL**: This project follows strict Test-Driven Development. NEVER write implementation code before tests exist.

## TDD Workflow (Mandatory)

### The Red-Green-Refactor Cycle

1. **Write tests FIRST** - Before any implementation code
2. **Run tests** - Verify they fail (red)
3. **Write minimum code** - Just enough to pass tests
4. **Run tests** - Verify they pass (green)
5. **Refactor** - Clean up while keeping tests green

### TDD is NON-NEGOTIABLE

Every feature, function, endpoint, or handler MUST follow this workflow:

```
1. Read specification (docs/specification.md or docs/v4-specification.md)
2. Write test cases from specification
3. Run tests → Confirm they FAIL
4. Write implementation
5. Run tests → Confirm they PASS
6. Refactor if needed
7. Commit with tests
```

**If you find yourself writing implementation code without tests existing first, STOP IMMEDIATELY and write the tests.**

## Test Commands

```bash
# Go server tests
make test
CGO_ENABLED=0 go test -v ./cmd/dev-console/...

# Run specific Go test
CGO_ENABLED=0 go test -v -run "TestV4WebSocket" ./cmd/dev-console/

# Extension tests (all)
node --test extension-tests/*.test.js

# Run specific extension test file
node --test extension-tests/websocket.test.js
node --test extension-tests/network-bodies.test.js
node --test extension-tests/on-demand.test.js
```

## Test Requirements

### Go Server Tests
- Use `testing` package (no external test frameworks)
- Use `httptest.NewRecorder()` for HTTP handler testing
- Use `t.TempDir()` for file system tests
- Cover: happy path, error cases, edge cases, protocol compliance
- Test MCP protocol responses match JSON-RPC 2.0 spec

### Extension Tests
- Use `node:test` (built-in Node.js test runner, no Jest/Vitest)
- Use `node:assert` for assertions
- Use `mock.fn()` and `mock.reset()` for test doubles
- Mock Chrome APIs (`chrome.runtime`, `chrome.tabs`, `chrome.storage`)
- Mock browser globals (`window`, `document`, `crypto`)

## What to Test

For every new feature:

### Server (Go)
- HTTP endpoint accepts valid input
- HTTP endpoint rejects invalid input (400)
- Buffer stores and retrieves data
- Buffer rotates/evicts when full
- Memory limits are enforced
- MCP tool returns correct JSON-RPC response
- Rate limiting triggers at threshold
- On-demand query flow (pending → result)
- Timeout behavior for on-demand queries

### Extension (JS)
- Constructor/function interception works
- Original behavior is preserved
- Events are emitted with correct payload
- Sampling/filtering applies correctly
- Truncation limits are respected
- Privacy (header sanitization, URL exclusion)
- Memory pressure detection and response
- Page load deferral works correctly

## Deriving Tests from Specifications

When implementing a feature:

1. **Read the specification** in `docs/`
2. **Identify all requirements** - explicit and implicit
3. **Write test cases for each requirement** before coding
4. **Include edge cases** mentioned or implied by the spec
5. **Map tests to spec sections** - every spec requirement should have a test

Example mapping from v4-specification.md:

```
Spec: "Message body truncation: 4KB per message. If exceeded, truncate and add truncated: true"

Tests derived:
- should truncate message data at 4KB
- should set truncated flag when data exceeds 4KB
- should not truncate messages within 4KB
- should not set truncated flag for short messages
```

## TDD Violations

Do NOT:

- Write implementation code before tests
- Skip tests "to save time"
- Write tests after implementation
- Commit code without corresponding tests
- Add "TODO: write tests later" comments
- Test mock implementations instead of real code

If you catch yourself writing implementation first, STOP and write the tests.

## Autonomous TDD Checks

After every code change:

1. Run `make test` - All Go tests must pass
2. Run `node --test extension-tests/*.test.js` - All JS tests must pass
3. Verify new code has corresponding tests
4. Fix any failures before committing
