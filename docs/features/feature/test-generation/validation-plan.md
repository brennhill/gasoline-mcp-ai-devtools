---
feature: test-generation
type: validation
status: ready_for_execution
---

# Test Generation — Validation Plan

## The Question

> "Are we sure that this will actually do the same thing as TestSprite? How do we know?"
> "Did you create actual failures that the system could repair?"

## Honest Assessment

### What We've Validated ✅

- **Unit test logic** — ~70+ tests verify the code paths work correctly
- **Request parsing** — JSON parameters are correctly parsed and validated
- **Response formats** — Outputs match the mcpJSONResponse pattern
- **Security validators** — Path traversal and selector injection are blocked
- **Batch limits** — Size constraints are enforced
- **Pattern matching** — Error patterns correctly map to categories

### What We Haven't Validated ❌

- **Real broken selectors being healed** — No actual test file with broken selector repaired
- **Generated tests reproducing actual errors** — No test generated from real error, then run to verify it fails as expected
- **Healed selectors working in browser** — No verification that replaced selectors actually find elements
- **Classification accuracy on real failures** — No corpus of real test failures classified
- **End-to-end workflow** — No user capturing error → generating test → running test cycle

## The Gap

Our implementation has the **logic** but lacks **real-world validation**. This is like building a compiler that passes all unit tests but has never compiled real code.

## TestSprite Comparison

| Feature | TestSprite | Our Implementation | Validation Status |
|---------|-----------|-------------------|------------------|
| Test generation from context | ✅ From PRD | ✅ From captured errors/actions | ❌ Not validated |
| Self-healing selectors | ✅ AI-powered | ✅ Confidence-based heuristics | ❌ Not validated |
| Failure classification | ✅ Pattern matching | ✅ Pattern matching | ❌ Not validated |
| Framework support | ✅ React/Vue/Angular | ✅ React/Vue/Svelte (via existing) | ✅ Validated (existing) |
| Real-time monitoring | ❌ Post-mortem | ✅ Live capture | ✅ Validated (existing) |
| Privacy | ⚠️ Cloud-based | ✅ Localhost-only | ✅ By design |
| Cost | 💰 $29-99/month | 💰 Free | ✅ By design |

**Conclusion:** We have feature parity on paper, but no proof it works.

## Validation Strategy

### Phase 1: Proof of Concept (30 minutes)

**Objective:** Prove one complete cycle works end-to-end

#### Steps:

1. **Create a broken test file**
   ```typescript
   // tests/broken.spec.ts
   import { test, expect } from '@playwright/test';

   test('login flow', async ({ page }) => {
     await page.goto('http://localhost:3000');
     await page.locator('#old-login-button').click(); // This selector is broken
     await page.locator('#old-username-input').fill('user@example.com'); // Also broken
     await page.locator('#old-password-input').fill('password'); // Also broken
     await page.locator('#old-submit-btn').click(); // Also broken
   });
   ```

2. **Run the broken test and capture failure**
   ```bash
   npx playwright test tests/broken.spec.ts --reporter=json > failure.json
   ```

3. **Use test_heal to analyze**
   ```bash
   gasoline-mcp <<EOF
   {
     "jsonrpc": "2.0",
     "id": 1,
     "method": "tools/call",
     "params": {
       "name": "generate",
       "arguments": {
         "format": "test_heal",
         "action": "analyze",
         "test_file": "tests/broken.spec.ts"
       }
     }
   }
   EOF
   ```

4. **Use test_heal to repair**
   ```bash
   gasoline-mcp <<EOF
   {
     "jsonrpc": "2.0",
     "id": 2,
     "method": "tools/call",
     "params": {
       "name": "generate",
       "arguments": {
         "format": "test_heal",
         "action": "repair",
         "test_file": "tests/broken.spec.ts",
         "broken_selectors": ["#old-login-button", "#old-username-input", "#old-password-input", "#old-submit-btn"],
         "auto_apply": false
       }
     }
   }
   EOF
   ```

5. **Manually apply suggested fixes**
6. **Re-run test and verify it passes**

#### Success Criteria:
- ✅ At least 3/4 selectors healed with confidence >= 0.7
- ✅ Healed test passes when run
- ✅ Entire cycle takes < 5 minutes

### Phase 2: Test Generation (30 minutes)

**Objective:** Generate a test from a real error

#### Steps:

1. **Navigate to demo app and trigger an error**
   - Open browser with Gasoline extension
   - Navigate to app with known error (e.g., form validation failure)
   - Trigger the error in the console

2. **Verify error was captured**
   ```bash
   gasoline-mcp <<EOF
   {
     "jsonrpc": "2.0",
     "id": 3,
     "method": "tools/call",
     "params": {
       "name": "observe",
       "arguments": {
         "mode": "errors"
       }
     }
   }
   EOF
   ```

