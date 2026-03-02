// Purpose: Bridge startup lifecycle knobs.
// Why: Keeps startup tuning constants isolated from orchestration and state logic.

package main

import "time"

// daemonStartupGracePeriod is a short wait window for first tool calls so
// clients don't fail on daemon boot races.
var daemonStartupGracePeriod = 250 * time.Millisecond
