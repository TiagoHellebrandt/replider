package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// SignalOfferUseCase turns a matched buying signal into a drafted offer in
// the approval queue (tese-agente-autonomo): a tender the account watches
// becomes a lead in the default funnel, with the tender's facts in its
// enrichment, and an AI draft waiting for a human yes. Nothing is sent —
// the queue's send gate is untouched.
//
// Everything here is best-effort: a signal that fails to become an offer is
// still a signal in the feed, and the next poll does not retry it (the
// LinkLead stamp marks it worked) — a salesperson can always draft it by
// hand from the queue.
// signalLeadLinker is the one signal-repo method this flow writes.
type signalLeadLinker interface {
	LinkLead(ctx context.Context, id, accountID, leadID string) error
	// Get is what the MANUAL path needs: the automatic flow already holds
	// the row it just stored, but a seller pressing "trabalhar este" sends
	// an id and nothing else.
	Get(ctx context.Context, id, accountID string) (*domain.Signal, error)
}

// signalParkReader bridges a tender's buyer to the health registry. Narrow
// and optional: a tender whose buyer is not a single establishment still
// becomes a lead, just without the equipment behind it.
type signalParkReader interface {
	FindEstablishmentByCNPJ(cnpj string) (*domain.CNESEstablishment, error)
	ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error)
}

type SignalOfferUseCase struct {
	signals  signalLeadLinker
	contacts domain.ContactRepository
	// identities is optional (SetIdentityRepo) — the index every door
	// consults before creating a contact.
	identities domain.ContactIdentityRepository
	// park is optional (SetParkReader) — see crossRegistry.
	park     signalParkReader
	funnels  domain.SalesFunnelRepository
	leads    domain.LeadRepository
	approach approachDrafter
}

// SetParkReader wires the tender→registry bridge.
func (uc *SignalOfferUseCase) SetParkReader(p signalParkReader) { uc.park = p }

// approachDrafter is the slice of ApproachUseCase this flow spends: one
// draft, with every cap that implies. An interface so tests exercise the
// flow without assembling the whole AI pipeline.
type approachDrafter interface {
	Draft(ctx context.Context, accountID, leadID string) (*domain.Lead, error)
}

func NewSignalOfferUseCase(
	signals signalLeadLinker,
	contacts domain.ContactRepository,
	funnels domain.SalesFunnelRepository,
	leads domain.LeadRepository,
	approach approachDrafter,
) *SignalOfferUseCase {
	return &SignalOfferUseCase{
		signals:  signals,
		contacts: contacts,
		funnels:  funnels,
		leads:    leads,
		approach: approach,
	}
}

// OfferFromSignal creates the lead + draft for one stored signal.
// WorkSignal puts a published tender in the funnel and stops there.
//
// The door for a seller who wants to do it themselves. Until now the only way
// from a signal to a lead was the automatic flow below, which always spends an
// AI draft on the account's own key — so a tender the seller wanted to chase by
// hand had no button at all: read the notice (and leave the product), open a
// draft that may not exist, or dismiss it. Meanwhile an expiring CONTRACT could
// be worked by hand, which made the asymmetry visible on the same screen.
//
// Nothing is written and nothing is sent. What the seller gets is the buyer in
// their customer list with the tender's facts attached — and, because a public
// notice publishes no e-mail and no phone, a card whose next piece of work is
// exactly the one the record cannot supply: finding out who to call.
//
// Idempotent: a signal already linked returns its existing lead. Pressing twice
// is navigation, not a second card for the same hospital.
func (uc *SignalOfferUseCase) WorkSignal(ctx context.Context, accountID, signalID string) (*domain.Lead, error) {
	signalID = strings.TrimSpace(signalID)
	if accountID == "" || signalID == "" {
		return nil, domain.ErrSignalNotFound
	}
	s, err := uc.signals.Get(ctx, signalID, accountID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, domain.ErrSignalNotFound
	}
	if s.LeadID != "" {
		lead, err := uc.leads.GetByID(ctx, s.LeadID)
		// Ownership is re-checked here and not trusted from the link: the
		// lead id came off a row, and a row is not an authorization.
		if err == nil && lead != nil && lead.AccountID == accountID {
			return lead, nil
		}
		// The link points at a lead that is gone (deleted funnel, cleanup).
		// Fall through and mint a fresh card rather than refuse the seller.
	}
	return uc.placeInFunnel(ctx, s)
}

