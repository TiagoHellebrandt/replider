package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M011. The case report is what the founder shows a stranger, so the
// tests care about one thing above all: it must not be able to overstate.
// Retries do not inflate, unknown values are visible rather than dropped,
// and averages over nothing stay unknown instead of collapsing to zero.

func newCaseFixture(t *testing.T) (rbacFixture, *usecases.CaseRecorder, *postgres.SalesEventRepository) {
	t.Helper()
	f := newRBACFixture(t)
	repo := postgres.NewSalesEventRepository(testPool)
	return f, usecases.NewCaseRecorder(repo), repo
}

func TestCase_CountsEachPipelinePointOnce(t *testing.T) {
	f, rec, repo := newCaseFixture(t)
	ctx := context.Background()

	rec.LeadCreated(ctx, f.accountID, f.leadOfA, "speedio")
	// A re-import (or a redelivered job) must not turn one lead into two.
	rec.LeadCreated(ctx, f.accountID, f.leadOfA, "speedio")
	rec.LeadCreated(ctx, f.accountID, f.leadOfB, "google_places")

	rec.ApproachSent(ctx, f.accountID, f.leadOfA, "whatsapp")
	rec.ApproachSent(ctx, f.accountID, f.leadOfB, "email")

	rec.MeetingBooked(ctx, f.accountID, f.leadOfA)
	rec.MeetingBooked(ctx, f.accountID, f.leadOfA) // retry

	m, err := repo.Metrics(ctx, f.accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)

	assert.Equal(t, int64(2), m.LeadsCreated, "a re-imported lead counts once")
	assert.Equal(t, map[string]int64{"speedio": 1, "google_places": 1}, m.LeadsBySource)
	assert.Equal(t, int64(2), m.ApproachesSent)
	assert.Equal(t, map[string]int64{"whatsapp": 1, "email": 1}, m.ApproachesByChannel)
	assert.Equal(t, int64(1), m.MeetingsBooked, "a retried booking counts once")
}

func TestCase_ReplyLatencyMeasuredFromTheApproach(t *testing.T) {
	f, _, repo := newCaseFixture(t)
	ctx := context.Background()

	base := time.Now().Add(-2 * time.Hour)
	clock := base
	rec := usecases.NewCaseRecorder(repo).WithClock(func() time.Time { return clock })

	rec.ApproachSent(ctx, f.accountID, f.leadOfA, "whatsapp")
	clock = base.Add(30 * time.Minute)
	rec.ReplyReceived(ctx, f.accountID, f.leadOfA, "whatsapp")

	// A reply with no approach before it (pure inbound) still counts as a
	// reply — it simply has no latency to report, and averaging a zero in
	// would flatter the number.
	rec.ReplyReceived(ctx, f.accountID, f.unowned, "whatsapp")

	m, err := repo.Metrics(ctx, f.accountID, base.Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)

	assert.Equal(t, int64(2), m.RepliesReceived)
	require.NotNil(t, m.AvgReplySeconds)
	assert.Equal(t, 1800, *m.AvgReplySeconds)
	require.NotNil(t, m.ReplyRate)
	assert.InDelta(t, 2.0, *m.ReplyRate, 0.001, "two replies to one approach is 200%, not capped")
}

// An empty period is the honest zero-state: counts are zero, but the
// derived numbers are UNKNOWN, not zero. "0s average response" would be
// the most flattering possible lie.
func TestCase_EmptyPeriodLeavesDerivedNumbersUnknown(t *testing.T) {
	f, _, repo := newCaseFixture(t)
	ctx := context.Background()

	m, err := repo.Metrics(ctx, f.accountID, time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)

	assert.Zero(t, m.LeadsCreated)
	assert.Nil(t, m.AvgReplySeconds)
	assert.Nil(t, m.ReplyRate)
}

