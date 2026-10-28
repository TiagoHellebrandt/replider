package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M007 + Item 1 (warmup fail-closed). These rules exist because the damage
// is permanent and lands on the CUSTOMER's domain: a fresh domain that blasts
// cold mail is burned at Gmail/Outlook, and later good behaviour never fully
// undoes it.
//
// The governing principle: EVERY guard fails closed. warmup_started_at NULL
// means "don't know how old this identity is", and not knowing never releases
// a cold send — that NULL sat unwritten for months while the old guard waved
// everything through, which is the contact_ref_id disease in a second organ.

func at(day int) time.Time {
	return time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC)
}

func verifiedMailbox() *EmailMailbox {
	v := at(1)
	return &EmailMailbox{
		Address:       "comercial@clinica.com.br",
		Domain:        "clinica.com.br",
		DailyCap:      DefaultDailyCap,
		DNSVerifiedAt: &v,
	}
}

// warmedSince stamps the ramp start, the way every proof route now does.
func warmedSince(m *EmailMailbox, start time.Time) *EmailMailbox {
	m.WarmupStartedAt = &start
	return m
}

// Gate 1: unauthenticated domain. Cold mail from it is rejected or
// spam-foldered by the receiver anyway — sending is pure downside.
func TestMailbox_ColdSendBlockedWithoutDNS(t *testing.T) {
	m := &EmailMailbox{DailyCap: 25}

	cold := m.CanSend(at(10), true)
	assert.False(t, cold.Allowed)
	assert.Equal(t, BlockDNSUnverified, cold.Reason)
	assert.Nil(t, cold.RetryAfter, "no amount of waiting fixes this — a human must publish DNS")

	// A REPLY still goes out: answering someone who wrote to you is the
	// opposite of a spam signal, and blocking it would make the product
	// useless while DNS propagates.
	reply := m.CanSend(at(10), false)
	assert.True(t, reply.Allowed)
}

// Gate 2 — THE test that failed on the old code: a verified mailbox whose
// warmup was never stamped used to send cold freely. NULL never releases.
func TestMailbox_NullWarmupFailsClosed(t *testing.T) {
	m := verifiedMailbox() // DNS verified, warmup_started_at NULL

	cold := m.CanSend(at(10), true)
	assert.False(t, cold.Allowed,
		"an identity of unknown age must not open cold threads — this passed before the fix")
	assert.Equal(t, BlockWarmupUnknown, cold.Reason)
	assert.Nil(t, cold.RetryAfter, "waiting fixes nothing — a verification must stamp the ramp")

	// Replies still flow: the identity gate is about mail WE initiate.
	assert.True(t, m.CanSend(at(10), false).Allowed)
}

// Inside the ramp, cold mail flows UNDER a small growing ceiling from day
// one. There is deliberately no total block: a handful of day-one sends
// teaches the provider gradually; a hard block followed by a burst does not.
func TestMailbox_InRampColdSendsUnderTheGrowingCeiling(t *testing.T) {
	m := warmedSince(verifiedMailbox(), at(1))

	day1 := m.CanSend(at(1), true)
	assert.True(t, day1.Allowed)
	assert.Equal(t, WarmupInitialCap, day1.Cap, "day one carries only the initial budget")

	// Above the day's limit → refused WITH the numbers, so the person can
	// plan instead of staring at a wall.
	today := at(1)
	m.SentDate = &today
	m.SentToday = WarmupInitialCap
	over := m.CanSend(at(1), true)
	assert.False(t, over.Allowed)
	assert.Equal(t, BlockDailyCap, over.Reason)
	assert.Equal(t, WarmupInitialCap, over.Used)
	assert.Equal(t, WarmupInitialCap, over.Cap)
	require.NotNil(t, over.RetryAfter, "temporal block: the queue defers, never drops")
}

// The ramp: a warming mailbox starts tiny and grows to its configured cap.
func TestMailbox_WarmupRampsTheDailyCap(t *testing.T) {
	m := warmedSince(verifiedMailbox(), at(1))
	m.DailyCap = 25

	day1 := m.EffectiveDailyCap(at(1))
	day10 := m.EffectiveDailyCap(at(10))
	day22 := m.EffectiveDailyCap(at(22)) // past WarmupDuration

	assert.Equal(t, WarmupInitialCap, day1)
	assert.Greater(t, day10, day1, "the ceiling grows during warmup")
	assert.Less(t, day10, 25)
	assert.Equal(t, 25, day22, "after the ramp the mailbox carries its full cap")
	assert.True(t, m.CanSend(at(22), true).Allowed, "past the ramp, cold flows at full speed")

	// Declaring warmup complete early jumps straight to the full cap: an
	// operator who knows the mailbox is established should not be throttled.
	done := at(5)
	m.WarmupCompletedAt = &done
	assert.Equal(t, 25, m.EffectiveDailyCap(at(6)))
	assert.True(t, m.CanSend(at(6), true).Allowed)
}

