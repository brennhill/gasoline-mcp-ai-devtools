// Purpose: Package observe — implementation helpers for the observe MCP tool's filtering, bundling, and summary modes.
// Why: Centralizes observe query behavior so evidence filtering stays predictable across all modes.
// Docs: docs/features/feature/observe/index.md

/*
Package observe provides the implementation for the observe MCP tool, which reads
captured browser state from extension buffers.

Key types:
  - Handler: function signature for observe mode handlers.
  - Deps: interface declaring dependencies required from the host server.

Key functions:
  - Schema: returns the MCP tool definition for the observe tool.
  - GetErrors: handles observe(what:"errors") mode.
  - GetLogs: handles observe(what:"logs") mode with level/source/URL filtering.
  - GetNetworkWaterfall: handles observe(what:"network_waterfall") with summary support.
  - GetErrorBundles: assembles correlated error context bundles.
  - GetSummarizedLogs: groups and fingerprints log entries into aggregated summaries.
*/
package observe
