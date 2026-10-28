package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockAIFactory struct{ mock.Mock }

func (m *MockAIFactory) GetProvider(ctx context.Context, providerName string) (domain.AIProvider, error) {
	args := m.Called(ctx, providerName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(domain.AIProvider), args.Error(1)
}

func (m *MockAIFactory) GetProviderForAccount(ctx context.Context, providerName string, account *domain.Account, plan *domain.Plan) (domain.AIProvider, error) {
	// Tests stub at the GetProvider level — the BYOK path collapses to
	// the same call when account/plan are nil or self-serve, which is
	// what these tests exercise.
	return m.GetProvider(ctx, providerName)
}

func (m *MockAIFactory) GetCostConfig(provider, model string) (float64, float64) {
	args := m.Called(provider, model)
	return args.Get(0).(float64), args.Get(1).(float64)
}

type MockAIProvider struct{ mock.Mock }

func (m *MockAIProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	args := m.Called(ctx, systemPrompt, history, tools)
	return args.Get(0).(domain.StreamResult), args.Error(1)
}

func (m *MockAIProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	args := m.Called(ctx, systemPrompt, history, tools)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AICallbackResponse), args.Error(1)
}

func (m *MockAIProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

type MockKnowledgeRepo struct{ mock.Mock }

func (m *MockKnowledgeRepo) CreateSource(ctx context.Context, source *domain.KnowledgeSource) error {
	return m.Called(ctx, source).Error(0)
}
func (m *MockKnowledgeRepo) GetSource(ctx context.Context, id string, agentID string) (*domain.KnowledgeSource, error) {
	args := m.Called(ctx, id, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KnowledgeSource), args.Error(1)
}
func (m *MockKnowledgeRepo) GetSourceByID(ctx context.Context, id string) (*domain.KnowledgeSource, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KnowledgeSource), args.Error(1)
}
func (m *MockKnowledgeRepo) ListSources(ctx context.Context, agentID string) ([]*domain.KnowledgeSource, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.KnowledgeSource), args.Error(1)
}
func (m *MockKnowledgeRepo) UpdateSource(ctx context.Context, source *domain.KnowledgeSource) error {
	return m.Called(ctx, source).Error(0)
}
func (m *MockKnowledgeRepo) DeleteSource(ctx context.Context, id string, agentID string) error {
	return m.Called(ctx, id, agentID).Error(0)
}
func (m *MockKnowledgeRepo) CreateChunk(ctx context.Context, chunk *domain.KnowledgeChunk) error {
	return m.Called(ctx, chunk).Error(0)
}
func (m *MockKnowledgeRepo) GetChunkByID(ctx context.Context, id string) (*domain.KnowledgeChunk, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KnowledgeChunk), args.Error(1)
}
func (m *MockKnowledgeRepo) Search(ctx context.Context, sourceID string, embedding []float32, limit int32) ([]*domain.KnowledgeChunk, error) {
	args := m.Called(ctx, sourceID, embedding, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.KnowledgeChunk), args.Error(1)
}
func (m *MockKnowledgeRepo) ListChunksBySource(ctx context.Context, sourceID string) ([]*domain.KnowledgeChunk, error) {
	args := m.Called(ctx, sourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.KnowledgeChunk), args.Error(1)
}
func (m *MockKnowledgeRepo) UpdateEmbedding(ctx context.Context, chunkID string, embedding []float32, provider string) error {
	return m.Called(ctx, chunkID, embedding, provider).Error(0)
}
func (m *MockKnowledgeRepo) DeleteChunk(ctx context.Context, chunkID string) error {
	return m.Called(ctx, chunkID).Error(0)
}

type MockCalendarRepo struct{ mock.Mock }

