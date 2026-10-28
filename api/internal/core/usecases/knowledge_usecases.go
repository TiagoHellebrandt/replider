package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

type KnowledgeUseCase struct {
	knowledgeRepo domain.KnowledgeRepository
	agentRepo     domain.AgentRepository
	aiProvider    domain.AIProvider
}

func NewKnowledgeUseCase(repo domain.KnowledgeRepository, agentRepo domain.AgentRepository, ai domain.AIProvider) *KnowledgeUseCase {
	return &KnowledgeUseCase{
		knowledgeRepo: repo,
		agentRepo:     agentRepo,
		aiProvider:    ai,
	}
}

type IngestKnowledgeDTO struct {
	AccountID  string
	SourceID   string
	Content    string
	SourceName string
	Metadata   any
}

type CreateSourceDTO struct {
	AccountID   string
	AgentID     string
	Name        string
	Description *string
}

func (uc *KnowledgeUseCase) CreateSource(ctx context.Context, dto CreateSourceDTO) (*domain.KnowledgeSource, error) {
	// Verify agent belongs to requesting account
	if err := uc.verifyAgentOwnership(ctx, dto.AgentID, dto.AccountID); err != nil {
		return nil, err
	}

	source := &domain.KnowledgeSource{
		ID:          ulid.Make().String(),
		AgentID:     dto.AgentID,
		Name:        dto.Name,
		Description: dto.Description,
		CreatedAt:   time.Now(),
	}

	if err := uc.knowledgeRepo.CreateSource(ctx, source); err != nil {
		return nil, err
	}

	return source, nil
}

func (uc *KnowledgeUseCase) ListSources(ctx context.Context, accountID, agentID string) ([]*domain.KnowledgeSource, error) {
	if err := uc.verifyAgentOwnership(ctx, agentID, accountID); err != nil {
		return nil, err
	}
	return uc.knowledgeRepo.ListSources(ctx, agentID)
}

func (uc *KnowledgeUseCase) Ingest(ctx context.Context, dto IngestKnowledgeDTO) (*domain.KnowledgeChunk, error) {
	// Verify ownership via source -> agent -> account
	source, err := uc.knowledgeRepo.GetSourceByID(ctx, dto.SourceID)
	if err != nil || source == nil {
		return nil, domain.ErrKnowledgeNotFound
	}
	if err := uc.verifyAgentOwnership(ctx, source.AgentID, dto.AccountID); err != nil {
		return nil, err
	}

	embedding, err := uc.aiProvider.GetEmbedding(ctx, dto.Content)
	if err != nil {
		return nil, err
	}

	chunk := &domain.KnowledgeChunk{
		ID:         ulid.Make().String(),
		SourceID:   dto.SourceID,
		Content:    dto.Content,
		Embedding:  embedding,
		SourceName: &dto.SourceName,
		Metadata:   dto.Metadata,
		CreatedAt:  time.Now(),
	}

	if err := uc.knowledgeRepo.CreateChunk(ctx, chunk); err != nil {
		return nil, err
	}

	return chunk, nil
}

func (uc *KnowledgeUseCase) ListChunks(ctx context.Context, accountID, sourceID string) ([]*domain.KnowledgeChunk, error) {
	source, err := uc.knowledgeRepo.GetSourceByID(ctx, sourceID)
	if err != nil || source == nil {
		return nil, domain.ErrKnowledgeNotFound
	}
	if err := uc.verifyAgentOwnership(ctx, source.AgentID, accountID); err != nil {
		return nil, err
	}
	return uc.knowledgeRepo.ListChunksBySource(ctx, sourceID)
}

