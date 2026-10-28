package domain

import (
	"context"
	"encoding/json"
	"time"
)

// LeadStatus tracks a lead through the outbound sales pipeline, from first
// discovery (new) through enrichment, human approval, contact, and final
// qualification outcome (won/lost).
type LeadStatus string

const (
	// LeadStatusNew is a freshly created lead, not yet processed.
	LeadStatusNew LeadStatus = "new"
	// LeadStatusProspected was surfaced by a prospecting search (US-022).
	LeadStatusProspected LeadStatus = "prospected"
	// LeadStatusEnriching is being enriched with decision-maker/company data.
	LeadStatusEnriching LeadStatus = "enriching"
	// LeadStatusEnriched has enrichment data attached and is ready to approach.
	LeadStatusEnriched LeadStatus = "enriched"
	// LeadStatusPendingApproval is waiting in the human-in-the-loop queue.
	LeadStatusPendingApproval LeadStatus = "pending_approval"
	// LeadStatusContacted had its first outbound approach sent.
	LeadStatusContacted LeadStatus = "contacted"
	// LeadStatusResponded replied to the outbound approach.
	LeadStatusResponded LeadStatus = "responded"
	// LeadStatusQualified met the funnel's required-metadata gate.
	LeadStatusQualified LeadStatus = "qualified"
	// LeadStatusWon converted into a sale.
	LeadStatusWon LeadStatus = "won"
	// LeadStatusLost did not convert.
	LeadStatusLost LeadStatus = "lost"
)

// IsValid reports whether s is one of the allowed lead statuses.
func (s LeadStatus) IsValid() bool {
	switch s {
	case LeadStatusNew, LeadStatusProspected, LeadStatusEnriching,
		LeadStatusEnriched, LeadStatusPendingApproval, LeadStatusContacted,
		LeadStatusResponded, LeadStatusQualified, LeadStatusWon, LeadStatusLost:
		return true
	default:
		return false
	}
}

