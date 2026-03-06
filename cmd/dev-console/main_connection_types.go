// Purpose: Connection-health payload types and startup error types for bridge connection flow.
// Why: Keeps typed protocol parsing and error semantics separate from orchestration logic.

package main

import (
	"encoding/json"
	"strings"
)

type serverVersionMismatchError struct {
	expected string
	actual   string
}

type nonGasolineServiceError struct {
	serviceName string
}

type healthMetadata struct {
	Version     string `json:"version"`
	Service     string `json:"service"`
	ServiceName string `json:"service-name"`
	Name        string `json:"name"`
}

func decodeHealthMetadata(body []byte) (healthMetadata, bool) {
	var meta healthMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return healthMetadata{}, false
	}
	return meta, true
}

func (m healthMetadata) resolvedServiceName() string {
	if strings.TrimSpace(m.ServiceName) != "" {
		return strings.TrimSpace(m.ServiceName)
	}
	if strings.TrimSpace(m.Service) != "" {
		return strings.TrimSpace(m.Service)
	}
	return strings.TrimSpace(m.Name)
}
