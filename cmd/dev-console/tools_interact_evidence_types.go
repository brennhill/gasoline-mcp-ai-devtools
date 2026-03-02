package main

type evidenceMode string

const (
	evidenceModeOff        evidenceMode = "off"
	evidenceModeOnMutation evidenceMode = "on_mutation"
	evidenceModeAlways     evidenceMode = "always"
)

const (
	evidenceRetryEnv       = "GASOLINE_EVIDENCE_RETRY_COUNT"
	evidenceMaxCapturesEnv = "GASOLINE_EVIDENCE_MAX_CAPTURES_PER_COMMAND"
)

type evidenceShot struct {
	Path     string
	Filename string
	Error    string
	Attempts int
}

type commandEvidenceState struct {
	mode          evidenceMode
	action        string
	shouldCapture bool
	maxCaptures   int
	clientID      string
	skipped       string

	before evidenceShot
	after  evidenceShot

	finalized bool
	cached    map[string]any
}

var evidenceCaptureFn = defaultEvidenceCapture
