package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// Turning a park hit into a queued offer (Ofertas phase 2).
//
// The radar shows "Hospital São Mateus tem 2 tomógrafos no CNES"; this is
// the "Preparar oferta" behind it: contact + lead in the default funnel +
// an AI draft in the approval queue. Same shape as OfferFromSignal on
// purpose — the CNES facts land in the lead's enrichment so the dossier can
// cite them (checkable registry facts, the same trust class the dossier
// already gives CNES), and NOTHING is sent from here.

type parkEstablishmentReader interface {
	GetEstablishment(cnes string) (*domain.CNESEstablishment, error)
	ResolveEquipmentCodes(nameQuery string) ([]domain.CNESEquipmentType, error)
	ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error)
}

type RadarParkOfferUseCase struct {
	park     parkEstablishmentReader
	contacts domain.ContactRepository
	// identities is optional (SetIdentityRepo): the index that says an
	// establishment's phone already belongs to a contact the account has.
	identities domain.ContactIdentityRepository
	leads      domain.LeadRepository
	funnels    domain.SalesFunnelRepository
	approach   approachDrafter
}

// SetIdentityRepo wires the contact-identity index.
func (uc *RadarParkOfferUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identities = r
}

func NewRadarParkOfferUseCase(
	park parkEstablishmentReader,
	contacts domain.ContactRepository,
	leads domain.LeadRepository,
	funnels domain.SalesFunnelRepository,
	approach approachDrafter,
) *RadarParkOfferUseCase {
	return &RadarParkOfferUseCase{
		park: park, contacts: contacts, leads: leads,
		funnels: funnels, approach: approach,
	}
}

// Offer creates the lead and queues the draft for one CNES establishment.
// Idempotency rides the contact's external id ("cnes:<id>"): a second call
// reuses the contact, and the lead upsert path dedups per funnel+contact.
func (uc *RadarParkOfferUseCase) Offer(ctx context.Context, accountID, cnes string) (*domain.Lead, error) {
	lead, existed, err := uc.EnsureLead(ctx, accountID, cnes)
	if err != nil {
		return nil, err
	}
	_ = existed
	// The draft rides the normal pipeline: dossier, matcher, product gate,
	// spend caps. A refusal surfaces to the caller — unlike the signal
	// auto-offer this is an interactive click, and "não deu" must say why.
	return uc.approach.Draft(ctx, accountID, lead.ID)
}

// EnsureLead is Offer without the draft: the establishment becomes a contact
// and a lead in the default funnel, and nothing is written.
//
// It exists because hunting and drafting are separate decisions. The daily
// hunt fills the funnel by itself; writing the approach costs a model call on
// the customer's own key, so that stays something a person asks for. Splitting
// here rather than copying keeps ONE implementation of "a CNES hit becomes a
// lead" — the identity rules (phone key, funnel+contact pair) are subtle
// enough that a second copy would drift.
//
// Reports whether the lead already existed, so a caller counting "new ground"
// does not count the same hospital every night.
func (uc *RadarParkOfferUseCase) EnsureLead(
	ctx context.Context, accountID, cnes string,
) (*domain.Lead, bool, error) {
	cnes = strings.TrimSpace(cnes)
	if cnes == "" {
		return nil, false, &domain.AppError{Code: "VALIDATION_ERROR", Message: "informe o cnes"}
	}
	est, err := uc.park.GetEstablishment(cnes)
	if err != nil {
		return nil, false, err
	}
	if est == nil {
		return nil, false, &domain.AppError{Code: "CNES_NOT_FOUND", Message: "estabelecimento não encontrado no cache CNES"}
	}

	funnel, err := uc.defaultFunnel(ctx, accountID)
	if err != nil {
		return nil, false, err
	}

	contact, opts := cnesContactDraft(accountID, cnes, est)
	resolver := newContactResolver(uc.contacts, uc.identities)

	// With no phone there is nothing to key on but the name, and the Places
	// hunt files the SAME hospital under its own place_id. Production ended
	// up with "Hospital Central de Alta Complexidade de Mato Grosso" twice
	// on the home screen — once from each source, both without a phone —
	// which means approaching one hospital twice with two different texts.
	//
	// Reused only on an UNAMBIGUOUS match, the same posture the dossier
	// bridge takes: "Santa Casa" names half the hospitals in the country,
	// and merging two of them is worse than showing one twice.
	//
	// The twin still gets the registry's blanks filled in — it is the same
	// hospital, and the address the registry publishes is exactly what the
	// approval queue was missing.
	if contact.Phone == nil {
		if twin := uc.soleContactNamed(ctx, accountID, est.TradeName); twin != nil {
			uc.completeFromRegistry(ctx, resolver, twin, contact)
			return uc.leadForExistingContact(ctx, accountID, cnes, est, funnel, twin)
		}
	}

	contact, _, err = resolver.resolve(ctx, contact, opts)
	if err != nil {
		return nil, false, fmt.Errorf("radar park offer: contact upsert: %w", err)
	}

	return uc.leadForExistingContact(ctx, accountID, cnes, est, funnel, contact)
}

