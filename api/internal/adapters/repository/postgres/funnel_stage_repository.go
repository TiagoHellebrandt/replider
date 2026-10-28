package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// FunnelStageRepository persists Kanban columns and the lead↔stage move
// (US-M008).
type FunnelStageRepository struct {
	db DBTX
}

func NewFunnelStageRepository(db DBTX) *FunnelStageRepository {
	return &FunnelStageRepository{db: db}
}

var _ domain.FunnelStageRepository = (*FunnelStageRepository)(nil)

const stageColumns = `id, account_id, funnel_id, name, position, maps_to_status, is_terminal, created_at, updated_at`

func scanStage(row pgx.Row) (*domain.FunnelStage, error) {
	var s domain.FunnelStage
	var status string
	if err := row.Scan(&s.ID, &s.AccountID, &s.FunnelID, &s.Name, &s.Position,
		&status, &s.IsTerminal, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	s.MapsToStatus = domain.LeadStatus(status)
	return &s, nil
}

func (r *FunnelStageRepository) ListByFunnel(ctx context.Context, accountID, funnelID string) ([]*domain.FunnelStage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+stageColumns+` FROM funnel_stages
		 WHERE account_id = $1 AND funnel_id = $2
		 ORDER BY position ASC, created_at ASC`,
		accountID, funnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.FunnelStage{}
	for rows.Next() {
		s, err := scanStage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *FunnelStageRepository) GetByID(ctx context.Context, accountID, id string) (*domain.FunnelStage, error) {
	s, err := scanStage(r.db.QueryRow(ctx,
		`SELECT `+stageColumns+` FROM funnel_stages WHERE id = $1 AND account_id = $2`,
		id, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *FunnelStageRepository) Create(ctx context.Context, stage *domain.FunnelStage) (*domain.FunnelStage, error) {
	if err := stage.Validate(); err != nil {
		return nil, err
	}
	if stage.ID == "" {
		stage.ID = ulid.Make().String()
	}
	out, err := scanStage(r.db.QueryRow(ctx,
		`INSERT INTO funnel_stages (id, account_id, funnel_id, name, position, maps_to_status, is_terminal, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		 RETURNING `+stageColumns,
		stage.ID, stage.AccountID, stage.FunnelID, stage.Name, stage.Position,
		string(stage.MapsToStatus), stage.IsTerminal))
	if isUniqueViolation(err) {
		// Two columns with the same name make the board ambiguous to drag
		// onto — a UI mistake worth a clear error, not a 500.
		return nil, domain.ErrStageNameTaken
	}
	return out, err
}

func (r *FunnelStageRepository) Update(ctx context.Context, stage *domain.FunnelStage) (*domain.FunnelStage, error) {
	if err := stage.Validate(); err != nil {
		return nil, err
	}
	out, err := scanStage(r.db.QueryRow(ctx,
		`UPDATE funnel_stages
		 SET name = $3, position = $4, maps_to_status = $5, is_terminal = $6, updated_at = NOW()
		 WHERE id = $1 AND account_id = $2
		 RETURNING `+stageColumns,
		stage.ID, stage.AccountID, stage.Name, stage.Position,
		string(stage.MapsToStatus), stage.IsTerminal))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrStageNotFound
	}
	if isUniqueViolation(err) {
		return nil, domain.ErrStageNameTaken
	}
	return out, err
}

// Delete removes a column. Leads sitting in it are NOT deleted — the FK is
// ON DELETE SET NULL, so they fall back onto the column matching their
// canonical status. Losing a column must never lose deals.
func (r *FunnelStageRepository) Delete(ctx context.Context, accountID, id string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM funnel_stages WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

// EnsureDefaults seeds the standard board once.
//
// Idempotency here is not a nicety: a funnel whose columns the customer
// deleted or renamed must not have the defaults silently recreated on the
// next board load. The presence of ANY stage means the customer owns the
// board now.
func (r *FunnelStageRepository) EnsureDefaults(ctx context.Context, accountID, funnelID string) ([]*domain.FunnelStage, error) {
	existing, err := r.ListByFunnel(ctx, accountID, funnelID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing, nil
	}

	for i, def := range domain.DefaultFunnelStages() {
		stage := &domain.FunnelStage{
			ID:           ulid.Make().String(),
			AccountID:    accountID,
			FunnelID:     funnelID,
			Name:         def.Name,
			Position:     (i + 1) * domain.StagePositionStep,
			MapsToStatus: def.Status,
		}
		if _, err := r.Create(ctx, stage); err != nil {
			// A concurrent first-load may have seeded it already; re-read
			// rather than failing the board.
			if errors.Is(err, domain.ErrStageNameTaken) {
				return r.ListByFunnel(ctx, accountID, funnelID)
			}
			return nil, err
		}
	}
	return r.ListByFunnel(ctx, accountID, funnelID)
}

// MoveLead writes the board position and the derived canonical status in ONE
// statement.
//
// Two statements would leave a window where the card moved but the pipeline
// did not — the AI, the approval queue and analytics would all disagree with
// what the salesperson sees, which is exactly the failure this design exists
// to prevent.
//
// boardPos nil clears the manual rank (a plain column drop hands the card
// back to the automatic order). touch=false leaves updated_at alone: a
// same-column reorder is a view preference, and bumping the timestamp would
// reset the "parado há N dias" clock the ordering itself feeds on.
func (r *FunnelStageRepository) MoveLead(ctx context.Context, accountID, leadID, stageID string, status domain.LeadStatus, boardPos *float64, touch bool) (*domain.Lead, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE leads
		 SET stage_id  = $3,
		     status    = $4,
		     board_pos = $5,
		     updated_at = CASE WHEN $6::bool THEN NOW() ELSE leads.updated_at END
		 WHERE id = $1 AND account_id = $2
		 RETURNING id, account_id, funnel_id, contact_id, status, source, score,
		           collected_metadata, enrichment, approach_draft, buy_intent_at,
		           qualified_at, owner_user_id, visibility, stage_id, board_pos,
		           created_at, updated_at`,
		leadID, accountID, stageID, string(status), boardPos, touch)

	var l domain.Lead
	var statusOut, visibility string
	var collected, enrichment []byte
	if err := row.Scan(&l.ID, &l.AccountID, &l.FunnelID, &l.ContactID, &statusOut, &l.Source,
		&l.Score, &collected, &enrichment, &l.ApproachDraft, &l.BuyIntentAt, &l.QualifiedAt,
		&l.OwnerUserID, &visibility, &l.StageID, &l.BoardPos, &l.CreatedAt, &l.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLeadNotFound
		}
		return nil, err
	}
	l.Status = domain.LeadStatus(statusOut)
	l.Visibility = domain.ResourceVisibility(visibility)
	l.CollectedMetadata = collected
	l.Enrichment = enrichment
	return &l, nil
}

// BoardNeighborPositions gathers everything domain.ComputeBoardPos needs in
// one round-trip: the neighbors' manual positions (an empty id, an unknown
// id and a never-placed neighbor all resolve to NULL alike) and the target
// column's current maximum.
func (r *FunnelStageRepository) BoardNeighborPositions(ctx context.Context, accountID, stageID, beforeLeadID, afterLeadID string) (before, after, maxInStage *float64, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT
		   (SELECT board_pos FROM leads WHERE id = $3 AND account_id = $1),
		   (SELECT board_pos FROM leads WHERE id = $4 AND account_id = $1),
		   (SELECT MAX(board_pos) FROM leads WHERE account_id = $1 AND stage_id = $2)`,
		accountID, stageID, beforeLeadID, afterLeadID).
		Scan(&before, &after, &maxInStage)
	return before, after, maxInStage, err
}

// SeedBoardOrder gives every lead in the column a concrete position, in the
// exact order the board renders today: already-placed first (their relative
// order kept), then the automatic most-stalled-first (updated_at ASC, the
// app's comparator), id as the deterministic tiebreak on both sides.
func (r *FunnelStageRepository) SeedBoardOrder(ctx context.Context, accountID, stageID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE leads SET board_pos = ranked.rn * $3
		   FROM (
		     SELECT id, ROW_NUMBER() OVER (
		       ORDER BY board_pos ASC NULLS LAST, updated_at ASC NULLS LAST, id ASC
		     ) AS rn
		       FROM leads
		      WHERE account_id = $1 AND stage_id = $2
		   ) ranked
		  WHERE leads.id = ranked.id`,
		accountID, stageID, domain.BoardPositionStep)
	return err
}

// ResetBoardOrder clears one column's manual ordering ("ordenar por
// parados"). updated_at is deliberately untouched — see MoveLead.
func (r *FunnelStageRepository) ResetBoardOrder(ctx context.Context, accountID, funnelID, stageID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE leads SET board_pos = NULL
		 WHERE account_id = $1 AND funnel_id = $2 AND stage_id = $3 AND board_pos IS NOT NULL`,
		accountID, funnelID, stageID)
	return err
}
