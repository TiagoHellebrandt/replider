package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/replider/api/internal/core/domain"
)

type PromptExperimentRepository struct {
	db DBTX
}

func NewPromptExperimentRepository(db DBTX) *PromptExperimentRepository {
	return &PromptExperimentRepository{db: db}
}

func (r *PromptExperimentRepository) Create(ctx context.Context, e *domain.PromptExperiment) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO prompt_experiments (
		    id, account_id, agent_id, name, status,
		    variant_a_body, variant_b_body, variant_a_label, variant_b_label,
		    split_pct_b, created_at, updated_at
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())`,
		e.ID, e.AccountID, e.AgentID, e.Name, e.Status,
		e.VariantABody, e.VariantBBody, e.VariantALabel, e.VariantBLabel, e.SplitPctB,
	)
	return err
}

func (r *PromptExperimentRepository) GetByID(ctx context.Context, id string) (*domain.PromptExperiment, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, account_id, agent_id, name, status,
		        variant_a_body, variant_b_body, variant_a_label, variant_b_label,
		        split_pct_b, winner, started_at, stopped_at, created_at, updated_at
		 FROM prompt_experiments WHERE id = $1`, id)
	return scanExperiment(row)
}

func (r *PromptExperimentRepository) GetRunningByAgentID(ctx context.Context, agentID string) (*domain.PromptExperiment, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, account_id, agent_id, name, status,
		        variant_a_body, variant_b_body, variant_a_label, variant_b_label,
		        split_pct_b, winner, started_at, stopped_at, created_at, updated_at
		 FROM prompt_experiments WHERE agent_id = $1 AND status = 'running' LIMIT 1`, agentID)
	return scanExperiment(row)
}

func (r *PromptExperimentRepository) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.PromptExperiment, error) {
	return r.list(ctx, `WHERE account_id = $1`, accountID, limit, offset)
}

func (r *PromptExperimentRepository) ListByAgentID(ctx context.Context, agentID string, limit, offset int) ([]*domain.PromptExperiment, error) {
	return r.list(ctx, `WHERE agent_id = $1`, agentID, limit, offset)
}

func (r *PromptExperimentRepository) list(ctx context.Context, where, scope string, limit, offset int) ([]*domain.PromptExperiment, error) {
	q := `SELECT id, account_id, agent_id, name, status,
	             variant_a_body, variant_b_body, variant_a_label, variant_b_label,
	             split_pct_b, winner, started_at, stopped_at, created_at, updated_at
	      FROM prompt_experiments ` + where +
		` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, scope, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.PromptExperiment, 0)
	for rows.Next() {
		e, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *PromptExperimentRepository) UpdateStatus(ctx context.Context, id, status string, startedAt, stoppedAt *time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE prompt_experiments
		 SET status = $2,
		     started_at = COALESCE($3, started_at),
		     stopped_at = COALESCE($4, stopped_at),
		     updated_at = NOW()
		 WHERE id = $1`,
		id, status, startedAt, stoppedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			strings.Contains(pgErr.ConstraintName, "uniq_prompt_experiments_one_running_per_agent") {
			return domain.ErrExperimentAlreadyRunning
		}
	}
	return err
}

func (r *PromptExperimentRepository) SetWinner(ctx context.Context, id string, winner *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE prompt_experiments SET winner = $2, updated_at = NOW() WHERE id = $1`,
		id, winner,
	)
	return err
}

