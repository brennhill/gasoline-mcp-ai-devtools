// cli_hook.go — Dispatch for `gasoline hook <name>` subcommands.
// Provides compiled Go implementations of Claude Code PostToolUse hooks.

package main

import (
	"fmt"
	"os"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/hook"
)

// hookSubcommands maps hook names to their implementations.
var hookSubcommands = map[string]func() int{
	"compress-output": runHookCompressOutput,
	"quality-gate":    runHookQualityGate,
}

// runHookMode dispatches to the named hook subcommand. Returns exit code.
func runHookMode(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: gasoline hook <name>\n")
		fmt.Fprintf(os.Stderr, "  Available hooks: compress-output, quality-gate\n")
		return 2
	}
	name := args[1]
	fn, ok := hookSubcommands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown hook: %s\n", name)
		fmt.Fprintf(os.Stderr, "  Available hooks: compress-output, quality-gate\n")
		return 2
	}
	return fn()
}

// runHookCompressOutput reads hook input from stdin, compresses Bash output,
// and writes additionalContext JSON to stdout.
func runHookCompressOutput() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0 // Silent exit on bad input — hook protocol.
	}

	result := hook.CompressOutput(input)
	if result == nil {
		return 0
	}

	// Post stats to daemon (best-effort).
	port := os.Getenv("GASOLINE_PORT")
	if port == "" {
		port = fmt.Sprintf("%d", defaultPort)
	}
	result.PostStats(port)

	if err := hook.WriteOutput(os.Stdout, result.FormatContext()); err != nil {
		return 1
	}
	return 0
}

// runHookQualityGate reads hook input from stdin, checks the file against
// project standards, and writes additionalContext JSON to stdout.
func runHookQualityGate() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0 // Silent exit on bad input — hook protocol.
	}

	result := hook.RunQualityGate(input)
	if result == nil {
		return 0
	}

	if err := hook.WriteOutput(os.Stdout, result.Context); err != nil {
		return 1
	}
	return 0
}
