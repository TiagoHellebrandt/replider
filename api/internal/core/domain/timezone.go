package domain

import (
	"strings"
	"time"
)

// Timezone resolution.
//
// Every customer-facing schedule — a daily send cap, a cadence's business
// hours, "is today somebody's birthday" — belongs in the CUSTOMER's timezone,
// not the platform's. A cap that rolls over at Brazilian midnight resets in
// the middle of the working afternoon for an account in Lisbon, and a cadence
// that respects 08:00–18:00 in Cuiabá emails a customer in Tokyo at four in
// the morning.
//
// The source of truth is accounts.timezone (IANA name, migration 000028).
// This is the one place that turns it into a *time.Location, so a new caller
// cannot quietly reintroduce a hardcoded one.

// FallbackTimezone is used when an account has no timezone recorded.
//
// UTC, deliberately. The column is NOT NULL with a default, so an empty value
// means a fixture or a legacy row — a genuine "we do not know". Guessing a
// country there would be the same bug as hardcoding one, just harder to find.
const FallbackTimezone = "UTC"

// LoadLocation resolves an IANA timezone name, never returning nil.
//
// An unknown name degrades to UTC rather than failing the caller: a typo in
// one account's settings must not stop a worker that serves every account.
func LoadLocation(tz string) *time.Location {
	name := strings.TrimSpace(tz)
	if name == "" {
		name = FallbackTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil || loc == nil {
		return time.UTC
	}
	return loc
}

// StartOfDayIn is local midnight for a timezone — the boundary a daily cap or
// a per-day counter resets on.
func StartOfDayIn(now time.Time, tz string) time.Time {
	loc := LoadLocation(tz)
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

// LocalDate is the calendar date an account is currently living in.
//
// What "today" means for a birthday: the greeting has to arrive on the day
// the customer's own calendar says, not the day the server's does.
func LocalDate(now time.Time, tz string) (year int, month time.Month, day int) {
	local := now.In(LoadLocation(tz))
	return local.Year(), local.Month(), local.Day()
}
