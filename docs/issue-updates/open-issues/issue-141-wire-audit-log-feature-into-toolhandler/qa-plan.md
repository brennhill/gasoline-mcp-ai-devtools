# QA Plan: Issue #141

## Source
- Issue: https://github.com/brennhill/gasoline-mcp-ai-devtools/issues/141
- Title: Wire audit_log feature into ToolHandler

## QA Goal
Verify that the behavior change for Issue #141 is correct, stable, and non-regressive across impacted workflows.

## Test Strategy
1. Contract validation tests: request parsing, parameter validation, response shape.
2. Runtime behavior tests: successful path, expected failure path, timeout/retry path.
3. Integration tests: end-to-end command execution across the MCP-to-extension/server boundary.
4. Regression tests: adjacent tool modes and previously fixed issue paths.

## Test Scenarios
1. Happy path
   Expected: terminal success with required output fields.
2. Invalid input
   Expected: deterministic structured error with guidance.
3. Missing prerequisite state (for example disconnected extension/tracked target missing)
   Expected: explicit failure reason and no silent drop.
4. Async polling path
   Expected: command transitions from queued/pending to final terminal state.
5. Backward compatibility
   Expected: previously supported requests still succeed unchanged.

## Evidence to Capture
1. Exact request payload used.
2. Exact response payload returned.
3. Correlation ID lifecycle (if async).
4. Any observed logs or telemetry proving correct state transitions.

## Exit Criteria
1. All scenarios pass.
2. No P1/P2 regressions introduced in neighboring workflows.
3. Documentation and schema references are consistent with shipped behavior.
