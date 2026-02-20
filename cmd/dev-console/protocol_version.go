// protocol_version.go — MCP protocol version negotiation and backward compatibility.
package main

import "encoding/json"

const (
	mcpProtocolVersionLatest = "2025-06-18"
	mcpProtocolVersionLegacy = "2024-11-05"
)

// negotiateProtocolVersion returns the protocol version selected for initialize.
// Supports latest and one legacy version for backward compatibility.
func negotiateProtocolVersion(rawParams json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"` // SPEC:MCP
	}
	if len(rawParams) > 0 {
		_ = json.Unmarshal(rawParams, &params)
	}

	switch params.ProtocolVersion {
	case mcpProtocolVersionLatest, mcpProtocolVersionLegacy:
		return params.ProtocolVersion
	default:
		return mcpProtocolVersionLatest
	}
}