func TestCase_PeriodBoundsAreRespectedAndAccountScoped(t *testing.T) {
	f, _, repo := newCaseFixture(t)
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	rec := usecases.NewCaseRecorder(repo).WithClock(func() time.Time { return old })
	rec.ApproachSent(ctx, f.accountID, f.leadOfA, "whatsapp")

	// Another account's pipeline must never appear in this one's report.
	other := seedIdentityAccount(t)
	usecases.NewCaseRecorder(repo).ApproachSent(ctx, other, "", "whatsapp")

	m, err := repo.Metrics(ctx, f.accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Zero(t, m.ApproachesSent, "an event outside the window is outside the report")

	wide, err := repo.Metrics(ctx, f.accountID, old.Add(-time.Hour), time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(1), wide.ApproachesSent)
}

// A lead created by a path that never recorded a source still happened;
// reporting it as "unknown" keeps the breakdown adding up to the total.
func TestCase_MissingSourceIsVisibleNotDropped(t *testing.T) {
	f, rec, repo := newCaseFixture(t)
	ctx := context.Background()

	rec.LeadCreated(ctx, f.accountID, f.leadOfA, "")

	m, err := repo.Metrics(ctx, f.accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), m.LeadsCreated)
	assert.Equal(t, int64(1), m.LeadsBySource["unknown"])
}

// Instrumentation must never be able to break what it measures.
func TestCase_NilRecorderIsSafe(t *testing.T) {
	var rec *usecases.CaseRecorder
	ctx := context.Background()
	id := ulid.Make().String()

	assert.NotPanics(t, func() {
		rec.LeadCreated(ctx, id, id, "speedio")
		rec.ApproachSent(ctx, id, id, "whatsapp")
		rec.ReplyReceived(ctx, id, id, "whatsapp")
		rec.MeetingBooked(ctx, id, id)
	})
}

// The meeting counter is an explicit human confirmation (the platform only
// checks availability, it never books), so it goes through the ownership
// rules like any other mutation.
func TestCase_MeetingBookedRespectsOwnership(t *testing.T) {
	f, rec, repo := newCaseFixture(t)
	ctx := context.Background()
	f.uc.SetCaseRecorder(rec)

	err := f.uc.MarkMeetingBooked(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.privateB)
	assert.ErrorIs(t, err, domain.ErrForbidden)

	require.NoError(t, f.uc.MarkMeetingBooked(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.leadOfA))

	m, err := repo.Metrics(ctx, f.accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), m.MeetingsBooked)
}

// US-M012. The report is the thing a founder screenshots and sends to a
// stranger, so its arithmetic has to survive being read closely.
func TestCaseReport_ComparesAgainstAnEqualLengthWindow(t *testing.T) {
	f, _, repo := newCaseFixture(t)
	ctx := context.Background()
	uc := usecases.NewCaseMetricsUseCase(repo)

	now := time.Now()
	// Two approaches inside the window, one in the window before it.
	inside := usecases.NewCaseRecorder(repo).WithClock(func() time.Time { return now.Add(-2 * time.Hour) })
	inside.ApproachSent(ctx, f.accountID, f.leadOfA, "whatsapp")
	inside.ApproachSent(ctx, f.accountID, f.leadOfB, "email")
	before := usecases.NewCaseRecorder(repo).WithClock(func() time.Time { return now.Add(-30 * time.Hour) })
	before.ApproachSent(ctx, f.accountID, f.unowned, "whatsapp")

	report, err := uc.Report(ctx, f.accountID, now.Add(-24*time.Hour), now, 0)
	require.NoError(t, err)

	assert.Equal(t, int64(2), report.Current.ApproachesSent)
	// The comparison window is the SAME length immediately before — that is
	// what makes a delta mean anything.
	assert.Equal(t, int64(1), report.Previous.ApproachesSent)
	assert.Equal(t, int64(1), report.ApproachesDelta)

	// The estimate carries its own assumption so a reader can disagree with
	// the arithmetic instead of being asked to trust the number.
	assert.Equal(t, usecases.DefaultMinutesPerApproach, report.MinutesPerApproach)
	assert.InDelta(t, 2*float64(usecases.DefaultMinutesPerApproach)/60, report.HoursSaved, 0.001)

	// ...and it follows the caller's assumption when they supply one.
	custom, err := uc.Report(ctx, f.accountID, now.Add(-24*time.Hour), now, 30)
	require.NoError(t, err)
	assert.Equal(t, 30, custom.MinutesPerApproach)
	assert.InDelta(t, 1.0, custom.HoursSaved, 0.001)
}

