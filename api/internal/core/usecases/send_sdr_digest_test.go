package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M010. Two properties matter more than the copy: it must not
// double-send across a restart, and it must not interrupt anyone on a day
// with nothing to act on.

type fakeDigestRepo struct {
	due     []string
	digests map[string]*domain.SDRDigest
	sent    map[string]time.Time
	// tz per account; empty means UTC, like the SQL's COALESCE.
	tz       map[string]string
	now      func() time.Time
	buildErr error
	// builtWindows records the repliesSince the use case asked for, so a
	// test can assert the window was measured in the account's own day.
	builtWindows map[string]time.Time
}

func (f *fakeDigestRepo) zone(id string) string { return f.tz[id] }

// ListDue mirrors the SQL: the local clock must have reached sendHour, and
// the stamp must predate the account's own local midnight.
func (f *fakeDigestRepo) ListDue(_ context.Context, sendHour, _ int) ([]domain.DigestCandidate, error) {
	now := time.Now()
	if f.now != nil {
		now = f.now()
	}
	var out []domain.DigestCandidate
	for _, id := range f.due {
		tz := f.zone(id)
		if now.In(domain.LoadLocation(tz)).Hour() < sendHour {
			continue
		}
		if at, ok := f.sent[id]; ok && !at.Before(domain.StartOfDayIn(now, tz)) {
			continue
		}
		out = append(out, domain.DigestCandidate{AccountID: id, Timezone: tz})
	}
	return out, nil
}

func (f *fakeDigestRepo) Build(_ context.Context, accountID string, repliesSince, _ time.Time) (*domain.SDRDigest, error) {
	if f.builtWindows == nil {
		f.builtWindows = map[string]time.Time{}
	}
	f.builtWindows[accountID] = repliesSince
	if f.buildErr != nil {
		return nil, f.buildErr
	}
	if d, ok := f.digests[accountID]; ok {
		return d, nil
	}
	return &domain.SDRDigest{AccountID: accountID}, nil
}

func (f *fakeDigestRepo) MarkSent(_ context.Context, accountID string, at time.Time) error {
	if f.sent == nil {
		f.sent = map[string]time.Time{}
	}
	f.sent[accountID] = at
	return nil
}

type fakeDigestPusher struct {
	subscribed map[string]bool
	pushes     []domain.PushPayload
}

func (f *fakeDigestPusher) NotifyAccount(_ context.Context, _, _ string, p domain.PushPayload) {
	f.pushes = append(f.pushes, p)
}

func (f *fakeDigestPusher) HasSubscribers(_ context.Context, accountID string) bool {
	return f.subscribed[accountID]
}

type fakeDigestEmailer struct {
	sentTo []string
	err    error
}

func (f *fakeDigestEmailer) SendSDRDigestEmail(to, _ string, _ []domain.DigestLine, _, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.sentTo = append(f.sentTo, to)
	return nil
}

// sdrAdminResolver answers with a per-account admin so the fallback path
// has somewhere to write to.
type sdrAdminResolver struct{}

func (sdrAdminResolver) GetAdminByAccountID(_ context.Context, accountID string) (*domain.User, error) {
	return &domain.User{ID: "u-" + accountID, Email: accountID + "@test.com", Name: "Admin"}, nil
}

func digestUC(repo *fakeDigestRepo, pusher SDRDigestPusher, emailer SDRDigestEmailer, now time.Time) *SendSDRDigestUseCase {
	repo.now = func() time.Time { return now }
	uc := NewSendSDRDigestUseCase(repo, sdrAdminResolver{}, "https://app.replider.com")
	uc.SetDelivery(pusher, emailer)
	return uc.WithClock(func() time.Time { return now })
}

// The one that must never regress: the worker restarting mid-day must not
// send the same account a second digest.
func TestSDRDigest_DoesNotDoubleSendAcrossARestart(t *testing.T) {
	now := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
	repo := &fakeDigestRepo{
		due:     []string{"acc1"},
		digests: map[string]*domain.SDRDigest{"acc1": {PendingApprovals: 3}},
	}
	pusher := &fakeDigestPusher{subscribed: map[string]bool{"acc1": true}}
	uc := digestUC(repo, pusher, nil, now)

	sent, err := uc.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)

	// Same day, fresh process: the DB stamp is what survives, so nothing
	// goes out again.
	again, err := digestUC(repo, pusher, nil, now.Add(2*time.Hour)).Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, again)
	assert.Len(t, pusher.pushes, 1)
}

// A quiet day is skipped AND left unstamped, so the account gets its
// digest the moment there is something worth saying.
func TestSDRDigest_QuietAccountIsSkippedAndStaysDue(t *testing.T) {
	now := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
	repo := &fakeDigestRepo{due: []string{"acc1"}}
	pusher := &fakeDigestPusher{subscribed: map[string]bool{"acc1": true}}

	sent, err := digestUC(repo, pusher, nil, now).Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sent)
	assert.Empty(t, pusher.pushes)
	assert.Empty(t, repo.sent, "a skipped account must stay due")

	// Work appears later the same day — now it sends.
	repo.digests = map[string]*domain.SDRDigest{"acc1": {NewReplies: 1}}
	sent, err = digestUC(repo, pusher, nil, now.Add(time.Hour)).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
}

