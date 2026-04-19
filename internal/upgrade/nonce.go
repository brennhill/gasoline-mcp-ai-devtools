// nonce.go — Per-process random nonce used to gate the upgrade-install endpoint.
// Why: The extension fetches the current nonce over its authenticated channel and
// presents it back when requesting an install; an attacker who cannot read the
// daemon's memory cannot forge a valid request even if they hit the endpoint directly.

package upgrade

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
)

// nonceBytes is the number of random bytes behind each nonce. 32 bytes hex-encoded
// yields a 64-character token.
const nonceBytes = 32

// Nonce holds a single random token generated once at construction time.
type Nonce struct {
	value string
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

// Verify reports whether token matches the current nonce. Uses constant-time
// comparison on matching-length inputs; rejects wrong-length inputs first so the
// timing side-channel only exposes the (already public) length.
func (n *Nonce) Verify(token string) bool {
	if len(token) != len(n.value) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(n.value)) == 1
}
