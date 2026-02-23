//go:build windows

package main

import "os"

func signalResumeProcess(_ *os.Process) {}
