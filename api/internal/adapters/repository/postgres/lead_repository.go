package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// LeadRepository implements domain.LeadRepository over the sqlc-generated
// Queries (leads.sql from US-003). Wraps the outbound-CRM lead entity used by
// the invisible data collection (US-011) and pipeline stories.
type LeadRepository struct {
	queries *Queries
}

func NewLeadRepository(db DBTX) *LeadRepository {
	return &LeadRepository{queries: New(db)}
}

func toDomainLead(row Lead) *domain.Lead {
	collected := json.RawMessage(row.CollectedMetadata)
	if len(collected) == 0 {
		collected = json.RawMessage("{}")
	}
	enrichment := json.RawMessage(row.Enrichment)
	if len(enrichment) == 0 {
		enrichment = json.RawMessage("{}")
	}
	matched := json.RawMessage(row.MatchedProducts)
	objections := json.RawMessage(row.AnticipatedObjections)
	if len(matched) == 0 {
		matched = json.RawMessage("[]")
	}
	return &domain.Lead{
		ID:                    row.ID,
		AccountID:             row.AccountID,
		FunnelID:              row.FunnelID,
		ContactID:             row.ContactID,
		Status:                domain.LeadStatus(row.Status),
		Source:                row.Source,
		Score:                 int(row.Score),
		CollectedMetadata:     collected,
		Enrichment:            enrichment,
		MatchedProducts:       matched,
		AnticipatedObjections: objections,
		ApproachDraft:         row.ApproachDraft,
		BuyIntentAt:           timestamptzPtr(row.BuyIntentAt),
		QualifiedAt:           timestamptzPtr(row.QualifiedAt),
		OwnerUserID:           row.OwnerUserID,
		Visibility:            domain.ResourceVisibility(row.Visibility),
		StageID:               row.StageID,
		BoardPos:              row.BoardPos,
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

func (r *LeadRepository) Upsert(ctx context.Context, lead *domain.Lead) error {
	q := getQueries(ctx, r.queries)
	row, err := q.UpsertLead(ctx, UpsertLeadParams{
		ID:                    lead.ID,
		AccountID:             lead.AccountID,
		FunnelID:              lead.FunnelID,
		ContactID:             lead.ContactID,
		Status:                string(lead.Status),
		Source:                lead.Source,
		Score:                 int32(lead.Score), //nolint:gosec // G115: lead score is a small bounded int
		CollectedMetadata:     toBytes(lead.CollectedMetadata),
		Enrichment:            toBytes(lead.Enrichment),
		MatchedProducts:       matchedOrEmpty(lead.MatchedProducts),
		AnticipatedObjections: matchedOrEmpty(lead.AnticipatedObjections),
		ApproachDraft:         lead.ApproachDraft,
		BuyIntentAt:           timeToTimestamptz(lead.BuyIntentAt),
		QualifiedAt:           timeToTimestamptz(lead.QualifiedAt),
		OwnerUserID:           lead.OwnerUserID,
		// The column has a CHECK ('team','private'); an unset visibility on
		// a fresh lead means the default, not an empty string.
		Visibility: string(defaultVisibility(lead.Visibility)),
	})
	if err != nil {
		return err
	}
	// Reflect the resulting (possibly pre-existing) id back onto the caller's
	// struct so it can address the row it just wrote.
	lead.ID = row.ID
	return nil
}

func (r *LeadRepository) GetByFunnelAndContact(ctx context.Context, funnelID, contactID string) (*domain.Lead, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetLeadByFunnelAndContact(ctx, GetLeadByFunnelAndContactParams{
		FunnelID:  funnelID,
		ContactID: contactID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainLead(row), nil
}

func (r *LeadRepository) GetByID(ctx context.Context, id string) (*domain.Lead, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetLeadByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return toDomainLead(row), nil
}

func (r *LeadRepository) ListByFunnel(ctx context.Context, funnelID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	q := getQueries(ctx, r.queries)
	var statusArg *string
	if status != "" {
		s := string(status)
		statusArg = &s
	}
	rows, err := q.ListLeadsByFunnel(ctx, ListLeadsByFunnelParams{
		FunnelID:  funnelID,
		Status:    statusArg,
		RowLimit:  limit,
		RowOffset: offset,
	})
	if err != nil {
		return nil, err
	}
	leads := make([]*domain.Lead, 0, len(rows))
	for _, row := range rows {
		leads = append(leads, toDomainLead(row))
	}
	return leads, nil
}

func (r *LeadRepository) UpdateMetadata(ctx context.Context, id string, collectedMetadata, enrichment json.RawMessage) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateLeadMetadata(ctx, UpdateLeadMetadataParams{
		ID:                id,
		CollectedMetadata: toBytes(collectedMetadata),
		Enrichment:        toBytes(enrichment),
	})
}

func (r *LeadRepository) UpdateStatus(ctx context.Context, id string, status domain.LeadStatus) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateLeadStatus(ctx, UpdateLeadStatusParams{
		ID:     id,
		Status: string(status),
	})
}

// ClaimForSend is the atomic pending_approval → contacted flip the approval
// send relies on (usecases.leadSendClaimer). False = the lead was not
// pending, i.e. somebody else already claimed it.
func (r *LeadRepository) ClaimForSend(ctx context.Context, id string) (bool, error) {
	q := getQueries(ctx, r.queries)
	if _, err := q.ClaimLeadForSend(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *LeadRepository) MarkBuyIntent(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkLeadBuyIntent(ctx, id)
}

func (r *LeadRepository) MarkQualified(ctx context.Context, id string) (bool, error) {
	q := getQueries(ctx, r.queries)
	// The query only matches when qualified_at IS NULL, so pgx.ErrNoRows means
	// the lead was already qualified — report "not freshly stamped", not an error.
	if _, err := q.MarkLeadQualified(ctx, id); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// defaultVisibility normalizes an unset/invalid visibility to the team
// default. Sales works better when the floor can see each other's pipeline;
// `private` is the deliberate exception, never an accident of a zero value.
func defaultVisibility(v domain.ResourceVisibility) domain.ResourceVisibility {
	if v.Valid() {
		return v
	}
	return domain.VisibilityTeam
}

// AssignOwner sets or clears the lead's responsible salesperson (US-F009).
func (r *LeadRepository) AssignOwner(ctx context.Context, accountID, id string, ownerUserID *string) (*domain.Lead, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.AssignLeadOwner(ctx, AssignLeadOwnerParams{
		ID:          id,
		AccountID:   accountID,
		OwnerUserID: ownerUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Cross-account or missing — indistinguishable on purpose.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomainLead(row), nil
}

// SetVisibility troca entre team e private (US-F009).
//
// The query has existed since the migration and never had a caller: the read
// already filtered `private` for the salesperson, and nothing in the product
// could mark a lead as such — the filter ran over a column that was always
// 'team'.
func (r *LeadRepository) SetVisibility(ctx context.Context, accountID, id string, visibility domain.ResourceVisibility) (*domain.Lead, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.SetLeadVisibility(ctx, SetLeadVisibilityParams{
		ID:         id,
		AccountID:  accountID,
		Visibility: string(visibility),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomainLead(row), nil
}

// ListByFunnelForOwner filters the pipeline to what a vendedor may see.
func (r *LeadRepository) CountByStatus(ctx context.Context, accountID string) (map[domain.LeadStatus]int64, error) {
	rows, err := getQueries(ctx, r.queries).CountLeadsByStatusForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.LeadStatus]int64, len(rows))
	for _, row := range rows {
		out[domain.LeadStatus(row.Status)] = row.Total
	}
	return out, nil
}

func (r *LeadRepository) CountByStatusForOwner(ctx context.Context, accountID, ownerUserID string) (map[domain.LeadStatus]int64, error) {
	rows, err := getQueries(ctx, r.queries).CountLeadsByStatusForOwner(ctx, CountLeadsByStatusForOwnerParams{
		AccountID:   accountID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[domain.LeadStatus]int64, len(rows))
	for _, row := range rows {
		out[domain.LeadStatus(row.Status)] = row.Total
	}
	return out, nil
}

func (r *LeadRepository) CountUnowned(ctx context.Context, accountID string) (int64, error) {
	return getQueries(ctx, r.queries).CountUnownedLeadsForAccount(ctx, accountID)
}

// ListForApprovalQueue picks the account-wide or the vendedor-scoped query by
// whether the filter names an owner. Both carry the funnel narrow and the
// ordering, so neither view can end up filtering a page in memory.
func (r *LeadRepository) ListForApprovalQueue(ctx context.Context, f domain.ApprovalQueueFilter, limit, offset int32) ([]*domain.Lead, error) {
	q := getQueries(ctx, r.queries)

	// An empty funnel means "every funnel", which the query reads as a NULL
	// narg rather than as an id that matches nothing.
	var funnelArg *string
	if f.FunnelID != "" {
		id := f.FunnelID
		funnelArg = &id
	}

	var (
		rows []Lead
		err  error
	)
	if f.OwnerUserID != "" {
		rows, err = q.ListLeadsByAccountAndStatusForOwner(ctx, ListLeadsByAccountAndStatusForOwnerParams{
			AccountID:   f.AccountID,
			Status:      string(f.Status),
			OwnerUserID: f.OwnerUserID,
			FunnelID:    funnelArg,
			NewestFirst: f.NewestFirst,
			Limit:       limit,
			Offset:      offset,
		})
	} else {
		rows, err = q.ListLeadsByAccountAndStatus(ctx, ListLeadsByAccountAndStatusParams{
			AccountID:   f.AccountID,
			Status:      string(f.Status),
			FunnelID:    funnelArg,
			NewestFirst: f.NewestFirst,
			Limit:       limit,
			Offset:      offset,
		})
	}
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Lead, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainLead(row))
	}
	return out, nil
}

func (r *LeadRepository) CountApprovalQueueByFunnel(ctx context.Context, f domain.ApprovalQueueFilter) ([]domain.QueueFunnelCount, error) {
	q := getQueries(ctx, r.queries)

	if f.OwnerUserID != "" {
		rows, err := q.CountQueueByFunnelForOwner(ctx, CountQueueByFunnelForOwnerParams{
			AccountID:   f.AccountID,
			Status:      string(f.Status),
			OwnerUserID: f.OwnerUserID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]domain.QueueFunnelCount, 0, len(rows))
		for _, row := range rows {
			out = append(out, domain.QueueFunnelCount{
				FunnelID: row.FunnelID, FunnelName: row.FunnelName, Total: row.Total,
			})
		}
		return out, nil
	}

	rows, err := q.CountQueueByFunnelForAccount(ctx, CountQueueByFunnelForAccountParams{
		AccountID: f.AccountID,
		Status:    string(f.Status),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.QueueFunnelCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.QueueFunnelCount{
			FunnelID: row.FunnelID, FunnelName: row.FunnelName, Total: row.Total,
		})
	}
	return out, nil
}

func (r *LeadRepository) ListByFunnelForOwner(ctx context.Context, accountID, funnelID, ownerUserID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	q := getQueries(ctx, r.queries)
	var statusArg *string
	if status != "" {
		s := string(status)
		statusArg = &s
	}
	rows, err := q.ListLeadsByFunnelForOwner(ctx, ListLeadsByFunnelForOwnerParams{
		AccountID:   accountID,
		FunnelID:    funnelID,
		OwnerUserID: ownerUserID,
		Status:      statusArg,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Lead, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainLead(row))
	}
	return out, nil
}

// matchedOrEmpty keeps the column NOT NULL: a lead that was never drafted
// carries "[]", not NULL, so readers never special-case it.
func matchedOrEmpty(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("[]")
	}
	return toBytes(raw)
}

// CountUndeliverablePending answers, for one account's approval queue, how
// many drafts have no address of each kind.
//
// The caller decides which of the three numbers applies, because that depends
// on the channels the account has connected — a fact the lead table knows
// nothing about.
func (r *LeadRepository) CountUndeliverablePending(ctx context.Context, accountID string) (domain.UndeliverableQueueCounts, error) {
	row, err := getQueries(ctx, r.queries).CountUndeliverablePendingApproval(ctx, accountID)
	if err != nil {
		return domain.UndeliverableQueueCounts{}, err
	}
	return domain.UndeliverableQueueCounts{
		NoAddress: row.NoAddress,
		NoEmail:   row.NoEmail,
		NoPhone:   row.NoPhone,
		Pending:   row.Pending,
	}, nil
}
