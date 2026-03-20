// Purpose: Package schema — MCP tool schema definitions for all five tools (observe, analyze, generate, configure, interact).
// Why: Keeps tool interfaces strict and synchronized across server, extension, and clients.
// Docs: docs/features/feature/api-schema/index.md

/*
Package schema defines the MCP tool input schemas for all five Gasoline tools.

Each tool has a dedicated schema function that returns an mcp.MCPTool with the
tool's name, description, and JSON Schema for its input parameters.

Key functions:
  - ObserveToolSchema: returns the observe tool definition.
  - analyzeToolSchema: returns the analyze tool definition.
  - generateToolSchema: returns the generate tool definition.
  - configureToolSchema: returns the configure tool definition.
  - InteractToolSchema: returns the interact tool definition.
*/
package schema
