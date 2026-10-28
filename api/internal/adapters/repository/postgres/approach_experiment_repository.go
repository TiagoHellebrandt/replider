package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own (US-F216).
type ApproachExperimentRepository struct{ db DBTX }

func NewApproachExperimentRepository(db DBTX) *ApproachExperimentRepository {
	return &ApproachExperimentRepository{db: db}
}

const approachExperimentCols = `id, account_id, funnel_id, name, status,
	variant_a_suffix, variant_b_suffix, variant_a_label, variant_b_label,
	variant_a_model, variant_b_model, split_pct_b, winner, created_at, updated_at`

func scanApproachExperiment(row pgx.Row) (*domain.ApproachExperiment, error) {
	e := &domain.ApproachExperiment{}
	err := row.Scan(&e.ID, &e.AccountID, &e.FunnelID, &e.Name, &e.Status,
		&e.VariantASuffix, &e.VariantBSuffix, &e.VariantALabel, &e.VariantBLabel,
		&e.VariantAModel, &e.VariantBModel, &e.SplitPctB, &e.Winner,
		&e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return e, nil
}

func (r *ApproachExperimentRepository) GetRunningByFunnel(ctx context.Context, funnelID string) (*domain.ApproachExperiment, error) {
	return scanApproachExperiment(r.db.QueryRow(ctx,
		`SELECT `+approachExperimentCols+` FROM approach_experiments
		 WHERE funnel_id = $1 AND status = 'running'`, funnelID))
}

func (r *ApproachExperimentRepository) GetByID(ctx context.Context, id string) (*domain.ApproachExperiment, error) {
	return scanApproachExperiment(r.db.QueryRow(ctx,
		`SELECT `+approachExperimentCols+` FROM approach_experiments WHERE id = $1`, id))
}

func (r *ApproachExperimentRepository) Create(ctx context.Context, e *domain.ApproachExperiment) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO approach_experiments
		 (id, account_id, funnel_id, name, status, variant_a_suffix, variant_b_suffix,
		  variant_a_label, variant_b_label, variant_a_model, variant_b_model, split_pct_b)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		e.ID, e.AccountID, e.FunnelID, e.Name, e.Status,
		e.VariantASuffix, e.VariantBSuffix, e.VariantALabel, e.VariantBLabel,
		e.VariantAModel, e.VariantBModel, e.SplitPctB)
	return err
}

func (r *ApproachExperimentRepository) Stop(ctx context.Context, id, winner string) error {
	var w any
	if winner == "A" || winner == "B" {
		w = winner
	}
	_, err := r.db.Exec(ctx,
		`UPDATE approach_experiments
		    SET status = 'stopped', winner = $2, updated_at = NOW()
		  WHERE id = $1 AND status = 'running'`, id, w)
	return err
}

func (r *ApproachExperimentRepository) ListByAccount(ctx context.Context, accountID string, limit int32) ([]*domain.ApproachExperiment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+approachExperimentCols+` FROM approach_experiments
		  WHERE account_id = $1 ORDER BY created_at DESC LIMIT $2`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.ApproachExperiment
	for rows.Next() {
		e, err := scanApproachExperiment(rows)
		if err != nil {
			return nil, err
		}
		if e != nil {
			out = append(out, e)
		}
	}
	return out, rows.Err()
}

// UpsertResult replaces a lead's row rather than adding one: re-drafting the
// same lead must not inflate an arm with the same lead several times. The
// outcome resets to pending because the new draft has not been judged.
func (r *ApproachExperimentRepository) UpsertResult(ctx context.Context, res *domain.ApproachVariantResult) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO approach_variant_results
		   (id, experiment_id, lead_id, variant, drafted_text, outcome, risk_count)
		 VALUES ($1,$2,$3,$4,$5,'pending',$6)
		 ON CONFLICT (experiment_id, lead_id) DO UPDATE
		    SET drafted_text = EXCLUDED.drafted_text,
		        variant      = EXCLUDED.variant,
		        risk_count   = EXCLUDED.risk_count,
		        outcome      = 'pending',
		        edit_ratio   = 0,
		        decided_at   = NULL`,
		res.ID, res.ExperimentID, res.LeadID, res.Variant, res.DraftedText, res.RiskCount)
	return err
}

func (r *ApproachExperimentRepository) GetResult(ctx context.Context, experimentID, leadID string) (*domain.ApproachVariantResult, error) {
	res := &domain.ApproachVariantResult{}
	err := r.db.QueryRow(ctx,
		`SELECT id, experiment_id, lead_id, variant, drafted_text, outcome,
		        edit_ratio, risk_count, decided_at, created_at
		   FROM approach_variant_results
		  WHERE experiment_id = $1 AND lead_id = $2`, experimentID, leadID).
		Scan(&res.ID, &res.ExperimentID, &res.LeadID, &res.Variant, &res.DraftedText,
			&res.Outcome, &res.EditRatio, &res.RiskCount, &res.DecidedAt, &res.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return res, nil
}

// SetOutcome closes a result, and only once: the guard on outcome='pending'
// means a second approval (a retry, a double click) cannot overwrite the
// first verdict and quietly change the tally.
func (r *ApproachExperimentRepository) SetOutcome(ctx context.Context, experimentID, leadID string, outcome domain.ApproachOutcome, editRatio float32) error {
	_, err := r.db.Exec(ctx,
		`UPDATE approach_variant_results
		    SET outcome = $3, edit_ratio = $4, decided_at = NOW()
		  WHERE experiment_id = $1 AND lead_id = $2 AND outcome = 'pending'`,
		experimentID, leadID, string(outcome), editRatio)
	return err
}

// Stats tallies both arms. Rates are over DECIDED drafts: a queue nobody has
// worked through yet would otherwise read as a losing arm.
func (r *ApproachExperimentRepository) Stats(ctx context.Context, experimentID string) ([]domain.ApproachArmStats, error) {
	rows, err := r.db.Query(ctx,
		`SELECT variant,
		        COUNT(*)                                                  AS drafted,
		        COUNT(*) FILTER (WHERE outcome <> 'pending')              AS decided,
		        COUNT(*) FILTER (WHERE outcome = 'approved_unedited')     AS unedited,
		        COUNT(*) FILTER (WHERE outcome = 'approved_edited')       AS edited,
		        COUNT(*) FILTER (WHERE outcome = 'discarded')             AS discarded,
		        COALESCE(AVG(edit_ratio) FILTER (WHERE outcome LIKE 'approved%'), 0),
		        COALESCE(AVG(risk_count), 0)
		   FROM approach_variant_results
		  WHERE experiment_id = $1
		  GROUP BY variant`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ApproachArmStats
	for rows.Next() {
		var s domain.ApproachArmStats
		if err := rows.Scan(&s.Variant, &s.Drafted, &s.Decided, &s.Unedited,
			&s.Edited, &s.Discarded, &s.AvgEditRatio, &s.AvgRisks); err != nil {
			return nil, err
		}
		if s.Decided > 0 {
			s.UneditedRate = float64(s.Unedited) / float64(s.Decided)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
