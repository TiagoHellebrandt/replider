package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestKnowledgeUseCase_CreateSource(t *testing.T) {
	ctx := context.Background()

	t.Run("should create source when agent is owned by account", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)
		knowledgeRepo.On("CreateSource", ctx, mock.MatchedBy(func(s *domain.KnowledgeSource) bool {
			return s.AgentID == "agent1" && s.Name == "FAQ" && len(s.ID) == 26
		})).Return(nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		source, err := uc.CreateSource(ctx, CreateSourceDTO{
			AccountID: "acc1",
			AgentID:   "agent1",
			Name:      "FAQ",
		})

		assert.NoError(t, err)
		assert.NotNil(t, source)
		assert.Equal(t, "agent1", source.AgentID)
		assert.Equal(t, "FAQ", source.Name)
		knowledgeRepo.AssertExpectations(t)
		agentRepo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent belongs to different account", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agent := &domain.Agent{ID: "agent1", AccountID: "other-acc"}
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		source, err := uc.CreateSource(ctx, CreateSourceDTO{
			AccountID: "acc1",
			AgentID:   "agent1",
			Name:      "FAQ",
		})

		assert.Nil(t, source)
		assert.Equal(t, domain.ErrAgentNotFound, err)
	})
}

func TestKnowledgeUseCase_ListSources(t *testing.T) {
	ctx := context.Background()

	t.Run("should list sources for owned agent", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		sources := []*domain.KnowledgeSource{
			{ID: "src1", AgentID: "agent1", Name: "FAQ"},
			{ID: "src2", AgentID: "agent1", Name: "Docs"},
		}
		knowledgeRepo.On("ListSources", ctx, "agent1").Return(sources, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		result, err := uc.ListSources(ctx, "acc1", "agent1")

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		knowledgeRepo.AssertExpectations(t)
	})
}

func TestKnowledgeUseCase_Ingest(t *testing.T) {
	ctx := context.Background()

	t.Run("should ingest chunk with embedding for owned source", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		source := &domain.KnowledgeSource{ID: "src1", AgentID: "agent1"}
		agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}

		knowledgeRepo.On("GetSourceByID", ctx, "src1").Return(source, nil)
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)
		aiProvider.On("GetEmbedding", ctx, "Hello world").Return([]float32{0.1, 0.2, 0.3}, nil)
		knowledgeRepo.On("CreateChunk", ctx, mock.MatchedBy(func(c *domain.KnowledgeChunk) bool {
			return c.SourceID == "src1" && c.Content == "Hello world" && len(c.ID) == 26
		})).Return(nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		chunk, err := uc.Ingest(ctx, IngestKnowledgeDTO{
			AccountID:  "acc1",
			SourceID:   "src1",
			Content:    "Hello world",
			SourceName: "test.txt",
		})

		assert.NoError(t, err)
		assert.NotNil(t, chunk)
		assert.Equal(t, "src1", chunk.SourceID)
		knowledgeRepo.AssertExpectations(t)
		aiProvider.AssertExpectations(t)
	})

	t.Run("should return ErrKnowledgeNotFound when source does not exist", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		knowledgeRepo.On("GetSourceByID", ctx, "bad-src").Return(nil, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		chunk, err := uc.Ingest(ctx, IngestKnowledgeDTO{
			AccountID: "acc1",
			SourceID:  "bad-src",
			Content:   "text",
		})

		assert.Nil(t, chunk)
		assert.Equal(t, domain.ErrKnowledgeNotFound, err)
	})

	t.Run("should return ErrAgentNotFound when source belongs to different account", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		source := &domain.KnowledgeSource{ID: "src1", AgentID: "agent1"}
		agent := &domain.Agent{ID: "agent1", AccountID: "other-acc"}

		knowledgeRepo.On("GetSourceByID", ctx, "src1").Return(source, nil)
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		chunk, err := uc.Ingest(ctx, IngestKnowledgeDTO{
			AccountID: "acc1",
			SourceID:  "src1",
			Content:   "text",
		})

		assert.Nil(t, chunk)
		assert.Equal(t, domain.ErrAgentNotFound, err)
	})
}