func (uc *SignalOfferUseCase) OfferFromSignal(ctx context.Context, s *domain.Signal) error {
	if s == nil || s.AccountID == "" || s.ID == "" {
		return nil
	}
	if s.LeadID != "" || s.DismissedAt != nil {
		return nil // already worked or waved away
	}

	lead, err := uc.placeInFunnel(ctx, s)
	if err != nil {
		return err
	}

	// The draft rides the normal pipeline: dossier, matcher, spend caps. A
	// cap refusal leaves the lead in the funnel undrafted — real state,
	// the queue's draft button still works when the cycle resets.
	if _, err := uc.approach.Draft(ctx, s.AccountID, lead.ID); err != nil {
		slog.WarnContext(ctx, "signal offer: draft refused; lead kept without draft",
			"account_id", s.AccountID, "signal_id", s.ID, "lead_id", lead.ID, "error", err)
	}
	return nil
}

// placeInFunnel is everything both paths share: the buyer becomes a contact,
// the tender becomes a card, and the signal is marked worked. The ONLY thing
// the automatic path adds on top is the AI draft — which is precisely what the
// manual one must not do.
func (uc *SignalOfferUseCase) placeInFunnel(ctx context.Context, s *domain.Signal) (*domain.Lead, error) {
	funnel, err := uc.defaultFunnel(ctx, s.AccountID)
	if err != nil || funnel == nil {
		return nil, fmt.Errorf("signal offer: no default funnel for account %s: %w", s.AccountID, err)
	}

	// The registry record behind the buyer, when the buyer IS one — read
	// once and used twice: to reach the organization, and to say what it
	// already owns.
	est := uc.establishmentFor(ctx, s)

	contact, err := uc.upsertOrgContact(ctx, s, est)
	if err != nil {
		return nil, fmt.Errorf("signal offer: contact upsert: %w", err)
	}

	lead := &domain.Lead{
		ID:                ulid.Make().String(),
		AccountID:         s.AccountID,
		FunnelID:          funnel.ID,
		ContactID:         contact.ID,
		Status:            domain.LeadStatusProspected,
		Source:            "pncp_signal",
		CollectedMetadata: json.RawMessage("{}"),
		Enrichment:        uc.crossRegistry(signalEnrichment(s), est),
	}
	if err := uc.leads.Upsert(ctx, lead); err != nil {
		return nil, fmt.Errorf("signal offer: lead upsert: %w", err)
	}

	// LINKED BEFORE THE DRAFT, and the order is the whole point.
	//
	// The dossier only carries a signal that is already linked to THIS lead
	// (addSignals refuses an unlinked one, correctly — an unlinked signal is
	// a guess). Drafting first meant the matcher and the prompt ran with the
	// tender invisible, and they said so: production offers carry the
	// recorded reason "não há dados específicos sobre a demanda do cliente
	// no contexto" while the tender object sat one row away.
	//
	// The result was an offer for multiparameter monitors answering a tender
	// for medical oxygen — a message that tells a buyer we did not read what
	// they published.
	//
	// Linked even if the draft then fails: the signal WAS worked into a lead,
	// and re-linking it every poll would mint duplicate leads.
	if err := uc.signals.LinkLead(ctx, s.ID, s.AccountID, lead.ID); err != nil {
		slog.WarnContext(ctx, "signal offer: could not link signal to lead",
			"signal_id", s.ID, "lead_id", lead.ID, "error", err)
	}
	return lead, nil
}

// defaultFunnel finds the account's "Clientes" funnel — every account has
// exactly one (created on signup, backfilled by migration 000198).
func (uc *SignalOfferUseCase) defaultFunnel(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	all, err := uc.funnels.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, f := range all {
		if f.IsDefault {
			return f, nil
		}
	}
	return nil, fmt.Errorf("no default funnel")
}

