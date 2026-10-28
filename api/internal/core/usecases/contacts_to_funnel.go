package usecases

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Putting customers she ALREADY has into a funnel.
//
// Every other door into the pipeline assumes the person is new: Create wants a
// typed phone (and refuses without one), the discovery import only accepts
// addresses its own Sent-folder scan produced, and the prospecting import only
// accepts Google Places results. So a base of customers brought in by CSV landed
// in /contacts and stopped there — to work one, you re-typed their name and
// phone into the manual form. With two thousand rows that is not a workflow.
//
// This adds the missing door and nothing else: no contact is created, no
// identifier is normalised, no name is overwritten. The contact already exists
// and is already indexed; the only new fact is which funnel it belongs to.

// MaxContactsPerFunnelImport bounds one request.
//
// The batch costs one lookup per contact (see ensureLeadForContact), so an
// unbounded selection is an unbounded request. The limit REFUSES rather than
// truncating: silently importing the first N of a 2000-row "select all" would
// report success for work it did not do, and the missing leads would only
// surface weeks later as customers nobody followed up.
const MaxContactsPerFunnelImport = 500

// ContactsToFunnelInput is one "send these to this funnel" action.
type ContactsToFunnelInput struct {
	AccountID  string
	FunnelID   string
	ContactIDs []string
	// OwnerUserID assigns the created leads. Empty leaves them unassigned,
	// which stays visible to the whole team.
	OwnerUserID string
}

// ContactToFunnelProblem names one contact that did not make it.
//
// Per contact rather than per batch: the answer "3 of 40 failed" without saying
// which three leaves her re-importing all forty to find out.
type ContactToFunnelProblem struct {
	ContactID string `json:"contact_id"`
	Reason    string `json:"reason"`
}

// ContactsToFunnelResult separates the three outcomes, because they call for
// three different reactions: created is progress, skipped is "already done"
// (harmless, and the reason a re-run is safe), failed needs attention.
type ContactsToFunnelResult struct {
	Created  int                      `json:"created"`
	Skipped  int                      `json:"skipped"`
	Failed   int                      `json:"failed"`
	LeadIDs  []string                 `json:"lead_ids"`
	Problems []ContactToFunnelProblem `json:"problems,omitempty"`
}

// Problem reasons. Stable strings the client maps to a sentence.
const (
	ContactToFunnelReasonNotFound = "contact_not_found"
	ContactToFunnelReasonFailed   = "create_failed"
)

// CreateFromContacts puts existing contacts into a funnel, in bulk.
//
// One contact's failure never ends the batch: with a base imported from a
// spreadsheet some rows are always broken, and aborting on the first one means
// the good 1,990 depend on the bad 10 being fixed first.
func (uc *LeadUseCase) CreateFromContacts(
	ctx context.Context, in ContactsToFunnelInput,
) (*ContactsToFunnelResult, error) {
	if uc.contactRepo == nil {
		return nil, domain.ErrLeadCreateUnavailable
	}
	if strings.TrimSpace(in.FunnelID) == "" {
		return nil, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "escolha o funil onde os clientes vão entrar",
		}
	}
	// Ownership of the destination first, before the payload is judged: a
	// caller who may not write to this funnel should learn that, not that their
	// selection was malformed.
	funnel, err := uc.funnelRepo.GetByID(ctx, in.FunnelID)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != in.AccountID {
		return nil, domain.ErrUnauthorized
	}

	ids := dedupeNonEmpty(in.ContactIDs)
	if len(ids) == 0 {
		return nil, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "selecione pelo menos um cliente",
		}
	}
	if len(ids) > MaxContactsPerFunnelImport {
		return nil, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "selecione no máximo 500 clientes por vez",
		}
	}

	// One round trip for the whole selection, and account-scoped: a contact id
	// from another tenant comes back missing, indistinguishable from one that
	// never existed, so this endpoint cannot be used to probe for ids.
	contacts, err := uc.contactRepo.ListByIDs(ctx, in.AccountID, ids)
	if err != nil {
		return nil, err
	}
	found := make(map[string]bool, len(contacts))
	for _, c := range contacts {
		if c != nil {
			found[c.ID] = true
		}
	}

	out := &ContactsToFunnelResult{}
	for _, id := range ids {
		if !found[id] {
			out.Failed++
			out.Problems = append(out.Problems, ContactToFunnelProblem{
				ContactID: id, Reason: ContactToFunnelReasonNotFound,
			})
			continue
		}
		leadID, created, err := uc.ensureLeadForContact(ctx, ensureLeadInput{
			AccountID:   in.AccountID,
			FunnelID:    funnel.ID,
			ContactID:   id,
			OwnerUserID: in.OwnerUserID,
			// Named so the funnel can tell these apart from a cold prospecting
			// import and from the Sent-folder scan: somebody already in her
			// customer base is not a stranger, and an approach written as if
			// they were would read as amnesia.
			Source: "existing_customer",
		})
		if err != nil {
			out.Failed++
			out.Problems = append(out.Problems, ContactToFunnelProblem{
				ContactID: id, Reason: ContactToFunnelReasonFailed,
			})
			continue
		}
		out.LeadIDs = append(out.LeadIDs, leadID)
		if created {
			out.Created++
			continue
		}
		// Already in this funnel. Not an error, and the reason running this
		// twice is safe: the existing lead keeps whatever stage it had reached.
		out.Skipped++
	}
	return out, nil
}

// ensureLeadInput carries the fields ensureLeadForContact needs. A struct rather
// than six positional strings, which is a signature nobody reads correctly.
type ensureLeadInput struct {
	AccountID   string
	FunnelID    string
	ContactID   string
	OwnerUserID string
	Source      string
}

// ensureLeadForContact is the single "this contact belongs in this funnel" rule.
//
// Shared with CreateFromDiscovered so the dedupe, the starting status, the
// default visibility and the instrumentation exist once. A second copy of those
// is a second set of rules, and they drift.
//
// created=false means a lead was already there and was left ALONE — importing
// the same person twice must never reset a pipeline that has moved on.
func (uc *LeadUseCase) ensureLeadForContact(
	ctx context.Context, in ensureLeadInput,
) (leadID string, created bool, err error) {
	if existing, err := uc.leadRepo.GetByFunnelAndContact(ctx, in.FunnelID, in.ContactID); err == nil && existing != nil {
		return existing.ID, false, nil
	}

	lead := &domain.Lead{
		ID:                ulid.Make().String(),
		AccountID:         in.AccountID,
		FunnelID:          in.FunnelID,
		ContactID:         in.ContactID,
		Status:            domain.LeadStatusNew,
		Source:            in.Source,
		CollectedMetadata: json.RawMessage("{}"),
		Enrichment:        json.RawMessage("{}"),
		Visibility:        domain.VisibilityTeam,
	}
	if owner := strings.TrimSpace(in.OwnerUserID); owner != "" {
		lead.OwnerUserID = &owner
	}
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		return "", false, err
	}
	uc.cases.LeadCreated(ctx, in.AccountID, lead.ID, lead.Source)
	return lead.ID, true, nil
}

// dedupeNonEmpty trims, drops blanks and collapses repeats while keeping the
// order the caller sent. The same contact selected twice must count once, or the
// reported totals do not add up to the selection.
func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
