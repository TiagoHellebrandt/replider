package e2e

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The batch-job ledger is what stops a night from being submitted twice and
// what the collector walks — its round-trip (JSONB items included) and the
// open-job guard are pinned against real Postgres.

func TestAIBatchJobRepository_RoundTripAndGuard(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewAIBatchJobRepository(testPool)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)

	job := &domain.AIBatchJob{
		ID: ulid.Make().String(), AccountID: accountID, Kind: "overnight_drafts",
		Provider: "gemini", Model: "gemini-2.5-flash", JobRef: "batches/e2e1",
		Items: []domain.AIBatchItem{
			{LeadID: "l1", FunnelID: "f1"},
			{LeadID: "l2", FunnelID: "f1"},
		},
	}
	require.NoError(t, repo.Create(ctx, job))

	// In flight → guard on, and the job shows up for the collector with its
	// items intact.
	open, err := repo.HasOpenJob(ctx, accountID, "overnight_drafts")
	require.NoError(t, err)
	assert.True(t, open)

	submitted, err := repo.ListSubmitted(ctx)
	require.NoError(t, err)
	require.Len(t, submitted, 1)
	got := submitted[0]
	assert.Equal(t, job.ID, got.ID)
	assert.Equal(t, "batches/e2e1", got.JobRef)
	assert.Equal(t, job.Items, got.Items)
	assert.Equal(t, domain.AIBatchSubmitted, got.Status)

	// Another kind is a different guard.
	open, err = repo.HasOpenJob(ctx, accountID, "opportunity_report")
	require.NoError(t, err)
	assert.False(t, open)

	// Collected → guard clears and the collector stops seeing it.
	require.NoError(t, repo.SetStatus(ctx, job.ID, domain.AIBatchCollected, ""))
	open, err = repo.HasOpenJob(ctx, accountID, "overnight_drafts")
	require.NoError(t, err)
	assert.False(t, open)
	submitted, err = repo.ListSubmitted(ctx)
	require.NoError(t, err)
	assert.Empty(t, submitted)
}

func TestAIBatchJobRepository_FailedKeepsTheErrorForThePostMortem(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewAIBatchJobRepository(testPool)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)

	job := &domain.AIBatchJob{
		ID: ulid.Make().String(), AccountID: accountID, Kind: "overnight_drafts",
		Provider: "gemini", Model: "gemini-2.5-flash", JobRef: "batches/e2e2",
		Items: []domain.AIBatchItem{{LeadID: "l1", FunnelID: "f1"}},
	}
	require.NoError(t, repo.Create(ctx, job))
	require.NoError(t, repo.SetStatus(ctx, job.ID, domain.AIBatchFailed, "JOB_STATE_EXPIRED"))

	var status, errMsg string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT status, error FROM ai_batch_jobs WHERE id = $1`, job.ID).Scan(&status, &errMsg))
	assert.Equal(t, domain.AIBatchFailed, status)
	assert.Equal(t, "JOB_STATE_EXPIRED", errMsg)

	// A failed job clears the in-flight guard — the next night must run.
	open, err := repo.HasOpenJob(ctx, accountID, "overnight_drafts")
	require.NoError(t, err)
	assert.False(t, open)
}
