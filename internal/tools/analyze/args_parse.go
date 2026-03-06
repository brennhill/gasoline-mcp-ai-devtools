// Purpose: Shared JSON argument parsing helpers for analyze tool argument structs.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import "encoding/json"

// parseAnalyzeArgs unmarshals optional JSON args into a typed struct pointer.
func parseAnalyzeArgs[T any](args json.RawMessage) (*T, error) {
	var params T
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
	}
	return &params, nil
}
