package main

import (
	"flag"
	"io"
	"os"
	"testing"
)

func TestRegisterFlagsAcceptsPersist(t *testing.T) {
	// registerFlags uses global flag state and os.Args.
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet(originalArgs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{originalArgs[0], "--persist", "--port", "7891"}

	parsed := registerFlags()
	if parsed == nil {
		t.Fatal("registerFlags() returned nil")
	}
	if parsed.port == nil || *parsed.port != 7891 {
		t.Fatalf("registerFlags() port = %v, want 7891", parsed.port)
	}
}

func TestRegisterFlagsAcceptsPersistFalse(t *testing.T) {
	// registerFlags uses global flag state and os.Args.
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet(originalArgs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{originalArgs[0], "--persist=false", "--max-entries", "777"}

	parsed := registerFlags()
	if parsed == nil {
		t.Fatal("registerFlags() returned nil")
	}
	if parsed.maxEntries == nil || *parsed.maxEntries != 777 {
		t.Fatalf("registerFlags() maxEntries = %v, want 777", parsed.maxEntries)
	}
}

func TestRegisterFlagsAcceptsParallel(t *testing.T) {
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet(originalArgs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{originalArgs[0], "--parallel"}

	parsed := registerFlags()
	if parsed == nil {
		t.Fatal("registerFlags() returned nil")
	}
	if parsed.parallelMode == nil || !*parsed.parallelMode {
		t.Fatalf("registerFlags() parallelMode = %v, want true", parsed.parallelMode)
	}
}
