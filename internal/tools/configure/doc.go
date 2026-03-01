// Purpose: Package configure — implementation helpers for the configure MCP tool's policy, rewrite, and audit flows.
// Why: Centralizes configure logic so policy/rewrite behavior remains deterministic and testable.
// Docs: docs/features/feature/config-profiles/index.md

/*
Package configure provides the implementation for the configure MCP tool, which
manages session settings, noise rules, test boundaries, and capabilities discovery.

Key types:
  - Deps: interface declaring dependencies required from the host server.
  - TestBoundaryStartResult: validated parameters for test isolation boundaries.

Key functions:
  - SummarizeAuditEntries: aggregates audit entries into tool call counts and success rates.
  - RewriteNoiseRuleArgs: normalizes noise_action to the canonical action field.
  - DescribeCapabilities: introspects tool schemas to build capability descriptions.
  - ParseTestBoundaryStart: validates test_boundary_start parameters.
*/
package configure
