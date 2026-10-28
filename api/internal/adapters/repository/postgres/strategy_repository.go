package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type StrategyRepository struct{ db DBTX }

func NewStrategyRepository(db DBTX) *StrategyRepository {
	return &StrategyRepository{db: db}
}

const strategyCols = `id, account_id, name, version, status, icp, channel_mix,
	sequence_id, tone, arguments, objections, triggers, author_user_id,
	author_name, created_at, updated_at`

func scanStrategy(row pgx.Row) (*domain.Strategy, error) {
	s := &domain.Strategy{}
	var status string
	var sequenceID, authorID *string
	var objections []byte
	err := row.Scan(&s.ID, &s.AccountID, &s.Name, &s.Version, &status, &s.ICP,
		&s.ChannelMix, &sequenceID, &s.Tone, &s.Arguments, &objections,
		&s.Triggers, &authorID, &s.AuthorName, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.Status = domain.StrategyStatus(status)
	if sequenceID != nil {
		s.SequenceID = *sequenceID
	}
	if authorID != nil {
		s.AuthorUserID = *authorID
	}
	if len(objections) > 0 {
		_ = json.Unmarshal(objections, &s.Objections)
	}
	return s, nil
}

func (r *StrategyRepository) List(ctx context.Context, accountID string) ([]*domain.Strategy, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+strategyCols+` FROM strategies
		  WHERE account_id = $1
		  -- Promoted first, retired last: the list should open on what the
		  -- team is actually using.
		  ORDER BY CASE status
		             WHEN 'promoted' THEN 0
		             WHEN 'testing'  THEN 1
		             WHEN 'draft'    THEN 2
		             ELSE 3
		           END, updated_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Strategy
	for rows.Next() {
		s, err := scanStrategy(rows)
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

func (r *StrategyRepository) Get(ctx context.Context, id, accountID string) (*domain.Strategy, error) {
	return scanStrategy(r.db.QueryRow(ctx,
		`SELECT `+strategyCols+` FROM strategies WHERE id = $1 AND account_id = $2`,
		id, accountID))
}

// Upsert bumps the version only when the CONTENT changed.
//
// Renaming a strategy, promoting it, or linking a cadence must not orphan the
// numbers it earned — the same rule as email templates, for the same reason.
func (r *StrategyRepository) Upsert(ctx context.Context, s *domain.Strategy) error {
	objections, err := json.Marshal(s.Objections)
	if err != nil {
		return err
	}
	args, triggers := s.Arguments, s.Triggers
	if args == nil {
		args = []string{}
	}
	if triggers == nil {
		triggers = []string{}
	}
	var sequenceID, authorID *string
	if s.SequenceID != "" {
		sequenceID = &s.SequenceID
	}
	if s.AuthorUserID != "" {
		authorID = &s.AuthorUserID
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO strategies (id, account_id, name, version, status, icp,
		    channel_mix, sequence_id, tone, arguments, objections, triggers,
		    author_user_id, author_name)
		 VALUES ($1,$2,$3,1,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 ON CONFLICT (id) DO UPDATE
		    SET name           = EXCLUDED.name,
		        status         = EXCLUDED.status,
		        icp            = EXCLUDED.icp,
		        channel_mix    = EXCLUDED.channel_mix,
		        sequence_id    = EXCLUDED.sequence_id,
		        tone           = EXCLUDED.tone,
		        arguments      = EXCLUDED.arguments,
		        objections     = EXCLUDED.objections,
		        triggers       = EXCLUDED.triggers,
		        version = CASE
		          WHEN strategies.icp         IS DISTINCT FROM EXCLUDED.icp
		            OR strategies.tone        IS DISTINCT FROM EXCLUDED.tone
		            OR strategies.channel_mix IS DISTINCT FROM EXCLUDED.channel_mix
		            OR strategies.arguments   IS DISTINCT FROM EXCLUDED.arguments
		            OR strategies.objections  IS DISTINCT FROM EXCLUDED.objections
		            OR strategies.triggers    IS DISTINCT FROM EXCLUDED.triggers
		          THEN strategies.version + 1
		          ELSE strategies.version
		        END,
		        updated_at = NOW()
		  WHERE strategies.account_id = EXCLUDED.account_id`,
		s.ID, s.AccountID, s.Name, string(s.Status), s.ICP, s.ChannelMix,
		sequenceID, s.Tone, args, objections, triggers, authorID, s.AuthorName)
	return err
}

func (r *StrategyRepository) SetStatus(ctx context.Context, id, accountID string, status domain.StrategyStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE strategies SET status = $3, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`, id, accountID, string(status))
	return err
}

// Stats tallies what each strategy VERSION produced.
//
// Grouped by (strategy, version) rather than by strategy: an approach sent
// under version 2 must not be counted toward version 3's numbers, which is
// the whole reason versions exist.
func (r *StrategyRepository) Stats(ctx context.Context, accountID string) ([]domain.StrategyStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT strategy_id,
		        COALESCE(strategy_version, 1) AS version,
		        COUNT(*) AS approaches,
		        COUNT(*) FILTER (
		          WHERE status IN ('responded','qualified','won')
		        ) AS replied,
		        COUNT(*) FILTER (WHERE status = 'won') AS won
		   FROM leads
		  WHERE account_id = $1 AND strategy_id IS NOT NULL
		  GROUP BY strategy_id, COALESCE(strategy_version, 1)
		  ORDER BY strategy_id, version`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.StrategyStat
	for rows.Next() {
		var s domain.StrategyStat
		if err := rows.Scan(&s.StrategyID, &s.Version, &s.Approaches,
			&s.Replied, &s.Won); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// AttributeLead stamps which strategy version reached a lead.
//
// Write-once: the attribution is what the lead was worked WITH, and letting a
// later save overwrite it would let a promoted strategy quietly inherit the
// results of the one it replaced.
func (r *StrategyRepository) AttributeLead(ctx context.Context, leadID, strategyID string, version int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE leads SET strategy_id = $2, strategy_version = $3
		  WHERE id = $1 AND strategy_id IS NULL`, leadID, strategyID, version)
	return err
}

var _ domain.StrategyRepository = (*StrategyRepository)(nil)
