package domain

import (
	"context"
	"strings"
	"time"
)

// Kanban stages (US-M008).
//
// A lead's canonical `status` is what the PIPELINE reasons about: the AI
// advances it, the approval queue gates on it, analytics group by it. A
// board column is what the CUSTOMER thinks in — "Diagnóstico", "Proposta
// enviada", "Aguardando comodato". Forcing them onto our enum makes the
// board useless to them; letting them rename the enum breaks everything
// downstream.
//
// Stages resolve that: a column is a presentation-layer object that MAPS
// ONTO a canonical status. Moving a card writes both.

// FunnelStage is one board column.
type FunnelStage struct {
	ID           string     `json:"id"`
	AccountID    string     `json:"account_id"`
	FunnelID     string     `json:"funnel_id"`
	Name         string     `json:"name"`
	Position     int        `json:"position"`
	MapsToStatus LeadStatus `json:"maps_to_status"`
	IsTerminal   bool       `json:"is_terminal"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Validate normalizes and rejects a stage that would break the board.
func (s *FunnelStage) Validate() error {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return &AppError{Code: "VALIDATION_ERROR", Message: "stage name is required"}
	}
	if len([]rune(s.Name)) > 40 {
		// Longer names do not fit a column header; truncating in the UI
		// would hide which column you are dragging onto.
		return &AppError{Code: "VALIDATION_ERROR", Message: "stage name must be at most 40 characters"}
	}
	if !s.MapsToStatus.IsValid() {
		return ErrInvalidLeadStatus
	}
	s.IsTerminal = s.MapsToStatus.IsTerminal()
	return nil
}

// IsTerminal reports whether a status ends the pipeline. Terminal columns
// are rendered differently and excluded from "stuck lead" alerts — nagging
// someone about a lead they already won is how a feature gets muted.
func (s LeadStatus) IsTerminal() bool {
	return s == LeadStatusWon || s == LeadStatusLost
}

// DefaultFunnelStages is the board a funnel starts with: the canonical
// pipeline, named in the customer's language, minus the states that are
// MACHINE states rather than places a human parks a deal.
//
// `enriching` is deliberately absent as a column — it lasts seconds and
// belongs to the machine, not the board. A lead in it still resolves onto the
// column of the status it is transitioning between, so no card disappears.
//
// `pending_approval` used to be absent for the same stated reason, and the
// reason was wrong. It does not last minutes: on 2026-09-10 two accounts held
// 89 leads there, the oldest for forty days, and it was the single biggest
// bucket in the whole pipeline — the company's actual bottleneck, folded
// invisibly into "Pesquisados" and "Prospectados". A board whose busiest state
// has no column cannot show a team where the work is stuck.
func DefaultFunnelStages() []struct {
	Name   string
	Status LeadStatus
} {
	return []struct {
		Name   string
		Status LeadStatus
	}{
		{"Novos", LeadStatusNew},
		{"Prospectados", LeadStatusProspected},
		{"Pesquisados", LeadStatusEnriched},
		{"Esperando você", LeadStatusPendingApproval},
		{"Abordados", LeadStatusContacted},
		{"Responderam", LeadStatusResponded},
		{"Qualificados", LeadStatusQualified},
		{"Ganhos", LeadStatusWon},
		{"Perdidos", LeadStatusLost},
	}
}

// StagePositionStep is the gap left between stages so a column can be
// inserted between two others without renumbering the whole board.
const StagePositionStep = 100

// ResolveStageForStatus picks the column a stage-less lead belongs to.
//
// Every lead created before US-M008 — and every lead created by a path that
// does not know about stages (inbound, prospecting) — has no stage_id. The
// board must still show them, so they fall onto the first column matching
// their canonical status. Transient states (enriching)
// TransientStatusColumn maps a status with no column of its own onto the
// stable one a human would look for it in.
//
// Exported and shared because TWO screens project these, and until
// 2026-09-09 only one of them did. The board resolved pending_approval onto
// "Pesquisados"; the home screen's mini funnel read the raw status and found
// nothing there. Same account, same second: the board said 60 of 60 leads,
// the home funnel said 19, and 41 written offers had no bar at all.
//
// `pending_approval` stays listed even though the board now has a column for
// it, and the two uses do not collide. ResolveStageForStatus matches the exact
// status FIRST, so a board with the column puts the card there; this fold is
// the fallback for a funnel whose columns were customised without one, and the
// projection the compressed HOME ladder needs — six rungs cannot carry every
// status, and a count with no rung is a count nobody sees.
var TransientStatusColumn = map[LeadStatus]LeadStatus{
	LeadStatusEnriching:       LeadStatusProspected,
	LeadStatusPendingApproval: LeadStatusEnriched,
}

// ProjectStatusCounts folds transient statuses into the column they render
// in, so any consumer counting by column agrees with the board.
func ProjectStatusCounts(byStatus map[LeadStatus]int64) map[LeadStatus]int64 {
	out := make(map[LeadStatus]int64, len(byStatus))
	for status, n := range byStatus {
		if alt, ok := TransientStatusColumn[status]; ok {
			out[alt] += n
			continue
		}
		out[status] += n
	}
	return out
}

// have no column of their own and resolve to their nearest stable one.
func ResolveStageForStatus(stages []*FunnelStage, status LeadStatus) *FunnelStage {
	for _, st := range stages {
		if st.MapsToStatus == status {
			return st
		}
	}
	if alt, ok := TransientStatusColumn[status]; ok {
		for _, st := range stages {
			if st.MapsToStatus == alt {
				return st
			}
		}
	}
	if len(stages) > 0 {
		return stages[0]
	}
	return nil
}

// FunnelStageRepository persists board columns.
type FunnelStageRepository interface {
	ListByFunnel(ctx context.Context, accountID, funnelID string) ([]*FunnelStage, error)
	GetByID(ctx context.Context, accountID, id string) (*FunnelStage, error)
	Create(ctx context.Context, stage *FunnelStage) (*FunnelStage, error)
	Update(ctx context.Context, stage *FunnelStage) (*FunnelStage, error)
	Delete(ctx context.Context, accountID, id string) error
	// EnsureDefaults seeds the default board for a funnel that has none.
	// Idempotent: a funnel whose columns the customer already edited (or
	// deleted) must not have them silently recreated on the next read.
	EnsureDefaults(ctx context.Context, accountID, funnelID string) ([]*FunnelStage, error)
	// MoveLead sets a lead's stage, its derived canonical status AND its
	// manual board position in one statement — a card that moved on the
	// board but not in the pipeline is the bug this whole design exists to
	// avoid. boardPos nil clears the manual rank (the card goes back to the
	// automatic order); touch=false keeps updated_at, so a same-column
	// reorder does not reset the "parado há N dias" clock.
	MoveLead(ctx context.Context, accountID, leadID, stageID string, status LeadStatus, boardPos *float64, touch bool) (*Lead, error)
	// BoardNeighborPositions resolves the board_pos of the two neighbors a
	// card was dropped between (either id may be empty) plus the MAX
	// board_pos of the target column — the inputs ComputeBoardPos needs. A
	// neighbor without a manual position resolves to nil, exactly like an
	// absent one.
	BoardNeighborPositions(ctx context.Context, accountID, stageID, beforeLeadID, afterLeadID string) (before, after, maxInStage *float64, err error)
	// SeedBoardOrder materialises a column's CURRENT visual order into
	// board_pos (row_number × step over: placed first by position, then the
	// automatic most-stalled-first, id as the tiebreak). Called before a
	// midpoint is computed against a neighbor that has no position yet —
	// without it the first manual drop in an automatic column became the
	// only placed card and jumped to the top.
	SeedBoardOrder(ctx context.Context, accountID, stageID string) error
	// ResetBoardOrder clears every manual position in one column, restoring
	// the automatic most-stalled-first order. Deliberately does NOT touch
	// updated_at: resetting a view preference is not lead activity.
	ResetBoardOrder(ctx context.Context, accountID, funnelID, stageID string) error
}

// BoardPositionStep is the gap a card gets when it is placed at an edge of
// a column's placed set, so future drops between it and the edge keep
// halving comfortably before float precision matters.
const BoardPositionStep = 1024

// ComputeBoardPos turns a drop slot into a fractional rank.
//
// The inputs are the manual positions of the card ABOVE (before) and BELOW
// (after) the slot, with a never-placed neighbor already resolved to nil.
// Because placed cards always render before the never-placed ones, the
// cases collapse cleanly:
//
//   - between two placed cards → their midpoint;
//   - below the last placed card (after is nil) → before + step;
//   - above the first placed card (before is nil) → after − step;
//   - among only never-placed cards → bottom of the placed set
//     (maxInStage + step), or 0 when the column has no placed card yet.
//
// The last case is deliberate top/bottom semantics: a drop relative to a
// never-placed neighbor cannot be expressed more precisely than "after
// everything already placed", and pretending otherwise would invent an
// order the server cannot keep.
func ComputeBoardPos(before, after, maxInStage *float64) float64 {
	switch {
	case before != nil && after != nil:
		return (*before + *after) / 2
	case before != nil:
		return *before + BoardPositionStep
	case after != nil:
		return *after - BoardPositionStep
	case maxInStage != nil:
		return *maxInStage + BoardPositionStep
	default:
		return 0
	}
}

var (
	ErrStageNotFound    = &AppError{Code: "FUNNEL_STAGE_NOT_FOUND", Message: "stage not found"}
	ErrStageWrongFunnel = &AppError{
		Code:    "FUNNEL_STAGE_WRONG_FUNNEL",
		Message: "a etapa pertence a outro funil",
	}
	ErrStageNameTaken = &AppError{
		Code:    "FUNNEL_STAGE_NAME_TAKEN",
		Message: "já existe uma etapa com esse nome neste funil",
	}
	// ErrStageBackward: a lead the pipeline already reached (contacted or
	// later) cannot be dragged back to a "new"/"prospected" column — that
	// used to make it draftable again and queue a second cold approach to
	// somebody already in a conversation (analysis 2026-09-01).
	ErrStageBackward = &AppError{
		Code:    "FUNNEL_STAGE_BACKWARD",
		Message: "um lead já contatado não volta para o começo do funil; marque como perdido ou crie outro lead",
	}
)

// IsBackwardMove reports whether moving a lead in `from` to a column that
// means `to` would rewind the pipeline past the first contact.
func IsBackwardMove(from, to LeadStatus) bool {
	reached := map[LeadStatus]bool{
		LeadStatusContacted: true, LeadStatusResponded: true, LeadStatusQualified: true,
		LeadStatusWon: true, LeadStatusLost: true,
	}
	early := map[LeadStatus]bool{
		LeadStatusNew: true, LeadStatusProspected: true, LeadStatusEnriching: true,
		LeadStatusEnriched: true, LeadStatusPendingApproval: true,
	}
	return reached[from] && early[to]
}