// An empty account produces a report, not an error — and one whose derived
// numbers are absent rather than flatteringly zero.
func TestCaseReport_EmptyAccountIsNotAnError(t *testing.T) {
	f, _, repo := newCaseFixture(t)
	ctx := context.Background()

	report, err := usecases.NewCaseMetricsUseCase(repo).
		Report(context.Background(), f.accountID, time.Now().Add(-24*time.Hour), time.Now(), 0)
	require.NoError(t, err)
	assert.Zero(t, report.Current.ApproachesSent)
	assert.Zero(t, report.ApproachesDelta)
	assert.Nil(t, report.Current.AvgReplySeconds)
	assert.Nil(t, report.Current.ReplyRate)
	assert.Zero(t, report.HoursSaved)
	_ = ctx
}

// A backwards period is a caller mistake, not a report of "nothing".
func TestCaseReport_RejectsAnInvertedPeriod(t *testing.T) {
	f, _, repo := newCaseFixture(t)
	now := time.Now()

	_, err := usecases.NewCaseMetricsUseCase(repo).
		Report(context.Background(), f.accountID, now, now.Add(-24*time.Hour), 0)
	require.Error(t, err)
}

// US-M013. The monthly caps are counted off the instrumentation log, so the
// number the customer is charged against is the same one their case report
// shows them — two counters would eventually disagree, and the customer
// would be right either way.
func TestSDRTier_MonthlyLeadCapCountsFromTheEventLog(t *testing.T) {
	f, rec, repo := newCaseFixture(t)
	ctx := context.Background()

	rec.LeadCreated(ctx, f.accountID, f.leadOfA, "speedio")
	rec.LeadCreated(ctx, f.accountID, f.leadOfB, "speedio")

	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())
	used, err := repo.CountSince(ctx, f.accountID, domain.SalesEventLeadCreated, monthStart)
	require.NoError(t, err)
	assert.Equal(t, int64(2), used)

	// Another account's leads never consume this account's allowance.
	other := seedIdentityAccount(t)
	otherUsed, err := repo.CountSince(ctx, other, domain.SalesEventLeadCreated, monthStart)
	require.NoError(t, err)
	assert.Zero(t, otherUsed)

	// Last month's leads do not either — the allowance resets.
	old := usecases.NewCaseRecorder(repo).
		WithClock(func() time.Time { return monthStart.Add(-24 * time.Hour) })
	old.LeadCreated(ctx, f.accountID, f.unowned, "speedio")
	stillUsed, err := repo.CountSince(ctx, f.accountID, domain.SalesEventLeadCreated, monthStart)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stillUsed)
}

// US-M010. The digest's counts come from one SQL pass, and the query is
// where "actionable" is actually decided — so the boundaries matter: a
// reply from last week is not new, a lead contacted an hour ago is not
// stalled, and a won lead is nobody's outstanding work.
func TestSDRDigest_CountsOnlyWhatIsActionableNow(t *testing.T) {
	f, _, _ := newCaseFixture(t)
	ctx := context.Background()
	repo := postgres.NewSDRDigestRepository(testPool)

	// One statement per Exec: pgx prepares statements, and a prepared
	// statement cannot carry multiple commands.
	for _, step := range []struct {
		sql string
		id  string
	}{
		{`UPDATE leads SET status = 'pending_approval' WHERE id = $1`, f.leadOfA},
		{`UPDATE leads SET status = 'responded', updated_at = NOW() - interval '2 hours' WHERE id = $1`, f.leadOfB},
		{`UPDATE leads SET status = 'contacted', updated_at = NOW() - interval '5 days' WHERE id = $1`, f.unowned},
		{`UPDATE leads SET status = 'contacted', updated_at = NOW() WHERE id = $1`, f.privateB},
	} {
		_, err := testPool.Exec(ctx, step.sql, step.id)
		require.NoError(t, err)
	}

	now := time.Now()
	d, err := repo.Build(ctx, f.accountID, now.Add(-24*time.Hour), now.Add(-domain.StalledLeadAfter))
	require.NoError(t, err)

	assert.Equal(t, int64(1), d.PendingApprovals)
	assert.Equal(t, int64(1), d.NewReplies, "a reply from two hours ago is new")
	assert.Equal(t, int64(1), d.StalledLeads, "only the one contacted 5 days ago; today's is still warm")
	// The fixture's unassigned lead — and it is 'contacted', so still live.
	assert.Equal(t, int64(1), d.UnownedLeads)

	// A reply older than the window is not "new" any more.
	_, err = testPool.Exec(ctx,
		`UPDATE leads SET updated_at = NOW() - interval '8 days' WHERE id = $1`, f.leadOfB)
	require.NoError(t, err)
	stale, err := repo.Build(ctx, f.accountID, now.Add(-24*time.Hour), now.Add(-domain.StalledLeadAfter))
	require.NoError(t, err)
	assert.Zero(t, stale.NewReplies)
}

