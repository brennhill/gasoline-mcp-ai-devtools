// runner.go — Detached installer spawn and argv builder for the upgrade endpoint.
// Why: The upgrade endpoint is fire-and-forget — the daemon will be killed by
// the script it spawned, so the spawn must detach stdio and own its session.

package upgrade

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrUnsupportedPlatform is returned by Spawn on platforms where self-update is
// not supported (currently Windows — install.sh is bash).
var ErrUnsupportedPlatform = errors.New("upgrade: self-update is not supported on this platform")

// buildInstallCmd constructs the argv for running the pinned install script via
// bash -c 'curl -sSL <url> | bash'. The URL is validated — it must be an https
// URL with no shell-special characters so single-quote embedding is safe.
func buildInstallCmd(pinnedURL string) (name string, args []string, err error) {
	if pinnedURL == "" {
		return "", nil, errors.New("upgrade: pinned URL is required")
	}
	u, parseErr := url.Parse(pinnedURL)
	if parseErr != nil {
		return "", nil, fmt.Errorf("upgrade: invalid URL: %w", parseErr)
	}
	if u.Scheme != "https" {
		return "", nil, fmt.Errorf("upgrade: URL must be https, got %q", u.Scheme)
	}
	if strings.ContainsAny(pinnedURL, "'`$\n\r\\\"") {
		return "", nil, errors.New("upgrade: URL contains shell-special characters")
	}
	script := fmt.Sprintf("curl -sSL '%s' | bash", pinnedURL)
	return "bash", []string{"-c", script}, nil
}
