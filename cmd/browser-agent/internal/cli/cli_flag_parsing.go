// cli_flag_parsing.go — Shared CLI flag parsing primitives used by tool-specific command parsers.
// Why: Keeps flag decoding/validation logic DRY across observe/analyze/generate/configure/interact parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --- Generic flag parser ---

// CLIFlagKind defines the type of a CLI flag value.
type CLIFlagKind int

const (
	FlagString CLIFlagKind = iota
	FlagInt
	FlagBool
	FlagStringList
	FlagJSON
	FlagJSONOrString
	FlagIntOrString
)

// CLIFlagSpec maps a CLI flag to an MCP argument key and its value type.
type CLIFlagSpec struct {
	MCPKey string
	Kind   CLIFlagKind
}

// ParseFlagsBySpec parses CLI args against a spec map and returns MCP argument key-value pairs.
func ParseFlagsBySpec(args []string, specs map[string]CLIFlagSpec) (map[string]any, error) {
	out := make(map[string]any)
	for i := 0; i < len(args); i++ {
		flag := args[i]
		spec, ok := specs[flag]
		if !ok {
			return nil, fmt.Errorf("unknown flag: %s", flag)
		}
		switch spec.Kind {
		case FlagBool:
			out[spec.MCPKey] = true
		case FlagString:
			val, next, err := RequireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = val
			i = next
		case FlagInt:
			val, next, err := RequireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			n, err := ParseIntValue(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = n
			i = next
		case FlagStringList:
			val, next, err := RequireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = ParseCSVList(val)
			i = next
		case FlagJSON:
			val, next, err := RequireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			var parsed any
			if err := json.Unmarshal([]byte(val), &parsed); err != nil {
				return nil, fmt.Errorf("%s: invalid JSON: %w", flag, err)
			}
			out[spec.MCPKey] = parsed
			i = next
		case FlagJSONOrString:
			val, next, err := RequireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = ParseJSONOrString(val)
			i = next
		case FlagIntOrString:
			val, next, err := RequireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			if n, err := strconv.Atoi(val); err == nil {
				out[spec.MCPKey] = n
			} else {
				out[spec.MCPKey] = val
			}
			i = next
		default:
			return nil, fmt.Errorf("unsupported parser kind for %s", flag)
		}
	}
	return out, nil
}

// RequireFlagValue returns the next arg as the flag's value, erroring if missing or another flag.
func RequireFlagValue(args []string, idx int) (string, int, error) {
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

// ParseIntValue parses a string as an integer.
func ParseIntValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q: %w", s, err)
	}
	return n, nil
}

// ParseCSVList splits a comma-separated string into a trimmed string slice.
func ParseCSVList(s string) []string {
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

// ParseJSONOrString attempts to parse s as JSON object; returns the raw string on failure.
func ParseJSONOrString(s string) any {
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return s
}

// --- Low-level flag parsers ---

// CLIParseFlag extracts a string flag value from args, returning the value and remaining args.
func CLIParseFlag(args []string, flag string) (string, []string) {
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
