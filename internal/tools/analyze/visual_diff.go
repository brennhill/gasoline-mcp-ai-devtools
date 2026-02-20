// visual_diff.go — Visual regression baseline/diff argument parsing and helpers.
package analyze

import (
	"encoding/json"
	"errors"
)

// VisualBaselineArgs holds parsed arguments for saving a visual baseline.
type VisualBaselineArgs struct {
	Name string `json:"name"`
}

// ParseVisualBaselineArgs validates and parses visual baseline arguments.
func ParseVisualBaselineArgs(args json.RawMessage) (*VisualBaselineArgs, error) {
	var params struct {
		Name string `json:"name"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
	}
	if params.Name == "" {
		return nil, errors.New("required parameter 'name' is missing")
	}
	return &VisualBaselineArgs{Name: params.Name}, nil
}

// VisualDiffArgs holds parsed arguments for visual diff comparison.
type VisualDiffArgs struct {
	Baseline  string `json:"baseline"`
	Threshold int    `json:"threshold"`
}

// ParseVisualDiffArgs validates and parses visual diff arguments.
func ParseVisualDiffArgs(args json.RawMessage) (*VisualDiffArgs, error) {
	var params struct {
		Baseline  string `json:"baseline"`
		Threshold int    `json:"threshold"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
	}
	if params.Baseline == "" {
		return nil, errors.New("required parameter 'baseline' is missing")
	}
	if params.Threshold <= 0 {
		params.Threshold = 30
	}
	if params.Threshold > 255 {
		params.Threshold = 255
	}
	return &VisualDiffArgs{
		Baseline:  params.Baseline,
		Threshold: params.Threshold,
	}, nil
}

// BaselineMetadata stores information about a saved visual baseline.
type BaselineMetadata struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	SavedAt   string `json:"saved_at"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Name      string `json:"name"`
	Timestamp int64  `json:"timestamp"`
}
