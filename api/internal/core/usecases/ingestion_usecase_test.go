package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// stubFetcher is the test double for URLFetcher. Two knobs: the text
// to return and the error to return. Exactly one is expected to be
// non-zero per call.
type stubFetcher struct {
	text string
	err  error
}

func (s *stubFetcher) FetchAndExtract(_ context.Context, _ string) (string, error) {
	return s.text, s.err
}

// stubPDFDownloader + stubPDFExtractor are the ports the PDF branch
// of Process talks to. Kept as fixed-response stubs (no mocktail) so
// the tests stay readable — we're verifying orchestration, not
// call-argument plumbing.
type stubPDFDownloader struct {
	data []byte
	err  error
}

func (s *stubPDFDownloader) DownloadObject(_ context.Context, _ string, _ int64) ([]byte, error) {
	return s.data, s.err
}

type stubPDFExtractor struct {
	text string
	err  error
}

func (s *stubPDFExtractor) Extract(_ []byte) (string, error) {
	return s.text, s.err
}

func TestCreateURLJob_Success(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)

	knowledgeRepo.On("GetSourceByID", ctx, "src-1").Return(&domain.KnowledgeSource{
		ID: "src-1", AgentID: "agent-1",
	}, nil)
	agentRepo.On("GetByID", ctx, "agent-1").Return(&domain.Agent{
		ID: "agent-1", AccountID: "acc-1",
	}, nil)
	jobRepo.On("Create", ctx, mock.MatchedBy(func(j *domain.IngestionJob) bool {
		return j.Type == domain.IngestionJobTypeURL &&
			j.Status == domain.IngestionJobPending &&
			j.Input == "https://example.com" &&
			j.AccountID == "acc-1" &&
			j.KnowledgeSourceID == "src-1"
	})).Return(nil)

	uc := NewIngestionUseCase(jobRepo, knowledgeRepo, agentRepo, nil, nil)
	job, err := uc.CreateURLJob(ctx, CreateURLJobDTO{
		AccountID: "acc-1",
		SourceID:  "src-1",
		URL:       "https://example.com",
	})
	assert.NoError(t, err)
	assert.Equal(t, domain.IngestionJobPending, job.Status)
	jobRepo.AssertExpectations(t)
}

func TestCreateURLJob_WrongAccountReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)

	knowledgeRepo.On("GetSourceByID", ctx, "src-1").Return(&domain.KnowledgeSource{
		ID: "src-1", AgentID: "agent-1",
	}, nil)
	// The source's agent belongs to "acc-OTHER", the caller is "acc-1"
	agentRepo.On("GetByID", ctx, "agent-1").Return(&domain.Agent{
		ID: "agent-1", AccountID: "acc-OTHER",
	}, nil)

	uc := NewIngestionUseCase(jobRepo, knowledgeRepo, agentRepo, nil, nil)
	_, err := uc.CreateURLJob(ctx, CreateURLJobDTO{
		AccountID: "acc-1",
		SourceID:  "src-1",
		URL:       "https://example.com",
	})
	assert.ErrorIs(t, err, domain.ErrKnowledgeNotFound)
	jobRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateURLJob_MissingSource(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)

	knowledgeRepo.On("GetSourceByID", ctx, "src-1").Return(nil, nil)

	uc := NewIngestionUseCase(jobRepo, knowledgeRepo, agentRepo, nil, nil)
	_, err := uc.CreateURLJob(ctx, CreateURLJobDTO{
		AccountID: "acc-1", SourceID: "src-1", URL: "https://x",
	})
	assert.ErrorIs(t, err, domain.ErrKnowledgeNotFound)
}

