package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// RadarBuyerLeadRepository answers one question the radar panel needs and
// nothing else: what, if anything, has this account already done about this
// buyer.
//
// It exists as its own narrow reader because the question spans funnels — a
// buyer is one CNPJ and a lead lives in one funnel, so `GetByFunnelAndContact`
// can only answer it if the caller already knows which funnel to look in, and
// the radar does not.
type RadarBuyerLeadRepository struct{ db *pgxpool.Pool }

func NewRadarBuyerLeadRepository(db *pgxpool.Pool) *RadarBuyerLeadRepository {
	return &RadarBuyerLeadRepository{db: db}
}

// LeadStateForContact returns the most recently touched lead this account
// holds for the contact: what was done about it, and which Kanban column it
// sits in. No lead returns a zero state and no error — "nothing here" is an
// answer, not a failure.
//
// The stage resolves the way the board resolves it (domain.ResolveStageForStatus):
// the column the card was dragged into, and when it was never dragged — which is
// every lead the radar and the nightly hunt create — the column that column-maps
// onto its canonical status. Reading `stage_id` alone would report "no stage" for
// the entire robot-built funnel, which is most of it.
func (r *RadarBuyerLeadRepository) LeadStateForContact(
	ctx context.Context, accountID, contactID string,
) (domain.RadarLeadState, error) {
	const q = `
		SELECT l.id, l.status,
		       (l.approach_draft IS NOT NULL AND l.approach_draft <> ''),
		       COALESCE(placed.name, mapped.name, ''),
		       l.funnel_id
		FROM leads l
		LEFT JOIN funnel_stages placed ON placed.id = l.stage_id
		LEFT JOIN LATERAL (
			SELECT fs.name FROM funnel_stages fs
			WHERE fs.funnel_id = l.funnel_id AND fs.maps_to_status = l.status
			ORDER BY fs.position
			LIMIT 1
		) mapped ON TRUE
		WHERE l.account_id = $1 AND l.contact_id = $2
		ORDER BY l.updated_at DESC
		LIMIT 1`
	var out domain.RadarLeadState
	var status string
	err := r.db.QueryRow(ctx, q, accountID, contactID).
		Scan(&out.LeadID, &status, &out.Drafted, &out.Stage, &out.FunnelID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RadarLeadState{}, nil
	}
	if err != nil {
		return domain.RadarLeadState{}, err
	}
	out.Status = domain.LeadStatus(status)
	return out, nil
}
