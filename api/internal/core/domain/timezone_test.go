package domain

import (
	"testing"
	"time"
)

// The bug this exists to prevent: a daily cap that rolls over at some fixed
// foreign midnight resets in the middle of the operator's working afternoon.
func TestStartOfDayIn_IsTheAccountsOwnMidnight(t *testing.T) {
	// 02:00 UTC on 12 May. In São Paulo (UTC-3) it is still 23:00 on the 11th.
	now := time.Date(2026, 5, 12, 2, 0, 0, 0, time.UTC)

	sp := StartOfDayIn(now, "America/Sao_Paulo")
	if y, m, d := sp.Date(); y != 2026 || m != time.May || d != 11 {
		t.Fatalf("expected 11 May in São Paulo, got %s", sp)
	}

	// Same instant, Tokyo: already the 12th, and well into the morning.
	tokyo := StartOfDayIn(now, "Asia/Tokyo")
	if y, m, d := tokyo.Date(); y != 2026 || m != time.May || d != 12 {
		t.Fatalf("expected 12 May in Tokyo, got %s", tokyo)
	}
}

// A typo in one account's settings must not stop a worker that serves every
// account.
func TestLoadLocation_UnknownNameDegradesToUTC(t *testing.T) {
	if loc := LoadLocation("Mars/Olympus_Mons"); loc != time.UTC {
		t.Fatalf("expected UTC for an unknown zone, got %v", loc)
	}
	if loc := LoadLocation(""); loc == nil || loc.String() != FallbackTimezone {
		t.Fatalf("expected the fallback for an empty zone, got %v", loc)
	}
}

func TestLocalDate_FollowsTheAccount(t *testing.T) {
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)

	// New Year in Lisbon; still New Year's Eve in São Paulo.
	if _, _, d := LocalDate(now, "America/Sao_Paulo"); d != 31 {
		t.Fatalf("expected 31 December in São Paulo, got day %d", d)
	}
	if y, _, _ := LocalDate(now, "Europe/Lisbon"); y != 2026 {
		t.Fatalf("expected 2026 in Lisbon, got %d", y)
	}
}