func TestProcess_HappyPathPersistsChunks(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	aiProvider := new(MockAIProvider)
	fetcher := &stubFetcher{text: longSampleText()}

	// Embedding provider returns the same vector for every chunk.
	aiProvider.On("GetEmbedding", ctx, mock.AnythingOfType("string")).
		Return([]float32{0.1, 0.2, 0.3}, nil)

	// Capture how many chunks got persisted so the assert below can
	// match the Complete argument.
	var persisted int
	knowledgeRepo.On("CreateChunk", ctx, mock.MatchedBy(func(c *domain.KnowledgeChunk) bool {
		persisted++
		return c.SourceID == "src-1" && c.SourceName != nil && *c.SourceName == "https://example.com"
	})).Return(nil)

	jobRepo.On("Complete", ctx, "job-1", mock.AnythingOfType("int32")).Return(nil)

	uc := NewIngestionUseCase(jobRepo, knowledgeRepo, nil, aiProvider, fetcher)

	job := &domain.IngestionJob{
		ID:                "job-1",
		AccountID:         "acc-1",
		KnowledgeSourceID: "src-1",
		Type:              domain.IngestionJobTypeURL,
		Input:             "https://example.com",
		Status:            domain.IngestionJobRunning,
	}
	result := uc.Process(ctx, job)

	assert.Equal(t, domain.IngestionJobCompleted, result.Status)
	assert.GreaterOrEqual(t, persisted, 1)
	assert.Equal(t, int32(persisted), result.ChunksCreated)
	jobRepo.AssertExpectations(t)
}

func TestProcess_FetcherFailure_MapsToFriendlyMessage(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	fetcher := &stubFetcher{err: services.ErrURLTimeout}

	jobRepo.On("Fail", ctx, "job-1", services.ErrURLTimeout.Error()).Return(nil)

	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, fetcher)
	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: domain.IngestionJobTypeURL, Input: "https://slow",
	})
	assert.Equal(t, domain.IngestionJobFailed, result.Status)
	if assert.NotNil(t, result.ErrorMessage) {
		assert.Equal(t, services.ErrURLTimeout.Error(), *result.ErrorMessage)
	}
	jobRepo.AssertExpectations(t)
}

func TestProcess_TooShortContent_FailsAsInsufficient(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	// Fetcher would normally reject <200 chars itself, but if the
	// extractor returns something longer that chunks to zero pieces
	// (all-whitespace text), we fall into Process's own guard.
	fetcher := &stubFetcher{err: services.ErrContentTooShort}

	jobRepo.On("Fail", ctx, "job-1", services.ErrContentTooShort.Error()).Return(nil)

	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, fetcher)
	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: domain.IngestionJobTypeURL, Input: "https://thin",
	})
	assert.Equal(t, domain.IngestionJobFailed, result.Status)
}

func TestProcess_WrongJobTypeFails(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	jobRepo.On("Fail", ctx, "job-1", mock.MatchedBy(func(msg string) bool {
		return true // exact message is an implementation detail
	})).Return(nil)

	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, &stubFetcher{})
	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: "pdf",
	})
	assert.Equal(t, domain.IngestionJobFailed, result.Status)
}

// ──────────────────────────────── PDF branch ────────────────────────────────

func TestCreatePDFJob_Success(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)

	knowledgeRepo.On("GetSourceByID", ctx, "src-1").Return(&domain.KnowledgeSource{
		ID: "src-1", AgentID: "agent-1",
	}, nil)
	agentRepo.On("GetByID", ctx, "agent-1").Return(&domain.Agent{
		ID: "agent-1", AccountID: "acc-1",
	}, nil)
	jobRepo.On("Create", ctx, mock.MatchedBy(func(j *domain.IngestionJob) bool {
		return j.Type == domain.IngestionJobTypePDF &&
			j.Status == domain.IngestionJobPending &&
			j.Input == "accounts/acc-1/123/menu.pdf" &&
			j.AccountID == "acc-1"
	})).Return(nil)

	uc := NewIngestionUseCase(jobRepo, knowledgeRepo, agentRepo, nil, nil)
	job, err := uc.CreatePDFJob(ctx, CreatePDFJobDTO{
		AccountID:  "acc-1",
		SourceID:   "src-1",
		ObjectPath: "accounts/acc-1/123/menu.pdf",
	})
	assert.NoError(t, err)
	assert.Equal(t, domain.IngestionJobTypePDF, job.Type)
}

func TestCreatePDFJob_RejectsForeignObjectPath(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)

	// Caller is acc-1 but object_path claims acc-OTHER — the prefix
	// guard must reject before any DB work so a malicious client
	// can't point the worker at another tenant's upload.
	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, nil)
	_, err := uc.CreatePDFJob(ctx, CreatePDFJobDTO{
		AccountID:  "acc-1",
		SourceID:   "src-1",
		ObjectPath: "accounts/acc-OTHER/123/menu.pdf",
	})
	assert.ErrorIs(t, err, domain.ErrKnowledgeNotFound)
	jobRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreatePDFJob_EmptyObjectPathFails(t *testing.T) {
	uc := NewIngestionUseCase(new(MockIngestionJobRepo), nil, nil, nil, nil)
	_, err := uc.CreatePDFJob(context.Background(), CreatePDFJobDTO{
		AccountID: "acc-1", SourceID: "src-1", ObjectPath: "",
	})
	assert.Error(t, err)
}

