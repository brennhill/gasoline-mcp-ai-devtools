// tools_observe_analysis_unit_test.go — Unit tests for parseTimelineIncludes.
package main

import "testing"

func TestParseTimelineIncludes(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    timelineIncludes
	}{
		{
			name:  "empty input returns all true",
			input: nil,
			want:  timelineIncludes{actions: true, errors: true, network: true, ws: true},
		},
		{
			name:  "empty slice returns all true",
			input: []string{},
			want:  timelineIncludes{actions: true, errors: true, network: true, ws: true},
		},
		{
			name:  "single valid value actions",
			input: []string{"actions"},
			want:  timelineIncludes{actions: true},
		},
		{
			name:  "single valid value errors",
			input: []string{"errors"},
			want:  timelineIncludes{errors: true},
		},
		{
			name:  "single valid value network",
			input: []string{"network"},
			want:  timelineIncludes{network: true},
		},
		{
			name:  "single valid value websocket",
			input: []string{"websocket"},
			want:  timelineIncludes{ws: true},
		},
		{
			name:  "all four values",
			input: []string{"actions", "errors", "network", "websocket"},
			want:  timelineIncludes{actions: true, errors: true, network: true, ws: true},
		},
		{
			name:  "unknown value ignored",
			input: []string{"bogus"},
			want:  timelineIncludes{},
		},
		{
			name:  "case sensitive mismatch",
			input: []string{"Actions"},
			want:  timelineIncludes{},
		},
		{
			name:  "mixed valid and invalid",
			input: []string{"actions", "bogus", "network"},
			want:  timelineIncludes{actions: true, network: true},
		},
		{
			name:  "duplicates are idempotent",
			input: []string{"errors", "errors"},
			want:  timelineIncludes{errors: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTimelineIncludes(tt.input)
			if got != tt.want {
				t.Errorf("parseTimelineIncludes(%v) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}
