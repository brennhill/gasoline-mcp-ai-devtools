// Purpose: Package generate — implementation helpers for the generate MCP tool's artifact emission.
// Why: Centralizes artifact generation logic to avoid drift across output formats.
// Docs: docs/features/feature/test-generation/index.md

/*
Package generate provides the implementation for the generate MCP tool, which
produces artifacts from captured browser data.

Key types:
  - Deps: interface declaring dependencies required from the host server.
  - TestGenParams: parsed arguments for generate({format: "test"}).

Key functions:
  - BuildCSPDirectives: extracts origins from network bodies and groups them by CSP directive.
  - GenerateTestScript: produces a Playwright test script from captured actions and reproduction data.
*/
package generate
