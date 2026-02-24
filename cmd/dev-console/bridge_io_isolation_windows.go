// Purpose: Implements bridge transport lifecycle, forwarding, and reconnect behavior.
// Why: Keeps client tool calls resilient across daemon restarts and transport disruptions.
// Docs: docs/features/feature/bridge-restart/index.md

package main

import "os"

func duplicateStdoutForTransport(stdout *os.File) (*os.File, error) {
	// Windows fallback: keep existing stdout handle as transport writer and rely
	// on process-level stream reassignment below.
	return stdout, nil
}

func redirectProcessStdStreams(target *os.File) error {
	os.Stdout = target
	os.Stderr = target
	return nil
}