// DeleteSource removes a knowledge source and cascades to its chunks
// (FK ON DELETE CASCADE handles the chunks). Ownership is verified via
// source -> agent -> account so a tenant can't delete another tenant's
// data even knowing the source ID.
func (uc *KnowledgeUseCase) DeleteSource(ctx context.Context, accountID, sourceID string) error {
	source, err := uc.knowledgeRepo.GetSourceByID(ctx, sourceID)
	if err != nil || source == nil {
		return domain.ErrKnowledgeNotFound
	}
	if err := uc.verifyAgentOwnership(ctx, source.AgentID, accountID); err != nil {
		return err
	}
	return uc.knowledgeRepo.DeleteSource(ctx, sourceID, source.AgentID)
}

func (uc *KnowledgeUseCase) DeleteChunk(ctx context.Context, accountID, chunkID string) error {
	chunk, err := uc.knowledgeRepo.GetChunkByID(ctx, chunkID)
	if err != nil || chunk == nil {
		return domain.ErrKnowledgeNotFound
	}
	source, err := uc.knowledgeRepo.GetSourceByID(ctx, chunk.SourceID)
	if err != nil || source == nil {
		return domain.ErrKnowledgeNotFound
	}
	if err := uc.verifyAgentOwnership(ctx, source.AgentID, accountID); err != nil {
		return err
	}
	return uc.knowledgeRepo.DeleteChunk(ctx, chunkID)
}

func (uc *KnowledgeUseCase) verifyAgentOwnership(ctx context.Context, agentID, accountID string) error {
	agent, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.AccountID != accountID {
		return domain.ErrAgentNotFound
	}
	return nil
}

// ──────────────────────── Bulk ingestion flows ────────────────────────
//
// The two entry points below create a knowledge source AND all its
// chunks in a single round-trip, suited for the onboarding endpoints
// POST /knowledge/sources/upload and POST /knowledge/sources/url. They
// both reuse the chunker + embedding path so retrieval behaves the
// same as the legacy "create source + ingest chunk" call pair.
//
// If any chunk fails mid-loop we keep the ones already persisted and
// surface the count — the alternative (rollback everything) forces the
// user to resubmit a 10 MB PDF or re-crawl a site on a transient
// embedding hiccup, and leaves no partial value behind.

// CreateSourceFromFileDTO carries a validated file payload + metadata.
// Bytes + Mime have already been checked against the size cap and
// magic-byte sniff at the HTTP layer — the use case trusts them.
type CreateSourceFromFileDTO struct {
	AccountID   string
	AgentID     string
	Name        string
	Description *string
	FileBytes   []byte
	Mime        string
}

// CreateSourceFromFileResult carries the persisted source plus summary
// counters the API returns to the client.
type CreateSourceFromFileResult struct {
	Source        *domain.KnowledgeSource
	ChunksCreated int
	// Preview is the first ~200 chars of the extracted text (trimmed at
	// a word boundary) so the UI can confirm indexing worked without
	// having to re-download the file.
	Preview string
}

// CreateSourceFromFile extracts text from a PDF / DOCX / TXT upload,
// splits it into chunks, embeds each one, and persists a single
// knowledge_source + N chunks atomically from the caller's point of
// view. The returned Preview doubles as a sanity check ("yes, the PDF
// text looked like a menu, not gibberish") for the dashboard toast.
func (uc *KnowledgeUseCase) CreateSourceFromFile(ctx context.Context, dto CreateSourceFromFileDTO) (*CreateSourceFromFileResult, error) {
	if err := uc.verifyAgentOwnership(ctx, dto.AgentID, dto.AccountID); err != nil {
		return nil, err
	}

	text, err := services.ExtractText(dto.FileBytes, dto.Mime)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, errors.New("no text extracted from file")
	}

	source, created, err := uc.persistSourceWithChunks(ctx, dto.AgentID, dto.Name, dto.Description, map[string][]string{
		dto.Name: {text},
	})
	if err != nil {
		return nil, err
	}
	return &CreateSourceFromFileResult{
		Source:        source,
		ChunksCreated: created,
		Preview:       previewOf(text),
	}, nil
}

