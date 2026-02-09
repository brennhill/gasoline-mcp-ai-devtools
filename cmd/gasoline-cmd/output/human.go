// human.go — Human-readable output formatter.
// Produces pretty, colored output for terminal use.
package output

import (
	"fmt"
	"strings"
)

// HumanFormatter produces human-readable output.
type HumanFormatter struct{}

// Format writes a human-readable representation of the result.
func (h *HumanFormatter) Format(w Writer, result *Result) error {
	var sb strings.Builder

	if result.Success {
		sb.WriteString(fmt.Sprintf("[OK] %s %s — Success\n", result.Tool, result.Action))
	} else {
		sb.WriteString(fmt.Sprintf("[Error] %s %s — Failed\n", result.Tool, result.Action))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", result.Error))
		}
	}

	// If there's raw text content from the MCP server, print it
	if result.TextContent != "" {
		sb.WriteString("\n")
		sb.WriteString(result.TextContent)
		if !strings.HasSuffix(result.TextContent, "\n") {
			sb.WriteString("\n")
		}
	}

	// Print key data fields
	if result.Data != nil && result.TextContent == "" {
		for k, v := range result.Data {
			sb.WriteString(fmt.Sprintf("   %s: %v\n", k, v))
		}
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}
