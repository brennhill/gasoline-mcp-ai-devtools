// Purpose: Classifies log entries by severity into fingerprinted error and warning maps for checkpoint diffs.
// Why: Separates console log classification from the main checkpoint diff computation.
package checkpoint

import gasTypes "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"

type fingerprintEntry struct {
	message string
	source  string
	count   int
}

type classifiedLogs struct {
	totalNew     int
	errorMap     map[string]*fingerprintEntry
	errorOrder   []string
	warningMap   map[string]*fingerprintEntry
	warningOrder []string
}

func classifyLogEntries(entries []gasTypes.LogEntry, severity string) classifiedLogs {
	cl := classifiedLogs{
		errorMap:   make(map[string]*fingerprintEntry),
		warningMap: make(map[string]*fingerprintEntry),
	}
	for _, entry := range entries {
		cl.totalNew++
		level, _ := entry["level"].(string)
		msg := extractLogMessage(entry)
		source, _ := entry["source"].(string)

		switch {
		case level == "error":
			addToFingerprintMap(cl.errorMap, &cl.errorOrder, msg, source)
		case (level == "warn" || level == "warning") && severity != "errors_only":
			addToFingerprintMap(cl.warningMap, &cl.warningOrder, msg, source)
		}
	}
	return cl
}

func extractLogMessage(entry gasTypes.LogEntry) string {
	msg, _ := entry["msg"].(string)
	if msg == "" {
		msg, _ = entry["message"].(string)
	}
	return msg
}

func addToFingerprintMap(m map[string]*fingerprintEntry, order *[]string, msg, source string) {
	fp := FingerprintMessage(msg)
	if existing, ok := m[fp]; ok {
		existing.count++
		return
	}
	m[fp] = &fingerprintEntry{message: truncateMessage(msg), source: source, count: 1}
	*order = append(*order, fp)
}

func buildConsoleEntries(m map[string]*fingerprintEntry, order []string) []ConsoleEntry {
	var entries []ConsoleEntry
	for i, fp := range order {
		if i >= maxDiffEntriesPerCat {
			break
		}
		e := m[fp]
		entries = append(entries, ConsoleEntry{
			Message: e.message,
			Source:  e.source,
			Count:   e.count,
		})
	}
	return entries
}
