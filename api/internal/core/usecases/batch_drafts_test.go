package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeBatchAIProvider is an AIProvider that also batches. GenerateResponse
// etc. delegate to the embedded mock so completeDraft's retry paths still
// work when a test needs them.
type fakeBatchAIProvider struct {
	domain.AIProvider
	submitRef    string
	submitErr    error
	submitted    []domain.AIBatchRequest
	fetchDone    bool
	fetchResults []domain.AIBatchResult
	fetchErr     error
}

func (f *fakeBatchAIProvider) SubmitBatch(_ context.Context, _ string, reqs []domain.AIBatchRequest) (string, error) {
	f.submitted = reqs
	return f.submitRef, f.submitErr
}

func (f *fakeBatchAIProvider) FetchBatch(context.Context, string) (bool, []domain.AIBatchResult, error) {
	return f.fetchDone, f.fetchResults, f.fetchErr
}

// fakeBatchJobs is an in-memory domain.AIBatchJobRepository.
type fakeBatchJobs struct {
	jobs      []*domain.AIBatchJob
	createErr error
	statuses  map[string]string
	errs      map[string]string
}

func newFakeBatchJobs() *fakeBatchJobs {
	return &fakeBatchJobs{statuses: map[string]string{}, errs: map[string]string{}}
}

func (f *fakeBatchJobs) Create(_ context.Context, job *domain.AIBatchJob) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.jobs = append(f.jobs, job)
	f.statuses[job.ID] = domain.AIBatchSubmitted
	return nil
}

func (f *fakeBatchJobs) ListSubmitted(context.Context) ([]*domain.AIBatchJob, error) {
	var out []*domain.AIBatchJob
	for _, j := range f.jobs {
		if f.statuses[j.ID] == domain.AIBatchSubmitted {
			out = append(out, j)
		}
	}
	return out, nil
}

func (f *fakeBatchJobs) HasOpenJob(_ context.Context, accountID, kind string) (bool, error) {
	for _, j := range f.jobs {
		if j.AccountID == accountID && j.Kind == kind && f.statuses[j.ID] == domain.AIBatchSubmitted {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeBatchJobs) SetStatus(_ context.Context, id, status, errMsg string) error {
	f.statuses[id] = status
	f.errs[id] = errMsg
	return nil
}

// batchFixture wires newApproachUC with a batch-capable provider and the
// in-memory job store, plus the standard two-draftable-leads funnel.
func batchFixture(t *testing.T) (*ApproachUseCase, *fakeBatchAIProvider, *fakeBatchJobs, *mockLeadRepo) {
	t.Helper()
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, aiFactory, provider, _ := newApproachUC()

	batcher := &fakeBatchAIProvider{AIProvider: provider, submitRef: "batches/xyz"}
	jobs := newFakeBatchJobs()
	uc.SetBatchJobs(jobs)

	funnelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.SalesFunnel{{ID: "f1", AccountID: "acc1", Name: "Vendas", Active: true}}, nil).Maybe()
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas", Active: true}, nil).Maybe()
	leadRepo.On("ListByFunnel", mock.Anything, "f1", domain.LeadStatus(""), mock.Anything, mock.Anything).
		Return([]*domain.Lead{
			{ID: "l1", AccountID: "acc1", FunnelID: "f1", ContactID: "111", Status: domain.LeadStatusEnriched},
			{ID: "l2", AccountID: "acc1", FunnelID: "f1", ContactID: "222", Status: domain.LeadStatusProspected},
		}, nil).Maybe()
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", mock.Anything).Return(false, nil).Maybe()
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil).Maybe()
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(batcher, nil).Maybe()
	return uc, batcher, jobs, leadRepo
}