// upsertOrgContact registers the tender's buyer as a contact. The dedup key
// is the CNPJ when published, else the source's own id — a public body has
// no WhatsApp, and that is fine: the offer is preparation, the seller picks
// the channel.
func (uc *SignalOfferUseCase) upsertOrgContact(
	ctx context.Context, s *domain.Signal, est *domain.CNESEstablishment,
) (*domain.Contact, error) {
	externalID := strings.TrimSpace(s.CNPJ)
	if externalID == "" {
		externalID = "pncp:" + s.ExternalID
	}
	contact := &domain.Contact{
		ID:         ulid.Make().String(),
		AccountID:  s.AccountID,
		ExternalID: externalID,
	}
	if name := strings.TrimSpace(s.Organization); name != "" {
		contact.Name = &name
		contact.CompanyName = &name
	}
	opts := contactResolveOpts{}
	// "a public body has no WhatsApp" was true of the NOTICE, not of the
	// buyer: when the same CNPJ is one establishment in the health registry,
	// the phone and the address are published there. The card the seller
	// gets stops being one whose only next step is finding out who to call.
	if est != nil {
		if est.Phone != nil {
			if phone, ok := normalizeCNESPhone(*est.Phone); ok {
				contact.Phone = &phone
			}
		}
		if est.Email != nil {
			if e := strings.ToLower(strings.TrimSpace(*est.Email)); domain.IsEmailAddress(e) {
				contact.Email = &e
			}
		}
		// Same reason as the radar door: a registry address reaches the
		// organization, and thirty-three establishments share some of them.
		opts.EmailNotAKey = true
		// The radar files this hospital under "cnes:<id>" or its phone.
		// Named here so the tender does not mint a second card for a
		// hospital the nightly sweep already put in the funnel.
		opts.LegacyKeys = append(opts.LegacyKeys, "cnes:"+est.CNES)
	}
	// Same resolver as every other door, so a buyer the account already
	// knows (under this CNPJ) is reused rather than minted again.
	saved, _, err := newContactResolver(uc.contacts, uc.identities).resolve(ctx, contact, opts)
	return saved, err
}

// establishmentFor is the tender's buyer as the health registry knows it, or
// nil. Best-effort: the offer is worth making without it.
func (uc *SignalOfferUseCase) establishmentFor(ctx context.Context, s *domain.Signal) *domain.CNESEstablishment {
	if uc.park == nil || s == nil || strings.TrimSpace(s.CNPJ) == "" {
		return nil
	}
	est, err := uc.park.FindEstablishmentByCNPJ(s.CNPJ)
	if err != nil {
		slog.WarnContext(ctx, "signal offer: could not read the registry for this buyer",
			"signal_id", s.ID, "error", err)
		return nil
	}
	return est
}

// crossRegistry folds what the buyer already owns into the tender's facts.
//
// The two public sources answer different halves of the same question — the
// notice says what they are buying, the registry says what they run — and
// they were never joined: production on 2026-09-17 held 60 leads carrying a
// park and 12 carrying a tender, and not one carrying both. The draft can
// cite either; it could never say "vocês publicaram o edital X e já operam N
// destes aparelhos", which is the only sentence that proves we read anything.
func (uc *SignalOfferUseCase) crossRegistry(
	raw json.RawMessage, est *domain.CNESEstablishment,
) json.RawMessage {
	if est == nil || uc.park == nil {
		return raw
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return raw
	}
	m["cnes"] = est.CNES
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
	if items, _, err := uc.park.ParkByCNES(est.CNES); err == nil && len(items) > 0 {
		parts := make([]string, 0, len(items))
		for _, c := range items {
			parts = append(parts, fmt.Sprintf("%dx %s", c.QtExist, c.Name))
		}
		m["installed_park"] = strings.Join(parts, ", ")
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// SetIdentityRepo wires the contact-identity index.
func (uc *SignalOfferUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identities = r
}

// signalEnrichment flattens the tender into the lead's enrichment blob —
// the keys the dossier renders and the draft may cite.
func signalEnrichment(s *domain.Signal) json.RawMessage {
	m := map[string]any{
		"source":       "pncp",
		"edital":       s.Title,
		"orgao":        s.Organization,
		"edital_url":   s.URL,
		"edital_id":    s.ExternalID,
		"company_name": s.Organization,
	}
	if s.Description != "" {
		m["edital_descricao"] = s.Description
	}
	if s.ValueCents > 0 {
		// Through the shared formatter, not %.2f: that rendered a R$ 234 mil
		// tender as "R$ 234298.28" — dot-decimal, no grouping — and put that
		// exact string in front of the model as the value it may quote. A
		// number a Brazilian buyer would read as two hundred and thirty-four
		// is a number the message must never say out loud.
		m["edital_valor_estimado"] = domain.FormatMoneyCents(s.ValueCents, domain.CurrencyBRL)
	}
	if s.ClosesAt != nil {
		m["edital_prazo"] = s.ClosesAt.Format(time.DateOnly)
	}
	if s.CNPJ != "" {
		m["cnpj"] = s.CNPJ
	}
	if s.State != "" {
		m["uf"] = s.State
	}
	if s.City != "" {
		m["cidade"] = s.City
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
