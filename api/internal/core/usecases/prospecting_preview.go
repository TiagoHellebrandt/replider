package usecases

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/prospecting"
	"github.com/replider/api/internal/core/domain"
)

// Prospecting preview (§2, D4).
//
// Before this, a search WAS an import: you typed a keyword and however many
// businesses came back became leads. That is fine when the keyword is good
// and expensive when it is not — a bad search spends the month's lead
// allowance on a list nobody would have picked, and the allowance is the part
// the customer pays for.
//
// So the search now answers first, and the person chooses. The duplicates are
// marked rather than hidden: seeing "já está na sua base" next to eight of
// twenty results is what teaches somebody their keyword is too broad.

// PreviewCandidate is one business the search found.
type PreviewCandidate struct {
	// ExternalID is the dedup key — the E.164 phone when there is one, the
	// place id otherwise. Round-trips to the import so the second call does
	// not have to search again.
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	Address    string `json:"address,omitempty"`
	Website    string `json:"website,omitempty"`
	PlaceID    string `json:"place_id,omitempty"`

	// AlreadyLead is true when this business is already in the funnel.
	// SHOWN rather than filtered: a list that silently drops half its
	// results looks like a weak source, when the real news is that the
	// search is repeating work already done.
	AlreadyLead bool `json:"already_lead"`
}

// ProspectingPreview is the answer to "what would this search import?".
type ProspectingPreview struct {
	Candidates []PreviewCandidate `json:"candidates"`
	// NewCount excludes the duplicates: the number the button should offer
	// to import.
	NewCount       int `json:"new_count"`
	DuplicateCount int `json:"duplicate_count"`

	// RemainingThisMonth is the lead allowance left, or nil when the plan
	// does not meter leads. The cost of a decision belongs next to the
	// decision — finding out afterwards that a search ate the month is how
	// somebody stops trusting the feature.
	RemainingThisMonth *int `json:"remaining_this_month,omitempty"`
	// WouldImport is what the button will actually create: the smaller of
	// the selection and what the allowance permits.
	WouldImport int `json:"would_import"`
}

// Preview runs the search and creates NOTHING.
func (uc *ProspectingUseCase) Preview(ctx context.Context, dto ProspectingSearchDTO) (*ProspectingPreview, error) {
	if uc.provider == nil {
		return nil, domain.ErrProspectingNotConfigured
	}
	funnel, err := uc.ownedFunnel(ctx, dto)
	if err != nil {
		return nil, err
	}

	query := strings.TrimSpace(dto.Query)
	if query == "" && dto.RadiusMeters <= 0 {
		return nil, domain.ErrProspectingInputRequired
	}

	maxLeads := uc.recipientCap(ctx, dto.AccountID)
	out := &ProspectingPreview{Candidates: []PreviewCandidate{}}
	if remaining, ok := uc.monthlyLeadsRemaining(ctx, dto.AccountID); ok {
		out.RemainingThisMonth = &remaining
		if remaining < maxLeads {
			maxLeads = remaining
		}
	}

	// Asked for MORE than the cap on purpose. The cap limits what may be
	// imported, not what may be seen — and a preview that hides the last
	// three results because the allowance is short leaves somebody choosing
	// from a list they cannot tell is truncated.
	candidates, err := uc.search(ctx, dto, query, uc.recipientCap(ctx, dto.AccountID))
	if err != nil {
		return nil, err
	}

	for _, cand := range candidates {
		externalID, phone := candidateExternalID(cand)
		if externalID == "" {
			// No phone and no place id: nothing to key a contact on, so
			// nothing that could become a lead. Omitted rather than shown
			// as unselectable, which would just be a row that does nothing.
			continue
		}
		item := PreviewCandidate{
			ExternalID: externalID, Name: cand.Name, Phone: phone,
			Address: cand.Address, Website: cand.Website, PlaceID: cand.PlaceID,
			AlreadyLead: uc.alreadyLead(ctx, dto.AccountID, funnel.ID, externalID),
		}
		if item.AlreadyLead {
			out.DuplicateCount++
		} else {
			out.NewCount++
		}
		out.Candidates = append(out.Candidates, item)
	}

	out.WouldImport = out.NewCount
	if out.WouldImport > maxLeads {
		out.WouldImport = maxLeads
	}
	return out, nil
}

