// nonce.go — Per-process random nonce used to gate the upgrade-install endpoint.
// Why: The extension fetches the current nonce over its authenticated channel and
// presents it back when requesting an install; an attacker who cannot read the
// daemon's memory cannot forge a valid request even if they hit the endpoint directly.
// The nonce is also bound to the Origin that first fetched it so a second
// extension on the same browser cannot replay it.

package upgrade

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"sync"
)

// nonceBytes is the number of random bytes behind each nonce. 32 bytes hex-encoded
// yields a 64-character token.
const nonceBytes = 32

// Nonce holds a single random token generated once at construction time, plus
// the Origin that first fetched it.
type Nonce struct {
	value string

	mu            sync.Mutex
	pinnedOrigin  string
}

// NewNonce allocates a fresh Nonce. Panics if the system RNG fails (if it does,
// we have bigger problems than an upgrade check).
func NewNonce() *Nonce {
	buf := make([]byte, nonceBytes)
	if _, err := rand.Read(buf); err != nil {
		panic("upgrade: crypto/rand.Read failed: " + err.Error())
	}
	return &Nonce{value: hex.EncodeToString(buf)}
}

// Current returns the nonce as a 64-character lowercase hex string.
func (n *Nonce) Current() string {
	return n.value
}

// Pin records the Origin that fetched the nonce. First non-empty caller wins;
// subsequent calls (even with a different origin) are ignored so legitimate
// repeat polls from the same extension don't hand the binding to an attacker
// who raced in afterwards. Empty origins are always ignored — the caller must
// validate and reject an empty Origin header before calling Pin.
func (n *Nonce) Pin(origin string) {
	if origin == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.pinnedOrigin == "" {
		n.pinnedOrigin = origin
	}
}

// PinnedOrigin returns the origin recorded by Pin, or empty string if the
// nonce has never been fetched.
func (n *Nonce) PinnedOrigin() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.pinnedOrigin
}

// Verify reports whether token matches the current nonce AND origin matches the
// pinned Origin. Uses constant-time comparison on matching-length tokens;
// rejects wrong-length inputs first so the timing side-channel only exposes
// the (already public) length. An unpinned nonce (no prior GET) is rejected —
// callers must fetch the nonce first so its Origin gets recorded.
func (n *Nonce) Verify(token, origin string) bool {
	if len(token) != len(n.value) {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(n.value)) != 1 {
		return false
	}
	n.mu.Lock()
	pinned := n.pinnedOrigin
	n.mu.Unlock()
	if pinned == "" {
		return false
	}
	return origin == pinned
}