// Email is a FALLBACK. An account that can receive a push gets exactly one
// notification, not two.
func TestSDRDigest_EmailOnlyWhenPushCannotReach(t *testing.T) {
	now := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
	repo := &fakeDigestRepo{
		due: []string{"pushy", "mailer"},
		digests: map[string]*domain.SDRDigest{
			"pushy":  {PendingApprovals: 1},
			"mailer": {StalledLeads: 4},
		},
	}
	pusher := &fakeDigestPusher{subscribed: map[string]bool{"pushy": true}}
	emailer := &fakeDigestEmailer{}

	sent, err := digestUC(repo, pusher, emailer, now).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, sent)

	require.Len(t, pusher.pushes, 1)
	assert.Equal(t, "sdr_digest", pusher.pushes[0].Tag)
	// The deep link is absolute, because a push click has no page context
	// to resolve a relative path against.
	assert.Equal(t, "https://app.replider.com/prospect?tab=queue", pusher.pushes[0].ClickURL)

	assert.Equal(t, []string{"mailer@test.com"}, emailer.sentTo,
		"only the account that cannot receive a push gets an email")
}

// A delivery that failed must not be stamped as sent, or the account
// silently loses that day's digest.
func TestSDRDigest_FailedDeliveryStaysDue(t *testing.T) {
	now := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
	repo := &fakeDigestRepo{
		due:     []string{"acc1"},
		digests: map[string]*domain.SDRDigest{"acc1": {NewReplies: 2}},
	}
	emailer := &fakeDigestEmailer{err: errors.New("smtp down")}

	sent, err := digestUC(repo, &fakeDigestPusher{}, emailer, now).Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sent)
	assert.Empty(t, repo.sent, "a failed send must leave the account due")
}

// The regression this file exists for since 2026-09-09: the digest used to
// fire on the SERVER's hour. At 08:00 UTC every São Paulo account got its
// "start of your day" notification at 05:00 — the exact case the feature
// set out to avoid. The window is the ACCOUNT's local clock now.
func TestSDRDigest_WaitsForTheAccountsOwnMorning(t *testing.T) {
	// 08:30 UTC is 05:30 in São Paulo and 09:30 in Lisbon.
	now := time.Date(2026, 9, 9, 8, 30, 0, 0, time.UTC)
	repo := &fakeDigestRepo{
		due: []string{"saopaulo", "lisbon"},
		tz: map[string]string{
			"saopaulo": "America/Sao_Paulo",
			"lisbon":   "Europe/Lisbon",
		},
		digests: map[string]*domain.SDRDigest{
			"saopaulo": {PendingApprovals: 41},
			"lisbon":   {PendingApprovals: 2},
		},
	}
	pusher := &fakeDigestPusher{subscribed: map[string]bool{"saopaulo": true, "lisbon": true}}

	sent, err := digestUC(repo, pusher, nil, now).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, sent, "only the account whose local morning has arrived")
	assert.NotContains(t, repo.sent, "saopaulo", "05:30 is not a working hour")
	assert.Contains(t, repo.sent, "lisbon")

	// Three hours later it is 08:30 in São Paulo — now it goes out.
	sent, err = digestUC(repo, pusher, nil, now.Add(3*time.Hour)).Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Contains(t, repo.sent, "saopaulo")
}

// The day the digest covers is the ACCOUNT's day. A window measured from
// the server's midnight would start mid-afternoon for a western account and
// report replies that arrived before the period it claims to cover.
func TestSDRDigest_WindowIsTheAccountsOwnDay(t *testing.T) {
	now := time.Date(2026, 9, 9, 15, 0, 0, 0, time.UTC)
	repo := &fakeDigestRepo{
		due:     []string{"cuiaba"},
		tz:      map[string]string{"cuiaba": "America/Cuiaba"},
		digests: map[string]*domain.SDRDigest{"cuiaba": {NewReplies: 1}},
	}

	_, err := digestUC(repo, &fakeDigestPusher{}, &fakeDigestEmailer{}, now).Execute(context.Background())
	require.NoError(t, err)

	want := domain.StartOfDayIn(now, "America/Cuiaba").Add(-24 * time.Hour)
	assert.True(t, repo.builtWindows["cuiaba"].Equal(want),
		"replies window must start at the account's own local midnight")
}

// One account failing to build must not stop the rest of the run.
func TestSDRDigest_BuildFailureDoesNotAbortTheRun(t *testing.T) {
	now := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
	repo := &fakeDigestRepo{due: []string{"acc1"}, buildErr: errors.New("db blip")}

	sent, err := digestUC(repo, &fakeDigestPusher{}, &fakeDigestEmailer{}, now).Execute(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sent)
}
