package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"

type fastPathTelemetrySummary = health.FastPathTelemetrySummary

func evaluateFastPathFailureThreshold(summary fastPathTelemetrySummary, minSamples int, maxFailureRatio float64) error {
	return health.EvaluateFastPathFailureThreshold(summary, minSamples, maxFailureRatio)
}
