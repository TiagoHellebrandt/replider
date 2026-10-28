package usecases

import (
	"sync"
	"time"

	"github.com/pquerna/otp/totp"
)

// totpStep is the TOTP time step (RFC 6238 default, and what
// totp.Validate assumes).
const totpStep = 30 * time.Second

// totpReplayGuard remembers, per user, the time step of the last code that
// was accepted, and refuses any code for that step or an earlier one. RFC
// 6238 §5.2: a verifier MUST NOT accept a second attempt of an OTP after a
// successful validation. Without it a code shoulder-surfed or intercepted
// from a login stays valid for the ~90s skew window and can be replayed.
//
// In memory, per use-case instance: one API process in production, and the
// worst case of a restart is one replay inside the current step. The map
// holds one int64 per user who logged in with 2FA, so it needs no janitor.
type totpReplayGuard struct {
	mu       sync.Mutex
	lastStep map[string]int64
	now      func() time.Time
}

func newTOTPReplayGuard() *totpReplayGuard {
	return &totpReplayGuard{lastStep: map[string]int64{}, now: time.Now}
}

// accept validates code against secret with the usual ±1 step skew and, on
// a match, records the step it matched. It returns false for a wrong code
// AND for a correct code whose step is not newer than the last accepted
// one — the caller cannot tell the two apart, which is intended.
func (g *totpReplayGuard) accept(userID, code, secret string) bool {
	now := g.now()
	matched := int64(-1)
	for _, offset := range []time.Duration{0, -totpStep, totpStep} {
		at := now.Add(offset)
		expected, err := totp.GenerateCode(secret, at)
		if err != nil {
			return false
		}
		if expected == code {
			matched = at.Unix() / int64(totpStep/time.Second)
			break
		}
	}
	if matched < 0 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if last, ok := g.lastStep[userID]; ok && matched <= last {
		return false
	}
	g.lastStep[userID] = matched
	return true
}