// Gate 3: the daily cap defers rather than fails — the approach still goes
// out, just tomorrow.
func TestMailbox_DailyCapDefersToTomorrow(t *testing.T) {
	m := warmedSince(verifiedMailbox(), at(1).Add(-60*24*time.Hour))
	m.DailyCap = 3
	today := at(10)
	m.SentDate = &today
	m.SentToday = 3

	d := m.CanSend(today, true)
	assert.False(t, d.Allowed)
	assert.Equal(t, BlockDailyCap, d.Reason)
	assert.Equal(t, 3, d.Used)
	assert.Equal(t, 3, d.Cap)
	require.NotNil(t, d.RetryAfter)
	assert.Equal(t, 11, d.RetryAfter.Day(), "resets at the next UTC midnight")

	// Under the cap, the caller learns how much budget is left so a batch
	// can size itself instead of failing halfway.
	m.SentToday = 1
	ok := m.CanSend(today, true)
	assert.True(t, ok.Allowed)
	assert.Equal(t, 2, ok.Remaining)
}

// A counter from a previous day is meaningless: yesterday's 25 must not
// block today's first send.
func TestMailbox_CounterFromAnotherDayIsIgnored(t *testing.T) {
	m := warmedSince(verifiedMailbox(), at(1).Add(-60*24*time.Hour))
	m.DailyCap = 5
	yesterday := at(9)
	m.SentDate = &yesterday
	m.SentToday = 5

	d := m.CanSend(at(10), true)
	assert.True(t, d.Allowed)
	assert.Equal(t, 5, d.Remaining)
}

// An unset cap must fall back to the standard ceiling, never to zero — a
// zero cap would silently stop all sending with no error anywhere.
func TestMailbox_ZeroCapFallsBackToDefault(t *testing.T) {
	m := warmedSince(verifiedMailbox(), at(1).Add(-60*24*time.Hour))
	m.DailyCap = 0
	assert.Equal(t, DefaultDailyCap, m.EffectiveDailyCap(at(10)))
	assert.True(t, m.CanSend(at(10), true).Allowed)
}

// The ramp length and the full-speed ceiling are env policy
// (EMAIL_WARMUP_RAMP_DAYS / EMAIL_WARMUP_DAILY_CAP). Garbage keeps the
// defaults — misconfiguration must never disable the guard.
func TestMailbox_WarmupConfigIsAdjustableButNeverDisablable(t *testing.T) {
	origDuration, origCap := WarmupDuration, DefaultDailyCap
	t.Cleanup(func() { WarmupDuration, DefaultDailyCap = origDuration, origCap })

	SetEmailWarmupConfig(7, 50)
	assert.Equal(t, 7, WarmupRampDays())
	m := warmedSince(verifiedMailbox(), at(1))
	m.DailyCap = 0
	assert.Equal(t, 50, m.EffectiveDailyCap(at(30)), "past the shorter ramp, the env ceiling applies")

	SetEmailWarmupConfig(0, -3)
	assert.Equal(t, 7, WarmupRampDays(), "zero/negative keep the previous policy")
	assert.Equal(t, 50, DefaultDailyCap)
}

func TestNextUTCMidnight(t *testing.T) {
	got := NextUTCMidnight(time.Date(2026, 7, 10, 23, 59, 59, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), got)

	// A local-time input still resolves against the UTC day, because the
	// stored counter is keyed by UTC date.
	loc := time.FixedZone("BRT", -3*3600)
	got = NextUTCMidnight(time.Date(2026, 7, 10, 22, 0, 0, 0, loc)) // 01:00 UTC on the 11th
	assert.Equal(t, time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), got)
}

// --- Two transports, two ways to prove an identity ------------------------

// When the mail goes out through the customer's OWN provider, that provider
// publishes SPF/DKIM for the domain and signs the message. The proof event
// stamps the warmup start (MarkSMTPProven does it atomically), and cold mail
// then flows on the flat low-tier ceiling — no ramp, because the address is
// not new to anyone at Gmail.
func TestMailbox_ProvenByTheCustomersOwnProviderMaySendCold(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	proven := now.Add(-time.Hour)

	m := &EmailMailbox{
		Address:         "contato@clinica.com.br",
		SMTPVerifiedAt:  &proven,
		WarmupStartedAt: &proven,
	}

	assert.False(t, m.DomainAuthenticated(), "none of OUR records are published")
	assert.True(t, m.SMTPProven())
	assert.True(t, m.SendingProven())

	decision := m.CanSend(now, true)
	assert.True(t, decision.Allowed)
	assert.Equal(t, SMTPProvenDailyCap, decision.Remaining)
}

// The proof without the stamp no longer releases: NULL fails closed on this
// tier too. (In production the stamp and the proof are one UPDATE, so this
// state only exists for rows that predate the fix and missed the backfill.)
func TestMailbox_SMTPProvenWithoutWarmupStampStillRefusesCold(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	proven := now.Add(-time.Hour)
	m := &EmailMailbox{Address: "contato@clinica.com.br", SMTPVerifiedAt: &proven}

	cold := m.CanSend(now, true)
	assert.False(t, cold.Allowed)
	assert.Equal(t, BlockWarmupUnknown, cold.Reason)
}

