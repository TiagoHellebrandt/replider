package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// Overnight drafting against a real database (§6.17, K2).
//
// Two guarantees are in SQL and nowhere else: exactly one run per account per
// local night (two pods tick in the same minute), and the morning briefing shows
// once. Both are about spending an allowance, so both are worth a real row.

func localDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func TestOvernightDraft_SettingsRoundTripWithTheirDefaults(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)

	// Never opted in.
	missing, err := repo.Settings(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, missing, "an account that never opted in has no row")

	settings := &domain.OvernightDraftSettings{
		AccountID: accountID, Enabled: true, NightlyCap: 40, RunHour: 3,
	}
	require.NoError(t, repo.SaveSettings(ctx, settings))

	stored, err := repo.Settings(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.True(t, stored.Enabled)
	assert.Equal(t, 40, stored.NightlyCap)
	assert.Equal(t, 3, stored.RunHour)

	// Opting out is an update, not a delete: the cap and the hour survive so
	// turning it back on does not lose the configuration.
	settings.Enabled = false
	require.NoError(t, repo.SaveSettings(ctx, settings))
	off, err := repo.Settings(ctx, accountID)
	require.NoError(t, err)
	assert.False(t, off.Enabled)
	assert.Equal(t, 40, off.NightlyCap)
}

func TestOvernightDraft_OnlyOnePodWinsTheNight(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)
	require.NoError(t, repo.SaveSettings(ctx, &domain.OvernightDraftSettings{
		AccountID: accountID, Enabled: true, NightlyCap: 40, RunHour: 3,
	}))

	night := localDate(time.Now())

	first, err := repo.ClaimNight(ctx, accountID, night)
	require.NoError(t, err)
	assert.True(t, first, "the first pod claims the night")

	// The second pod spends nothing. Without this both would draft a night's
	// allowance for the same account.
	second, err := repo.ClaimNight(ctx, accountID, night)
	require.NoError(t, err)
	assert.False(t, second)

	// Tomorrow is a new night.
	tomorrow, err := repo.ClaimNight(ctx, accountID, night.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.True(t, tomorrow)
}

func TestOvernightDraft_ADisabledAccountCannotBeClaimed(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)
	require.NoError(t, repo.SaveSettings(ctx, &domain.OvernightDraftSettings{
		AccountID: accountID, Enabled: false, NightlyCap: 40, RunHour: 3,
	}))

	// The claim carries the enabled check itself: an account that opted out
	// between the listing and the claim must not have its allowance spent.
	claimed, err := repo.ClaimNight(ctx, accountID, localDate(time.Now()))
	require.NoError(t, err)
	assert.False(t, claimed)
}

func TestOvernightDraft_ListEnabledCarriesTheAccountTimezone(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	_, err := testPool.Exec(ctx,
		`UPDATE accounts SET timezone = 'America/Manaus' WHERE id = $1`, accountID)
	require.NoError(t, err)

	repo := postgres.NewOvernightDraftRepository(testPool)
	require.NoError(t, repo.SaveSettings(ctx, &domain.OvernightDraftSettings{
		AccountID: accountID, Enabled: true, NightlyCap: 40, RunHour: 3,
	}))

	targets, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	// The schedule is stated in local hours, so the worker needs the timezone
	// with the row — one query, not a lookup per account.
	assert.Equal(t, "America/Manaus", targets[0].Timezone)
}

func TestOvernightDraft_ASuspendedAccountIsNotDraftedFor(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)
	require.NoError(t, repo.SaveSettings(ctx, &domain.OvernightDraftSettings{
		AccountID: accountID, Enabled: true, NightlyCap: 40, RunHour: 3,
	}))

	_, err := testPool.Exec(ctx,
		`UPDATE accounts SET suspended_at = NOW() WHERE id = $1`, accountID)
	require.NoError(t, err)

	targets, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	// Spending AI budget for an account whose service is suspended is spending
	// money on somebody who is not paying.
	assert.Empty(t, targets)
}

func TestOvernightDraft_TheBriefingShowsOnceAndOnlyWhenThereIsNews(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)
	now := time.Now()

	// A night that drafted nothing.
	require.NoError(t, repo.SaveRun(ctx, &domain.OvernightDraftRun{
		ID: ulid.Make().String(), AccountID: accountID,
		RanOn: localDate(now.AddDate(0, 0, -1)), Drafted: 0, StartedAt: now,
	}))
	empty, err := repo.LatestUnseen(ctx, accountID)
	require.NoError(t, err)
	// "0 abordagens prontas" every morning is how a good habit becomes a
	// dismissed banner.
	assert.Nil(t, empty)

	// A night that produced something.
	runID := ulid.Make().String()
	require.NoError(t, repo.SaveRun(ctx, &domain.OvernightDraftRun{
		ID: runID, AccountID: accountID, RanOn: localDate(now),
		Drafted: 40, SkippedBudget: 12, StartedAt: now,
	}))

	briefing, err := repo.LatestUnseen(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, briefing)
	assert.Equal(t, 40, briefing.Drafted)
	// The cut travels with it, so "40 prontas" when 52 were asked for can
	// explain the missing 12.
	assert.Equal(t, 12, briefing.SkippedBudget)

	require.NoError(t, repo.MarkSeen(ctx, accountID, runID, now))
	seen, err := repo.LatestUnseen(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, seen, "a briefing already read must not reappear all day")
}

func TestOvernightDraft_MarkSeenIsAccountScoped(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)
	now := time.Now()

	runID := ulid.Make().String()
	require.NoError(t, repo.SaveRun(ctx, &domain.OvernightDraftRun{
		ID: runID, AccountID: mine, RanOn: localDate(now), Drafted: 5, StartedAt: now,
	}))

	// Another tenant naming my run id matches zero rows.
	require.NoError(t, repo.MarkSeen(ctx, theirs, runID, now))
	still, err := repo.LatestUnseen(ctx, mine)
	require.NoError(t, err)
	assert.NotNil(t, still, "another account must not be able to mark my briefing read")
}

func TestOvernightDraft_ARetriedNightOverwritesRatherThanDuplicates(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewOvernightDraftRepository(testPool)
	now := time.Now()
	night := localDate(now)

	// A partial run, then the complete one after a retry.
	require.NoError(t, repo.SaveRun(ctx, &domain.OvernightDraftRun{
		ID: ulid.Make().String(), AccountID: accountID, RanOn: night,
		Drafted: 5, StartedAt: now,
	}))
	require.NoError(t, repo.SaveRun(ctx, &domain.OvernightDraftRun{
		ID: ulid.Make().String(), AccountID: accountID, RanOn: night,
		Drafted: 40, StartedAt: now,
	}))

	var rows, drafted int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(MAX(drafted), 0) FROM overnight_draft_runs
		 WHERE account_id = $1`, accountID).Scan(&rows, &drafted))

	// One night, one row — and the complete count wins over the partial.
	assert.Equal(t, 1, rows)
	assert.Equal(t, 40, drafted)
}
