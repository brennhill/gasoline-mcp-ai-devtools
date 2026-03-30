// bridge_startup.go -- Bridge startup lifecycle knobs.
// Why: Keeps startup tuning constants isolated from orchestration and state logic.

package bridge

import "time"

// daemonStartupGracePeriod is a short wait window for first tool calls so
// clients don't fail on daemon boot races.
var daemonStartupGracePeriod = 2 * time.Second

// daemonStartupReadyTimeout bounds how long a bridge waits for a spawned daemon
// to report healthy before treating the attempt as failed.
var daemonStartupReadyTimeout = 2 * time.Second

// daemonPeerWaitTimeout is the follower wait budget while another bridge is
// expected to finish daemon startup under contention.
var daemonPeerWaitTimeout = 2 * time.Second

// daemonPeerPollInterval controls peer readiness polling cadence.
var daemonPeerPollInterval = 100 * time.Millisecond

// daemonPeerFallbackWaitTimeout adds a final short wait when another bridge
// still owns the startup lock but has not surfaced readiness yet.
var daemonPeerFallbackWaitTimeout = 250 * time.Millisecond

// daemonStartupLockStaleAfter defines when a startup lock is considered stale
// and can be reclaimed by another bridge.
var daemonStartupLockStaleAfter = 2 * time.Second
