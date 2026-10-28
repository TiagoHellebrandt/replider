package domain

import (
	"context"
	"time"
)

// IngestionJobStatus is the lifecycle state of a knowledge-ingestion
// job. It exists as a typed string to keep the worker's state machine
// grep-able and to give the API a stable enum-shaped contract.
type IngestionJobStatus string

const (
	IngestionJobPending   IngestionJobStatus = "pending"
	IngestionJobRunning   IngestionJobStatus = "running"
	IngestionJobCompleted IngestionJobStatus = "completed"
	IngestionJobFailed    IngestionJobStatus = "failed"
)

// IngestionJobType declares which kind of input the worker should
// process. 'url' is the only supported type today; 'pdf' is a planned
// next phase and already in the CHECK constraint so the migration
// doesn't need to ship twice.
type IngestionJobType string

const (
	IngestionJobTypeURL IngestionJobType = "url"
	IngestionJobTypePDF IngestionJobType = "pdf"
)

// IngestionJob is the async contract between the HTTP layer (which
// creates a pending row and returns immediately) and the worker (which
// drains pending rows, runs the pipeline, and writes the terminal
// state + chunks_created counter back).
type IngestionJob struct {
	ID                string             `json:"id"`
	AccountID         string             `json:"account_id"`
	KnowledgeSourceID string             `json:"knowledge_source_id"`
	Type              IngestionJobType   `json:"type"`
	Input             string             `json:"input"`
	Status            IngestionJobStatus `json:"status"`
	ErrorMessage      *string            `json:"error_message,omitempty"`
	ChunksCreated     int32              `json:"chunks_created"`
	StartedAt         *time.Time         `json:"started_at,omitempty"`
	FinishedAt        *time.Time         `json:"finished_at,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
}

// IsTerminal returns true when the job reached a final state. The
// dashboard stops polling on terminal; the worker skips it entirely.
func (j *IngestionJob) IsTerminal() bool {
	return j.Status == IngestionJobCompleted || j.Status == IngestionJobFailed
}

// IngestionJobRepository persists ingestion jobs. Implementations must
// make ClaimNext race-safe across multiple worker processes — today
// there's only one, but the query already uses FOR UPDATE SKIP LOCKED.
type IngestionJobRepository interface {
	Create(ctx context.Context, job *IngestionJob) error
	GetByID(ctx context.Context, id, accountID string) (*IngestionJob, error)
	ListBySource(ctx context.Context, sourceID, accountID string, limit int32) ([]*IngestionJob, error)
	// ClaimNext atomically moves the oldest pending job to 'running'
	// and returns it. Returns (nil, nil) when the queue is empty so
	// the worker can just sleep instead of branching on error kind.
	ClaimNext(ctx context.Context) (*IngestionJob, error)
	Complete(ctx context.Context, id string, chunksCreated int32) error
	Fail(ctx context.Context, id string, errorMessage string) error
}
