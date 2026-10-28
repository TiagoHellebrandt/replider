package domain

import (
	"context"
	"time"
)

// Batch generation (custo-BYOK): the overnight flows submit their prompts
// as ONE provider batch job — 50% of the standard price, results within
// 24h — and a collector persists the answers before the morning. Nobody
// ever waits on a batch interactively: the whole point of the nightly
// design is that latency is free between 3am and 8am.

// AIBatchRequest is one prompt inside a job. Key identifies the answer
// (the lead id, for drafts).
type AIBatchRequest struct {
	Key          string
	SystemPrompt string
	UserContent  string
}

// AIBatchResult is one answer. Err is the per-item failure, when the job
// succeeded but this request did not.
type AIBatchResult struct {
	Key   string
	Text  string
	Usage AIUsage
	Err   string
}

// ErrBatchUnsupported says the account's provider cannot batch — the
// caller falls back to the synchronous path, it is not a failure.
var ErrBatchUnsupported = &AppError{Code: "AI_BATCH_UNSUPPORTED", Message: "provider does not support batch generation"}

// AIBatchProvider is the optional capability a provider may implement.
// The factory returns the provider; callers type-assert.
type AIBatchProvider interface {
	SubmitBatch(ctx context.Context, displayName string, reqs []AIBatchRequest) (jobRef string, err error)
	// FetchBatch reports (done=false, nil, nil) while the job is still
	// running; done=true with results when it finished (failed items carry
	// Err); a job-level failure returns done=true with an error.
	FetchBatch(ctx context.Context, jobRef string) (done bool, results []AIBatchResult, err error)
}

// AIBatchJobStatus is the job lifecycle.
const (
	AIBatchSubmitted = "submitted"
	AIBatchCollected = "collected"
	AIBatchFailed    = "failed"
)

// AIBatchItem ties one request key back to what it was about.
type AIBatchItem struct {
	LeadID   string `json:"lead_id"`
	FunnelID string `json:"funnel_id"`
	// SystemPrompt and Payload are what was SENT for this lead, kept so the
	// collector can post-process the answer (and retry a malformed one)
	// without preparing the draft again — preparing runs the dossier and
	// the offer matcher, a paid model call, and used to run three times per
	// lead across submit/probe/land (analysis 2026-09-01). Empty on jobs
	// submitted before this field existed; the collector then re-prepares.
	SystemPrompt string `json:"system_prompt,omitempty"`
	Payload      string `json:"payload,omitempty"`
}

// AIBatchJob is one submitted provider job.
type AIBatchJob struct {
	ID        string        `json:"id"`
	AccountID string        `json:"account_id"`
	Kind      string        `json:"kind"` // "overnight_drafts" for now
	Provider  string        `json:"provider"`
	Model     string        `json:"model"`
	JobRef    string        `json:"job_ref"`
	Status    string        `json:"status"`
	Items     []AIBatchItem `json:"items"`
	Error     string        `json:"error,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// AIBatchJobRepository stores jobs. Raw-SQL repo.
type AIBatchJobRepository interface {
	Create(ctx context.Context, job *AIBatchJob) error
	// ListSubmitted returns every job still waiting on the provider.
	ListSubmitted(ctx context.Context) ([]*AIBatchJob, error)
	// HasOpenJob answers whether the account already has a job in flight —
	// the next night must not re-submit the same leads.
	HasOpenJob(ctx context.Context, accountID, kind string) (bool, error)
	// SetStatus stamps the outcome (error kept for the post-mortem).
	SetStatus(ctx context.Context, id, status, errMsg string) error
}
