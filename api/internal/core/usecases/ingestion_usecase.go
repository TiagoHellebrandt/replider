package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// ErrPDFScannedOrEmpty is returned when a PDF yielded essentially no
// extractable text (<minPDFTextChars). Most common cause: scanned
// documents with no text layer. Copy is user-facing Portuguese,
// matching the style of the services.ErrURL* sentinels.
var ErrPDFScannedOrEmpty = errors.New(
	"PDF parece conter apenas imagens. Para PDFs escaneados, envie a versão com texto selecionável.",
)

// URLFetcher is the port the worker uses to fetch + extract text from
// a URL. Kept as an interface so tests can swap in a stub that returns
// canned content and errors without hitting the network.
type URLFetcher interface {
	FetchAndExtract(ctx context.Context, rawURL string) (string, error)
}

// PDFDownloader is the port the worker uses to fetch PDF bytes from
// object storage. Mirror of GCSAdapter.DownloadObject shrunk to the
// single method so the use case stays storage-agnostic and tests can
// substitute an in-memory byte buffer.
type PDFDownloader interface {
	DownloadObject(ctx context.Context, objectPath string, maxBytes int64) ([]byte, error)
}

// PDFExtractor is the port the worker uses to turn PDF bytes into
// plain text. services.ExtractText + services.DetectMimeByMagic are
// the production implementation; a stub in tests returns fixed text.
type PDFExtractor interface {
	Extract(data []byte) (string, error)
}

// knowledgePDFMaxBytes mirrors the 10 MiB cap the upload-url
// endpoint enforces — the worker re-reads the object and should
// never handle a file bigger than we told GCS to accept, even if
// something slipped through.
const knowledgePDFMaxBytes = 10 * 1024 * 1024

// minPDFTextChars is the floor below which we treat a PDF as
// "scanned / image-only". Below this the chunker barely produces
// anything and RAG quality collapses, so we fail loudly with copy
// that tells the user how to fix it instead of silently indexing
// a useless chunk. OCR is a planned next phase.
const minPDFTextChars = 100

// IngestionUseCase owns the async URL ingestion pipeline: it creates
// pending jobs on the HTTP boundary and, via Process, executes the
// fetch → extract → chunk → embed → persist sequence on the worker
// goroutine. Splitting the two responsibilities into one UC keeps the
// transactional boundary clean (create returns before embedding runs)
// while still sharing ownership checks + domain types.
type IngestionUseCase struct {
	jobRepo       domain.IngestionJobRepository
	knowledgeRepo domain.KnowledgeRepository
	agentRepo     domain.AgentRepository
	aiProvider    domain.AIProvider
	fetcher       URLFetcher
	pdfDownloader PDFDownloader
	pdfExtractor  PDFExtractor
}

func NewIngestionUseCase(
	jobRepo domain.IngestionJobRepository,
	knowledgeRepo domain.KnowledgeRepository,
	agentRepo domain.AgentRepository,
	aiProvider domain.AIProvider,
	fetcher URLFetcher,
) *IngestionUseCase {
	return &IngestionUseCase{
		jobRepo:       jobRepo,
		knowledgeRepo: knowledgeRepo,
		agentRepo:     agentRepo,
		aiProvider:    aiProvider,
		fetcher:       fetcher,
	}
}

// SetPDFPipeline wires the PDF download + extract ports. Kept as a
// setter rather than a constructor arg so the existing URL-only
// callers still compile; main.go wires both in one go. Without this
// call, PDF jobs fail with "pdf pipeline not configured".
func (uc *IngestionUseCase) SetPDFPipeline(downloader PDFDownloader, extractor PDFExtractor) {
	uc.pdfDownloader = downloader
	uc.pdfExtractor = extractor
}

// CreateURLJobDTO is the HTTP boundary input. SourceID + AccountID
// come from the URL path + JWT; URL is user-supplied and validated
// downstream by the fetcher.
type CreateURLJobDTO struct {
	AccountID string
	SourceID  string
	URL       string
}