// CreateSourceFromURLDTO configures a crawl. MaxPages is clamped at
// the use-case layer against crawler limits; 0 means "default".
type CreateSourceFromURLDTO struct {
	AccountID string
	AgentID   string
	URL       string
	MaxPages  int
	// Name defaults to the URL's host when empty.
	Name string
}

// CreateSourceFromURLResult reports the crawl fan-out so the UI can
// show "indexed N pages, M chunks".
type CreateSourceFromURLResult struct {
	Source        *domain.KnowledgeSource
	PagesCrawled  int
	ChunksCreated int
}

// CreateSourceFromURL fetches same-origin HTML pages starting at URL,
// extracts text, chunks, embeds, and persists a single source grouping
// all the pages. Each chunk's metadata carries the page URL so the RAG
// retriever can later cite where an answer came from.
func (uc *KnowledgeUseCase) CreateSourceFromURL(ctx context.Context, dto CreateSourceFromURLDTO) (*CreateSourceFromURLResult, error) {
	if err := uc.verifyAgentOwnership(ctx, dto.AgentID, dto.AccountID); err != nil {
		return nil, err
	}

	limits := services.DefaultCrawlerLimits()
	if dto.MaxPages > 0 && dto.MaxPages < limits.MaxPages {
		limits.MaxPages = dto.MaxPages
	}

	pages, err := services.CrawlURL(ctx, dto.URL, limits)
	if err != nil {
		return nil, err
	}

	perSource := make(map[string][]string, len(pages))
	for _, p := range pages {
		perSource[p.URL] = append(perSource[p.URL], p.Text)
	}

	name := dto.Name
	if name == "" {
		name = fmt.Sprintf("Site crawl: %s", dto.URL)
	}
	source, created, err := uc.persistSourceWithChunks(ctx, dto.AgentID, name, nil, perSource)
	if err != nil {
		return nil, err
	}
	return &CreateSourceFromURLResult{
		Source:        source,
		PagesCrawled:  len(pages),
		ChunksCreated: created,
	}, nil
}

// SeedFromTemplateDTO carries the minimal inputs SeedFromTemplate
// needs. AgentID + AccountID scope the ownership check; the template
// lookup is handled inside the UC so the handler doesn't import
// domain.GetAgentTemplate directly.
type SeedFromTemplateDTO struct {
	AccountID    string
	AgentID      string
	TemplateID   string
	Placeholders map[string]string
	// Locale picks the language for admin-facing labels (source name +
	// per-chunk Title). Empty / unknown values default to "pt", matching
	// the platform's primary audience. Chunk Content stays Portuguese
	// because the agent persona always replies in PT-BR.
	Locale string
}

// SeedFromTemplateResult reports how many seed chunks got indexed.
// Callers render "6 perguntas adicionadas" on success.
type SeedFromTemplateResult struct {
	Source        *domain.KnowledgeSource
	ChunksCreated int
}