func TestProcessPDF_HappyPath(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	aiProvider := new(MockAIProvider)
	downloader := &stubPDFDownloader{data: []byte("%PDF-fake")}
	extractor := &stubPDFExtractor{text: longSampleText()}

	aiProvider.On("GetEmbedding", ctx, mock.AnythingOfType("string")).
		Return([]float32{0.1, 0.2, 0.3}, nil)
	var persisted int
	knowledgeRepo.On("CreateChunk", ctx, mock.MatchedBy(func(c *domain.KnowledgeChunk) bool {
		persisted++
		return c.SourceID == "src-1"
	})).Return(nil)
	jobRepo.On("Complete", ctx, "job-1", mock.AnythingOfType("int32")).Return(nil)

	uc := NewIngestionUseCase(jobRepo, knowledgeRepo, nil, aiProvider, nil)
	uc.SetPDFPipeline(downloader, extractor)

	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: domain.IngestionJobTypePDF,
		KnowledgeSourceID: "src-1", Input: "accounts/acc-1/123/menu.pdf",
	})
	assert.Equal(t, domain.IngestionJobCompleted, result.Status)
	assert.GreaterOrEqual(t, persisted, 1)
}

func TestProcessPDF_ScannedPDFFailsWithFriendlyMessage(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	downloader := &stubPDFDownloader{data: []byte("%PDF-image-only")}
	// Simulates a scanned PDF: extractor returns <100 chars.
	extractor := &stubPDFExtractor{text: "Só uma legenda."}

	jobRepo.On("Fail", ctx, "job-1", ErrPDFScannedOrEmpty.Error()).Return(nil)

	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, nil)
	uc.SetPDFPipeline(downloader, extractor)

	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: domain.IngestionJobTypePDF,
	})
	assert.Equal(t, domain.IngestionJobFailed, result.Status)
	if assert.NotNil(t, result.ErrorMessage) {
		assert.Equal(t, ErrPDFScannedOrEmpty.Error(), *result.ErrorMessage)
	}
}

func TestProcessPDF_DownloadFailureFailsJob(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	downloader := &stubPDFDownloader{err: errors.New("gcs: object not found")}

	jobRepo.On("Fail", ctx, "job-1", mock.AnythingOfType("string")).Return(nil)

	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, nil)
	uc.SetPDFPipeline(downloader, &stubPDFExtractor{})

	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: domain.IngestionJobTypePDF,
	})
	assert.Equal(t, domain.IngestionJobFailed, result.Status)
}

func TestProcessPDF_PipelineNotConfiguredFails(t *testing.T) {
	ctx := context.Background()
	jobRepo := new(MockIngestionJobRepo)
	jobRepo.On("Fail", ctx, "job-1", "pdf pipeline not configured").Return(nil)

	// Not calling SetPDFPipeline — simulates a deploy where someone
	// forgot to wire the GCS adapter. Worker must fail-fast instead
	// of panicking on a nil pointer.
	uc := NewIngestionUseCase(jobRepo, nil, nil, nil, nil)
	result := uc.Process(ctx, &domain.IngestionJob{
		ID: "job-1", Type: domain.IngestionJobTypePDF,
	})
	assert.Equal(t, domain.IngestionJobFailed, result.Status)
}

func TestFriendlyMessage_UnwrapsStructuredErrors(t *testing.T) {
	// Wrapping via fmt.Errorf + %w should still resolve to the pretty copy.
	wrapped := errors.Join(services.ErrURLTooLarge, errors.New("extra context"))
	assert.Equal(t, services.ErrURLTooLarge.Error(), friendlyMessage(wrapped))

	// Unknown errors fall through to their own message.
	assert.Equal(t, "boom", friendlyMessage(errors.New("boom")))
}

// longSampleText returns >200 chars so the chunker produces at least
// one chunk and the min-length guard passes.
func longSampleText() string {
	base := "Replider é uma plataforma SaaS omnichannel de atendimento com IA. "
	out := ""
	for i := 0; i < 20; i++ {
		out += base
	}
	return out
}