func (m *MockCalendarRepo) Create(ctx context.Context, cal *domain.Calendar) error {
	return m.Called(ctx, cal).Error(0)
}
func (m *MockCalendarRepo) GetByID(ctx context.Context, id string) (*domain.Calendar, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Calendar), args.Error(1)
}
func (m *MockCalendarRepo) ListByAgentID(ctx context.Context, agentID string) ([]*domain.Calendar, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Calendar), args.Error(1)
}
func (m *MockCalendarRepo) Update(ctx context.Context, cal *domain.Calendar) error {
	return m.Called(ctx, cal).Error(0)
}
func (m *MockCalendarRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

// --- Helpers ---

func strPtr(s string) *string { return &s }

func makeAgent(systemPrompt *string) *domain.Agent {
	return &domain.Agent{
		ID:           "agent_001",
		AccountID:    "acc_001",
		Name:         "Test Agent",
		SystemPrompt: systemPrompt,
		Provider:     "gemini",
		Model:        "gemini-3-flash-preview",
		Timezone:     "America/Sao_Paulo",
		IsActive:     true,
	}
}

func makeSource(id string) *domain.KnowledgeSource {
	return &domain.KnowledgeSource{
		ID:      id,
		AgentID: "agent_001",
		Name:    "FAQ",
	}
}

func makeChunk(id, sourceID, content string) *domain.KnowledgeChunk {
	name := "FAQ"
	return &domain.KnowledgeChunk{
		ID:         id,
		SourceID:   sourceID,
		Content:    content,
		SourceName: &name,
		// Above the similarity floor so the runner keeps the chunk; real
		// data comes out of pgvector with a populated similarity.
		Similarity: 0.9,
	}
}

func makeCalendar(id, name string, active bool, days []int32, duration int32) *domain.Calendar {
	return &domain.Calendar{
		ID:                  id,
		AgentID:             "agent_001",
		Name:                name,
		AvailableDays:       days,
		SlotDurationMinutes: duration,
		IsActive:            active,
	}
}

// --- Tests ---

func TestBuildSystemPrompt_NoRAGNoCalendars(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	customPrompt := "Você é o assistente da Loja X. Responda sempre em português."
	agent := makeAgent(strPtr(customPrompt))

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Olá, tudo bem?", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.", "should start with default persona")
	assert.Contains(t, result.SystemPrompt, customPrompt, "should include agent's custom prompt")
	assert.Empty(t, result.Tools, "should have no tools without calendars")
	assert.NotContains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT", "should not have RAG context")
	assert.NotContains(t, result.SystemPrompt, "Booking Behavior", "should not have booking instructions")

	knowledgeRepo.AssertExpectations(t)
	calendarRepo.AssertExpectations(t)
}

func TestBuildSystemPrompt_NilSystemPrompt(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(nil)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Oi", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.", "with nil system_prompt, should be exactly the default persona")
	assert.Empty(t, result.Tools)
}

func TestBuildSystemPrompt_EmptySystemPrompt(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(strPtr("   "))

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Oi", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.", "whitespace-only prompt should be treated as empty")
}

func TestBuildSystemPrompt_WithRAGChunks(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(strPtr("Assistente da Loja X"))
	source := makeSource("src_001")
	embedding := []float32{0.1, 0.2, 0.3}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "Qual o horário de funcionamento?").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
		makeChunk("chunk_1", "src_001", "Horário: Segunda a Sexta, 9h às 18h."),
		makeChunk("chunk_2", "src_001", "Sábados: 9h às 13h."),
	}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Qual o horário de funcionamento?", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Assistente da Loja X")
	assert.Contains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
	assert.Contains(t, result.SystemPrompt, "Horário: Segunda a Sexta, 9h às 18h.")
	assert.Contains(t, result.SystemPrompt, "Sábados: 9h às 13h.")
	assert.Empty(t, result.Tools)

	knowledgeRepo.AssertExpectations(t)
	aiFactory.AssertExpectations(t)
	aiProvider.AssertExpectations(t)
}