// cnesContactDraft turns one registry record into the contact this account
// should hold for it, and the rules for recognising that contact if it is
// already there.
//
// ONE implementation, because the nightly sweep has to ask precisely what
// this writes. When they drifted — the writer keying by phone, the sweep
// looking for "cnes:<id>" — the sweep called 52 hospitals new ground every
// night and created none of them.
func cnesContactDraft(accountID, cnes string, est *domain.CNESEstablishment) (*domain.Contact, contactResolveOpts) {
	contact := &domain.Contact{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ExternalID: "cnes:" + cnes,
	}
	// "cnes:<id>" is what an older version of this door wrote; the CNPJ is
	// what the TENDER door writes for the same organization. A hospital the
	// account already met through a published notice must not gain a second
	// card because the registry knows it by another name.
	opts := contactResolveOpts{
		LegacyKeys:   []string{"cnes:" + cnes},
		EmailNotAKey: true,
	}
	if est == nil {
		return contact, opts
	}
	if name := strings.TrimSpace(est.TradeName); name != "" {
		contact.Name = &name
		contact.CompanyName = &name
	}
	// The CNES registry writes phones as "65 3618-8000"; the contact store
	// only accepts E.164. Normalize with the country prefix — and when it
	// still does not parse, keep the raw value in the enrichment only: a
	// lead without a phone beats a 500. A reachable establishment is keyed
	// the way every door keys a phone (domain.ContactKey), so the hospital
	// somebody imported from a spreadsheet is THIS contact, not a twin;
	// "cnes:<id>" stays the key only for one with no phone.
	if est.Phone != nil {
		if phone, ok := normalizeCNESPhone(*est.Phone); ok {
			contact.Phone = &phone
			if key, ok := domain.ContactKey(phone, ""); ok {
				contact.ExternalID = key
			}
		}
	}
	// The address the queue was waiting for.
	//
	// The registry publishes an e-mail for 3.208 of Mato Grosso's 5.266
	// establishments and this door copied the name and the phone and left it
	// behind. Measured on 2026-09-17: 48 of the account's 60 hunted
	// hospitals had a published address, none of their contacts carried one,
	// and 50 approved-ready drafts sat in the queue with nowhere to go. It
	// is filled, never used as a key (see EmailNotAKey).
	if est.Email != nil {
		if e := strings.ToLower(strings.TrimSpace(*est.Email)); domain.IsEmailAddress(e) {
			contact.Email = &e
		}
	}
	if est.CNPJ != nil {
		if d := onlyDigitsOf(*est.CNPJ); len(d) == 14 {
			opts.LegacyKeys = append(opts.LegacyKeys, d)
		}
	}
	return contact, opts
}

// completeFromRegistry fills the blanks of a contact we matched by name with
// what the registry publishes. Best-effort: a hospital in the funnel with a
// stale record beats no hospital at all.
func (uc *RadarParkOfferUseCase) completeFromRegistry(
	ctx context.Context, resolver *contactResolver, existing, draft *domain.Contact,
) {
	if resolver == nil || existing == nil || draft == nil {
		return
	}
	if !resolver.backfill(existing, draft) {
		return
	}
	if err := uc.contacts.Update(ctx, existing); err != nil {
		slog.WarnContext(ctx, "radar park offer: could not complete contact from registry",
			"contact_id", existing.ID, "error", err)
	}
}

