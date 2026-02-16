// observe_network_body_filter.go — JSON filtering helpers for observe(network_bodies).
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
)

const maxBodyKeyMatches = 100

type jsonPathToken struct {
	key     string
	index   int
	isIndex bool
}

func applyNetworkBodyFilter(body capture.NetworkBody, bodyKey, bodyPath string) (capture.NetworkBody, bool, error) {
	if bodyKey == "" && bodyPath == "" {
		return body, true, nil
	}
	if body.ResponseBody == "" {
		return body, false, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(body.ResponseBody), &decoded); err != nil {
		// Filtering only applies to JSON response bodies.
		return body, false, nil
	}

	if bodyPath != "" {
		value, ok, err := extractJSONPath(decoded, bodyPath)
		if err != nil {
			return body, false, err
		}
		if !ok {
			return body, false, nil
		}
		return encodeFilteredNetworkBody(body, value)
	}

	matches := make([]any, 0, 4)
	collectJSONValuesByKey(decoded, bodyKey, &matches, maxBodyKeyMatches)
	if len(matches) == 0 {
		return body, false, nil
	}
	if len(matches) == 1 {
		return encodeFilteredNetworkBody(body, matches[0])
	}
	return encodeFilteredNetworkBody(body, matches)
}

func encodeFilteredNetworkBody(body capture.NetworkBody, value any) (capture.NetworkBody, bool, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return body, false, fmt.Errorf("failed to encode filtered value: %w", err)
	}
	filtered := body
	filtered.ResponseBody = string(raw)
	filtered.ResponseTruncated = false
	filtered.BinaryFormat = ""
	filtered.FormatConfidence = 0
	return filtered, true, nil
}

func extractJSONPath(root any, path string) (any, bool, error) {
	tokens, err := parseJSONPath(path)
	if err != nil {
		return nil, false, err
	}

	current := root
	for _, token := range tokens {
		if token.isIndex {
			items, ok := current.([]any)
			if !ok || token.index < 0 || token.index >= len(items) {
				return nil, false, nil
			}
			current = items[token.index]
			continue
		}

		object, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		next, ok := object[token.key]
		if !ok {
			return nil, false, nil
		}
		current = next
	}

	return current, true, nil
}

func parseJSONPath(path string) ([]jsonPathToken, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	if strings.HasPrefix(trimmed, "$.") {
		trimmed = trimmed[2:]
	} else if strings.HasPrefix(trimmed, "$") {
		trimmed = trimmed[1:]
	}

	if trimmed == "" {
		return []jsonPathToken{}, nil
	}

	tokens := make([]jsonPathToken, 0, 6)
	for i := 0; i < len(trimmed); {
		switch trimmed[i] {
		case '.':
			i++
			if i >= len(trimmed) {
				return nil, fmt.Errorf("path cannot end with '.'")
			}
		case '[':
			endOffset := strings.IndexByte(trimmed[i:], ']')
			if endOffset < 0 {
				return nil, fmt.Errorf("missing closing ']' in path")
			}
			end := i + endOffset
			inner := strings.TrimSpace(trimmed[i+1 : end])
			if inner == "" {
				return nil, fmt.Errorf("empty [] segment in path")
			}

			if (inner[0] == '\'' && inner[len(inner)-1] == '\'') || (inner[0] == '"' && inner[len(inner)-1] == '"') {
				key := inner[1 : len(inner)-1]
				if key == "" {
					return nil, fmt.Errorf("empty key segment in path")
				}
				tokens = append(tokens, jsonPathToken{key: key})
			} else {
				index, err := strconv.Atoi(inner)
				if err != nil || index < 0 {
					return nil, fmt.Errorf("invalid array index %q", inner)
				}
				tokens = append(tokens, jsonPathToken{index: index, isIndex: true})
			}
			i = end + 1
		default:
			start := i
			for i < len(trimmed) && trimmed[i] != '.' && trimmed[i] != '[' {
				i++
			}
			key := strings.TrimSpace(trimmed[start:i])
			if key == "" {
				return nil, fmt.Errorf("invalid key segment in path")
			}
			tokens = append(tokens, jsonPathToken{key: key})
		}
	}

	return tokens, nil
}

func collectJSONValuesByKey(node any, key string, out *[]any, max int) {
	if len(*out) >= max {
		return
	}

	switch typed := node.(type) {
	case map[string]any:
		if value, ok := typed[key]; ok {
			*out = append(*out, value)
			if len(*out) >= max {
				return
			}
		}
		for _, child := range typed {
			collectJSONValuesByKey(child, key, out, max)
			if len(*out) >= max {
				return
			}
		}
	case []any:
		for _, child := range typed {
			collectJSONValuesByKey(child, key, out, max)
			if len(*out) >= max {
				return
			}
		}
	}
}
