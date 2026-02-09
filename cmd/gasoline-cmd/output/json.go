// json.go — JSON output formatter.
// Produces machine-parseable JSON output.
package output

import (
	"encoding/json"
)

// JSONFormatter produces JSON output.
type JSONFormatter struct{}

// Format writes a JSON representation of the result.
func (f *JSONFormatter) Format(w Writer, result *Result) error {
	// Build the output map for clean JSON structure
	out := map[string]any{
		"success": result.Success,
		"tool":    result.Tool,
		"action":  result.Action,
	}

	if result.Error != "" {
		out["error"] = result.Error
	}

	// Merge data fields into the output
	for k, v := range result.Data {
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

// StreamFormatter writes newline-delimited JSON events.
type StreamFormatter struct{}

// WriteEvent writes a single streaming event as JSON.
func (f *StreamFormatter) WriteEvent(w Writer, event *StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}
