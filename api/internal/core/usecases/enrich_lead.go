package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// EnrichLeadUseCase drives on-demand decision-maker enrichment for a lead
// (US-023). The HTTP path returns 202 immediately: Start validates ownership +
// BYOK synchronously (so a non-Gemini account gets ENRICHMENT_REQUIRES_GEMINI
// right away), flips the lead to `enriching`, then runs the grounded search in
// the background and stores the result on leads.enrichment, moving the lead to
// `enriched`.
type EnrichLeadUseCase struct {
	leadRepo    domain.LeadRepository
	contactRepo domain.ContactRepository
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	// eventRepo counts the month's enrichments for the tier cap, and
	// cases records each run. Both optional (SetCaseInstrumentation):
	// unwired, enrichment behaves exactly as before.
	eventRepo domain.SalesEventRepository
	cases     *CaseRecorder
	// meter prices the enrichment call (US-M015).
	meter *AIMeter
	// spend is the budget-only gate derived from the meter. Nil allows.
	spend   *spendGate
	factory domain.LeadEnrichmentFactory

	// dispatch runs the background enrichment. Defaults to a detached
	// goroutine; tests override it to run synchronously.
	dispatch func(func())
}

func NewEnrichLeadUseCase(
	leadRepo domain.LeadRepository,
	contactRepo domain.ContactRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	factory domain.LeadEnrichmentFactory,
) *EnrichLeadUseCase {
	return &EnrichLeadUseCase{
		leadRepo:    leadRepo,
		contactRepo: contactRepo,
		accountRepo: accountRepo,
		planRepo:    planRepo,
		factory:     factory,
		dispatch:    func(f func()) { go f() },
	}
}

// Start validates the lead + account BYOK and kicks off async enrichment. It
// returns synchronously (the caller responds 202) after the lead is flipped to
// `enriching`. Synchronous errors: LEAD_NOT_FOUND (or cross-account),
// ENRICHMENT_REQUIRES_GEMINI, BYOK_NOT_CONFIGURED.
// SetCaseInstrumentation wires the monthly enrichment cap and its counter
// (US-M013 / US-M011).
func (uc *EnrichLeadUseCase) SetCaseInstrumentation(repo domain.SalesEventRepository, rec *CaseRecorder) {
	uc.eventRepo = repo
	uc.cases = rec
}

// SetAIMeter wires transparent token metering (US-M015).
func (uc *EnrichLeadUseCase) SetAIMeter(m *AIMeter) {
	uc.meter = m
	uc.spend = budgetGateFromMeter(uc.accountRepo, m)
}

func (uc *EnrichLeadUseCase) Start(ctx context.Context, accountID, leadID string) error {
	lead, err := uc.leadRepo.GetByID(ctx, leadID)
	if err != nil {
		return err
	}
	// Treat missing and cross-account identically so a guessed id can't leak
	// another tenant's lead.
	if lead == nil || lead.AccountID != accountID {
		return domain.ErrLeadNotFound
	}

	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && *account.PlanID != "" {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}

	// Monthly enrichment allowance (US-M013). Checked BEFORE the provider
	// is resolved, so an account at its ceiling is told why instead of
	// spending an AI call to find out.
	if limit := account.EffectiveMaxEnrichmentsPerMonth(plan); limit > 0 && uc.eventRepo != nil {
		now := time.Now()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		if used, err := uc.eventRepo.CountSince(ctx, accountID, domain.SalesEventEnrichmentRun, monthStart); err == nil && used >= int64(limit) {
			return &domain.AppError{
				Code:    "PLAN_LIMIT_EXCEEDED_ENRICHMENTS",
				Message: "você atingiu o limite de enriquecimentos do seu plano neste mês",
			}
		}
	}

	if err := uc.spend.ensureAnyFor(ctx, account); err != nil {
		return err
	}
	provider, err := uc.factory.ProviderForAccount(ctx, account, plan)
	if err != nil {
		return err
	}
	uc.cases.EnrichmentRun(ctx, accountID, lead.ID)

	priorStatus := lead.Status
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, domain.LeadStatusEnriching); err != nil {
		return err
	}
	lead.Status = domain.LeadStatusEnriching

	// Detach from the request context so the goroutine survives the response.
	bg := context.WithoutCancel(ctx)
	uc.dispatch(func() {
		uc.complete(bg, provider, lead, priorStatus, plan)
	})
	return nil
}