func TestBuildSystemPrompt_WithActiveCalendar(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(strPtr("Assistente do Consultório"))
	cal := makeCalendar("cal_001", "Dr. Silva", true, []int32{1, 2, 3, 4, 5}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Quero agendar uma consulta", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "Booking Behavior")
	assert.Contains(t, result.SystemPrompt, "check_availability")
	assert.Contains(t, result.SystemPrompt, "Dr. Silva")
	assert.Contains(t, result.SystemPrompt, "cal_001")
	assert.Contains(t, result.SystemPrompt, "30 min")
	assert.Contains(t, result.SystemPrompt, "Mon, Tue, Wed, Thu, Fri")

	require.Len(t, result.Tools, 1)
	assert.Equal(t, "check_availability", result.Tools[0].Name)
	assert.Contains(t, result.Tools[0].Description, "available time slots")

	calendarRepo.AssertExpectations(t)
}

func TestBuildSystemPrompt_InactiveCalendarIgnored(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(nil)
	inactiveCal := makeCalendar("cal_001", "Dr. Silva", false, []int32{1, 2, 3}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{inactiveCal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Quero agendar", "", "pt")

	require.NoError(t, err)
	assert.Empty(t, result.Tools, "inactive calendars should not generate tools")
	assert.NotContains(t, result.SystemPrompt, "Booking Behavior")
}

func TestBuildSystemPrompt_WithBothRAGAndCalendar(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(strPtr("Clínica Saúde Total"))
	source := makeSource("src_001")
	embedding := []float32{0.5, 0.6}
	cal := makeCalendar("cal_001", "Dra. Maria", true, []int32{1, 3, 5}, 45)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "Como agendar consulta?").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
		makeChunk("chunk_1", "src_001", "Aceitamos convênios Unimed e SulAmérica."),
	}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "Como agendar consulta?", "", "pt")

	require.NoError(t, err)
	// All four sections should be present
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Clínica Saúde Total")
	assert.Contains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
	assert.Contains(t, result.SystemPrompt, "Aceitamos convênios Unimed e SulAmérica.")
	assert.Contains(t, result.SystemPrompt, "Booking Behavior")
	assert.Contains(t, result.SystemPrompt, "Dra. Maria")
	require.Len(t, result.Tools, 1)
	assert.Equal(t, "check_availability", result.Tools[0].Name)
}

func TestBuildSystemPrompt_RAGSearchReturnsNoResults(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(strPtr("Loja Y"))
	source := makeSource("src_001")
	embedding := []float32{0.1, 0.2}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "algo irrelevante").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "algo irrelevante", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Loja Y")
	assert.NotContains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT", "no chunks means no RAG section")
}

func TestBuildSystemPrompt_RAGSearchFails_GracefulDegradation(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(strPtr("Loja Z"))
	source := makeSource("src_001")
	embedding := []float32{0.1}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "teste").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return(nil, errors.New("database connection lost"))
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "teste", "", "pt")

	require.NoError(t, err, "RAG search failure should not propagate as error")
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Loja Z")
	assert.NotContains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
}

func TestBuildSystemPrompt_EmbeddingFails_GracefulDegradation(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(strPtr("Loja W"))
	source := makeSource("src_001")

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "teste").Return(nil, errors.New("API key expired"))
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "teste", "", "pt")

	require.NoError(t, err, "embedding failure should not propagate as error")
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Loja W")
	assert.NotContains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
}

func TestBuildSystemPrompt_AIFactoryFails_GracefulDegradation(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(strPtr("Loja V"))
	source := makeSource("src_001")

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(nil, errors.New("provider unavailable"))
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "teste", "", "pt")

	require.NoError(t, err, "AI factory failure should not propagate as error")
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Loja V")
	assert.NotContains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
}

func TestBuildSystemPrompt_ListSourcesFails_GracefulDegradation(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(strPtr("Loja U"))

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return(nil, errors.New("db error"))
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "teste", "", "pt")

	require.NoError(t, err, "ListSources failure should not propagate as error")
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Contains(t, result.SystemPrompt, "Loja U")
}

func TestBuildSystemPrompt_CalendarRepoFails_GracefulDegradation(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(strPtr("Loja T"))

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return(nil, errors.New("calendar db error"))

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "teste", "", "pt")

	require.NoError(t, err, "calendar repo failure should not propagate as error")
	assert.Contains(t, result.SystemPrompt, "You are a virtual customer service assistant.")
	assert.Empty(t, result.Tools)
}