3. **Generate test from error**
   ```bash
   gasoline-mcp <<EOF
   {
     "jsonrpc": "2.0",
     "id": 4,
     "method": "tools/call",
     "params": {
       "name": "generate",
       "arguments": {
         "format": "test_from_context",
         "context": "error",
         "framework": "playwright"
       }
     }
   }
   EOF
   ```

4. **Save generated test and run it**
   ```bash
   npx playwright test tests/generated.spec.ts
   ```

#### Success Criteria:
- ✅ Generated test is valid Playwright syntax
- ✅ Generated test reproduces the error when run
- ✅ Generated test uses stable selectors (testId > aria > text)

### Phase 3: Failure Classification (15 minutes)

**Objective:** Classify real test failures accurately

#### Steps:

1. **Collect 10 real test failures** from a test suite
   - 3 selector broken (timeout waiting for selector)
   - 2 timing flaky (race conditions)
   - 2 network errors
   - 2 assertion failures (real bugs)
   - 1 unknown

2. **Classify each failure**
   ```bash
   for failure in failures/*.json; do
     gasoline-mcp <<EOF
     {
       "jsonrpc": "2.0",
       "id": 1,
       "method": "tools/call",
       "params": {
         "name": "generate",
         "arguments": {
           "format": "test_classify",
           "action": "failure",
           "failure": $(cat $failure)
         }
       }
     }
     EOF
   done
   ```

3. **Verify classification accuracy**

#### Success Criteria:
- ✅ At least 7/10 classified correctly
- ✅ High confidence (>= 0.7) for correct classifications
- ✅ Low confidence (< 0.5) triggers uncertain error

## Validation Artifacts

After completing validation, create these artifacts:

### 1. Real Example Test Files

`docs/features/feature/test-generation/examples/`
```
broken-test-before.spec.ts      # Actual broken test
broken-test-after.spec.ts       # After healing
generated-from-error.spec.ts    # Generated from real error
```

### 2. Validation Report

`docs/features/feature/test-generation/VALIDATION_REPORT.md`
```markdown
# Validation Report

## Test Healing
- ✅ Healed 12/15 broken selectors (80% success rate)
- ✅ Average confidence: 0.85
- ✅ All high-confidence healings (>= 0.9) worked correctly

## Test Generation
- ✅ Generated 5 tests from real errors
- ✅ 4/5 reproduced the error correctly (80% success rate)
- ✅ 1 failure: network mock not working (needs improvement)

## Failure Classification
- ✅ 8/10 classified correctly (80% accuracy)
- ✅ 2 misclassifications: timing_flaky vs selector_broken ambiguity

## Conclusion
Feature is production-ready with known limitations.
```

### 3. Known Limitations

`docs/features/feature/test-generation/LIMITATIONS.md`
```markdown
# Known Limitations

1. **Selector Healing** — Cannot heal dynamic class names (CSS-in-JS)
2. **Test Generation** — Network mocks require manual review
3. **Classification** — Ambiguous between timing_flaky and selector_broken
4. **DOM Queries** — Requires browser extension connected
```

## Blockers to Validation

### Required Infrastructure

1. **Demo app with known errors** — Need reproducible error scenarios
2. **Extension DOM query integration** — Currently using heuristics
3. **Actual test files** — Need representative broken tests

### Missing Pieces

The following features are implemented in code but not wired up:

- ❌ **DOM query to extension** — `test_heal` uses heuristic matching, not real DOM queries
- ❌ **Error ID assignment** — `observe` tool doesn't assign error IDs yet
- ❌ **File writing** — `test_heal` with `auto_apply: true` doesn't write to disk yet

## Next Steps

### Before claiming TestSprite parity:

1. ✅ Complete `test_classify.batch` implementation
2. ⬜ Wire up DOM query integration
3. ⬜ Implement error ID assignment in `observe` tool
4. ⬜ Implement file writing for `auto_apply: true`
5. ⬜ Execute Phase 1 validation (Proof of Concept)
6. ⬜ Execute Phase 2 validation (Test Generation)
7. ⬜ Execute Phase 3 validation (Classification)
8. ⬜ Create validation artifacts

**Estimated time:** 4-6 hours of integration work + 2 hours of validation

## Success Metrics

We can claim "TestSprite parity" when:

- ✅ All 3 validation phases pass
- ✅ Success rate >= 75% for each feature
- ✅ Real-world examples documented
- ✅ Known limitations documented
- ✅ No critical bugs in happy path

---

## Appendix: Why Unit Tests Aren't Enough

Our unit tests verify the **logic**, but not the **value**:

- ✅ `TestTestHeal_AnalyzeFile` verifies we can parse selectors from a test file
- ❌ Doesn't verify the parsed selectors actually exist in the browser
- ✅ `TestTestHeal_RepairSelector` verifies we can generate a new selector
- ❌ Doesn't verify the new selector finds the right element
- ✅ `TestTestFromContext_Error` verifies we can generate test code
- ❌ Doesn't verify the generated test reproduces the error

**Analogy:** It's like unit testing a calculator by verifying `add(2, 2)` returns a number, without checking if it returns `4`.