func TestKnowledgeUseCase_DeleteChunk(t *testing.T) {
	ctx := context.Background()

	t.Run("should delete chunk when ownership is verified", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		chunk := &domain.KnowledgeChunk{ID: "chunk1", SourceID: "src1"}
		source := &domain.KnowledgeSource{ID: "src1", AgentID: "agent1"}
		agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}

		knowledgeRepo.On("GetChunkByID", ctx, "chunk1").Return(chunk, nil)
		knowledgeRepo.On("GetSourceByID", ctx, "src1").Return(source, nil)
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)
		knowledgeRepo.On("DeleteChunk", ctx, "chunk1").Return(nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		err := uc.DeleteChunk(ctx, "acc1", "chunk1")

		assert.NoError(t, err)
		knowledgeRepo.AssertExpectations(t)
	})

	t.Run("should return ErrKnowledgeNotFound when chunk does not exist", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		knowledgeRepo.On("GetChunkByID", ctx, "bad-chunk").Return(nil, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		err := uc.DeleteChunk(ctx, "acc1", "bad-chunk")

		assert.Equal(t, domain.ErrKnowledgeNotFound, err)
	})

	t.Run("should return ErrAgentNotFound when chunk belongs to different account", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		chunk := &domain.KnowledgeChunk{ID: "chunk1", SourceID: "src1"}
		source := &domain.KnowledgeSource{ID: "src1", AgentID: "agent1"}
		agent := &domain.Agent{ID: "agent1", AccountID: "other-acc"}

		knowledgeRepo.On("GetChunkByID", ctx, "chunk1").Return(chunk, nil)
		knowledgeRepo.On("GetSourceByID", ctx, "src1").Return(source, nil)
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		err := uc.DeleteChunk(ctx, "acc1", "chunk1")

		assert.Equal(t, domain.ErrAgentNotFound, err)
	})
}

func TestKnowledgeUseCase_CreateSourceFromFile(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path: extracts txt, chunks, embeds, persists source + chunks", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		// Source persisted exactly once with the caller-supplied name.
		knowledgeRepo.On("CreateSource", ctx, mock.MatchedBy(func(s *domain.KnowledgeSource) bool {
			return s.AgentID == "agent1" && s.Name == "FAQ.txt"
		})).Return(nil)

		// The short input fits in one chunk → one GetEmbedding + one CreateChunk.
		aiProvider.On("GetEmbedding", ctx, mock.Anything).
			Return([]float32{0.1, 0.2}, nil)
		knowledgeRepo.On("CreateChunk", ctx, mock.MatchedBy(func(c *domain.KnowledgeChunk) bool {
			return c.SourceName != nil && *c.SourceName == "FAQ.txt" &&
				len(c.Embedding) == 2 && c.Content != ""
		})).Return(nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		res, err := uc.CreateSourceFromFile(ctx, CreateSourceFromFileDTO{
			AccountID: "acc1",
			AgentID:   "agent1",
			Name:      "FAQ.txt",
			FileBytes: []byte("Somos uma clínica aberta de segunda a sexta."),
			Mime:      "text/plain; charset=utf-8",
		})

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 1, res.ChunksCreated)
		assert.Contains(t, res.Preview, "clínica")
		knowledgeRepo.AssertExpectations(t)
		aiProvider.AssertExpectations(t)
	})

	t.Run("rejects unsupported mime", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		_, err := uc.CreateSourceFromFile(ctx, CreateSourceFromFileDTO{
			AccountID: "acc1",
			AgentID:   "agent1",
			Name:      "thing.gif",
			FileBytes: []byte("binary"),
			Mime:      "image/gif",
		})
		assert.Error(t, err)
		// Must not have touched the repos.
		knowledgeRepo.AssertNotCalled(t, "CreateSource", mock.Anything, mock.Anything)
	})

	t.Run("denies when agent belongs to another account", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agent := &domain.Agent{ID: "agent1", AccountID: "other"}
		agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		_, err := uc.CreateSourceFromFile(ctx, CreateSourceFromFileDTO{
			AccountID: "acc1",
			AgentID:   "agent1",
			Name:      "FAQ.txt",
			FileBytes: []byte("hello"),
			Mime:      "text/plain",
		})
		assert.Equal(t, domain.ErrAgentNotFound, err)
	})
}

func TestKnowledgeUseCase_CreateSourceFromURL_RejectsInvalidURL(t *testing.T) {
	// Full crawl test requires a live server; the URL-validation path
	// exercises enough of the wiring to catch the obvious regressions.
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	agentRepo := new(MockAgentRepo)
	aiProvider := new(MockAIProvider)

	agent := &domain.Agent{ID: "agent1", AccountID: "acc1"}
	agentRepo.On("GetByID", ctx, "agent1").Return(agent, nil)

	uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
	_, err := uc.CreateSourceFromURL(ctx, CreateSourceFromURLDTO{
		AccountID: "acc1",
		AgentID:   "agent1",
		URL:       "http://localhost:8080", // http + loopback = double-deny
		MaxPages:  5,
	})
	assert.Error(t, err)
	knowledgeRepo.AssertNotCalled(t, "CreateSource", mock.Anything, mock.Anything)
}

