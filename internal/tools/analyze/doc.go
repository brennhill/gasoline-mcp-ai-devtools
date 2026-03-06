// Purpose: Package analyze — implementation helpers for the analyze MCP tool's inspection and diff modes.
// Why: Centralizes analysis logic to keep handler behavior consistent across command paths.
// Docs: docs/features/feature/analyze-tool/index.md

/*
Package analyze provides the implementation for the analyze MCP tool, which performs
active analysis of browser state.

Key types:
  - Deps: interface declaring dependencies required from the host server.
  - ComputedStylesArgs: parsed arguments for computed styles queries.
  - FormsArgs: parsed arguments for form discovery queries.
  - Region: rectangular area of changed pixels in image diffs.

Key functions:
  - ParseComputedStylesArgs: validates computed styles query parameters.
  - ParseFormsArgs: validates form discovery query parameters.
  - DiffImages: computes pixel-level differences between two screenshots.
  - ValidateURLs: validates batches of URLs concurrently with SSRF-safe transport.
  - ParseVisualBaselineArgs: validates visual baseline save/diff parameters.
*/
package analyze
