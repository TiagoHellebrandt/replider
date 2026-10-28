package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/prospecting"
	"github.com/replider/api/internal/core/domain"
)

// hardProspectingCap bounds how many candidates a single search collects,
// independent of the plan cap, so a misconfigured request can never page the
// directory provider unboundedly.
const hardProspectingCap = 120

// prospectingProvider is the narrow port the use case depends on — satisfied by
// the Google Places adapter (prospecting.GooglePlacesAdapter) and by a fake in
// tests. Declared here (rather than taking the concrete type) so the HTTP layer
// can be swapped out under unit test.
type prospectingProvider interface {
	TextSearch(ctx context.Context, p prospecting.TextSearchParams) ([]prospecting.Candidate, error)
	NearbySearch(ctx context.Context, p prospecting.NearbySearchParams) ([]prospecting.Candidate, error)
}

// ProspectingUseCase runs a directory search for a funnel and turns the
// resulting businesses into prospected leads (US-022). It dedups contacts by
// (account_id, external_id) — phone when available, place_id otherwise — so a
// re-run of the same search never creates duplicate contacts or leads, and it
// caps the number of leads created per run at the account's per-campaign
// recipient limit.
type ProspectingUseCase struct {
	// eventRepo counts this month's leads for the tier cap (US-M013).
	eventRepo domain.SalesEventRepository
	// cases records generated leads for the case report (US-M011). Nil is
	// a valid state — instrumentation never blocks prospecting.
	cases *CaseRecorder

	funnelRepo  domain.SalesFunnelRepository
	contactRepo domain.ContactRepository
	leadRepo    domain.LeadRepository
	searchRepo  domain.ProspectingSearchRepository
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	provider    prospectingProvider
	// identityRepo indexes prospected phones as contact identities
	// (US-F005). Optional — see SetIdentityRepo.
	identityRepo domain.ContactIdentityRepository
	// placesQuota / placesDailyCap bound Places requests per account per
	// day (a platform cost rail). Optional — see SetPlacesDailyCap.
	placesQuota    *placesDailyCounter
	placesDailyCap int
}

// SetCaseRecorder wires case instrumentation (US-M011).
func (uc *ProspectingUseCase) SetCaseRecorder(r *CaseRecorder) {
	uc.cases = r
}

func NewProspectingUseCase(
	funnelRepo domain.SalesFunnelRepository,
	contactRepo domain.ContactRepository,
	leadRepo domain.LeadRepository,
	searchRepo domain.ProspectingSearchRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	provider prospectingProvider,
) *ProspectingUseCase {
	return &ProspectingUseCase{
		funnelRepo:  funnelRepo,
		contactRepo: contactRepo,
		leadRepo:    leadRepo,
		searchRepo:  searchRepo,
		accountRepo: accountRepo,
		planRepo:    planRepo,
		provider:    provider,
	}
}

// ProspectingSearchDTO carries the search criteria. Either a free-text Query
// (Text Search) or a positive RadiusMeters around Latitude/Longitude (Nearby
// Search) is required; a Query takes precedence when both are present.
type ProspectingSearchDTO struct {
	AccountID    string
	FunnelID     string
	Query        string
	Latitude     float64
	Longitude    float64
	RadiusMeters float64
	LanguageCode string

	// MaxResults is the ceiling the caller asked for; 0 means "no request" and the
	// plan's limit is the only one. It NARROWS the plan's ceiling, never widens
	// it — a number in the chat does not buy allowance.
	MaxResults int
}

// ProspectingResult summarises a search run for the UI.
type ProspectingResult struct {
	SearchID         string   `json:"search_id"`
	ResultsCount     int      `json:"results_count"`
	LeadsCreated     int      `json:"leads_created"`
	SkippedDuplicate int      `json:"skipped_duplicate"`
	SkippedNoContact int      `json:"skipped_no_contact"`
	LeadIDs          []string `json:"lead_ids"`
}