// SeedFromTemplate creates a single "Perguntas frequentes" knowledge
// source under the given agent and indexes the template's seed Q/A
// pairs. Placeholders (e.g. {{NOME_DA_EMPRESA}}) are applied to each
// chunk's content before embedding. Idempotent in spirit — the
// handler calls this ONCE per agent creation; subsequent calls would
// create a second source, which is acceptable (the user can delete
// it) but not the intended flow.
//
// Returns domain.ErrAgentNotFound when the agent doesn't belong to
// the account, and a "TEMPLATE_NOT_FOUND" AppError when the template
// id is unknown — same shape as the template-lookup endpoint so the
// Flutter error parser can reuse its code.
func (uc *KnowledgeUseCase) SeedFromTemplate(ctx context.Context, dto SeedFromTemplateDTO) (*SeedFromTemplateResult, error) {
	if err := uc.verifyAgentOwnership(ctx, dto.AgentID, dto.AccountID); err != nil {
		return nil, err
	}
	tpl := domain.GetAgentTemplate(dto.TemplateID)
	if tpl == nil {
		return nil, &domain.AppError{Code: "TEMPLATE_NOT_FOUND", Message: "agent template not found"}
	}
	if len(tpl.SeedKnowledge) == 0 {
		return &SeedFromTemplateResult{Source: nil, ChunksCreated: 0}, nil
	}

	// Each seed chunk becomes its own "source_name" so RAG results can
	// cite the Q/A title. Placeholders are substituted per chunk so
	// "Horário de atendimento" renders the user's actual hours, not
	// the literal token. Title is localised per dto.Locale so an EN
	// dashboard sees "Business hours" while the indexed Content stays
	// PT (the agent persona is PT-only by design).
	texts := make(map[string][]string, len(tpl.SeedKnowledge))
	for _, seed := range tpl.SeedKnowledge {
		content := seed.Content
		for k, v := range dto.Placeholders {
			content = strings.ReplaceAll(content, "{{"+k+"}}", v)
		}
		title := domain.LocalizeSeedTitle(seed.Title, dto.Locale)
		texts[title] = []string{content}
	}

	source, created, err := uc.persistSourceWithChunks(
		ctx, dto.AgentID, domain.SeedSourceName(dto.Locale), nil, texts,
	)
	if err != nil {
		return nil, err
	}
	return &SeedFromTemplateResult{Source: source, ChunksCreated: created}, nil
}

// persistSourceWithChunks is shared between file and URL ingestion.
// textsBySourceName maps a chunk's `source_name` metadata (e.g. page
// URL, or the file name) to its extracted text blocks. Each block is
// chunked with the default size + overlap, embedded, and written.
//
// Failures on individual chunks are logged and skipped so one flaky
// embedding call doesn't wipe an otherwise-successful import. Caller
// sees the final count and can decide whether to warn the user.
func (uc *KnowledgeUseCase) persistSourceWithChunks(
	ctx context.Context,
	agentID string,
	name string,
	description *string,
	textsBySourceName map[string][]string,
) (*domain.KnowledgeSource, int, error) {
	source := &domain.KnowledgeSource{
		ID:          ulid.Make().String(),
		AgentID:     agentID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
	if err := uc.knowledgeRepo.CreateSource(ctx, source); err != nil {
		return nil, 0, err
	}

	created := 0
	for sourceName, blocks := range textsBySourceName {
		for _, block := range blocks {
			pieces := services.ChunkText(block, services.DefaultChunkSize, services.DefaultChunkOverlap)
			for _, piece := range pieces {
				if piece == "" {
					continue
				}
				embedding, err := uc.aiProvider.GetEmbedding(ctx, piece)
				if err != nil {
					// Skip the chunk but keep going. The user still
					// gets partial indexing; a retry later will fill in.
					continue
				}
				sn := sourceName
				chunk := &domain.KnowledgeChunk{
					ID:         ulid.Make().String(),
					SourceID:   source.ID,
					Content:    piece,
					Embedding:  embedding,
					SourceName: &sn,
					CreatedAt:  time.Now(),
				}
				if err := uc.knowledgeRepo.CreateChunk(ctx, chunk); err != nil {
					continue
				}
				created++
			}
		}
	}
	if created == 0 {
		return source, 0, errors.New("no chunks created")
	}
	return source, created, nil
}

// previewOf returns a short (~200 char) head of s, trimmed at a word
// boundary when possible. Purely cosmetic — the dashboard uses it to
// show "here's what we ingested" without loading the full text.
func previewOf(s string) string {
	const maxPreview = 200
	if len(s) <= maxPreview {
		return s
	}
	cut := maxPreview
	// Walk back a handful of chars to land on whitespace. If none
	// found quickly, accept the hard cut — the preview is a nicety.
	for i := 0; i < 40 && cut > 0; i++ {
		if s[cut] == ' ' || s[cut] == '\n' {
			break
		}
		cut--
	}
	return s[:cut] + "…"
}
