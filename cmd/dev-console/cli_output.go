// cli_output.go — Output formatters for CLI mode: human, json, csv.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// cliResult holds parsed output data for formatting.
type cliResult struct {
	Success     bool
	Tool        string
	Action      string
	TextContent string
	Error       string
	Data        map[string]any
}

// formatResult formats the MCP tool result and writes to stdout. Returns exit code.
func formatResult(format, tool, action string, result *MCPToolResult) int {
	cliRes := buildCLIResult(tool, action, result)
	var err error

	switch format {
	case "json":
		err = formatJSON(os.Stdout, cliRes)
	case "csv":
		err = formatCSV(os.Stdout, cliRes)
	default:
		err = formatHuman(os.Stdout, cliRes)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] output error: %v\n", err)
		return 1
	}

	if !cliRes.Success {
		return 1
	}
	return 0
}

// buildCLIResult extracts text content from MCPToolResult and attempts JSON parse.
func buildCLIResult(tool, action string, result *MCPToolResult) *cliResult {
	r := &cliResult{
		Success: !result.IsError,
		Tool:    tool,
		Action:  action,
	}

	// Concatenate text from content blocks
	var parts []string
	for _, block := range result.Content {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	r.TextContent = strings.Join(parts, "\n")

	if result.IsError {
		r.Error = r.TextContent
		return r
	}

	// Try to parse as JSON for structured output
	var data map[string]any
	if err := json.Unmarshal([]byte(r.TextContent), &data); err == nil {
		r.Data = data
	}

	return r
}

// formatHuman writes human-readable output.
func formatHuman(w io.Writer, r *cliResult) error {
	var sb strings.Builder

	if r.Success {
		sb.WriteString(fmt.Sprintf("[OK] %s %s\n", r.Tool, r.Action))
	} else {
		sb.WriteString(fmt.Sprintf("[Error] %s %s\n", r.Tool, r.Action))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", r.Error))
		}
	}

	if r.TextContent != "" {
		sb.WriteString("\n")
		sb.WriteString(r.TextContent)
		if !strings.HasSuffix(r.TextContent, "\n") {
			sb.WriteString("\n")
		}
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// formatJSON writes pretty-printed JSON output with merged data fields.
func formatJSON(w io.Writer, r *cliResult) error {
	out := map[string]any{
		"success": r.Success,
		"tool":    r.Tool,
		"action":  r.Action,
	}

	if r.Error != "" {
		out["error"] = r.Error
	}

	// Merge data fields into output
	for k, v := range r.Data {
		out[k] = v
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// formatCSV writes CSV output with header + data row.
func formatCSV(w io.Writer, r *cliResult) error {
	// Collect data keys for additional columns
	var dataKeys []string
	for k := range r.Data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)

	header := []string{"success", "tool", "action", "error"}
	header = append(header, dataKeys...)

	var sb strings.Builder
	cw := csv.NewWriter(&sb)

	if err := cw.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	row := []string{
		fmt.Sprintf("%t", r.Success),
		r.Tool,
		r.Action,
		r.Error,
	}
	for _, k := range dataKeys {
		val := ""
		if v, ok := r.Data[k]; ok {
			val = fmt.Sprintf("%v", v)
		}
		row = append(row, val)
	}

	if err := cw.Write(row); err != nil {
		return fmt.Errorf("write CSV row: %w", err)
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
