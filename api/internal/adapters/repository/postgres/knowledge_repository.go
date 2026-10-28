package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
	"github.com/replider/api/internal/core/domain"
)

// --- KnowledgeRepository ---

type KnowledgeRepository struct {
	queries *Queries
}

func NewKnowledgeRepository(db DBTX) *KnowledgeRepository {
	return &KnowledgeRepository{queries: New(db)}
}

func (r *KnowledgeRepository) CreateSource(ctx context.Context, source *domain.KnowledgeSource) error {
	q := getQueries(ctx, r.queries)
	return q.CreateKnowledgeSource(ctx, CreateKnowledgeSourceParams{
		ID:          source.ID,
		AgentID:     source.AgentID,
		Name:        source.Name,
		Description: source.Description,
		CreatedAt:   pgtype.Timestamptz{Time: source.CreatedAt, Valid: true},
	})
}

func (r *KnowledgeRepository) GetSource(ctx context.Context, id string, agentID string) (*domain.KnowledgeSource, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetKnowledgeSource(ctx, GetKnowledgeSourceParams{
		ID:      id,
		AgentID: agentID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.KnowledgeSource{
		ID:          row.ID,
		AgentID:     row.AgentID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *KnowledgeRepository) GetSourceByID(ctx context.Context, id string) (*domain.KnowledgeSource, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetKnowledgeSourceByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.KnowledgeSource{
		ID:          row.ID,
		AgentID:     row.AgentID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *KnowledgeRepository) ListSources(ctx context.Context, agentID string) ([]*domain.KnowledgeSource, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListKnowledgeSources(ctx, agentID)
	if err != nil {
		return nil, err
	}

	sources := make([]*domain.KnowledgeSource, len(rows))
	for i, row := range rows {
		sources[i] = &domain.KnowledgeSource{
			ID:          row.ID,
			AgentID:     row.AgentID,
			Name:        row.Name,
			Description: row.Description,
			CreatedAt:   row.CreatedAt.Time,
		}
	}
	return sources, nil
}

func (r *KnowledgeRepository) ListSourcesByAgent(ctx context.Context, agentID string) ([]*domain.KnowledgeSource, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListKnowledgeSourcesByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	sources := make([]*domain.KnowledgeSource, len(rows))
	for i, row := range rows {
		sources[i] = &domain.KnowledgeSource{
			ID:          row.ID,
			AgentID:     row.AgentID,
			Name:        row.Name,
			Description: row.Description,
			CreatedAt:   row.CreatedAt.Time,
		}
	}
	return sources, nil
}

func (r *KnowledgeRepository) UpdateSource(ctx context.Context, source *domain.KnowledgeSource) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateKnowledgeSource(ctx, UpdateKnowledgeSourceParams{
		ID:          source.ID,
		Name:        source.Name,
		Description: source.Description,
		AgentID:     source.AgentID,
	})
}

func (r *KnowledgeRepository) DeleteSource(ctx context.Context, id string, agentID string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteKnowledgeSource(ctx, DeleteKnowledgeSourceParams{
		ID:      id,
		AgentID: agentID,
	})
}

func (r *KnowledgeRepository) CreateChunk(ctx context.Context, chunk *domain.KnowledgeChunk) error {
	q := getQueries(ctx, r.queries)
	metadata, _ := json.Marshal(chunk.Metadata)
	vec := pgvector.NewVector(chunk.Embedding)
	return q.CreateKnowledgeChunk(ctx, CreateKnowledgeChunkParams{
		ID:         chunk.ID,
		SourceID:   chunk.SourceID,
		Content:    chunk.Content,
		Embedding:  &vec,
		Metadata:   metadata,
		Provider:   chunk.Provider,
		SourceName: chunk.SourceName,
		CreatedAt:  pgtype.Timestamptz{Time: chunk.CreatedAt, Valid: true},
	})
}

func (r *KnowledgeRepository) Search(ctx context.Context, sourceID string, embedding []float32, limit int32) ([]*domain.KnowledgeChunk, error) {
	q := getQueries(ctx, r.queries)
	vec := pgvector.NewVector(embedding)
	rows, err := q.SearchKnowledge(ctx, SearchKnowledgeParams{
		SourceID:  sourceID,
		Embedding: &vec,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}

	chunks := make([]*domain.KnowledgeChunk, len(rows))
	for i, row := range rows {
		var metadata any
		_ = json.Unmarshal(row.Metadata, &metadata)
		chunks[i] = &domain.KnowledgeChunk{
			ID:         row.ID,
			SourceID:   row.SourceID,
			Content:    row.Content,
			Metadata:   metadata,
			Provider:   row.Provider,
			SourceName: row.SourceName,
			Similarity: row.Similarity,
		}
	}
	return chunks, nil
}

func (r *KnowledgeRepository) ListChunksBySource(ctx context.Context, sourceID string) ([]*domain.KnowledgeChunk, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListKnowledgeChunksBySource(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	chunks := make([]*domain.KnowledgeChunk, len(rows))
	for i, row := range rows {
		var metadata any
		_ = json.Unmarshal(row.Metadata, &metadata)
		chunks[i] = &domain.KnowledgeChunk{
			ID:         row.ID,
			SourceID:   row.SourceID,
			Content:    row.Content,
			Metadata:   metadata,
			Provider:   row.Provider,
			SourceName: row.SourceName,
		}
	}
	return chunks, nil
}

func (r *KnowledgeRepository) UpdateEmbedding(ctx context.Context, id string, embedding []float32, provider string) error {
	q := getQueries(ctx, r.queries)
	vec := pgvector.NewVector(embedding)
	return q.UpdateKnowledgeChunkEmbedding(ctx, UpdateKnowledgeChunkEmbeddingParams{
		ID:        id,
		Embedding: &vec,
		Provider:  &provider,
	})
}

func (r *KnowledgeRepository) GetChunkByID(ctx context.Context, id string) (*domain.KnowledgeChunk, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetKnowledgeChunkByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.KnowledgeChunk{
		ID:         row.ID,
		SourceID:   row.SourceID,
		Content:    row.Content,
		Provider:   row.Provider,
		SourceName: row.SourceName,
		Metadata:   row.Metadata,
		CreatedAt:  row.CreatedAt.Time,
	}, nil
}

func (r *KnowledgeRepository) DeleteChunk(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteKnowledgeChunk(ctx, id)
}
