package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// EmbeddingCacheRepository persists query embeddings keyed by
// (text_hash, model) so the RAG hot path can skip the provider call for
// repeated inputs (US-022). It is hand-written (not sqlc) because the
// embedding column is stored untyped (`vector`, no fixed dimension) to
// hold embeddings from providers with different sizes.
type EmbeddingCacheRepository struct {
	db DBTX
}

func NewEmbeddingCacheRepository(db DBTX) *EmbeddingCacheRepository {
	return &EmbeddingCacheRepository{db: db}
}

// embeddingCacheTTL bounds how long a cached embedding is served before
// it's treated as a miss. 30 days per US-022 acceptance criteria.
const embeddingCacheTTL = "30 days"

// Get returns the cached embedding for (textHash, model), or (nil, nil)
// when there is no live (non-expired) row.
func (r *EmbeddingCacheRepository) Get(ctx context.Context, textHash, model string) ([]float32, error) {
	var vec pgvector.Vector
	err := r.db.QueryRow(ctx,
		`SELECT embedding FROM embedding_cache
		 WHERE text_hash = $1 AND model = $2
		   AND created_at > now() - $3::interval`,
		textHash, model, embeddingCacheTTL,
	).Scan(&vec)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return vec.Slice(), nil
}

// Put stores (or refreshes) the embedding for (textHash, model) and
// opportunistically prunes rows past the TTL so the table doesn't grow
// unbounded without a cron job.
func (r *EmbeddingCacheRepository) Put(ctx context.Context, textHash, model string, embedding []float32) error {
	vec := pgvector.NewVector(embedding)
	_, err := r.db.Exec(ctx,
		`INSERT INTO embedding_cache (text_hash, model, embedding, created_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (text_hash, model)
		 DO UPDATE SET embedding = EXCLUDED.embedding, created_at = now()`,
		textHash, model, vec,
	)
	if err != nil {
		return err
	}
	// Best-effort lazy prune; failure here must not fail the write path.
	_, _ = r.db.Exec(ctx,
		`DELETE FROM embedding_cache WHERE created_at < now() - $1::interval`,
		embeddingCacheTTL,
	)
	return nil
}
