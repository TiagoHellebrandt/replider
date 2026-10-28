package usecases

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// oauthStateNonces makes a signed OAuth state single-use. The HMAC proves we
// minted the state and the expiry bounds its life, but neither stops the
// same state — copied from a browser history, a proxy log, or a redirect
// intercepted on the way back — from being presented twice. Both Google
// callbacks (mailbox, calendar) are unauthenticated browser redirects, so
// there is no session to bind the state to; a server-side nonce that is
// consumed on first use is the simplest thing that closes reuse.
//
// In memory: one API process in production, a 10-minute window, and a
// restart merely asks the person to click "connect" again.
type oauthStateNonces struct {
	mu     sync.Mutex
	issued map[string]time.Time // nonce -> expiry
	now    func() time.Time
}

func newOAuthStateNonces() *oauthStateNonces {
	return &oauthStateNonces{issued: map[string]time.Time{}, now: time.Now}
}

// issue mints a fresh nonce that verifies until exp.
func (n *oauthStateNonces) issue(exp time.Time) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is a broken host; the state's HMAC still
		// holds, so degrade to a time-derived value rather than panic.
		copy(b[:], []byte(exp.String()))
	}
	nonce := hex.EncodeToString(b[:])
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sweep()
	n.issued[nonce] = exp
	return nonce
}

// consume spends the nonce. False when it was never issued, already spent,
// or expired.
func (n *oauthStateNonces) consume(nonce string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	exp, ok := n.issued[nonce]
	if !ok {
		return false
	}
	delete(n.issued, nonce)
	return n.now().Before(exp)
}

// sweep forgets expired nonces. Called under the lock on every issue, so a
// burst of abandoned consent screens cannot grow the map without bound.
func (n *oauthStateNonces) sweep() {
	now := n.now()
	for k, exp := range n.issued {
		if !now.Before(exp) {
			delete(n.issued, k)
		}
	}
}