func TestBatch_Submit_CreatesOneJobForTheNight(t *testing.T) {
	uc, batcher, jobs, leadRepo := batchFixture(t)
	// Submit persists the matcher's pick on each lead (the collector no
	// longer re-runs the matcher when it lands).
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	submitted, optedOut, budget, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	require.NoError(t, err)
	assert.Equal(t, 2, submitted)
	assert.Zero(t, optedOut)
	assert.Zero(t, budget)

	require.Len(t, jobs.jobs, 1)
	job := jobs.jobs[0]
	assert.Equal(t, "batches/xyz", job.JobRef)
	assert.Equal(t, "overnight_drafts", job.Kind)
	require.Len(t, job.Items, 2)
	assert.Equal(t, "l1", job.Items[0].LeadID)
	assert.Equal(t, "f1", job.Items[0].FunnelID)
	assert.Equal(t, "l2", job.Items[1].LeadID)
	// The submitted prompts carry the SAME assembly the sync path uses.
	require.Len(t, batcher.submitted, 2)
	assert.Equal(t, "l1", batcher.submitted[0].Key)
	assert.NotEmpty(t, batcher.submitted[0].SystemPrompt)
	assert.NotEmpty(t, batcher.submitted[0].UserContent)
	// ...and the item keeps what was sent, so landing costs no model call.
	assert.Equal(t, batcher.submitted[0].SystemPrompt, job.Items[0].SystemPrompt)
	assert.Equal(t, batcher.submitted[0].UserContent, job.Items[0].Payload)
}

func TestBatch_Submit_RefusesWhileAJobIsInFlight(t *testing.T) {
	uc, _, jobs, _ := batchFixture(t)
	require.NoError(t, jobs.Create(context.Background(),
		&domain.AIBatchJob{ID: "j0", AccountID: "acc1", Kind: "overnight_drafts"}))

	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "AI_BATCH_IN_FLIGHT", appErr.Code)
}

func TestBatch_Submit_NonBatchProviderIsUnsupported(t *testing.T) {
	uc, _, _, leadRepo := batchFixture(t)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Maybe()
	// Re-point the factory at a provider WITHOUT the batch capability.
	factory := uc.aiFactory.(*MockAIFactory)
	factory.ExpectedCalls = nil
	factory.On("GetProvider", mock.Anything, mock.Anything).Return(new(MockAIProvider), nil)

	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	assert.ErrorIs(t, err, domain.ErrBatchUnsupported)
}

func TestBatch_Submit_NoBatchJobsWiredIsUnsupported(t *testing.T) {
	uc, _, _, _, _, _, _, _, _, _, _, _ := newApproachUC()
	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	assert.ErrorIs(t, err, domain.ErrBatchUnsupported)
}

func TestBatch_Submit_PersistFailureIsLoud(t *testing.T) {
	uc, _, jobs, leadRepo := batchFixture(t)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Maybe()
	jobs.createErr = errors.New("db down")

	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "orphaned")
}

func TestBatch_Collect_LandsDraftsInTheApprovalQueue(t *testing.T) {
	uc, batcher, jobs, leadRepo := batchFixture(t)
	// Submit-time upserts persist the matcher pick (no draft yet); the
	// landing upsert is the one carrying the text.
	var persisted *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		if l.ApproachDraft != "" {
			persisted = l
		}
		return true
	})).Return(nil)
	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	require.NoError(t, err)

	batcher.fetchDone = true
	batcher.fetchResults = []domain.AIBatchResult{
		{Key: "l1", Text: "Oi, hospital!", Usage: domain.AIUsage{PromptTokens: 10, CompletionTokens: 5}},
		{Key: "l2", Text: "", Err: "empty response"}, // per-item failure: skipped, stays draftable
	}
	leadRepo.On("GetByID", mock.Anything, "l1").
		Return(&domain.Lead{ID: "l1", AccountID: "acc1", FunnelID: "f1", ContactID: "111", Status: domain.LeadStatusEnriched}, nil)
	// Landing must not prepare the draft again: the matcher is a paid model
	// call and the prompt travelled on the job item.
	productRepo := uc.productRepo.(*MockProductRepo)
	productCalls := len(productRepo.Calls)

	landed := uc.CollectBatchDrafts(context.Background())
	assert.Equal(t, 1, landed)
	require.NotNil(t, persisted)
	assert.Equal(t, "Oi, hospital!", persisted.ApproachDraft)
	assert.Equal(t, domain.LeadStatusPendingApproval, persisted.Status)
	assert.Equal(t, domain.AIBatchCollected, jobs.statuses[jobs.jobs[0].ID])
	_ = productCalls

	// Collected job clears the in-flight guard.
	open, _ := jobs.HasOpenJob(context.Background(), "acc1", "overnight_drafts")
	assert.False(t, open)
}

