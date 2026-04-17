package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

type doctorCheck = health.DoctorCheck

func runDoctorChecks(cap *capture.Store) []doctorCheck {
	return health.RunDoctorChecks(cap)
}