// ImportSelected turns chosen candidates into leads.
//
// Takes the candidates back rather than a search id: re-running the search
// would cost another call to a paid directory and could return a different
// list, so the person would import something other than what they ticked.
func (uc *ProspectingUseCase) ImportSelected(ctx context.Context, dto ProspectingSearchDTO, selected []PreviewCandidate) (*ProspectingResult, error) {
	funnel, err := uc.ownedFunnel(ctx, dto)
	if err != nil {
		return nil, err
	}

	maxLeads := uc.recipientCap(ctx, dto.AccountID)
	if remaining, ok := uc.monthlyLeadsRemaining(ctx, dto.AccountID); ok {
		if remaining <= 0 {
			return nil, domain.ErrLeadMonthlyLimitReached
		}
		if remaining < maxLeads {
			maxLeads = remaining
		}
	}

	res := &ProspectingResult{LeadIDs: []string{}, ResultsCount: len(selected)}
	for _, item := range selected {
		if res.LeadsCreated >= maxLeads {
			break
		}
		if strings.TrimSpace(item.ExternalID) == "" {
			res.SkippedNoContact++
			continue
		}

		contact, err := uc.upsertContact(ctx, dto.AccountID, item.ExternalID, item.Phone,
			prospecting.Candidate{
				PlaceID: item.PlaceID, Name: item.Name, Phone: item.Phone,
				Address: item.Address, Website: item.Website,
			})
		if err != nil {
			res.SkippedNoContact++
			continue
		}

		// Re-checked at import time, not trusted from the preview: minutes
		// may have passed, and a colleague importing the same search in
		// parallel is exactly how the same business ends up worked twice.
		if existing, err := uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contact.ID); err == nil && existing != nil {
			res.SkippedDuplicate++
			continue
		}

		lead := &domain.Lead{
			ID:                ulid.Make().String(),
			AccountID:         dto.AccountID,
			FunnelID:          funnel.ID,
			ContactID:         contact.ID,
			Status:            domain.LeadStatusProspected,
			Source:            "google_places",
			CollectedMetadata: json.RawMessage("{}"),
			Enrichment: prospectingEnrichment(prospecting.Candidate{
				PlaceID: item.PlaceID, Name: item.Name, Phone: item.Phone,
				Address: item.Address, Website: item.Website,
			}),
		}
		if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
			res.SkippedNoContact++
			continue
		}
		uc.cases.LeadCreated(ctx, dto.AccountID, lead.ID, lead.Source)
		res.LeadsCreated++
		res.LeadIDs = append(res.LeadIDs, lead.ID)
	}
	return res, nil
}

// ownedFunnel resolves the funnel and enforces the account claim. Shared so
// the ownership check can never drift between preview and import.
func (uc *ProspectingUseCase) ownedFunnel(ctx context.Context, dto ProspectingSearchDTO) (*domain.SalesFunnel, error) {
	funnel, err := uc.funnelRepo.GetByID(ctx, dto.FunnelID)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}
	return funnel, nil
}

// alreadyLead answers the dedup question WITHOUT creating a contact.
//
// The preview must not write: a search somebody looks at and walks away from
// would otherwise leave a trail of contacts they never asked for.
func (uc *ProspectingUseCase) alreadyLead(ctx context.Context, accountID, funnelID, externalID string) bool {
	if uc.contactRepo == nil || uc.leadRepo == nil {
		return false
	}
	contact, err := uc.contactRepo.GetByExternalID(ctx, accountID, externalID)
	if err != nil || contact == nil {
		return false
	}
	existing, err := uc.leadRepo.GetByFunnelAndContact(ctx, funnelID, contact.ID)
	return err == nil && existing != nil
}
