// main.go — Entry point for gasoline-cmd CLI binary.
// Translates CLI commands to MCP JSON-RPC calls against gasoline-mcp server.
//
// Usage: gasoline-cmd <tool> <action> [options] [--flags]
//
// Tools: interact, observe, configure, generate
// Formats: --format human (default), --format json, --format csv
// Streaming: --stream for newline-delimited JSON progress
//
// Exit codes:
//   0 = success
//   1 = error (tool call failed)
//   2 = usage error (missing args, invalid flags)
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/dev-console/dev-console/cmd/gasoline-cmd/commands"
	"github.com/dev-console/dev-console/cmd/gasoline-cmd/config"
	"github.com/dev-console/dev-console/cmd/gasoline-cmd/output"
	"github.com/dev-console/dev-console/cmd/gasoline-cmd/server"
)

// version is set at build time via -ldflags.
var version = "1.0.0"

const usageText = `gasoline-cmd — CLI interface for Gasoline MCP tools

Usage:
  gasoline-cmd <tool> <action> [options] [--flags]

Tools:
  interact     Browser interaction (click, type, navigate, upload, etc.)
  observe      Browser observation (logs, errors, network, performance, accessibility)
  configure    Server configuration (noise rules, storage, clear buffers)
  generate     Generate artifacts (tests, reproductions, HAR, CSP)

Global Flags:
  --format <human|json|csv>   Output format (default: human)
  --server-port <port>        MCP server port (default: 7890)
  --timeout <ms>              Request timeout in ms (default: 5000)
  --stream                    Enable streaming progress (newline-delimited JSON)
  --no-auto-start             Don't auto-start server if not running
  --csv-file <path>           CSV input file for bulk operations
  --version                   Show version
  --help                      Show this help

Examples:
  gasoline-cmd interact click --selector "#submit-btn"
  gasoline-cmd interact type --selector "#title" --text "My Video"
  gasoline-cmd interact navigate --url "https://example.com"
  gasoline-cmd observe logs --limit 50 --min-level warn
  gasoline-cmd observe errors --limit 20
  gasoline-cmd observe network_waterfall --url "api.example.com"
  gasoline-cmd configure noise_rule --pattern "favicon.ico" --reason "noise"
  gasoline-cmd configure clear --buffer network
  gasoline-cmd generate test --test-name "my_test"
  gasoline-cmd generate har --url "api.example.com" --format json
`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the main entry point, separated for testability.
// Returns the exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}

	// Handle --version and --help before anything else
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("gasoline-cmd %s\n", version)
			return 0
		}
		if arg == "--help" || arg == "-h" {
			fmt.Print(usageText)
			return 0
		}
	}

	// Extract tool and action from positional args
	tool := args[0]
	if tool == "help" {
		fmt.Print(usageText)
		return 0
	}

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: missing action for tool %q\n\n", tool)
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}

	action := args[1]
	remaining := args[2:]

	// Extract global flags from remaining args
	flags, remaining := extractGlobalFlags(remaining)

	// Load configuration with cascade
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine working directory: %v\n", err)
		return 1
	}

	cfg, err := config.Load(cwd, flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: configuration: %v\n", err)
		return 2
	}

	// Check for --csv-file for bulk operations
	csvFile, remaining := extractFlag(remaining, "--csv-file")

	// Get formatter
	formatter := output.GetFormatter(cfg.Format)

	// Parse tool-specific arguments
	var mcpArgs map[string]any
	var toolName string

	switch tool {
	case "interact":
		toolName = "mcp__gasoline__interact"
		mcpArgs, err = commands.InteractArgs(action, remaining)
	case "observe":
		toolName = "mcp__gasoline__observe"
		mcpArgs, err = commands.ObserveArgs(action, remaining)
	case "configure":
		toolName = "mcp__gasoline__configure"
		mcpArgs, err = commands.ConfigureArgs(action, remaining)
	case "generate":
		toolName = "mcp__gasoline__generate"
		mcpArgs, err = commands.GenerateArgs(action, remaining)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown tool %q. Valid tools: interact, observe, configure, generate\n", tool)
		return 2
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// Handle bulk CSV operations
	if csvFile != "" {
		return runBulk(cfg, toolName, tool, action, mcpArgs, csvFile, formatter)
	}

	// Connect to server (auto-start if needed)
	client, err := server.EnsureRunning(cfg.ServerPort, cfg.AutoStartServer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Initialize MCP session
	if err := client.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: MCP initialize: %v\n", err)
		return 1
	}

	// Execute the tool call
	toolResult, err := client.CallTool(toolName, mcpArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Build output result
	textContent := ""
	if len(toolResult.Content) > 0 {
		textContent = toolResult.Content[0].Text
		if len(toolResult.Content) > 1 {
			var parts []string
			for _, c := range toolResult.Content {
				parts = append(parts, c.Text)
			}
			textContent = strings.Join(parts, "\n")
		}
	}

	result := commands.BuildResult(tool, action, textContent, toolResult.IsError)

	// Format and output
	if err := formatter.Format(os.Stdout, result); err != nil {
		fmt.Fprintf(os.Stderr, "Error: format output: %v\n", err)
		return 1
	}

	if !result.Success {
		return 1
	}
	return 0
}

// runBulk processes a CSV file for bulk operations.
func runBulk(cfg config.Config, toolName, tool, action string, baseArgs map[string]any, csvPath string, formatter output.Formatter) int {
	f, err := os.Open(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open CSV: %v\n", err)
		return 1
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: read CSV: %v\n", err)
		return 1
	}

	if len(records) < 2 {
		fmt.Fprintf(os.Stderr, "Error: CSV file must have header + at least 1 data row\n")
		return 2
	}

	headers := records[0]
	rows := records[1:]

	// Connect to server
	client, err := server.EnsureRunning(cfg.ServerPort, cfg.AutoStartServer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := client.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: MCP initialize: %v\n", err)
		return 1
	}

	var results []*output.Result
	hasFailure := false

	for _, row := range rows {
		// Build args for this row by merging base args with CSV columns
		rowArgs := make(map[string]any)
		for k, v := range baseArgs {
			rowArgs[k] = v
		}

		for i, header := range headers {
			if i < len(row) && row[i] != "" {
				// Map CSV column names to MCP arg names (snake_case)
				argName := commands.NormalizeAction(header)
				rowArgs[argName] = row[i]
			}
		}

		toolResult, err := client.CallTool(toolName, rowArgs)
		if err != nil {
			result := &output.Result{
				Success: false,
				Tool:    tool,
				Action:  action,
				Error:   err.Error(),
			}
			results = append(results, result)
			hasFailure = true
			continue
		}

		textContent := ""
		if len(toolResult.Content) > 0 {
			textContent = toolResult.Content[0].Text
		}

		result := commands.BuildResult(tool, action, textContent, toolResult.IsError)
		results = append(results, result)
		if !result.Success {
			hasFailure = true
		}
	}

	// Output all results
	if csvFormatter, ok := formatter.(*output.CSVFormatter); ok {
		if err := csvFormatter.FormatMultiple(os.Stdout, results); err != nil {
			fmt.Fprintf(os.Stderr, "Error: format output: %v\n", err)
			return 1
		}
	} else {
		for _, r := range results {
			if err := formatter.Format(os.Stdout, r); err != nil {
				fmt.Fprintf(os.Stderr, "Error: format output: %v\n", err)
				return 1
			}
		}
	}

	if hasFailure {
		return 1
	}
	return 0
}

// extractGlobalFlags extracts global flags from args and returns FlagOverrides + remaining args.
func extractGlobalFlags(args []string) (*config.FlagOverrides, []string) {
	flags := &config.FlagOverrides{}
	remaining := args

	// --format
	var format string
	format, remaining = extractFlag(remaining, "--format")
	if format != "" {
		flags.Format = &format
	}

	// --server-port
	var portStr string
	portStr, remaining = extractFlag(remaining, "--server-port")
	if portStr != "" {
		port := parseInt(portStr)
		if port > 0 {
			flags.ServerPort = &port
		}
	}

	// --timeout
	var timeoutStr string
	timeoutStr, remaining = extractFlag(remaining, "--timeout")
	if timeoutStr != "" {
		timeout := parseInt(timeoutStr)
		if timeout > 0 {
			flags.Timeout = &timeout
		}
	}

	// --stream (boolean flag)
	for i, a := range remaining {
		if a == "--stream" {
			stream := true
			flags.Stream = &stream
			remaining = append(remaining[:i], remaining[i+1:]...)
			break
		}
	}

	// --no-auto-start (boolean flag)
	for i, a := range remaining {
		if a == "--no-auto-start" {
			autoStart := false
			flags.AutoStartServer = &autoStart
			remaining = append(remaining[:i], remaining[i+1:]...)
			break
		}
	}

	return flags, remaining
}

// extractFlag removes a flag and its value from args, returning the value and remaining args.
func extractFlag(args []string, flag string) (string, []string) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			val := args[i+1]
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return val, remaining
		}
	}
	return "", args
}

// parseInt parses a string as a positive integer, returning 0 on failure.
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
