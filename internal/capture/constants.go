// constants.go — Buffer capacity and configuration constants.
// All configuration values for capture package.
package capture

import (
	"time"

	"github.com/dev-console/dev-console/internal/circuit"
)

const (
	// Buffer capacity constants (exported for health metrics)
	MaxWSEvents        = 500
	MaxNetworkBodies   = 100
	MaxExtensionLogs   = 500
	MaxEnhancedActions = 1000

	// RateLimitThreshold is re-exported from internal/circuit for backward compatibility.
	RateLimitThreshold = circuit.RateLimitThreshold

	maxActiveConns = 20
	maxClosedConns = 10

	// Network waterfall capacity configuration
	DefaultNetworkWaterfallCapacity = 1000
	MinNetworkWaterfallCapacity     = 100
	MaxNetworkWaterfallCapacity     = 10000

	defaultWSLimit       = 50
	defaultBodyLimit     = 20
	maxExtensionPostBody = 5 << 20         // 5MB - max size for incoming extension POST bodies
	maxRequestBodySize   = 8192            // 8KB - truncation limit for captured request bodies
	maxResponseBodySize  = 16384           // 16KB
	wsBufferMemoryLimit  = 4 * 1024 * 1024 // 4MB
	nbBufferMemoryLimit  = 8 * 1024 * 1024 // 8MB
	rateWindow           = 5 * time.Second // rolling window for msg/s calculation

	// extensionDisconnectThreshold is how long since last /sync before
	// the extension is considered disconnected. Pending queries are auto-expired
	// when the extension exceeds this threshold.
	extensionDisconnectThreshold = 10 * time.Second
)
