# Quality Gates

## Gate 1: Tests Pass

Before any commit:

```bash
# All Go tests pass
make test

# All extension tests pass
node --test extension-tests/*.test.js
```

**No code is committed with failing tests.**

## Gate 2: Test Quality

Tests MUST:

1. **Import source code** - Test the actual functions, not inline logic
2. **Test behavior** - Verify outputs for given inputs
3. **Cover specs** - Map to specification requirements
4. **Handle edges** - Empty inputs, nulls, boundaries, overflow
5. **Test errors** - Verify error handling and rejection

Tests MUST NOT:

- Define logic inline and test it (tautological)
- Test that mocks work correctly
- Skip edge cases or error paths
- Have misleading descriptions

### Quality Check Failures

**FAIL: No source imports**
```javascript
// BAD - tests inline logic
const truncate = (s, n) => s.slice(0, n)  // Defined in test!
test('truncates', () => { assert.strictEqual(truncate('hello', 3), 'hel') })

// GOOD - imports actual source
import { truncateWsMessage } from '../extension/inject.js'
test('truncates at 4KB', () => { ... })
```

**FAIL: Tautological assertion**
```javascript
// BAD - always true
const events = getEvents()
assert.ok(events.length >= 0)  // Always true!

// GOOD - asserts expected behavior
v4.AddWebSocketEvents([...])
assert.strictEqual(v4.GetWebSocketEventCount(), 2)
```

## Gate 3: Specification Coverage

Every requirement in `docs/specification.md` and `docs/v4-specification.md` must have corresponding tests.

**Checklist before marking a feature complete:**

- [ ] All spec requirements have tests
- [ ] All limits/thresholds tested (buffer sizes, truncation, timeouts)
- [ ] All SLO targets have validation tests
- [ ] Error conditions tested (invalid input, timeout, overflow)
- [ ] Protocol compliance tested (JSON-RPC, MCP)

## Gate 4: Static Analysis

```bash
# Go
go vet ./cmd/dev-console/

# Go build (all platforms)
make build
```

No warnings or errors from `go vet`.

## Gate 5: Performance SLOs (v4)

v4 features must not violate performance targets:

| Metric | Target | Validation |
|--------|--------|------------|
| fetch() wrapper overhead | < 0.5ms sync | Benchmark test |
| WebSocket handler overhead | < 0.1ms per msg | Benchmark test |
| Page load impact | < 20ms | Lighthouse comparison |
| Server memory | < 30MB | Load test |
| MCP tool response | < 200ms | Integration test |

## Test Correction Policy

**NEVER revert tests to make them pass if the reverted version doesn't actually test the code correctly.**

When tests fail:

1. **Understand the test's intent** - What SHOULD this test verify?
2. **Understand why it fails** - Is the code wrong or the test wrong?
3. **Fix the right thing**:
   - If code is wrong → fix the code
   - If test is wrong → fix the test to correctly verify the intended behavior
   - If it's unclear or the spec is missing, prompt user and demand resolution of the spec.

## Commit Policy

- One logical unit per commit
- Tests and implementation in the same commit (not separate)
- Commit message format: `type(scope): description`
  - Types: `feat`, `fix`, `test`, `docs`, `refactor`, `perf`
  - Scopes: `server`, `extension`, `mcp`, `build`
- Examples:
  - `feat(server): Add WebSocket event buffer with rotation`
  - `feat(extension): Implement adaptive sampling for WS messages`
  - `test(server): Add v4 MCP tool tests`
  - `fix(extension): Prevent body capture blocking main thread`