// Execute runs the search and creates leads. Ownership of the funnel is
// enforced against the account claim. The number of leads created is capped at
// the account's per-campaign recipient limit (the plan gate); the directory
// search is asked for at most that many candidates.
func (uc *ProspectingUseCase) Execute(ctx context.Context, dto ProspectingSearchDTO) (*ProspectingResult, error) {
	if uc.provider == nil {
		return nil, domain.ErrProspectingNotConfigured
	}

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

	query := strings.TrimSpace(dto.Query)
	if query == "" && dto.RadiusMeters <= 0 {
		return nil, domain.ErrProspectingInputRequired
	}

	maxLeads := uc.recipientCap(ctx, dto.AccountID)
	// The tier's MONTHLY lead allowance (US-M013) narrows the per-run cap.
	// Counted from the instrumentation log rather than a second counter,
	// so the number the customer is charged against is the same one the
	// case report shows them.
	if remaining, ok := uc.monthlyLeadsRemaining(ctx, dto.AccountID); ok {
		if remaining <= 0 {
			return nil, domain.ErrLeadMonthlyLimitReached
		}
		if remaining < maxLeads {
			maxLeads = remaining
		}
	}
	// Whoever asked for a quantity gets that quantity. Without this, "find 5
	// clinics" imported the plan's ceiling — 56 leads at once, every Places page
	// billed, and the user's request silently discarded.
	if dto.MaxResults > 0 && dto.MaxResults < maxLeads {
		maxLeads = dto.MaxResults
	}

	candidates, err := uc.search(ctx, dto, query, maxLeads)
	if err != nil {
		return nil, err
	}

	res := &ProspectingResult{LeadIDs: []string{}}
	res.ResultsCount = len(candidates)

	for _, cand := range candidates {
		if res.LeadsCreated >= maxLeads {
			break
		}
		externalID, phone := candidateExternalID(cand)
		if externalID == "" {
			// No phone and no place_id — nothing to key a contact on.
			res.SkippedNoContact++
			continue
		}

		contact, err := uc.upsertContact(ctx, dto.AccountID, externalID, phone, cand)
		if err != nil {
			res.SkippedNoContact++
			continue
		}

		// Dedup: a lead already exists for this (funnel, contact) pair, so the
		// search re-surfaced a known business. Leave it untouched.
		existing, err := uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contact.ID)
		if err == nil && existing != nil {
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
			Enrichment:        prospectingEnrichment(cand),
		}
		if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
			res.SkippedNoContact++
			continue
		}
		uc.cases.LeadCreated(ctx, dto.AccountID, lead.ID, lead.Source)
		res.LeadsCreated++
		res.LeadIDs = append(res.LeadIDs, lead.ID)
	}

	uc.recordSearch(ctx, dto, query, res)
	return res, nil
}

// recipientCap resolves the account's per-campaign recipient cap (the plan
// gate for how many prospects one run may create), clamped to a hard ceiling.
func (uc *ProspectingUseCase) recipientCap(ctx context.Context, accountID string) int {
	limit := hardProspectingCap
	if uc.accountRepo != nil {
		if account, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && account != nil {
			plan := domain.TrialPlan()
			if account.PlanID != nil && *account.PlanID != "" && uc.planRepo != nil {
				if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
					plan = p
				}
			}
			if rc := account.EffectiveMaxRecipientsPerCampaign(plan); rc > 0 && rc < limit {
				limit = rc
			}
		}
	}
	return limit
}