// CreateURLJob verifies ownership (account → agent → source), writes
// a pending ingestion_job row, and returns immediately. The worker
// will pick it up on its next tick. Does NOT fetch the URL here on
// purpose — a 15s blocking call in an HTTP handler is a footgun for
// load balancer timeouts and mobile connections.
func (uc *IngestionUseCase) CreateURLJob(ctx context.Context, dto CreateURLJobDTO) (*domain.IngestionJob, error) {
	if dto.URL == "" {
		return nil, fmt.Errorf("%w: url is required", services.ErrURLUnreachable)
	}

	source, err := uc.knowledgeRepo.GetSourceByID(ctx, dto.SourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, domain.ErrKnowledgeNotFound
	}
	// Account ownership is enforced through agent → account. A
	// mismatched account hitting this endpoint gets KNOWLEDGE_NOT_FOUND
	// rather than a generic 403, keeping resource existence private.
	agent, err := uc.agentRepo.GetByID(ctx, source.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != dto.AccountID {
		return nil, domain.ErrKnowledgeNotFound
	}

	job := &domain.IngestionJob{
		ID:                ulid.Make().String(),
		AccountID:         dto.AccountID,
		KnowledgeSourceID: dto.SourceID,
		Type:              domain.IngestionJobTypeURL,
		Input:             dto.URL,
		Status:            domain.IngestionJobPending,
		CreatedAt:         time.Now(),
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// CreatePDFJobDTO is the confirm-side input after the app has PUT
// the file to GCS. ObjectPath is what POST /upload-url returned —
// the UC persists it as ingestion_jobs.input so the worker can fetch
// + extract on its next tick.
type CreatePDFJobDTO struct {
	AccountID  string
	SourceID   string
	ObjectPath string
}

// CreatePDFJob verifies ownership, validates the object_path belongs
// to the calling account (it should start with "accounts/{id}/"),
// then enqueues a pending job with type=pdf. Does not download or
// extract — that happens on the worker.
func (uc *IngestionUseCase) CreatePDFJob(ctx context.Context, dto CreatePDFJobDTO) (*domain.IngestionJob, error) {
	if dto.ObjectPath == "" {
		return nil, fmt.Errorf("object_path is required")
	}
	// Object path was minted by /upload-url and is scoped to this
	// account. Checking the prefix prevents a malicious client from
	// pointing the worker at another tenant's upload.
	expectedPrefix := "accounts/" + dto.AccountID + "/"
	if !strings.HasPrefix(dto.ObjectPath, expectedPrefix) {
		return nil, domain.ErrKnowledgeNotFound
	}

	source, err := uc.knowledgeRepo.GetSourceByID(ctx, dto.SourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, domain.ErrKnowledgeNotFound
	}
	agent, err := uc.agentRepo.GetByID(ctx, source.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != dto.AccountID {
		return nil, domain.ErrKnowledgeNotFound
	}

	job := &domain.IngestionJob{
		ID:                ulid.Make().String(),
		AccountID:         dto.AccountID,
		KnowledgeSourceID: dto.SourceID,
		Type:              domain.IngestionJobTypePDF,
		Input:             dto.ObjectPath,
		Status:            domain.IngestionJobPending,
		CreatedAt:         time.Now(),
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// GetJob loads a single job scoped to the calling account. Used by the
// polling endpoint so the dashboard can show "Buscando..." → "Indexando..."
// → "Concluído" transitions.
func (uc *IngestionUseCase) GetJob(ctx context.Context, id, accountID string) (*domain.IngestionJob, error) {
	job, err := uc.jobRepo.GetByID(ctx, id, accountID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, domain.ErrKnowledgeNotFound
	}
	return job, nil
}

// ListJobsForSource returns the ingestion history for a source —
// limit clamped to [1, 100]. Used to render the "Últimas importações"
// strip under the URL tab.
func (uc *IngestionUseCase) ListJobsForSource(ctx context.Context, sourceID, accountID string, limit int32) ([]*domain.IngestionJob, error) {
	source, err := uc.knowledgeRepo.GetSourceByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, domain.ErrKnowledgeNotFound
	}
	agent, err := uc.agentRepo.GetByID(ctx, source.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.AccountID != accountID {
		return nil, domain.ErrKnowledgeNotFound
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return uc.jobRepo.ListBySource(ctx, sourceID, accountID, limit)
}

// Process is invoked by the worker for a just-claimed running job.
// Routes by job.Type to the URL or PDF pipeline. On success it
// transitions the job to 'completed'; on failure it records a
// user-facing message and transitions to 'failed'. Returns the final
// state for tests and the worker's logging.
func (uc *IngestionUseCase) Process(ctx context.Context, job *domain.IngestionJob) *domain.IngestionJob {
	var text string
	var err error

	switch job.Type {
	case domain.IngestionJobTypeURL:
		if uc.fetcher == nil {
			uc.fail(ctx, job, "fetcher not configured")
			return job
		}
		text, err = uc.fetcher.FetchAndExtract(ctx, job.Input)
	case domain.IngestionJobTypePDF:
		if uc.pdfDownloader == nil || uc.pdfExtractor == nil {
			uc.fail(ctx, job, "pdf pipeline not configured")
			return job
		}
		text, err = uc.extractPDFJob(ctx, job)
	default:
		uc.fail(ctx, job, fmt.Sprintf("unsupported job type %q", job.Type))
		return job
	}

	if err != nil {
		uc.fail(ctx, job, friendlyMessage(err))
		return job
	}

	chunksCreated, err := uc.persistChunks(ctx, job, text)
	if err != nil {
		uc.fail(ctx, job, friendlyMessage(err))
		return job
	}
	if chunksCreated == 0 {
		uc.fail(ctx, job, services.ErrContentTooShort.Error())
		return job
	}

	if err := uc.jobRepo.Complete(ctx, job.ID, int32(chunksCreated)); err != nil {
		slog.Error("ingestion: complete failed", "job_id", job.ID, "error", err)
	}
	job.Status = domain.IngestionJobCompleted
	job.ChunksCreated = int32(chunksCreated)
	return job
}

// extractPDFJob downloads the PDF bytes from storage, runs them
// through the extractor, and applies the scanned-PDF guard before
// returning the extracted text.
func (uc *IngestionUseCase) extractPDFJob(ctx context.Context, job *domain.IngestionJob) (string, error) {
	data, err := uc.pdfDownloader.DownloadObject(ctx, job.Input, knowledgePDFMaxBytes)
	if err != nil {
		return "", err
	}
	text, err := uc.pdfExtractor.Extract(data)
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(text)) < minPDFTextChars {
		return "", ErrPDFScannedOrEmpty
	}
	return text, nil
}

// persistChunks splits `text` into chunks, embeds each, and inserts.
// Per-chunk embedding failures are logged and skipped so a single
// flaky provider call doesn't wipe an otherwise-successful import —
// the user will see a non-zero chunks_created and a warning in the
// server logs.
func (uc *IngestionUseCase) persistChunks(ctx context.Context, job *domain.IngestionJob, text string) (int, error) {
	pieces := services.ChunkText(text, services.DefaultChunkSize, services.DefaultChunkOverlap)
	created := 0
	sourceName := job.Input
	for _, piece := range pieces {
		if piece == "" {
			continue
		}
		embedding, err := uc.aiProvider.GetEmbedding(ctx, piece)
		if err != nil {
			slog.Warn("ingestion: embedding failed; skipping chunk",
				"job_id", job.ID, "error", err)
			continue
		}
		sn := sourceName
		chunk := &domain.KnowledgeChunk{
			ID:         ulid.Make().String(),
			SourceID:   job.KnowledgeSourceID,
			Content:    piece,
			Embedding:  embedding,
			SourceName: &sn,
			CreatedAt:  time.Now(),
		}
		if err := uc.knowledgeRepo.CreateChunk(ctx, chunk); err != nil {
			slog.Warn("ingestion: chunk insert failed",
				"job_id", job.ID, "error", err)
			continue
		}
		created++
	}
	return created, nil
}

func (uc *IngestionUseCase) fail(ctx context.Context, job *domain.IngestionJob, message string) {
	if err := uc.jobRepo.Fail(ctx, job.ID, message); err != nil {
		slog.Error("ingestion: fail-status write failed",
			"job_id", job.ID, "error", err)
	}
	job.Status = domain.IngestionJobFailed
	job.ErrorMessage = &message
}

// friendlyMessage pulls the user-facing copy out of a wrapped error,
// falling back to the underlying error string if the error isn't one
// of our structured sentinels. Keeps the UI from ever rendering a raw
// Go error like "Get https://...: dial tcp: ...".
func friendlyMessage(err error) string {
	for _, known := range []error{
		services.ErrURLTimeout,
		services.ErrURLTooLarge,
		services.ErrContentTooShort,
		services.ErrURLUnsupportedCT,
		services.ErrURLUnreachable,
		ErrPDFScannedOrEmpty,
	} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return err.Error()
}