// An account already sent today is not due again — the guarantee that
// survives a restart, enforced in SQL rather than worker memory. Both halves
// of the window resolve in the ACCOUNT's timezone; sendHour 0 here means
// "any hour", so the test measures the stamp rule alone.
func TestSDRDigest_DueQueryRespectsTheDailyStamp(t *testing.T) {
	f, _, _ := newCaseFixture(t)
	ctx := context.Background()
	repo := postgres.NewSDRDigestRepository(testPool)

	ids := func(cs []domain.DigestCandidate) []string {
		out := make([]string, 0, len(cs))
		for _, c := range cs {
			out = append(out, c.AccountID)
		}
		return out
	}

	due, err := repo.ListDue(ctx, 0, 0)
	require.NoError(t, err)
	assert.Contains(t, ids(due), f.accountID, "an account never sent a digest is due")

	require.NoError(t, repo.MarkSent(ctx, f.accountID, time.Now()))
	after, err := repo.ListDue(ctx, 0, 0)
	require.NoError(t, err)
	assert.NotContains(t, ids(after), f.accountID)

	// Opting out removes the account regardless of the stamp.
	_, err = testPool.Exec(ctx,
		`UPDATE accounts SET sdr_digest_opt_out = TRUE, sdr_digest_last_sent_at = NULL WHERE id = $1`,
		f.accountID)
	require.NoError(t, err)
	optedOut, err := repo.ListDue(ctx, 0, 0)
	require.NoError(t, err)
	assert.NotContains(t, ids(optedOut), f.accountID)
}

// The 2026-09-09 regression, at the layer that actually decides it: the
// digest fired on the server's hour, so every Brazilian account got its
// "start of your day" notification at 05:28. An account whose local morning
// has not arrived must not be returned, and a timezone Postgres does not
// recognise must degrade to UTC rather than aborting the query for EVERYONE.
func TestSDRDigest_DueQueryWaitsForTheAccountsOwnMorning(t *testing.T) {
	f, _, _ := newCaseFixture(t)
	ctx := context.Background()
	repo := postgres.NewSDRDigestRepository(testPool)

	has := func(cs []domain.DigestCandidate) bool {
		for _, c := range cs {
			if c.AccountID == f.accountID {
				return true
			}
		}
		return false
	}

	// Pin the account far enough west that the server's hour and its own
	// cannot agree, then ask for an hour neither has reached.
	_, err := testPool.Exec(ctx,
		`UPDATE accounts SET timezone = 'Pacific/Kiritimati', sdr_digest_last_sent_at = NULL WHERE id = $1`,
		f.accountID)
	require.NoError(t, err)

	var localHour int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT EXTRACT(HOUR FROM now() AT TIME ZONE 'Pacific/Kiritimati')::int`).Scan(&localHour))

	if localHour < 23 {
		notYet, err := repo.ListDue(ctx, localHour+1, 0)
		require.NoError(t, err)
		assert.False(t, has(notYet), "an account whose local morning has not arrived is not due")
	}

	arrived, err := repo.ListDue(ctx, localHour, 0)
	require.NoError(t, err)
	assert.True(t, has(arrived), "once the local hour is reached the account is due")
	for _, c := range arrived {
		if c.AccountID == f.accountID {
			assert.Equal(t, "Pacific/Kiritimati", c.Timezone,
				"the timezone must travel with the row")
		}
	}

	// A typo in ONE account's timezone must not abort the query for every
	// other account — AT TIME ZONE raises on an unknown name.
	_, err = testPool.Exec(ctx,
		`UPDATE accounts SET timezone = 'Mars/Olympus_Mons' WHERE id = $1`, f.accountID)
	require.NoError(t, err)
	survived, err := repo.ListDue(ctx, 0, 0)
	require.NoError(t, err, "an unrecognised timezone must degrade to UTC, not fail the run")
	for _, c := range survived {
		if c.AccountID == f.accountID {
			assert.Equal(t, "UTC", c.Timezone)
		}
	}
}
