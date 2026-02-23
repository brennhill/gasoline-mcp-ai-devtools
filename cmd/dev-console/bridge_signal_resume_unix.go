//go:build !windows

package main

import (
	"os"
	"syscall"
)

func signalResumeProcess(p *os.Process) {
	_ = p.Signal(syscall.SIGCONT)
}
