package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"

type HealthMetrics = health.Metrics

func NewHealthMetrics() *health.Metrics {
	return health.NewMetrics()
}
