// Purpose: Centralizes /sync long-poll timeout policy.
// Why: Keeps production behavior stable while allowing much faster unit tests.

package capture

import (
	"os"
	"strings"
	"time"
)

const (
	syncLongPollDefaultTimeout = 5 * time.Second
	syncLongPollTestTimeout    = 100 * time.Millisecond
)

func syncLongPollTimeout() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return syncLongPollTestTimeout
	}
	return syncLongPollDefaultTimeout
}