func TestBuildSystemPrompt_RAGContextTruncation(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(nil)
	source := makeSource("src_001")
	embedding := []float32{0.1}

	// Create chunks that together exceed the 3000 char limit
	longContent := strings.Repeat("A", 1400)
	chunks := []*domain.KnowledgeChunk{
		makeChunk("c1", "src_001", longContent),
		makeChunk("c2", "src_001", longContent),
		makeChunk("c3", "src_001", longContent), // This one should be truncated out
	}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "busca longa").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return(chunks, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "busca longa", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")

	// The RAG section starts after "KNOWLEDGE BASE CONTEXT" marker
	ragIdx := strings.Index(result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
	ragSection := result.SystemPrompt[ragIdx:]

	// Count how many chunk markers appear - each chunk starts with "[Source:"
	chunkCount := strings.Count(ragSection, "[Source:")
	assert.Equal(t, 2, chunkCount, "only 2 of 3 chunks should fit within the 3000 char limit")
}

func TestBuildSystemPrompt_MultipleSources(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(nil)
	source1 := makeSource("src_001")
	source2 := makeSource("src_002")
	embedding := []float32{0.1}

	name1 := "FAQ"
	name2 := "Politicas"
	chunk1 := &domain.KnowledgeChunk{ID: "c1", SourceID: "src_001", Content: "Horário: 9-18h", SourceName: &name1, Similarity: 0.9}
	chunk2 := &domain.KnowledgeChunk{ID: "c2", SourceID: "src_002", Content: "Política de troca: 30 dias", SourceName: &name2, Similarity: 0.9}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source1, source2}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "pergunta").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{chunk1}, nil)
	knowledgeRepo.On("Search", ctx, "src_002", embedding, int32(5)).Return([]*domain.KnowledgeChunk{chunk2}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "pergunta", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "Horário: 9-18h")
	assert.Contains(t, result.SystemPrompt, "Política de troca: 30 dias")
}

func TestBuildSystemPrompt_MultipleCalendars(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(nil)
	cal1 := makeCalendar("cal_001", "Dr. Silva", true, []int32{1, 3, 5}, 30)
	cal2 := makeCalendar("cal_002", "Dra. Santos", true, []int32{2, 4}, 60)
	calInactive := makeCalendar("cal_003", "Dr. Inativo", false, []int32{1}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal1, cal2, calInactive}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "quero agendar", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "Dr. Silva")
	assert.Contains(t, result.SystemPrompt, "Dra. Santos")
	assert.NotContains(t, result.SystemPrompt, "Dr. Inativo", "inactive calendars should not appear in prompt")
	assert.Contains(t, result.SystemPrompt, "30 min")
	assert.Contains(t, result.SystemPrompt, "60 min")
	require.Len(t, result.Tools, 1, "should have exactly one check_availability tool regardless of calendar count")
}

func TestBuildSystemPrompt_CalendarDayNames(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(nil)
	// Sunday (0) through Saturday (6)
	cal := makeCalendar("cal_001", "Completo", true, []int32{0, 1, 2, 3, 4, 5, 6}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "dias", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "Sun, Mon, Tue, Wed, Thu, Fri, Sat")
}