// monthlyLeadsRemaining reports how many new leads the tier still allows
// this calendar month. (0, false) means "no cap applies" — an unwired
// event repo, an unknown plan, or a tier that predates the cap.
//
// A lookup failure returns "no cap" on purpose: a commercial ceiling must
// not become an outage in the feature that generates the customer's leads.
func (uc *ProspectingUseCase) monthlyLeadsRemaining(ctx context.Context, accountID string) (int, bool) {
	if uc.eventRepo == nil || uc.accountRepo == nil || uc.planRepo == nil {
		return 0, false
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.PlanID == nil {
		return 0, false
	}
	plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
	if err != nil || plan == nil {
		return 0, false
	}
	limit := account.EffectiveMaxLeadsPerMonth(plan)
	if limit <= 0 {
		return 0, false
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	used, err := uc.eventRepo.CountSince(ctx, accountID, domain.SalesEventLeadCreated, monthStart)
	if err != nil {
		return 0, false
	}
	return limit - int(used), true
}

// SetEventRepo wires the monthly lead cap's counter (US-M013).
func (uc *ProspectingUseCase) SetEventRepo(r domain.SalesEventRepository) {
	uc.eventRepo = r
}

// search dispatches to Text or Nearby search and maps provider errors to
// actionable domain errors.
func (uc *ProspectingUseCase) search(ctx context.Context, dto ProspectingSearchDTO, query string, max int) ([]prospecting.Candidate, error) {
	// Counted BEFORE the request: a refused search must cost nothing, and a
	// failed one still spent the request.
	if err := uc.placesQuota.take(dto.AccountID, uc.placesDailyCap); err != nil {
		return nil, err
	}
	var (
		candidates []prospecting.Candidate
		err        error
	)
	if query != "" {
		candidates, err = uc.provider.TextSearch(ctx, prospecting.TextSearchParams{
			Query:        query,
			LanguageCode: dto.LanguageCode,
			MaxResults:   max,
		})
	} else {
		candidates, err = uc.provider.NearbySearch(ctx, prospecting.NearbySearchParams{
			Latitude:     dto.Latitude,
			Longitude:    dto.Longitude,
			RadiusMeters: dto.RadiusMeters,
			LanguageCode: dto.LanguageCode,
			MaxResults:   max,
		})
	}
	if err != nil {
		switch {
		case errors.Is(err, prospecting.ErrAPIKeyMissing):
			return nil, domain.ErrProspectingNotConfigured
		case errors.Is(err, prospecting.ErrRateLimited):
			return nil, domain.ErrProspectingRateLimited
		default:
			return nil, &domain.AppError{Code: "PROSPECTING_SEARCH_FAILED", Message: "prospecting search failed: " + err.Error()}
		}
	}
	return candidates, nil
}

// upsertContact creates or refreshes the contact for one candidate.
//
// Since US-F005 a new contact gets an opaque ULID; external_id remains the
// channel-facing key (the phone in E.164, or the Google place_id when the
// business published no phone) that Upsert dedupes on. The phone, when we
// have one, is also indexed as a contact identity so a later email-first
// touch resolves to the same person.
func (uc *ProspectingUseCase) upsertContact(ctx context.Context, accountID, externalID, phone string, cand prospecting.Candidate) (*domain.Contact, error) {
	contact := &domain.Contact{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ExternalID: externalID,
		Metadata:   prospectingEnrichment(cand),
	}
	channelType := "whatsapp"
	contact.ChannelType = &channelType
	if name := strings.TrimSpace(cand.Name); name != "" {
		contact.Name = &name
		// A Places hit IS an organization.
		contact.CompanyName = &name
	}
	if phone != "" {
		contact.Phone = &phone
	}
	// Through the resolver: a business the account already has — imported
	// from a spreadsheet, typed in by hand, or written to us on WhatsApp —
	// is reused under whichever key that door wrote, never duplicated.
	saved, _, err := newContactResolver(uc.contactRepo, uc.identityRepo).
		resolve(ctx, contact, contactResolveOpts{})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// registerPhoneIdentity indexes the prospect's phone (US-F005). Best-effort:
// the index is a lookup accelerator, never a reason to fail an import that
// already persisted the contact.
func (uc *ProspectingUseCase) registerPhoneIdentity(ctx context.Context, contact *domain.Contact, phone string) {
	if uc.identityRepo == nil || contact == nil || phone == "" {
		return
	}
	if _, err := uc.identityRepo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: contact.AccountID,
		ContactID: contact.ID,
		Kind:      domain.IdentityKindPhone,
		Value:     phone,
	}); err != nil {
		slog.WarnContext(ctx, "prospecting: failed to register phone identity",
			"contact_id", contact.ID, "error", err)
	}
}

// SetIdentityRepo wires the contact-identity index (US-F005).
func (uc *ProspectingUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identityRepo = r
}

// recordSearch persists the audit row best-effort — a logging failure must not
// fail the search the admin already paid the API call for.
func (uc *ProspectingUseCase) recordSearch(ctx context.Context, dto ProspectingSearchDTO, query string, res *ProspectingResult) {
	if uc.searchRepo == nil {
		return
	}
	row := &domain.ProspectingSearch{
		ID:           ulid.Make().String(),
		AccountID:    dto.AccountID,
		FunnelID:     dto.FunnelID,
		Query:        query,
		ResultsCount: res.ResultsCount,
		LeadsCreated: res.LeadsCreated,
	}
	if dto.RadiusMeters > 0 {
		lat, lng, radius := dto.Latitude, dto.Longitude, dto.RadiusMeters
		row.Latitude, row.Longitude, row.RadiusMeters = &lat, &lng, &radius
	}
	if err := uc.searchRepo.Create(ctx, row); err == nil {
		res.SearchID = row.ID
	}
}

// candidateExternalID picks the dedup key for a candidate: the canonical
// contact key of its phone (digits, no '+' — domain.ContactKey) when present,
// otherwise its place_id. Returns ("", "") when neither is usable.
func candidateExternalID(cand prospecting.Candidate) (externalID, phone string) {
	if e164, ok := domain.NormalizeE164(cand.Phone); ok {
		key, _ := domain.ContactKey(e164, "")
		return key, e164
	}
	if id := strings.TrimSpace(cand.PlaceID); id != "" {
		return id, ""
	}
	return "", ""
}

// prospectingEnrichment captures the directory fields we don't have a first-
// class column for (place_id, address, website) as a JSON blob, reused for both
// the contact metadata and the lead's initial enrichment.
func prospectingEnrichment(cand prospecting.Candidate) json.RawMessage {
	m := map[string]string{"source": "google_places"}
	if v := strings.TrimSpace(cand.PlaceID); v != "" {
		m["place_id"] = v
	}
	if v := strings.TrimSpace(cand.Address); v != "" {
		m["address"] = v
	}
	if v := strings.TrimSpace(cand.Website); v != "" {
		m["website"] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}