// complete runs the grounded search and persists the result. It is best-effort:
// an empty answer still moves the lead to `enriched`; a hard provider failure
// logs and restores the prior status so the lead is not stuck `enriching`.
func (uc *EnrichLeadUseCase) complete(ctx context.Context, provider domain.LeadEnrichmentProvider, lead *domain.Lead, priorStatus domain.LeadStatus, plan *domain.Plan) {
	// Inject the company name (the contact's name = business name from
	// prospecting) so the provider can ground the search on it.
	if company := uc.companyName(ctx, lead); company != "" {
		lead.Enrichment = withCompanyName(lead.Enrichment, company)
	}

	data, err := provider.Enrich(ctx, lead)
	// Metered whatever the outcome: the grounded search ran and the
	// provider charged for it even when it found nothing.
	uc.meter.Record(ctx, lead.AccountID, "enrichment", plan.AIProvider, plan.AIModel, data.Usage)
	if err != nil {
		// Nothing to search for is not a failure to retry — it is a missing
		// field on the lead. Record WHY on the lead itself so the panel can
		// say it, instead of showing "we know nothing yet" beside a button
		// that would do nothing again.
		if errors.Is(err, domain.ErrEnrichmentNothingToGround) {
			lead.Status = priorStatus
			lead.Enrichment = withBlockedReason(lead.Enrichment, domain.EnrichmentBlockedNoIdentity)
			if persistErr := uc.leadRepo.Upsert(ctx, lead); persistErr != nil {
				slog.Error("lead enrichment: failed to record blocked reason", "lead_id", lead.ID, "error", persistErr)
			}
			return
		}
		slog.Warn("lead enrichment failed", "lead_id", lead.ID, "error", err)
		if restoreErr := uc.leadRepo.UpdateStatus(ctx, lead.ID, priorStatus); restoreErr != nil {
			slog.Error("lead enrichment: failed to restore status", "lead_id", lead.ID, "error", restoreErr)
		}
		return
	}

	lead.Enrichment = mergeEnrichment(lead.Enrichment, data)
	lead.Status = domain.LeadStatusEnriched
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		slog.Error("lead enrichment: failed to persist result", "lead_id", lead.ID, "error", err)
	}
}

// companyName resolves the contact's name (the prospected business name).
// Since US-F005 lead.contact_id is an opaque ULID, so the primary lookup is
// by id; the external_id lookup stays as a fallback for leads written by an
// older binary (or hand-built in a test), where contact_id IS the phone.
func (uc *EnrichLeadUseCase) companyName(ctx context.Context, lead *domain.Lead) string {
	if uc.contactRepo == nil {
		return ""
	}
	contact, err := uc.contactRepo.GetByID(ctx, lead.ContactID, lead.AccountID)
	if err != nil || contact == nil {
		contact, err = uc.contactRepo.GetByExternalID(ctx, lead.AccountID, lead.ContactID)
	}
	if err != nil || contact == nil || contact.Name == nil {
		return ""
	}
	return *contact.Name
}

// withCompanyName sets/overwrites the company_name key on the lead's enrichment
// JSON object so the provider can read it without a separate signature param.
func withCompanyName(enrichment json.RawMessage, company string) json.RawMessage {
	m := decodeEnrichmentMap(enrichment)
	m["company_name"] = company
	return reencode(m, enrichment)
}

// mergeEnrichment overlays the discovered EnrichmentData onto the lead's
// existing enrichment blob, preserving the prospecting seed (source, place_id,
// address, website) while adding the decision-maker + social fields.
func mergeEnrichment(enrichment json.RawMessage, data domain.EnrichmentData) json.RawMessage {
	m := decodeEnrichmentMap(enrichment)
	// A search ran, so whatever blocked the previous attempt no longer holds.
	delete(m, domain.EnrichmentBlockedKey)
	setIfNonEmpty(m, "decision_maker_name", data.DecisionMakerName)
	setIfNonEmpty(m, "decision_maker_role", data.DecisionMakerRole)
	setIfNonEmpty(m, "linkedin_url", data.LinkedInURL)
	setIfNonEmpty(m, "instagram_url", data.InstagramURL)
	// The findings, each with the page it was read from. Overwritten rather than
	// appended: a re-search is a fresh reading of the same company, and merging
	// two runs would leave a fact from six months ago sitting next to the one
	// that replaced it with nothing to tell them apart.
	if len(data.Findings) > 0 {
		m["findings"] = data.Findings
	}
	if len(data.Sources) > 0 {
		m["sources"] = data.Sources
	}
	if !data.EnrichedAt.IsZero() {
		m["enriched_at"] = data.EnrichedAt
	}
	return reencode(m, enrichment)
}

// withBlockedReason records why an enrichment attempt did no work.
func withBlockedReason(enrichment json.RawMessage, reason string) json.RawMessage {
	m := decodeEnrichmentMap(enrichment)
	m[domain.EnrichmentBlockedKey] = reason
	return reencode(m, enrichment)
}

func decodeEnrichmentMap(enrichment json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(enrichment) > 0 {
		_ = json.Unmarshal(enrichment, &m)
	}
	return m
}

func setIfNonEmpty(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// reencode marshals the map back to JSON, falling back to the original blob on
// the (practically impossible) marshal error so enrichment is never corrupted.
func reencode(m map[string]any, fallback json.RawMessage) json.RawMessage {
	b, err := json.Marshal(m)
	if err != nil {
		return fallback
	}
	return b
}