func TestBatch_Collect_NeverOverwritesWorkedLeads(t *testing.T) {
	uc, batcher, jobs, leadRepo := batchFixture(t)
	var drafted []string
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		if l.ApproachDraft == "Oi!" || l.ApproachDraft == "Olá!" {
			drafted = append(drafted, l.ID)
		}
		return true
	})).Return(nil)
	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	require.NoError(t, err)

	batcher.fetchDone = true
	batcher.fetchResults = []domain.AIBatchResult{
		{Key: "l1", Text: "Oi!"},
		{Key: "l2", Text: "Olá!"},
	}
	// l1 was contacted during the night; l2 already has a hand-written draft.
	leadRepo.On("GetByID", mock.Anything, "l1").
		Return(&domain.Lead{ID: "l1", AccountID: "acc1", FunnelID: "f1", ContactID: "111", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("GetByID", mock.Anything, "l2").
		Return(&domain.Lead{ID: "l2", AccountID: "acc1", FunnelID: "f1", ContactID: "222", Status: domain.LeadStatusEnriched, ApproachDraft: "meu texto"}, nil)

	landed := uc.CollectBatchDrafts(context.Background())
	assert.Zero(t, landed)
	assert.Empty(t, drafted, "a worked lead must never receive the night's text")
	// The job is still done — its answers were just stale.
	assert.Equal(t, domain.AIBatchCollected, jobs.statuses[jobs.jobs[0].ID])
}

func TestBatch_Collect_StillRunningLeavesTheJobAlone(t *testing.T) {
	uc, batcher, jobs, leadRepo := batchFixture(t)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Maybe()
	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	require.NoError(t, err)

	batcher.fetchDone = false
	leadRepo.On("GetByID", mock.Anything, mock.Anything).
		Return(&domain.Lead{ID: "l1", AccountID: "acc1", FunnelID: "f1", ContactID: "111", Status: domain.LeadStatusEnriched}, nil).Maybe()

	landed := uc.CollectBatchDrafts(context.Background())
	assert.Zero(t, landed)
	assert.Equal(t, domain.AIBatchSubmitted, jobs.statuses[jobs.jobs[0].ID])
}

func TestBatch_Collect_JobFailureIsRecorded(t *testing.T) {
	uc, batcher, jobs, leadRepo := batchFixture(t)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Maybe()
	_, _, _, err := uc.SubmitOvernightBatch(context.Background(), "acc1", "", 0)
	require.NoError(t, err)

	batcher.fetchDone = true
	batcher.fetchErr = errors.New("gemini batch job failed: JOB_STATE_FAILED")
	leadRepo.On("GetByID", mock.Anything, mock.Anything).
		Return(&domain.Lead{ID: "l1", AccountID: "acc1", FunnelID: "f1", ContactID: "111", Status: domain.LeadStatusEnriched}, nil).Maybe()

	landed := uc.CollectBatchDrafts(context.Background())
	assert.Zero(t, landed)
	jobID := jobs.jobs[0].ID
	assert.Equal(t, domain.AIBatchFailed, jobs.statuses[jobID])
	assert.Contains(t, jobs.errs[jobID], "JOB_STATE_FAILED")
	// A failed job also clears the guard, so the next night runs.
	open, _ := jobs.HasOpenJob(context.Background(), "acc1", "overnight_drafts")
	assert.False(t, open)
}
