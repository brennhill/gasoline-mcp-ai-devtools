// csv.go — CSV output formatter.
// Produces CSV output for bulk operations and piping.
package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
)

// CSVFormatter produces CSV output.
type CSVFormatter struct{}

// Format writes a single result as CSV (header + one row).
func (f *CSVFormatter) Format(w Writer, result *Result) error {
	return f.FormatMultiple(w, []*Result{result})
}

// FormatMultiple writes multiple results as CSV (header + N rows).
func (f *CSVFormatter) FormatMultiple(w Writer, results []*Result) error {
	if len(results) == 0 {
		return nil
	}

	// Collect all unique data keys across results for column headers
	keySet := make(map[string]bool)
	for _, r := range results {
		for k := range r.Data {
			keySet[k] = true
		}
	}

	// Build sorted data columns (for deterministic output)
	var dataKeys []string
	for k := range keySet {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)

	// Build header: success, tool, action, error, [data keys...]
	header := []string{"success", "tool", "action", "error"}
	header = append(header, dataKeys...)

	// Use a strings.Builder as an intermediate writer since csv.NewWriter
	// needs an io.Writer (not our Writer interface)
	var sb strings.Builder
	cw := csv.NewWriter(&sb)

	if err := cw.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	for _, r := range results {
		row := []string{
			fmt.Sprintf("%t", r.Success),
			r.Tool,
			r.Action,
			r.Error,
		}

		// Add data values in key order
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
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	_, err := io.WriteString(w.(io.Writer), sb.String())
	return err
}

// GetFormatter returns the appropriate formatter for the given format string.
func GetFormatter(format string) Formatter {
	switch format {
	case "json":
		return &JSONFormatter{}
	case "csv":
		return &CSVFormatter{}
	case "human":
		return &HumanFormatter{}
	default:
		return &HumanFormatter{} // fallback
	}
}