// Lead ties a contact to a sales funnel and accumulates the metadata the AI
// invisibly collected, optional enrichment data, a drafted outbound approach,
// and the lead's pipeline status (US-003).
type Lead struct {
	ID                string          `json:"id"`
	AccountID         string          `json:"account_id"`
	FunnelID          string          `json:"funnel_id"`
	ContactID         string          `json:"contact_id"`
	Status            LeadStatus      `json:"status"`
	Source            string          `json:"source"`
	Score             int             `json:"score"`
	CollectedMetadata json.RawMessage `json:"collected_metadata"`
	Enrichment        json.RawMessage `json:"enrichment"`
	// MatchedProducts is which of the account's products this lead's offer
	// is about, and why: [{product_id, name, reason}]. Chosen by the offer
	// matcher against the catalogue; the draft, the card and the approval
	// screen read the same pick. Empty until drafted.
	MatchedProducts json.RawMessage `json:"matched_products"`
	// AnticipatedObjections is what the matcher call expects THIS customer
	// to push back with, already answered: [{label, counter}]. Shown on the
	// approval card and injected into the draft so the text can preempt it.
	AnticipatedObjections json.RawMessage `json:"anticipated_objections"`
	ApproachDraft         string          `json:"approach_draft"`
	BuyIntentAt           *time.Time      `json:"buy_intent_at,omitempty"`
	QualifiedAt           *time.Time      `json:"qualified_at,omitempty"`
	// OwnerUserID is the salesperson responsible for this lead (US-F009).
	// Empty/nil = unassigned, which stays visible to the whole team — a
	// freshly prospected lead belongs to nobody yet.
	OwnerUserID *string `json:"owner_user_id,omitempty"`
	// Visibility governs whether teammates can see an OWNED lead. Mutation
	// is always owner-or-manager regardless of this value.
	Visibility ResourceVisibility `json:"visibility"`
	// StageID is the Kanban column the card sits in (US-M008). Nil for
	// leads created before the board existed, or by paths that know nothing
	// about it (inbound, prospecting) — those resolve onto the column
	// matching their canonical status.
	StageID *string `json:"stage_id,omitempty"`
	// BoardPos is the manually chosen rank inside the column (fractional
	// ranking, see ComputeBoardPos). Nil = never manually placed: the card
	// keeps the automatic most-stalled-first order, AFTER every placed one.
	// Written only by a drag (FunnelStageRepository.MoveLead) and cleared by
	// the column's reset-order action.
	BoardPos *float64 `json:"board_pos,omitempty"`
	// ContactLabel is a DISPLAY-ONLY join (never persisted): the human name,
	// phone or email behind ContactID. Every outbound approval shows the
	// operator who the message will reach, and a ULID answers that question
	// with nothing.
	ContactLabel string `json:"contact_label,omitempty"`
	// ContactAddress is the same kind of display-only join, but always the
	// ADDRESS the message would go to — never the name. The approval step
	// needs it: a name cannot catch a wrong number.
	ContactAddress string    `json:"contact_address,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ErrLeadMonthlyLimitReached is the SDR tier's monthly new-lead ceiling
// (US-M013). Distinct from the per-run cap: one says "this search stops
// here", the other says "this month is spent".
var ErrLeadMonthlyLimitReached = &AppError{
	Code:    "PLAN_LIMIT_EXCEEDED_LEADS",
	Message: "você atingiu o limite de leads novos do seu plano neste mês",
}

// ApprovalQueueFilter narrows the cross-funnel approval queue.
//
// It exists as a struct because every one of these has to reach the SQL. A
// filter that runs in Go over an already-paginated page drops every matching
// row past the limit, so the operator picks a funnel with 40 drafts in it and
// is shown the 6 that happened to land on page one.
type ApprovalQueueFilter struct {
	AccountID string
	// OwnerUserID scopes the read to a vendedor: their own leads plus the
	// unassigned and team-visible ones. Empty means the whole account, which
	// is what a gestor sees.
	OwnerUserID string
	Status      LeadStatus
	// FunnelID narrows to one funnel. Empty means every funnel — the default,
	// because a per-funnel queue lets a funnel nobody opened today rot.
	FunnelID string
	// NewestFirst flips the order. Default (false) is oldest first.
	NewestFirst bool
}

// QueueFunnelCount is one funnel's share of the approval queue.
//
// FunnelName can be empty when the funnel row is gone but its leads are not —
// reported as an unnamed bucket rather than dropped, because those drafts are
// still waiting on somebody.
type QueueFunnelCount struct {
	FunnelID   string `json:"funnel_id"`
	FunnelName string `json:"funnel_name"`
	Total      int64  `json:"total"`
}

// LeadRepository defines persistence operations for leads.
type LeadRepository interface {
	// Upsert creates the lead or, when a row already exists for the
	// (funnel_id, contact_id) pair, updates its mutable fields.
	Upsert(ctx context.Context, lead *Lead) error
	GetByFunnelAndContact(ctx context.Context, funnelID, contactID string) (*Lead, error)
	// GetByID returns the lead by its primary key, or (nil, nil) when no row
	// matches. Callers enforce account ownership against the returned row.
	GetByID(ctx context.Context, id string) (*Lead, error)
	// AssignOwner sets (or clears, with nil) the lead's responsible
	// salesperson. Account-scoped so a caller cannot reassign another
	// tenant's lead. Returns (nil, nil) when no row matched.
	AssignOwner(ctx context.Context, accountID, id string, ownerUserID *string) (*Lead, error)

	// SetVisibility flips the lead between team-visible and owner-private. The
	// read filter already existed; what was missing was anything writing it.
	SetVisibility(ctx context.Context, accountID, id string, visibility ResourceVisibility) (*Lead, error)
	// ListByFunnelForOwner is the vendedor's pipeline view: own leads plus
	// unassigned and team-visible ones, filtered IN SQL so pagination does
	// not silently hide rows past the page limit.
	ListByFunnelForOwner(ctx context.Context, accountID, funnelID, ownerUserID string, status LeadStatus, limit, offset int32) ([]*Lead, error)
	// ListByFunnel returns the funnel's leads ordered newest-first. When status
	// is non-empty it filters to that status; an empty status returns all of
	// them. limit/offset paginate the result.
	ListByFunnel(ctx context.Context, funnelID string, status LeadStatus, limit, offset int32) ([]*Lead, error)
	// ListForApprovalQueue is the cross-funnel queue (US-M020): the approval
	// backlog is one list for the operator, not one per funnel, and by default
	// the longest-waiting draft comes first because it is the one going stale.
	//
	// Both the funnel narrow and the ordering are applied in SQL — see
	// ApprovalQueueFilter.
	ListForApprovalQueue(ctx context.Context, f ApprovalQueueFilter, limit, offset int32) ([]*Lead, error)
	// CountApprovalQueueByFunnel reports how much of the queue sits in each
	// funnel, across the WHOLE queue rather than the page being shown.
	//
	// f.FunnelID and f.NewestFirst are ignored here on purpose: these counts
	// are the map of where the work is, and a map that empties itself when you
	// pick a destination is not a map.
	CountApprovalQueueByFunnel(ctx context.Context, f ApprovalQueueFilter) ([]QueueFunnelCount, error)
	UpdateMetadata(ctx context.Context, id string, collectedMetadata, enrichment json.RawMessage) error
	UpdateStatus(ctx context.Context, id string, status LeadStatus) error
	// MarkBuyIntent stamps buy_intent_at = NOW() the first time the lead signals
	// purchase intent. Idempotent: a no-op once the timestamp is set (US-012).
	MarkBuyIntent(ctx context.Context, id string) error
	// CountByStatus returns the whole pipeline's counts per status
	// (US-M023). The command centre shows totals, and a total derived
	// from a paginated page is a total that quietly shrinks.
	CountByStatus(ctx context.Context, accountID string) (map[LeadStatus]int64, error)
	// CountByStatusForOwner is the same under vendedor visibility, so the
	// tiles never claim more leads than the board would show.
	CountByStatusForOwner(ctx context.Context, accountID, ownerUserID string) (map[LeadStatus]int64, error)
	// CountUnowned counts live leads nobody is responsible for.
	CountUnowned(ctx context.Context, accountID string) (int64, error)
	// MarkQualified stamps qualified_at = NOW() and promotes status to
	// 'qualified' the first time the lead completes the funnel's required-metadata
	// gate. Returns true only when it freshly stamped the row; false (no error)
	// when the lead was already qualified, so the funnel webhook fires exactly
	// once (US-016).
	MarkQualified(ctx context.Context, id string) (bool, error)
}

var (
	ErrLeadNotFound         = &AppError{Code: "LEAD_NOT_FOUND", Message: "lead not found"}
	ErrInvalidLeadStatus    = &AppError{Code: "LEAD_INVALID_STATUS", Message: "lead status must be one of new/prospected/enriching/enriched/pending_approval/contacted/responded/qualified/won/lost"}
	ErrLeadFieldNotInSchema = &AppError{Code: "LEAD_FIELD_NOT_IN_SCHEMA", Message: "metadata field is not declared in the funnel schema"}

	// ErrLeadOwnerNotInAccount recusa designar um lead a alguém de fora da
	// conta. Um usuário inexistente recebe a MESMA resposta: distinguir os
	// the two would confirm, to anyone sweeping ULIDs, which ones exist
	// somewhere.
	ErrLeadOwnerNotInAccount = &AppError{Code: "LEAD_OWNER_NOT_IN_ACCOUNT", Message: "esse usuário não pertence a esta conta"}
	// ErrLeadAssignUnavailable is the refusal when the account check is not
	// wired. Assigning unchecked would be worse than not assigning.
	ErrLeadAssignUnavailable = &AppError{Code: "LEAD_ASSIGN_UNAVAILABLE", Message: "atribuição de dono indisponível nesta instalação"}
	// ErrBulkAssignEmpty refuses a targetless redistribution: a successful
	// "assigned 0" hides the empty list the client sent by mistake.
	ErrBulkAssignEmpty = &AppError{Code: "LEAD_BULK_ASSIGN_EMPTY", Message: "nenhum lead informado"}
	// ErrBulkAssignTooMany limita o tamanho do estrago de um clique.
	ErrBulkAssignTooMany = &AppError{Code: "LEAD_BULK_ASSIGN_TOO_MANY", Message: "no máximo 200 leads por vez"}
	// ErrInvalidLeadVisibility guarda o par team/private.
	ErrInvalidLeadVisibility = &AppError{Code: "LEAD_INVALID_VISIBILITY", Message: "visibility must be team or private"}

	// ErrLeadAlreadyContacted guards the human-in-the-loop draft path: a lead
	// that already had its approach sent (or moved past the approach stage)
	// cannot be re-drafted, preventing accidental duplicate outreach (US-020).
	ErrLeadAlreadyContacted = &AppError{Code: "LEAD_ALREADY_CONTACTED", Message: "lead has already been approached"}
	// ErrLeadNotPendingApproval enforces the approval queue: approve-send only
	// fires for a lead a human has drafted (status=pending_approval). There is
	// no path to send an approach that skipped the queue (US-020).
	ErrLeadNotPendingApproval = &AppError{Code: "LEAD_NOT_PENDING_APPROVAL", Message: "lead is not awaiting approval — generate a draft first"}
	// ErrLeadApproachTextRequired is returned when approve-send is called with
	// an empty final message.
	ErrLeadApproachTextRequired = &AppError{Code: "LEAD_APPROACH_TEXT_REQUIRED", Message: "approach text is required"}
	// ErrLeadFunnelNoChannel is returned when the lead's funnel has no channel
	// bound, so there is nowhere to send the approach.
	ErrLeadFunnelNoChannel = &AppError{Code: "LEAD_FUNNEL_NO_CHANNEL", Message: "the lead's funnel has no channel to send the approach through"}
	// ErrLeadSendFailed wraps a downstream channel send failure so the approach
	// stays in pending_approval with a visible error (US-020).
	ErrLeadSendFailed = &AppError{Code: "LEAD_SEND_FAILED", Message: "failed to send the approach message"}
	// ErrLeadNoConversation refuses a reply on a lead nobody has exchanged a
	// message with yet: there is no thread to answer on. A 409, not a 404 —
	// the lead exists; the conversation is what is missing.
	ErrLeadNoConversation = &AppError{Code: "LEAD_NO_CONVERSATION", Message: "this lead has no conversation to reply on yet"}
	// ErrLeadReplyTextRequired refuses an empty reply before anything is
	// persisted or sent.
	ErrLeadReplyTextRequired = &AppError{Code: "LEAD_REPLY_TEXT_REQUIRED", Message: "reply text is required"}
	// ErrLeadApproachTemplateRequired is returned when approve-send is called
	// without an approved WhatsApp template. A cold first-contact (the lead never
	// messaged us) is business-initiated and the Cloud API only delivers it via a
	// pre-approved template — a free-form text would be rejected by Meta (131047,
	// outside the 24h customer-service window). US-020.
	ErrLeadApproachTemplateRequired = &AppError{Code: "LEAD_APPROACH_TEMPLATE_REQUIRED", Message: "an approved WhatsApp template is required to approach this lead"}
	// ErrLeadContactNotMessageable is returned when the lead's contact id is not a
	// valid E.164 phone (e.g. a Google place_id-only prospect with no phone), so
	// there is nothing to send a WhatsApp message to. Caught before any Meta call.
	ErrLeadContactNotMessageable = &AppError{Code: "LEAD_CONTACT_NOT_MESSAGEABLE", Message: "lead has no valid phone number to message"}
	// ErrLeadPhoneInvalid rejects a manual lead creation whose phone does not
	// normalize to E.164 — the phone doubles as the contact key, so a bad one
	// would create an unmessageable lead.
	ErrLeadPhoneInvalid = &AppError{Code: "LEAD_INVALID_PHONE", Message: "lead phone must be a valid E.164 number"}
	// ErrLeadNeedsPhoneOrEmail rejects a manual lead with no way to reach it.
	// A name with no address is a note, not a lead.
	ErrLeadNeedsPhoneOrEmail = &AppError{Code: "LEAD_NEEDS_PHONE_OR_EMAIL", Message: "a lead needs a phone or an e-mail"}
	// ErrLeadEmailInvalid rejects a manual lead whose e-mail does not parse.
	ErrLeadEmailInvalid = &AppError{Code: "LEAD_INVALID_EMAIL", Message: "lead e-mail is not valid"}
	// ErrLeadCreateUnavailable is returned when manual lead creation was not
	// wired with a contact repository — a server misconfiguration, not a
	// client error.
	ErrLeadCreateUnavailable = &AppError{Code: "LEAD_CREATE_UNAVAILABLE", Message: "manual lead creation is not available"}
)

// RadarLeadState is the answer to "what has this account already done about
// this buyer, and where does the card sit" — the one lead read the radar's
// buyer block makes.
//
// It carries the LEAD ID because a state without a way to open it is a dead
// end: the panel says "está no seu quadro" and the seller then has to go find
// it by name. And it carries the STAGE because "in the funnel" is not a
// position — a card in "Ganhos" and a card nobody has touched are both "in the
// funnel", and they call for opposite next moves.
type RadarLeadState struct {
	LeadID   string
	FunnelID string
	Status   LeadStatus
	// Drafted says an approach text exists. Not that it was sent — the
	// approval queue is the gate, and BuyerQueued is exactly this state.
	Drafted bool
	// Stage is the Kanban column's name, empty when the funnel has no column
	// that maps onto the lead's status. Empty is rendered as nothing, never
	// as a guessed column.
	Stage string
}
