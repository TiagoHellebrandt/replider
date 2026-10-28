package usecases

import (
	"strings"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ErrAuthLocked is returned once an e-mail has accumulated too many failed
// logins in the window. 429, not 401: the credentials may well be right by
// now, and the client should back off rather than retry.
var ErrAuthLocked = &domain.AppError{
	Code:    "AUTH_LOCKED",
	Message: "too many failed login attempts — try again in a few minutes",
}

const (
	// loginLockoutThreshold failed attempts per e-mail within loginLockoutWindow
	// lock the e-mail for the remainder of the window.
	loginLockoutThreshold = 10
	loginLockoutWindow    = 15 * time.Minute
)

// loginLockout is an in-memory failed-login counter keyed by e-mail (never
// by IP: the per-IP limiter already exists, and a botnet spreads one target
// across many addresses). Fixed window: the first failure opens a window,
// failures inside it count, the window's end clears the slate. A successful
// login clears the e-mail immediately.
//
// In-memory is the deliberate choice — one API container in production,
// and the worst case of a restart is a fresh 10 attempts. Same pattern as
// middleware.keyedRateLimiter.
type loginLockout struct {
	mu      sync.Mutex
	entries map[string]*lockoutEntry
	now     func() time.Time
}

type lockoutEntry struct {
	failures    int
	windowStart time.Time
}

func newLoginLockout() *loginLockout {
	return &loginLockout{entries: map[string]*lockoutEntry{}, now: time.Now}
}

func lockoutKey(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// locked reports whether the e-mail is over the threshold inside a live
// window. Expired windows are forgotten on the way.
func (l *loginLockout) locked(email string) bool {
	key := lockoutKey(email)
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return false
	}
	if l.now().Sub(e.windowStart) >= loginLockoutWindow {
		delete(l.entries, key)
		return false
	}
	return e.failures >= loginLockoutThreshold
}

// fail records one failed attempt.
func (l *loginLockout) fail(email string) {
	key := lockoutKey(email)
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) >= loginLockoutWindow {
		l.entries[key] = &lockoutEntry{failures: 1, windowStart: now}
		l.sweep(now)
		return
	}
	e.failures++
}

// reset forgets the e-mail after a successful login.
func (l *loginLockout) reset(email string) {
	l.mu.Lock()
	delete(l.entries, lockoutKey(email))
	l.mu.Unlock()
}

// sweep drops expired windows so a stream of distinct e-mails cannot grow
// the map without bound. Called under the lock, on new-window creation, so
// its cost is paid once per window rather than per attempt.
func (l *loginLockout) sweep(now time.Time) {
	if len(l.entries) < 1024 {
		return
	}
	for k, e := range l.entries {
		if now.Sub(e.windowStart) >= loginLockoutWindow {
			delete(l.entries, k)
		}
	}
}
