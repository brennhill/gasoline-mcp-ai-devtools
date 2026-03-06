// Purpose: Provides a swappable stderr writer (stderrf) so diagnostic output can be redirected during tests or bridge mode.
// Why: Prevents stderr writes from corrupting MCP stdout transport while keeping diagnostic output accessible.

package main

import (
	"fmt"
	"io"
	"os"
)

var stderrSink io.Writer = os.Stderr

func setStderrSink(w io.Writer) {
	if w == nil {
		return
	}
	stderrSink = w
}

func stderrf(format string, args ...any) {
	_, _ = fmt.Fprintf(stderrSink, format, args...)
}