// soleContactNamed returns the account's one contact carrying this name, or
// nil when there are none or more than one. Folded and case-insensitive: the
// CNES registry shouts ("HOSPITAL CENTRAL…") and Places writes title case.
func (uc *RadarParkOfferUseCase) soleContactNamed(ctx context.Context, accountID, name string) *domain.Contact {
	want := strings.ToLower(domain.FoldAccents(strings.TrimSpace(name)))
	if want == "" {
		return nil
	}
	// A short query would match half the account; the search is a prefilter
	// and the exact fold below is the decision.
	found, err := uc.contacts.ListByAccountID(ctx, accountID, strings.TrimSpace(name), 10, 0)
	if err != nil {
		// Best-effort: a search that fails must not stop the lead being
		// created. The cost is a duplicate, not a missing hospital.
		return nil
	}
	var hit *domain.Contact
	for _, c := range found {
		if c == nil || c.Name == nil {
			continue
		}
		if strings.ToLower(domain.FoldAccents(strings.TrimSpace(*c.Name))) != want {
			continue
		}
		if hit != nil {
			return nil // ambiguous
		}
		hit = c
	}
	return hit
}

// leadForExistingContact is the second half of EnsureLead, once the contact
// this ground belongs to is settled.
func (uc *RadarParkOfferUseCase) leadForExistingContact(
	ctx context.Context, accountID, cnes string, est *domain.CNESEstablishment,
	funnel *domain.SalesFunnel, contact *domain.Contact,
) (*domain.Lead, bool, error) {
	// A second pass must not mint a second lead: the funnel+contact pair is
	// the identity of this ground.
	if existing, err := uc.leads.GetByFunnelAndContact(ctx, funnel.ID, contact.ID); err == nil && existing != nil {
		return existing, true, nil
	}

	lead := &domain.Lead{
		ID:                ulid.Make().String(),
		AccountID:         accountID,
		FunnelID:          funnel.ID,
		ContactID:         contact.ID,
		Status:            domain.LeadStatusProspected,
		Source:            "cnes_park",
		CollectedMetadata: json.RawMessage("{}"),
		Enrichment:        parkEnrichment(cnes, est, uc.equipmentFacts(cnes)),
	}
	if err := uc.leads.Upsert(ctx, lead); err != nil {
		return nil, false, fmt.Errorf("radar park offer: lead upsert: %w", err)
	}
	return lead, false, nil
}

func (uc *RadarParkOfferUseCase) defaultFunnel(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	all, err := uc.funnels.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, f := range all {
		if f.IsDefault {
			return f, nil
		}
	}
	return nil, fmt.Errorf("no default funnel for account %s", accountID)
}

// normalizeCNESPhone turns a registry phone ("65 3618-8000") into E.164,
// assuming Brazil — CNES is a Brazilian registry.
func normalizeCNESPhone(raw string) (string, bool) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	if len(digits) == 10 || len(digits) == 11 {
		digits = "55" + digits
	}
	return domain.NormalizeE164("+" + digits)
}

// equipmentFacts reads what the establishment declares, best-effort — the
// offer works without it, it is just weaker.
func (uc *RadarParkOfferUseCase) equipmentFacts(cnes string) []domain.CNESParkItem {
	items, _, err := uc.park.ParkByCNES(cnes)
	if err != nil {
		return nil
	}
	return items
}

// parkEnrichment flattens the public facts into the lead's enrichment blob.
// Counts only: CNES publishes how many, never how old.
func parkEnrichment(cnes string, est *domain.CNESEstablishment, counts []domain.CNESParkItem) json.RawMessage {
	m := map[string]any{
		"source": "cnes_park",
		"cnes":   cnes,
	}
	if v := strings.TrimSpace(est.TradeName); v != "" {
		m["company_name"] = v
	}
	if v := strings.TrimSpace(est.Address); v != "" {
		m["address"] = v
	}
	if est.Phone != nil {
		if v := strings.TrimSpace(*est.Phone); v != "" {
			m["phone"] = v
		}
	}
	if est.Email != nil {
		if v := strings.ToLower(strings.TrimSpace(*est.Email)); v != "" {
			m["email"] = v
		}
	}
	if len(counts) > 0 {
		parts := make([]string, 0, len(counts))
		for _, c := range counts {
			parts = append(parts, fmt.Sprintf("%dx %s", c.QtExist, c.Name))
		}
		m["installed_park"] = strings.Join(parts, ", ")
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
