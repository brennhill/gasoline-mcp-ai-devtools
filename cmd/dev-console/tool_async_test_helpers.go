package main

import "encoding/json"

// withDefaultAsyncMode normalizes tool args for unit tests:
// if sync/wait/background is not specified, default to sync:false so tests
// assert queue contracts without requiring an active extension session.
func withDefaultAsyncMode(argsJSON string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return argsJSON
	}
	if _, hasSync := args["sync"]; hasSync {
		return argsJSON
	}
	if _, hasWait := args["wait"]; hasWait {
		return argsJSON
	}
	if _, hasBackground := args["background"]; hasBackground {
		return argsJSON
	}
	args["sync"] = false
	normalized, err := json.Marshal(args)
	if err != nil {
		return argsJSON
	}
	return string(normalized)
}