func TestKnowledgeUseCase_SeedFromTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("indexes every seed chunk with placeholders substituted", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agentRepo.On("GetByID", ctx, "agent1").Return(
			&domain.Agent{ID: "agent1", AccountID: "acc1"}, nil,
		)
		knowledgeRepo.On("CreateSource", ctx, mock.MatchedBy(func(s *domain.KnowledgeSource) bool {
			return s.AgentID == "agent1" && s.Name == "Perguntas frequentes"
		})).Return(nil)

		// One embedding call + one chunk insert per seed Q/A.
		// Capture contents so we can assert placeholder substitution.
		seenContents := []string{}
		aiProvider.On("GetEmbedding", ctx, mock.MatchedBy(func(c string) bool {
			seenContents = append(seenContents, c)
			return true
		})).Return([]float32{0.1, 0.2}, nil)
		knowledgeRepo.On("CreateChunk", ctx, mock.Anything).Return(nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		res, err := uc.SeedFromTemplate(ctx, SeedFromTemplateDTO{
			AccountID:  "acc1",
			AgentID:    "agent1",
			TemplateID: "clinic",
			Placeholders: map[string]string{
				"NOME_DA_EMPRESA":     "Clínica Luz",
				"HORARIO_ATENDIMENTO": "seg-sex 9-18h",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, "Perguntas frequentes", res.Source.Name)
		// 7 seed chunks for the clinic template.
		assert.Equal(t, 7, res.ChunksCreated)
		// At least one chunk should have had the horário substitution applied.
		found := false
		for _, c := range seenContents {
			if contains(c, "seg-sex 9-18h") {
				found = true
			}
			assert.NotContains(t, c, "{{HORARIO_ATENDIMENTO}}",
				"placeholder leaked into embedded content")
		}
		assert.True(t, found, "expected at least one seed chunk to reference the horário")
	})

	t.Run("rejects unknown template id", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		agentRepo.On("GetByID", ctx, "agent1").Return(
			&domain.Agent{ID: "agent1", AccountID: "acc1"}, nil,
		)
		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, new(MockAIProvider))
		_, err := uc.SeedFromTemplate(ctx, SeedFromTemplateDTO{
			AccountID: "acc1", AgentID: "agent1", TemplateID: "nonexistent",
		})
		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "TEMPLATE_NOT_FOUND", appErr.Code)
		knowledgeRepo.AssertNotCalled(t, "CreateSource", mock.Anything, mock.Anything)
	})

	t.Run("locale=en localises source name and chunk titles", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		aiProvider := new(MockAIProvider)

		agentRepo.On("GetByID", ctx, "agent1").Return(
			&domain.Agent{ID: "agent1", AccountID: "acc1"}, nil,
		)
		// Source name is the EN translation.
		knowledgeRepo.On("CreateSource", ctx, mock.MatchedBy(func(s *domain.KnowledgeSource) bool {
			return s.AgentID == "agent1" && s.Name == "FAQ"
		})).Return(nil)

		// Capture chunk source_names — they should also be EN.
		seenSourceNames := map[string]struct{}{}
		aiProvider.On("GetEmbedding", ctx, mock.Anything).Return([]float32{0.1}, nil)
		knowledgeRepo.On("CreateChunk", ctx, mock.MatchedBy(func(c *domain.KnowledgeChunk) bool {
			if c.SourceName != nil {
				seenSourceNames[*c.SourceName] = struct{}{}
			}
			return true
		})).Return(nil)

		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, aiProvider)
		_, err := uc.SeedFromTemplate(ctx, SeedFromTemplateDTO{
			AccountID:  "acc1",
			AgentID:    "agent1",
			TemplateID: "clinic",
			Locale:     "en",
		})
		assert.NoError(t, err)
		// "Horário de atendimento" → "Business hours"; "Reembolso" → "Refunds"
		assert.Contains(t, seenSourceNames, "Business hours")
		assert.Contains(t, seenSourceNames, "Refunds")
		assert.NotContains(t, seenSourceNames, "Horário de atendimento")
	})

	t.Run("rejects foreign agent", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		agentRepo := new(MockAgentRepo)
		agentRepo.On("GetByID", ctx, "agent1").Return(
			&domain.Agent{ID: "agent1", AccountID: "OTHER"}, nil,
		)
		uc := NewKnowledgeUseCase(knowledgeRepo, agentRepo, new(MockAIProvider))
		_, err := uc.SeedFromTemplate(ctx, SeedFromTemplateDTO{
			AccountID: "acc1", AgentID: "agent1", TemplateID: "clinic",
		})
		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
	})
}

// contains is a trivial strings.Contains shim to keep the import list
// short in this test file (it already imports too much).
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