// Neither proof means nobody has established anything about this address beyond
// somebody typing it into a form.
func TestMailbox_UnprovenRefusesColdAndStillAnswersReplies(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	m := &EmailMailbox{Address: "contato@clinica.com.br"}

	cold := m.CanSend(now, true)
	assert.False(t, cold.Allowed)
	assert.Equal(t, BlockDNSUnverified, cold.Reason)

	// A reply is the opposite of a spam signal: answering someone who wrote to
	// you must never be blocked by a sending-identity gate.
	assert.True(t, m.CanSend(now, false).Allowed)
}

// The low tier is deliberately NOT the configurable daily_cap. That column is
// an operator decision about a domain authenticated for a transport we control;
// here we are borrowing a mailbox under a limit its provider sets and does not
// disclose, and the address at risk is the one they answer customers from.
func TestMailbox_TheSMTPTierIgnoresAConfiguredCap(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	proven := now.Add(-time.Hour)

	m := &EmailMailbox{
		Address:        "contato@clinica.com.br",
		SMTPVerifiedAt: &proven,
		DailyCap:       5000,
	}

	assert.Equal(t, SMTPProvenDailyCap, m.EffectiveDailyCap(now))
}

// Warmup teaches a receiving provider about a sending identity WE introduce.
// An address the customer has answered mail from for years is not new to
// anyone — ramping it would throttle a warm mailbox, and the customer would
// reasonably conclude we are broken.
func TestMailbox_WarmupDoesNotThrottleTheCustomersEstablishedMailbox(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	proven := now.Add(-time.Hour)
	started := now.Add(-24 * time.Hour) // day 1 of a ramp

	m := &EmailMailbox{
		Address:         "contato@clinica.com.br",
		SMTPVerifiedAt:  &proven,
		WarmupStartedAt: &started,
	}

	assert.True(t, m.CanSend(now, true).Allowed, "no ramp throttle on their own mailbox")
	assert.Equal(t, SMTPProvenDailyCap, m.EffectiveDailyCap(now),
		"no ramp — the ramp is about an identity we introduce")

	// Our transport, day 1: the ramp DOES apply — cold flows, but tiny.
	ours := &EmailMailbox{
		Address:         "contato@clinica.com.br",
		DNSVerifiedAt:   &proven,
		WarmupStartedAt: &started,
		DailyCap:        200,
	}
	day1 := ours.CanSend(now, true)
	assert.True(t, day1.Allowed)
	assert.Less(t, day1.Cap, 200, "day-one budget is the ramp's, not the configured cap")
}

// Our records outrank the SMTP proof: once the domain is authenticated for the
// transport we deliver through, the configurable ceiling and the warmup ramp
// are the ones that apply.
func TestMailbox_OurRecordsGovernOncePublished(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	stamp := now.Add(-30 * 24 * time.Hour)

	m := &EmailMailbox{
		Address:         "contato@clinica.com.br",
		SMTPVerifiedAt:  &stamp,
		DNSVerifiedAt:   &stamp,
		WarmupStartedAt: &stamp,
		DailyCap:        200,
	}

	assert.Equal(t, 200, m.EffectiveDailyCap(now))
	assert.True(t, m.CanSend(now, true).Allowed)
}

// The daily counter still bites on the low tier, and it defers rather than
// fails: the approach goes out tomorrow instead of being lost.
func TestMailbox_TheLowTierStillHasADailyCeiling(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	proven := now.Add(-time.Hour)
	today := now

	m := &EmailMailbox{
		Address:         "contato@clinica.com.br",
		SMTPVerifiedAt:  &proven,
		WarmupStartedAt: &proven,
		SentToday:       SMTPProvenDailyCap,
		SentDate:        &today,
	}

	decision := m.CanSend(now, true)
	assert.False(t, decision.Allowed)
	assert.Equal(t, BlockDailyCap, decision.Reason)
	require.NotNil(t, decision.RetryAfter, "the send is deferred, not dropped")
}

// The screen state is derived from the SAME rules the gate applies, so the UI
// and the refusal can never disagree about today's budget.
func TestMailbox_WarmupStatusMirrorsTheGate(t *testing.T) {
	m := warmedSince(verifiedMailbox(), at(1))
	today := at(2)
	m.SentDate = &today
	m.SentToday = 2

	s := NewEmailWarmupStatus(m, at(2))
	require.NotNil(t, s)
	assert.True(t, s.Started)
	assert.True(t, s.Warming)
	assert.Equal(t, 2, s.Day)
	assert.Equal(t, WarmupRampDays(), s.RampDays)
	assert.Equal(t, m.EffectiveDailyCap(at(2)), s.DailyCap)
	assert.Equal(t, 2, s.SentToday)
	assert.Equal(t, s.DailyCap-2, s.Remaining)

	// Never-started renders as exactly that — the screen must not invent
	// a ramp the gate does not see.
	bare := verifiedMailbox()
	s2 := NewEmailWarmupStatus(bare, at(2))
	assert.False(t, s2.Started)
	assert.False(t, s2.Warming)
	assert.Zero(t, s2.Day)
}
