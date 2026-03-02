// Purpose: Shared CLI flag parsing primitives used by tool-specific command parsers.
// Why: Keeps flag decoding/validation logic DRY across observe/analyze/generate/configure/interact parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --- Generic flag parser ---

type cliFlagKind int

const (
	flagString cliFlagKind = iota
	flagInt
	flagBool
	flagStringList
	flagJSON
	flagJSONOrString
	flagIntOrString
)

type cliFlagSpec struct {
	mcpKey string
	kind   cliFlagKind
}

func parseFlagsBySpec(args []string, specs map[string]cliFlagSpec) (map[string]any, error) {
	out := make(map[string]any)
	for i := 0; i < len(args); i++ {
		flag := args[i]
		spec, ok := specs[flag]
		if !ok {
			return nil, fmt.Errorf("unknown flag: %s", flag)
		}
		switch spec.kind {
		case flagBool:
			out[spec.mcpKey] = true
		case flagString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = val
			i = next
		case flagInt:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			n, err := parseIntValue(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = n
			i = next
		case flagStringList:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = parseCSVList(val)
			i = next
		case flagJSON:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			var parsed any
			if err := json.Unmarshal([]byte(val), &parsed); err != nil {
				return nil, fmt.Errorf("%s: invalid JSON: %w", flag, err)
			}
			out[spec.mcpKey] = parsed
			i = next
		case flagJSONOrString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.mcpKey] = parseJSONOrString(val)
			i = next
		case flagIntOrString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			if n, err := strconv.Atoi(val); err == nil {
				out[spec.mcpKey] = n
			} else {
				out[spec.mcpKey] = val
			}
			i = next
		default:
			return nil, fmt.Errorf("unsupported parser kind for %s", flag)
		}
	}
	return out, nil
}

func requireFlagValue(args []string, idx int) (string, int, error) {
	next := idx + 1
	if next >= len(args) {
		return "", idx, fmt.Errorf("cli_parse: no value provided after flag. Add a value after the flag")
	}
	val := args[next]
	if strings.HasPrefix(val, "--") {
		return "", idx, fmt.Errorf("cli_parse: expected a value but got flag %q. Provide a value between the flags", val)
	}
	return val, next, nil
}

func parseIntValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q: %w", s, err)
	}
	return n, nil
}

func parseCSVList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// parseJSONOrString attempts to parse s as JSON object; returns the raw string on failure.
func parseJSONOrString(s string) any {
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return s
}

// --- Low-level flag parsers ---

// cliParseFlag extracts a string flag value from args, returning the value and remaining args.
func cliParseFlag(args []string, flag string) (string, []string) {
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

// cliParseFlagInt extracts an integer flag value from args.
func cliParseFlagInt(args []string, flag string) (int, bool, []string) {
	val, remaining := cliParseFlag(args, flag)
	if val == "" {
		return 0, false, args
	}
	var n int
	for _, c := range val {
		if c < '0' || c > '9' {
			return 0, false, args
		}
		n = n*10 + int(c-'0')
	}
	return n, true, remaining
}

// cliParseFlagBool checks if a boolean flag is present in args.
func cliParseFlagBool(args []string, flag string) (bool, []string) {
	for i, a := range args {
		if a == flag {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}
