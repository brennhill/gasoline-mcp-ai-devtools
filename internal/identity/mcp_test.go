// Purpose: Tests for canonical MCP server identity constants.
// Why: Ensures identity values stay correct and legacy names don't accidentally include the current name.

package identity

import "testing"

func TestMCPServerName(t *testing.T) {
	t.Parallel()
	if MCPServerName != "kaboom-browser-devtools" {
		t.Fatalf("MCPServerName = %q, want %q", MCPServerName, "kaboom-browser-devtools")
	}
}

func TestLegacyMCPServerNames_ContainsExpected(t *testing.T) {
	t.Parallel()
	found := false
	for _, name := range LegacyMCPServerNames {
		if name == "kaboom-agentic-browser" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("LegacyMCPServerNames does not contain \"kaboom-agentic-browser\"")
	}
}

func TestLegacyMCPServerNames_NoSelfReference(t *testing.T) {
	t.Parallel()
	for _, name := range LegacyMCPServerNames {
		if name == MCPServerName {
			t.Fatalf("LegacyMCPServerNames must not contain MCPServerName (%q)", MCPServerName)
		}
	}
}
