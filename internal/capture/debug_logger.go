// debug_logger.go -- Re-exports debug logger for capture package backward compatibility.
// Why: Preserves capture package API while debug logging logic lives in internal/debuglog.

package capture

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/debuglog"

// DebugLogger is an alias to the canonical type in internal/debuglog.
type DebugLogger = debuglog.Logger

// NewDebugLogger re-exports debuglog.NewLogger for backward compatibility.
var NewDebugLogger = debuglog.NewLogger

const debugLogSize = debuglog.LogSize