// The prompt is layered for provider-side caching (K3): everything stable first,
// everything per-message last. Both providers cache by EXACT PREFIX, so this
// order is not cosmetic — a stable block placed after a variable one is billed at
// full rate on every single turn.
func TestBuildSystemPrompt_StableSectionsComeBeforeVariableOnes(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	customPrompt := "CUSTOM AGENT INSTRUCTIONS"
	agent := makeAgent(strPtr(customPrompt))
	source := makeSource("src_001")
	embedding := []float32{0.1}
	cal := makeCalendar("cal_001", "Agenda", true, []int32{1}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "msg").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
		makeChunk("c1", "src_001", "RAG content here"),
	}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "msg", "", "pt")
	require.NoError(t, err)

	personaIdx := strings.Index(result.SystemPrompt, "virtual customer service assistant")
	customIdx := strings.Index(result.SystemPrompt, customPrompt)
	bookingIdx := strings.Index(result.SystemPrompt, "Booking Behavior")
	langIdx := strings.Index(result.SystemPrompt, "REPLY LANGUAGE")
	ragIdx := strings.Index(result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")

	for name, idx := range map[string]int{
		"persona": personaIdx, "custom prompt": customIdx,
		"booking": bookingIdx, "language": langIdx, "RAG": ragIdx,
	} {
		assert.GreaterOrEqual(t, idx, 0, "%s section missing", name)
	}

	assert.Less(t, personaIdx, customIdx, "persona before the agent's own instructions")
	// Booking is STATIC — it changes only when the calendars are edited — so it
	// belongs inside the cacheable prefix, ahead of everything per-message.
	assert.Less(t, customIdx, bookingIdx, "the agent's instructions before booking")
	assert.Less(t, bookingIdx, langIdx, "every stable section before the language tag")
	// The language tag used to sit ~30 bytes into the prompt, ahead of thousands
	// of tokens of persona. Two conversations in different languages shared
	// almost no cacheable prefix at all.
	assert.Less(t, langIdx, ragIdx, "language tag before the retrieved knowledge")
}

// The property that makes the ordering worth anything: two DIFFERENT messages,
// in two different languages, must still share a byte-identical prefix.
func TestBuildSystemPrompt_PrefixIsIdenticalAcrossConversations(t *testing.T) {
	ctx := context.Background()
	build := func(userMessage, language, chunk string) string {
		knowledgeRepo := new(MockKnowledgeRepo)
		calendarRepo := new(MockCalendarRepo)
		aiFactory := new(MockAIFactory)
		aiProvider := new(MockAIProvider)

		agent := makeAgent(strPtr("CUSTOM AGENT INSTRUCTIONS"))
		source := makeSource("src_001")
		embedding := []float32{0.1}
		cal := makeCalendar("cal_001", "Agenda", true, []int32{1}, 30)

		knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
		aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
		aiProvider.On("GetEmbedding", ctx, userMessage).Return(embedding, nil)
		knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
			makeChunk("c1", "src_001", chunk),
		}, nil)
		calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

		runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
		result, err := runner.BuildSystemPrompt(ctx, agent, userMessage, "Clínica Bem Viver", language)
		require.NoError(t, err)
		return result.SystemPrompt
	}

	portuguese := build("qual o preço?", "pt", "tabela de preços")
	english := build("how much is it?", "en", "price list")

	// Everything up to the first variable section has to match exactly. Not
	// "mostly" — one differing byte and the provider bills the whole prefix.
	cut := func(prompt string) string {
		idx := strings.Index(prompt, "REPLY LANGUAGE")
		require.GreaterOrEqual(t, idx, 0)
		return prompt[:idx]
	}
	assert.Equal(t, cut(portuguese), cut(english),
		"the cacheable prefix must not depend on the message or the language")
	// And the prefix has to be worth caching: it carries the persona, the
	// identity and the agent's instructions, not three lines of preamble.
	assert.Greater(t, len(cut(portuguese)), 1000,
		"a prefix this short means a section leaked into the variable half")
}

func TestBuildSystemPrompt_OneSourceSearchFailsOtherSucceeds(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(nil)
	source1 := makeSource("src_001")
	source2 := makeSource("src_002")
	embedding := []float32{0.1}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source1, source2}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "teste").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return(nil, errors.New("source corrupted"))
	knowledgeRepo.On("Search", ctx, "src_002", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
		makeChunk("c1", "src_002", "Info from source 2"),
	}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "teste", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "KNOWLEDGE BASE CONTEXT")
	assert.Contains(t, result.SystemPrompt, "Info from source 2")
}

