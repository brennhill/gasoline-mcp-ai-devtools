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
