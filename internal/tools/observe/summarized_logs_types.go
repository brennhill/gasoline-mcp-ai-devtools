package observe

import "regexp"

// Pre-compiled fingerprinting regexes (initialized once at package load).
var (
	reANSI       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reUUID       = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reHexHash    = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)
	reNumbers    = regexp.MustCompile(`\d{3,}`)
	reLongQuoted = regexp.MustCompile(`"[^"]{21,}"`)
	reURL        = regexp.MustCompile(`https?://\S+`)
	reTimestamp  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	rePath       = regexp.MustCompile(`/[\w._-]+(/[\w._-]+)+`)
	reWhitespace = regexp.MustCompile(`\s+`)
	reSlugDup    = regexp.MustCompile(`_+`)
)

const maxFingerprintLen = 64

// logEntryView is a lightweight struct for grouping operations,
// extracted from the raw map[string]any log entries.
type logEntryView struct {
	Level   string
	Message string
	Source  string
	URL     string
	Line    any
	Column  any
	TS      string
	TabID   any
}

// LogGroup represents a group of repeated log entries.
type LogGroup struct {
	Fingerprint    string         `json:"fingerprint"`
	SampleMessage  string         `json:"sample_message"`
	Count          int            `json:"count"`
	LevelBreakdown map[string]int `json:"level_breakdown"`
	FirstSeen      string         `json:"first_seen"`
	LastSeen       string         `json:"last_seen"`
	IsPeriodic     bool           `json:"is_periodic"`
	PeriodSeconds  float64        `json:"period_seconds,omitempty"`
	Source         string         `json:"source"`
	Sources        []string       `json:"sources,omitempty"`
	timestamps     []string       // internal, for periodicity detection
	sourceCounts   map[string]int // internal, for primary source detection
}

// LogAnomaly represents a rare/unique log entry (the signal).
type LogAnomaly struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	URL     string `json:"url,omitempty"`
	Line    any    `json:"line,omitempty"`
	Column  any    `json:"column,omitempty"`
	TS      string `json:"timestamp,omitempty"`
	TabID   any    `json:"tab_id,omitempty"`
}