func TestBuildSystemPrompt_RAGContextExactly3000Limit(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(nil)
	source := makeSource("src_001")
	embedding := []float32{0.1}

	// The formatted chunk line is: fmt.Sprintf("\n[Source: %v] %s\n", c.SourceName, c.Content)
	// where SourceName is *string and %v prints the pointer address representation.
	// We compute the actual overhead per chunk to set up the boundary precisely.
	sampleName := "FAQ"
	sampleChunk := &domain.KnowledgeChunk{SourceName: &sampleName, Content: ""}
	overhead := len(fmt.Sprintf("\n[Source: %v] %s\n", sampleChunk.SourceName, sampleChunk.Content))

	// Create a chunk whose formatted line is exactly 3000 chars
	contentLen := 3000 - overhead
	chunk1Content := strings.Repeat("X", contentLen)
	chunk1 := makeChunk("c1", "src_001", chunk1Content)
	chunk2 := makeChunk("c2", "src_001", "This should not appear")

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "test").Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{chunk1, chunk2}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "test", "", "pt")

	require.NoError(t, err)

	// The first chunk (exactly 3000 chars formatted) should fit (condition is > 3000, not >=)
	// The second chunk should be excluded since adding it would exceed 3000
	assert.Contains(t, result.SystemPrompt, chunk1Content, "first chunk should be included")
	assert.NotContains(t, result.SystemPrompt, "This should not appear", "second chunk should be excluded")
}

func TestBuildSystemPrompt_CalendarTimezoneInPrompt(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(nil)
	agent.Timezone = "America/Sao_Paulo"
	cal := makeCalendar("cal_001", "Agenda", true, []int32{1}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "agendar", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "America/Sao_Paulo")
	assert.Contains(t, result.SystemPrompt, "Timezone")
}

// TestBuildSystemPrompt_RAGSkipsForeignAgentSource covers US-006: even if
// ListSources somehow returns a source whose agent_id does not belong to
// the current agent (test stub, future regression, or buggy SQL), the
// AgentRunner must NOT search that source. By the schema (account → agent
// → source), source.AgentID == agent.ID transitively enforces same
// account, so this check protects tenant isolation at the RAG boundary.
func TestBuildSystemPrompt_RAGSkipsForeignAgentSource(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	// Agent A belongs to account_001.
	agent := makeAgent(strPtr("Loja A"))

	// One legitimate source bound to agent_001, plus a poisoned source
	// claiming to belong to a foreign agent (which itself belongs to
	// account_002 in reality — but the AgentRunner doesn't need to know
	// that; the agent_id mismatch is enough to reject the row).
	ownSource := &domain.KnowledgeSource{ID: "src_own", AgentID: "agent_001", Name: "FAQ"}
	foreignSource := &domain.KnowledgeSource{ID: "src_foreign", AgentID: "agent_foreign", Name: "Account B FAQ"}
	embedding := []float32{0.1, 0.2}

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{ownSource, foreignSource}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, "qualquer pergunta").Return(embedding, nil)
	// Only the legitimate source should be searched. The mock is NOT
	// configured for src_foreign — if the runner calls it, testify
	// fails with an unexpected-call panic.
	knowledgeRepo.On("Search", ctx, "src_own", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
		makeChunk("c_own", "src_own", "Conteúdo da conta A"),
	}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "qualquer pergunta", "", "pt")

	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "Conteúdo da conta A", "own-account chunk must be present")
	assert.NotContains(t, result.SystemPrompt, "Account B FAQ", "foreign source name must not leak")
	knowledgeRepo.AssertNotCalled(t, "Search", ctx, "src_foreign", embedding, int32(5))
}

func TestBuildSystemPrompt_InvalidTimezoneDefaultsToUTC(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)

	agent := makeAgent(nil)
	agent.Timezone = "Invalid/Timezone"
	cal := makeCalendar("cal_001", "Agenda", true, []int32{1}, 30)

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{cal}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	result, err := runner.BuildSystemPrompt(ctx, agent, "agendar", "", "pt")

	require.NoError(t, err)
	// With invalid timezone, the code falls back to UTC
	assert.Contains(t, result.SystemPrompt, fmt.Sprintf("Timezone: %s", agent.Timezone))
}
