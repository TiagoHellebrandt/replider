package usecases

import (
	"context"
	"errors"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// The first offer (onboarding, last step — 2026-08-18).
//
// Everything before this step in the wizard exists so that this one can
// happen: the person leaves onboarding having read one real, approvable
// message about one of her customers (or, failing that, one hospital in her
// territory) and one of her products. It is the product's whole promise,
// shown instead of described.
//
// It writes through the same paths the daily product uses — the default
// funnel, contacts→lead, the approach drafter with its opt-out check and
// spend cap — so what she sees here is what she will see every morning, not
// a demo that behaves differently.

// FirstOfferStatus says what the wizard should render.
type FirstOfferStatus string

const (
	// FirstOfferReady — a lead with a draft, sitting in the approval queue.
	FirstOfferReady FirstOfferStatus = "ready"
	// FirstOfferNothingToOffer — no customer to write to and no territory to
	// search. Not an error: the wizard explains what to bring and finishes.
	FirstOfferNothingToOffer FirstOfferStatus = "nothing_to_offer"
)

// FirstOfferSource says who the offer is for.
type FirstOfferSource string

const (
	// FirstOfferFromCustomer — one of the customers she imported.
	FirstOfferFromCustomer FirstOfferSource = "customer"
	// FirstOfferFromProspect — a business found in her territory, because
	// she brought no customer list.
	FirstOfferFromProspect FirstOfferSource = "prospect"
)

// Reasons a first offer could not be written. The wizard keys its copy on
// these, so they are part of the API surface.
const (
	// FirstOfferReasonNoCustomersNoTerritory — she brought no customers and
	// there is nothing to search (no territory, or no prospector wired).
	FirstOfferReasonNoCustomersNoTerritory = "no_customers_no_territory"
	// FirstOfferReasonNoBusinessFound — the search ran and found nobody, or
	// could not run for a transient reason (rate limit).
	FirstOfferReasonNoBusinessFound = "no_business_found"
	// FirstOfferReasonProspectingNotConfigured — the platform has no
	// prospecting provider (no Places key). An operator gap, not hers.
	FirstOfferReasonProspectingNotConfigured = "prospecting_not_configured"
)

// FirstOfferResult is what the wizard renders.
type FirstOfferResult struct {
	Status FirstOfferStatus `json:"status"`
	// Reason is set when Status is not ready: one of the FirstOfferReason*
	// constants.
	Reason string           `json:"reason,omitempty"`
	Source FirstOfferSource `json:"source,omitempty"`
	Lead   *domain.Lead     `json:"lead,omitempty"`
	// Draft duplicates lead.approach_draft for the client's convenience.
	Draft string `json:"draft,omitempty"`
}

// firstOfferProspector is the slice of ProspectingUseCase this needs.
type firstOfferProspector interface {
	Execute(ctx context.Context, dto ProspectingSearchDTO) (*ProspectingResult, error)
}

// firstOfferDrafter is the slice of ApproachUseCase this needs.
type firstOfferDrafter interface {
	Draft(ctx context.Context, accountID, leadID string) (*domain.Lead, error)
}

// firstOfferLeadMaker is the slice of LeadUseCase this needs.
type firstOfferLeadMaker interface {
	CreateFromContacts(ctx context.Context, in ContactsToFunnelInput) (*ContactsToFunnelResult, error)
}

// FirstOfferUseCase writes the first offer.
type FirstOfferUseCase struct {
	accountRepo domain.AccountRepository
	funnelRepo  domain.SalesFunnelRepository
	contactRepo domain.ContactRepository
	leadRepo    domain.LeadRepository
	leads       firstOfferLeadMaker
	drafter     firstOfferDrafter
	// prospector is optional: without a Places key the wizard simply cannot
	// find a business, and says so instead of failing.
	prospector firstOfferProspector
}

func NewFirstOfferUseCase(
	accountRepo domain.AccountRepository,
	funnelRepo domain.SalesFunnelRepository,
	contactRepo domain.ContactRepository,
	leadRepo domain.LeadRepository,
	leads firstOfferLeadMaker,
	drafter firstOfferDrafter,
) *FirstOfferUseCase {
	return &FirstOfferUseCase{
		accountRepo: accountRepo,
		funnelRepo:  funnelRepo,
		contactRepo: contactRepo,
		leadRepo:    leadRepo,
		leads:       leads,
		drafter:     drafter,
	}
}

// SetProspector wires the territory fallback. Optional.
func (uc *FirstOfferUseCase) SetProspector(p firstOfferProspector) { uc.prospector = p }

// Execute finds or makes the account's first lead and drafts its offer.
//
// Idempotent on re-entry: a person who reloads the last step, or comes back
// after closing the tab, gets the SAME offer, not a second lead and a second
// spend. The pending draft in the default funnel is the marker.
func (uc *FirstOfferUseCase) Execute(ctx context.Context, accountID, userID string) (*FirstOfferResult, error) {
	funnel, err := uc.funnelRepo.EnsureDefault(ctx, accountID)
	if err != nil {
		return nil, err
	}

	// Already written? Show it again.
	if existing, err := uc.leadRepo.ListByFunnel(ctx, funnel.ID, domain.LeadStatusPendingApproval, 1, 0); err == nil && len(existing) > 0 && existing[0].ApproachDraft != "" {
		lead := existing[0]
		return &FirstOfferResult{
			Status: FirstOfferReady,
			Source: sourceOf(lead),
			Lead:   lead,
			Draft:  lead.ApproachDraft,
		}, nil
	}

	leadID, source, err := uc.pickLead(ctx, accountID, userID, funnel.ID)
	if err != nil {
		return nil, err
	}
	if leadID == "" {
		return &FirstOfferResult{Status: FirstOfferNothingToOffer, Reason: string(source)}, nil
	}

	lead, err := uc.drafter.Draft(ctx, accountID, leadID)
	if err != nil {
		return nil, err
	}
	return &FirstOfferResult{
		Status: FirstOfferReady,
		Source: source,
		Lead:   lead,
		Draft:  lead.ApproachDraft,
	}, nil
}

// pickLead returns the lead to write to and where it came from. An empty id
// means nothing to offer, and the "source" then carries the reason.
func (uc *FirstOfferUseCase) pickLead(ctx context.Context, accountID, userID, funnelID string) (string, FirstOfferSource, error) {
	// 1. A customer she brought. Any lead already in the funnel (imported
	//    but not yet drafted) counts first — it is hers.
	if leads, err := uc.leadRepo.ListByFunnel(ctx, funnelID, "", 1, 0); err == nil && len(leads) > 0 {
		return leads[0].ID, sourceOf(leads[0]), nil
	}
	// The first offer goes out by email (the account's channel), so among
	// the customers she imported, one WITH an email address comes first: a
	// phone-only contact drafts fine and then fails at approval with
	// "no address to write to" — the wrong first impression of the product.
	contacts, err := uc.contactRepo.ListByAccountID(ctx, accountID, "", 25, 0)
	if err != nil {
		return "", "", err
	}
	if len(contacts) > 0 {
		pick := contacts[0]
		for _, c := range contacts {
			if c != nil && c.Email != nil && domain.IsEmailAddress(*c.Email) {
				pick = c
				break
			}
		}
		res, err := uc.leads.CreateFromContacts(ctx, ContactsToFunnelInput{
			AccountID:   accountID,
			FunnelID:    funnelID,
			ContactIDs:  []string{pick.ID},
			OwnerUserID: userID,
		})
		if err != nil {
			return "", "", err
		}
		if len(res.LeadIDs) > 0 {
			return res.LeadIDs[0], FirstOfferFromCustomer, nil
		}
	}

	// 2. A business in her territory.
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if account == nil || account.Territory == nil || uc.prospector == nil {
		return "", FirstOfferSource(FirstOfferReasonNoCustomersNoTerritory), nil
	}
	res, err := uc.prospector.Execute(ctx, ProspectingSearchDTO{
		AccountID:    accountID,
		FunnelID:     funnelID,
		Query:        territoryQuery(account),
		LanguageCode: "pt-BR",
		MaxResults:   1,
	})
	if err != nil {
		// A search that cannot run (no key, rate limit) is not the person's
		// fault and must not strand the wizard; the reason is honest. A
		// platform with no Places key says so — "no business found" would
		// send her looking for a hospital that was never searched for.
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			if appErr.Code == domain.ErrProspectingNotConfigured.Code {
				return "", FirstOfferSource(FirstOfferReasonProspectingNotConfigured), nil
			}
			return "", FirstOfferSource(FirstOfferReasonNoBusinessFound), nil
		}
		return "", "", err
	}
	if len(res.LeadIDs) == 0 {
		return "", FirstOfferSource(FirstOfferReasonNoBusinessFound), nil
	}
	return res.LeadIDs[0], FirstOfferFromProspect, nil
}

// territoryQuery is what we ask Places for when she brought no customers:
// the kind of establishment her business lines sell to, in the first city
// she named (or the state). "hospital" is right for every line a
// hospital-equipment company has; a later version reads the catalogue.
func territoryQuery(a *domain.Account) string {
	where := a.Territory.UF
	if len(a.Territory.Cities) > 0 {
		where = a.Territory.Cities[0] + " " + a.Territory.UF
	}
	return strings.TrimSpace("hospital " + where)
}

// sourceOf maps a lead's provenance onto the two the wizard tells apart.
// "google_places" is what ProspectingUseCase stamps; everything else
// (existing_customer, manual, inbound) is somebody she already knows.
func sourceOf(lead *domain.Lead) FirstOfferSource {
	if lead != nil && lead.Source == "google_places" {
		return FirstOfferFromProspect
	}
	return FirstOfferFromCustomer
}