func (r *PromptExperimentRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM prompt_experiments WHERE id = $1`, id)
	return err
}

// MetricsForVariant performs the per-variant aggregation in a single
// query: counts of conversations, AI-resolved, handoff, CSAT averages,
// token totals, latency. NULL averages come back as nil pointers so
// the UI can show "—" instead of "0" when sample size is zero.
func (r *PromptExperimentRepository) MetricsForVariant(ctx context.Context, experimentID, variant string) (*domain.VariantMetrics, error) {
	row := r.db.QueryRow(ctx, `
		WITH ea AS (
		  SELECT conversation_id
		  FROM experiment_assignments
		  WHERE experiment_id = $1 AND variant = $2
		),
		conv AS (
		  SELECT c.id,
		         CASE WHEN c.resolved_at IS NOT NULL AND c.assigned_to IS NULL THEN 1 ELSE 0 END AS ai_resolved,
		         CASE WHEN c.assigned_to IS NOT NULL THEN 1 ELSE 0 END AS handed_off,
		         EXTRACT(EPOCH FROM (c.first_response_at - c.first_inbound_at)) AS first_response_secs
		  FROM conversations c
		  JOIN ea ON ea.conversation_id = c.id
		),
		tokens AS (
		  SELECT COALESCE(SUM(
		    COALESCE((m.metadata->>'tokens_in')::bigint, 0) +
		    COALESCE((m.metadata->>'tokens_out')::bigint, 0)
		  ), 0) AS total
		  FROM messages m
		  JOIN ea ON ea.conversation_id = m.conversation_id
		  WHERE m.direction = 'outbound' AND m.metadata ? 'experiment_id'
		    AND m.metadata->>'experiment_id' = $1
		    AND m.metadata->>'variant' = $2
		),
		csat AS (
		  SELECT AVG(rating)::float8 AS avg_rating, COUNT(*) AS responses
		  FROM csat_ratings r
		  JOIN ea ON ea.conversation_id = r.conversation_id
		  WHERE r.rating > 0
		),
		msgs AS (
		  SELECT AVG(cnt)::float8 AS avg_len
		  FROM (
		    SELECT COUNT(*) AS cnt FROM messages m JOIN ea ON ea.conversation_id = m.conversation_id GROUP BY m.conversation_id
		  ) s
		)
		SELECT
		  (SELECT COUNT(*) FROM ea),
		  (SELECT COALESCE(SUM(ai_resolved), 0) FROM conv),
		  (SELECT COALESCE(SUM(handed_off), 0) FROM conv),
		  (SELECT avg_rating FROM csat),
		  (SELECT responses FROM csat),
		  (SELECT total FROM tokens),
		  (SELECT AVG(first_response_secs)::float8 FROM conv WHERE first_response_secs IS NOT NULL),
		  (SELECT avg_len FROM msgs)
	`, experimentID, variant)

	out := &domain.VariantMetrics{Variant: variant}
	if err := row.Scan(
		&out.ConversationCount,
		&out.AIResolvedCount,
		&out.HandoffCount,
		&out.CSATAverage,
		&out.CSATResponses,
		&out.TokensTotal,
		&out.AvgFirstResponseSecs,
		&out.AvgConversationLen,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// SampleConversations returns up to bestLimit highest-rated and
// worstLimit lowest-rated conversations bucketed into the variant.
// Conversations with no messages are filtered so the bundle the
// suggester ships to the LLM stays useful.
func (r *PromptExperimentRepository) SampleConversations(
	ctx context.Context,
	experimentID, variant string,
	bestLimit, worstLimit int,
) ([]domain.ExperimentSample, []domain.ExperimentSample, error) {
	best, err := r.sampleByOrder(ctx, experimentID, variant, "DESC", bestLimit)
	if err != nil {
		return nil, nil, err
	}
	worst, err := r.sampleByOrder(ctx, experimentID, variant, "ASC", worstLimit)
	if err != nil {
		return nil, nil, err
	}
	return best, worst, nil
}

func (r *PromptExperimentRepository) sampleByOrder(
	ctx context.Context,
	experimentID, variant, direction string,
	limit int,
) ([]domain.ExperimentSample, error) {
	if limit <= 0 {
		return nil, nil
	}
	// Order: prefer rated conversations (NULLs LAST in DESC, NULLS FIRST
	// in ASC); resolved-without-handoff is a tiebreaker. Filter
	// conversations with no messages so the bundle the LLM sees never
	// includes empty stubs.
	q := `
		SELECT c.id,
		       r.rating,
		       (c.resolved_at IS NOT NULL AND c.assigned_to IS NULL) AS resolved
		FROM experiment_assignments ea
		JOIN conversations c ON c.id = ea.conversation_id
		LEFT JOIN csat_ratings r ON r.conversation_id = c.id AND r.rating > 0
		WHERE ea.experiment_id = $1 AND ea.variant = $2
		  AND EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id LIMIT 1)
		ORDER BY r.rating ` + direction + ` NULLS LAST, c.created_at DESC
		LIMIT $3`
	rows, err := r.db.Query(ctx, q, experimentID, variant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ExperimentSample, 0, limit)
	for rows.Next() {
		s := domain.ExperimentSample{}
		if err := rows.Scan(&s.ConversationID, &s.Rating, &s.Resolved); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanExperiment(row rowScanner) (*domain.PromptExperiment, error) {
	e := &domain.PromptExperiment{}
	if err := row.Scan(
		&e.ID, &e.AccountID, &e.AgentID, &e.Name, &e.Status,
		&e.VariantABody, &e.VariantBBody, &e.VariantALabel, &e.VariantBLabel,
		&e.SplitPctB, &e.Winner, &e.StartedAt, &e.StoppedAt, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return e, nil
}

var _ domain.PromptExperimentRepository = (*PromptExperimentRepository)(nil)
