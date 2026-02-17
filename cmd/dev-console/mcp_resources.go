// mcp_resources.go — Shared MCP resource and template catalogs for handler and bridge fast path.
package main

func mcpResources() []MCPResource {
	return []MCPResource{
		{
			URI:         "gasoline://capabilities",
			Name:        "Gasoline Capability Index",
			Description: "Compact capability index with task-to-playbook routing hints",
			MimeType:    "text/markdown",
		},
		{
			URI:         "gasoline://guide",
			Name:        "Gasoline Usage Guide",
			Description: "How to use Gasoline MCP tools for browser debugging",
			MimeType:    "text/markdown",
		},
		{
			URI:         "gasoline://quickstart",
			Name:        "Gasoline MCP Quickstart",
			Description: "Short, canonical MCP call examples and workflows",
			MimeType:    "text/markdown",
		},
	}
}

func mcpResourceTemplates() []any {
	return []any{
		map[string]any{
			"uriTemplate": "gasoline://playbook/{capability}/{level}",
			"name":        "Gasoline Capability Playbook",
			"description": "On-demand, token-efficient playbooks. Start with quick; use full for deep workflows.",
			"mimeType":    "text/markdown",
		},
		map[string]any{
			"uriTemplate": "gasoline://demo/{name}",
			"name":        "Gasoline Demo Script",
			"description": "Demo scripts for websockets, annotations, recording, and dependency vetting",
			"mimeType":    "text/markdown",
		},
	}
}
