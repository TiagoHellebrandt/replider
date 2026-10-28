package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// LeadDossierBuilder gathers what the account already knows about one lead.
//
// The pieces existed and never met. Co-purchase patterns learned from the
// account's own sales, what this customer has bought and when it runs out, the
// buying signals somebody recorded, the company's registered activity — all of
// it sat in its own table while the approach generator wrote from the funnel's
// persona and one product. The proposal was assembled from a fraction of what
// had already been paid for.
//
// Every source is OPTIONAL, and a source that fails is skipped rather than
// fatal. Nothing here is worth failing a draft over: a message written from
// less is still a message, and a salesperson staring at an error instead of a
// draft has lost more than the missing context was worth.
type LeadDossierBuilder struct {
	crossSell domain.CrossSellRepository
	portfolio domain.PortfolioRepository
	signals   domain.SignalRepository
	cnesPark  cnesParkReader
	mailbox   mailboxHistoryReader
	// contracts / contractFetch name the incumbent supplier from the public
	// procurement record — the only machine-readable source that can, since
	// CNES publishes equipment counts and never a brand.
	contracts     domain.PublicContractRepository
	contractFetch contractFetcher
}

func NewLeadDossierBuilder() *LeadDossierBuilder { return &LeadDossierBuilder{} }

// SetCrossSell wires the account's own co-purchase graph.
func (b *LeadDossierBuilder) SetCrossSell(r domain.CrossSellRepository) *LeadDossierBuilder {
	b.crossSell = r
	return b
}

// SetPortfolio wires what this customer already buys and when it runs out.
func (b *LeadDossierBuilder) SetPortfolio(r domain.PortfolioRepository) *LeadDossierBuilder {
	b.portfolio = r
	return b
}

// SetSignals wires recorded intent-to-buy events.
func (b *LeadDossierBuilder) SetSignals(r domain.SignalRepository) *LeadDossierBuilder {
	b.signals = r
	return b
}

// cnesParkReader is the slice of domain.CNESParkRepository the dossier
// needs: match the lead to a CNES id, read that id's park.
type cnesParkReader interface {
	// GetEstablishment resolves the id a hunted lead already carries in its
	// contact key — the cheapest and only exact answer.
	GetEstablishment(cnes string) (*domain.CNESEstablishment, error)
	FindEstablishmentByPhoneDigits(digits string) (*domain.CNESEstablishment, error)
	SearchEstablishments(query string, limit int) ([]domain.CNESEstablishment, error)
	ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error)
}

// SetCNESPark wires the public installed-park data (tese-agente-autonomo):
// when the lead resolves to a CNES establishment, its declared equipment
// enters the dossier as registry facts — checkable, so the draft may lean
// on them ("vi que vocês têm 12 ventiladores").
func (b *LeadDossierBuilder) SetCNESPark(r cnesParkReader) *LeadDossierBuilder {
	b.cnesPark = r
	return b
}

// mailboxHistoryReader is the slice of domain.DiscoveredContactRepository the
// dossier needs: the accumulated shape of the email relationship with one
// address. Envelope-only data, written by the mailbox discovery scan.
type mailboxHistoryReader interface {
	GetByAddresses(ctx context.Context, accountID string, addresses []string) ([]*domain.DiscoveredContact, error)
}

// SetMailboxHistory wires the connected mailbox's discovered relationship
// (issue #292): when the lead's address was seen in the customer's own Sent
// folder, the count, the date window and the last subject enter the dossier
// as facts — so a draft to somebody they have written to for years does not
// open like a cold call.
func (b *LeadDossierBuilder) SetMailboxHistory(r mailboxHistoryReader) *LeadDossierBuilder {
	b.mailbox = r
	return b
}

// howManySignalsToScan bounds the signal sweep.
//
// There is no "signals for this lead" query — a signal is linked to a lead only
// once somebody works it (Signal.LeadID). So the recent page is scanned and
// filtered in Go. Bounded because this runs on the draft path: a lead with no
// linked signal must not cost a table scan to find that out.
const howManySignalsToScan = 100

// crossSellInDossier is how many co-purchase suggestions travel.
//
// Three. A message that lists six things to buy is a catalogue, and the point
// of the pattern is one credible sentence, not a shopping list.
const crossSellInDossier = 3

// Build assembles the dossier. Never returns an error: a partial picture is the
// product working, and the caller has a draft to write either way.
func (b *LeadDossierBuilder) Build(
	ctx context.Context,
	accountID string,
	lead *domain.Lead,
	contact *domain.Contact,
) domain.LeadDossier {
	var d domain.LeadDossier
	if lead == nil {
		return d
	}

	b.addOwnHistory(ctx, accountID, lead, &d)
	b.addMailboxHistory(ctx, accountID, lead, contact, &d)
	b.addCoPurchase(ctx, accountID, lead, &d)
	b.addSignals(ctx, accountID, lead, &d)
	addCollected(lead, &d)
	addRegistryAndResearch(lead, &d)
	b.addInstalledPark(lead, contact, &d)
	// Who they buy from today. After the park on purpose: the park is what
	// they have, this is who put it there.
	b.addContracts(ctx, &d, lead, contact)
	noteTheGaps(lead, contact, &d)
	return d
}

// addMailboxHistory brings in the shape of the email relationship the
// customer already has with this address — count, date window, last subject.
// All of it envelope data from their own Sent folder; the draft may say "faz
// tempo que não nos falamos" or pick the last thread back up, but the label
// forbids asserting anything about what those messages agreed.
func (b *LeadDossierBuilder) addMailboxHistory(
	ctx context.Context,
	accountID string,
	lead *domain.Lead,
	contact *domain.Contact,
	d *domain.LeadDossier,
) {
	if b.mailbox == nil {
		return
	}
	address := leadEmailAddress(lead, contact)
	if address == "" {
		return
	}
	found, err := b.mailbox.GetByAddresses(ctx, accountID, []string{address})
	if err != nil {
		slog.Warn("dossier: mailbox history read failed", "lead_id", lead.ID, "error", err)
		return
	}
	if len(found) == 0 || found[0] == nil || found[0].SentCount <= 0 {
		return
	}
	rel := found[0]

	value := fmt.Sprintf("%d mensagem(ns) enviada(s) da nossa caixa para este endereço", rel.SentCount)
	if rel.FirstSentAt != nil && rel.LastSentAt != nil {
		value += fmt.Sprintf(", entre %s e %s",
			rel.FirstSentAt.Format("2006-01-02"), rel.LastSentAt.Format("2006-01-02"))
	} else if rel.LastSentAt != nil {
		value += fmt.Sprintf(", a última em %s", rel.LastSentAt.Format("2006-01-02"))
	}
	if s := strings.TrimSpace(rel.LastSubject); s != "" {
		value += fmt.Sprintf(". Último assunto: %q", s)
	}
	d.Add(domain.DossierFact{
		Kind:   domain.DossierOwnMailbox,
		Label:  "Já nos falamos por e-mail",
		Value:  value,
		Source: "varredura da caixa conectada (só envelopes)",
	})
}

// leadEmailAddress resolves the address the mailbox relationship is keyed by:
// the contact's own email, else a contact id that IS an address (email-channel
// leads store the address there).
func leadEmailAddress(lead *domain.Lead, contact *domain.Contact) string {
	if contact != nil && contact.Email != nil && domain.IsEmailAddress(*contact.Email) {
		return strings.ToLower(strings.TrimSpace(*contact.Email))
	}
	if lead != nil && domain.IsEmailAddress(lead.ContactID) {
		return strings.ToLower(strings.TrimSpace(lead.ContactID))
	}
	return ""
}

// parkItemsInDossier bounds the equipment list: enough to write "vi que
// vocês têm tomógrafo e 12 ventiladores", short of a full inventory dump
// that would drown the rest of the dossier.
const parkItemsInDossier = 10

// addInstalledPark resolves the lead to a CNES establishment (phone digits
// first — the strongest bridge — then an unambiguous name hit) and brings
// its declared equipment in as registry facts.
func (b *LeadDossierBuilder) addInstalledPark(lead *domain.Lead, contact *domain.Contact, d *domain.LeadDossier) {
	if b.cnesPark == nil {
		return
	}
	est := b.matchEstablishment(lead, contact)
	if est == nil {
		return
	}
	cnesID := strings.TrimSpace(est.CNES)
	park, comp, err := b.cnesPark.ParkByCNES(cnesID)
	if err != nil {
		slog.Warn("dossier: cnes park read failed", "lead_id", lead.ID, "cnes", cnesID, "error", err)
		return
	}
	if len(park) == 0 {
		return
	}
	shown := park
	if len(shown) > parkItemsInDossier {
		shown = shown[:parkItemsInDossier]
	}
	parts := make([]string, 0, len(shown))
	for _, p := range shown {
		parts = append(parts, fmt.Sprintf("%d %s", p.QtExist, parkItemLabel(p)))
	}
	value := strings.Join(parts, ", ")
	if len(park) > parkItemsInDossier {
		value += fmt.Sprintf(" (e mais %d tipos)", len(park)-parkItemsInDossier)
	}
	d.Add(domain.DossierFact{
		Kind:   domain.DossierRegistry,
		Label:  fmt.Sprintf("Parque de equipamentos declarado (%s)", est.TradeName),
		Value:  value,
		Source: fmt.Sprintf("CNES/DATASUS %s, competência %s", cnesID, domain.CompetenciaYearMonth(comp)),
	})
}

// parkItemLabel renders one park line honestly: a code the dictionary
// does not name stays a CODE ("código CNES 0565", with the official group
// when 000219 sourced one) — never dropped, never a guessed name.
func parkItemLabel(p domain.CNESParkItem) string {
	if !domain.IsCNESRawCodeName(p.Name) {
		return p.Name
	}
	label := domain.CNESUnknownCodeName(strings.TrimSpace(p.EquipCode))
	if g := strings.TrimSpace(p.GroupName); g != "" {
		label += " (" + g + ")"
	}
	return label
}

// matchEstablishment bridges lead→CNES. Phone digits win (a number is an
// identity); a name only counts when the search returns EXACTLY one hit —
// guessing a hospital and then asserting its equipment would put an
// invented fact in a signed message.
func (b *LeadDossierBuilder) matchEstablishment(lead *domain.Lead, contact *domain.Contact) *domain.CNESEstablishment {
	// The id the lead already carries, before any guessing. A lead the park
	// hunt created is keyed "cnes:<id>"; guessing by phone and name while the
	// answer sits in the key is how the park went missing on exactly the leads
	// the product created itself.
	if est := b.establishmentByKey(lead, contact); est != nil {
		return est
	}
	digits := leadPhoneDigits(lead, contact)
	if digits != "" {
		if est, err := b.cnesPark.FindEstablishmentByPhoneDigits(digits); err == nil && est != nil {
			return est
		}
	}
	if contact != nil && contact.Name != nil {
		name := strings.TrimSpace(*contact.Name)
		if len(name) >= 6 { // "Santa" alone matches half the country
			if found, err := b.cnesPark.SearchEstablishments(name, 2); err == nil && len(found) == 1 {
				return &found[0]
			}
		}
	}
	return nil
}

// establishmentByKey resolves the contact's own "cnes:<id>" key.
func (b *LeadDossierBuilder) establishmentByKey(lead *domain.Lead, contact *domain.Contact) *domain.CNESEstablishment {
	keys := make([]string, 0, 2)
	if contact != nil {
		keys = append(keys, contact.ExternalID)
	}
	if lead != nil {
		keys = append(keys, lead.ContactID)
	}
	for _, key := range keys {
		id := cnesIDFromKey(key)
		if id == "" {
			continue
		}
		if est, err := b.cnesPark.GetEstablishment(id); err == nil && est != nil {
			return est
		}
	}
	return nil
}

// leadPhoneDigits extracts the bare national digits the registry compares
// against: strip everything non-numeric, then the 55 country code.
func leadPhoneDigits(lead *domain.Lead, contact *domain.Contact) string {
	raw := ""
	if contact != nil && contact.Phone != nil && strings.TrimSpace(*contact.Phone) != "" {
		raw = *contact.Phone
	} else if lead != nil {
		raw = lead.ContactID
	}
	var sb strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	digits := sb.String()
	digits = strings.TrimPrefix(digits, "55")
	if len(digits) < 8 {
		return ""
	}
	return digits
}

// addOwnHistory records what this customer has bought from us, and when it runs
// out. The strongest material in an approach after their own words: it is not a
// guess about what they might need, it is what they already chose.
func (b *LeadDossierBuilder) addOwnHistory(
	ctx context.Context, accountID string, lead *domain.Lead, d *domain.LeadDossier,
) {
	if b.portfolio == nil || lead.ContactID == "" {
		return
	}
	customer, err := b.portfolio.Get(ctx, accountID, lead.ContactID)
	if err != nil {
		slog.Warn("dossier: portfolio read failed", "lead_id", lead.ID, "error", err)
		return
	}
	if customer == nil {
		return
	}
	if customer.Purchases > 0 {
		d.Add(domain.DossierFact{
			Kind:  domain.DossierOwnSales,
			Label: "Compras com a gente",
			Value: fmt.Sprintf("%d", customer.Purchases),
		})
	}
	// The reorder date is what turns "we sell masks" into "yours run out around
	// the 12th" — the difference between a pitch and a reason to reply.
	if due := customer.DaysUntilDue(time.Now()); due != nil {
		d.Add(domain.DossierFact{
			Kind:  domain.DossierOwnSales,
			Label: "Previsão de acabar o estoque",
			Value: reorderPhrase(*due),
		})
	}
}

// addCoPurchase brings the account's own co-purchase graph to bear: what
// customers who buy what THIS customer buys also take, with the numbers behind
// it.
func (b *LeadDossierBuilder) addCoPurchase(
	ctx context.Context, accountID string, lead *domain.Lead, d *domain.LeadDossier,
) {
	if b.crossSell == nil || lead.ContactID == "" {
		return
	}
	bought, err := b.crossSell.ItemsBoughtBy(ctx, accountID, lead.ContactID)
	if err != nil {
		slog.Warn("dossier: items-bought read failed", "lead_id", lead.ID, "error", err)
		return
	}
	if items := joinItems(bought); items != "" {
		d.Add(domain.DossierFact{
			Kind:  domain.DossierOwnSales,
			Label: "Já compra com a gente",
			Value: items,
		})
	}
	if len(bought) == 0 {
		// Nothing bought means no anchor, and a suggestion with no anchor is
		// the "everybody also buys gloves" noise the pattern floor exists to
		// keep out.
		return
	}
	pairs, err := b.crossSell.Pairs(ctx, accountID)
	if err != nil {
		slog.Warn("dossier: co-purchase read failed", "lead_id", lead.ID, "error", err)
		return
	}
	for _, s := range domain.SuggestCrossSell(bought, pairs, crossSellInDossier) {
		d.Add(domain.DossierFact{
			Kind:  domain.DossierPattern,
			Label: "Costuma acompanhar o que ele já compra",
			Value: s.Suggest,
			// The numerator and denominator, in words the salesperson can
			// repeat. Without them the line is an opinion in a confident voice.
			Evidence: fmt.Sprintf("%d clientes que compram %s também levam (%.0f%%)",
				s.Customers, s.Because, s.Confidence*100),
		})
	}
}

// addSignals records intent somebody already saw. A tender or a public notice
// is the one thing in a cold approach that is not cold.
func (b *LeadDossierBuilder) addSignals(
	ctx context.Context, accountID string, lead *domain.Lead, d *domain.LeadDossier,
) {
	if b.signals == nil {
		return
	}
	list, err := b.signals.List(ctx, accountID, howManySignalsToScan, 0)
	if err != nil {
		slog.Warn("dossier: signals read failed", "lead_id", lead.ID, "error", err)
		return
	}
	for _, s := range list {
		// Only signals already linked to THIS lead. An unlinked signal that
		// merely mentions a similar company is a guess, and attaching it would
		// have the message claim a tender the recipient never published.
		if s == nil || s.LeadID != lead.ID {
			continue
		}
		d.Add(domain.DossierFact{
			Kind:   domain.DossierSignal,
			Label:  "Sinal registrado",
			Value:  strings.TrimSpace(s.Title),
			Source: strings.TrimSpace(string(s.Source)),
		})
	}
}

// addCollected carries what the customer answered themselves. Highest weight in
// the dossier: they said it, so the message may state it back.
func addCollected(lead *domain.Lead, d *domain.LeadDossier) {
	for key, raw := range decodeEnrichmentMap(lead.CollectedMetadata) {
		if v := collectedValueOf(raw); v != "" {
			d.Add(domain.DossierFact{
				Kind:  domain.DossierCollected,
				Label: humaniseFieldKey(key),
				Value: v,
			})
		}
	}
}

// addRegistryAndResearch splits the enrichment blob by how each field was
// learned. It used to travel as one undifferentiated dump, so a registered
// activity code and a name a model read off a page arrived with identical
// authority.
func addRegistryAndResearch(lead *domain.Lead, d *domain.LeadDossier) {
	e := decodeEnrichmentMap(lead.Enrichment)
	sources := enrichmentSources(e)

	// The tender, when this lead came from one. Registry-grade because every
	// field is published on the PNCP and a buyer can check it against their
	// own notice — which is exactly why the message may state it.
	//
	// These five keys were written by signal_offer and read by nobody. The
	// draft had the object, the value, the deadline and the link one row
	// away and opened with "apoiamos a gestão de saúde com equipamentos
	// hospitalares". The landing page promises "proposta pronta pra cotação,
	// com prazo do edital no texto"; this is the row that makes that true.
	//
	// Ordered deliberately: what they are buying first, then when it closes.
	// A deadline is the reason to answer today, so it must not arrive after
	// the model has run out of attention.
	//
	// DossierTender, not DossierSignal: at DossierSignal's weight these facts
	// rendered below the registry and the inferred patterns, and every
	// tender-born draft in production ignored them. The tender is the reason
	// for the message, so it leads the dossier.
	tenderURL := enrichmentString(e, "edital_url")
	for _, f := range []struct{ key, label string }{
		{"edital", "Edital publicado (objeto)"},
		{"edital_prazo", "Prazo do edital"},
		{"edital_valor_estimado", "Valor estimado do edital"},
		{"edital_id", "Número do edital"},
	} {
		if v := enrichmentString(e, f.key); v != "" {
			d.Add(domain.DossierFact{
				Kind: domain.DossierTender, Label: f.label, Value: v,
				// The published notice, so the salesperson reviewing the
				// draft can open what it claims to answer. Empty for the
				// sources that publish no direct link.
				Source: tenderURL,
			})
		}
	}

	// Registered, checkable facts.
	for key, label := range map[string]string{
		"cnae_label": "Atividade registrada",
		"address":    "Endereço",
	} {
		if v := enrichmentString(e, key); v != "" {
			d.Add(domain.DossierFact{
				Kind: domain.DossierRegistry, Label: label, Value: v,
			})
		}
	}

	// What the research read, each fact with ITS OWN page.
	//
	// This used to walk four fixed keys and stamp every one with the union of
	// pages the call touched — so a salesperson checking where "quem decide"
	// came from could land on the page that actually supported the company's
	// activity. The findings carry their own citation, so the block now says
	// which page holds which fact, and hedges the ones the model would not
	// attribute.
	if added := addFindings(e, d); added {
		return
	}

	// No findings: an older enrichment, written before per-fact provenance
	// existed. Read the four legacy keys with the union of sources, which is the
	// most that answer can honestly support.
	for key, label := range researchLabels {
		if v := enrichmentString(e, key); v != "" {
			d.Add(domain.DossierFact{
				Kind: domain.DossierResearch, Label: label, Value: v,
				Source: sources,
			})
		}
	}
}

// researchLabels names the four fields an older enrichment could carry.
var researchLabels = map[string]string{
	"decision_maker_name": "Quem parece decidir",
	"decision_maker_role": "Cargo",
	"linkedin_url":        "LinkedIn",
	"instagram_url":       "Instagram",
}

// findingLabels turns a canonical field key into the question it answers, so the
// generator reads a fact rather than a column.
var findingLabels = map[string]string{
	"decision_maker_name": "Quem parece decidir",
	"decision_maker_role": "Cargo de quem decide",
	"linkedin_url":        "LinkedIn",
	"instagram_url":       "Instagram",
	"what_they_do":        "O que a empresa faz",
	"size":                "Porte",
	"region":              "Onde atua",
	"recent_event":        "Aconteceu recentemente",
	"current_supplier":    "Com quem já trabalha",
	"pain_signal":         "Dificuldade que ela mesma declarou",
}

// addFindings renders per-fact research, and reports whether there was any.
//
// A finding the model refused to attribute, or was not sure about, still travels
// — marked. It makes a good question even when it cannot be stated, and dropping
// it would throw away research already paid for. What must never happen is such
// a fact reaching a stranger worded as established.
func addFindings(e map[string]any, d *domain.LeadDossier) bool {
	raw, ok := e["findings"].([]any)
	if !ok || len(raw) == 0 {
		return false
	}
	var added bool
	for _, item := range raw {
		f, ok := item.(map[string]any)
		if !ok {
			continue
		}
		field := normaliseKey(enrichmentString(f, "field"))
		value := enrichmentString(f, "value")
		if field == "" || value == "" {
			continue
		}
		label := findingLabels[field]
		if label == "" {
			// A fact nobody enumerated. Humanised rather than dropped: the
			// prompt invites it, and discarding it here would make the invitation
			// a lie.
			label = humaniseFieldKey(field)
		}
		source := enrichmentString(f, "source")
		confident, _ := f["confident"].(bool)
		if !confident || source == "" {
			// Said out loud, in the label, because the model's own hedge is
			// information the draft needs. "Parece" is a word a message can use;
			// silence is not.
			label = "Indício a confirmar — " + label
		}
		d.Add(domain.DossierFact{
			Kind: domain.DossierResearch, Label: label, Value: value,
			Source: hostOnly(source),
		})
		added = true
	}
	return added
}

// normaliseKey mirrors the adapter's normalisation so a key stored by an older
// binary still matches the label table.
func normaliseKey(field string) string {
	return strings.NewReplacer(" ", "_", "-", "_").Replace(
		strings.ToLower(strings.TrimSpace(field)))
}

// hostOnly keeps the domain, which is what answers "can I trust this?".
func hostOnly(url string) string {
	if url == "" {
		return ""
	}
	return sourceHost(url)
}

// noteTheGaps names what was looked for and not found.
//
// The generator has to know the difference between "no decision maker" and
// "decision maker unknown": the first invites an invented name, the second
// invites a question. A first message that ASKS who decides is a good first
// message; one that guesses is the mistake this whole dossier exists to avoid.
func noteTheGaps(lead *domain.Lead, contact *domain.Contact, d *domain.LeadDossier) {
	e := decodeEnrichmentMap(lead.Enrichment)
	// Findings first: a name the deeper research found lives there, and reading
	// only the legacy key would have the dossier state the decision maker in one
	// section and call it unknown in the next.
	if researchedValue(e, "decision_maker_name") == "" {
		d.MarkMissing("quem decide a compra")
	}
	if enrichmentString(e, "cnae_label") == "" {
		d.MarkMissing("o que a empresa faz oficialmente (atividade registrada)")
	}
	if contact == nil || contact.Name == nil || strings.TrimSpace(*contact.Name) == "" {
		d.MarkMissing("o nome de quem vai ler a mensagem")
	}
}

// researchedValue reads a researched field from wherever it was stored — the
// findings array, or the legacy top-level key.
//
// A hedged finding counts as known: it is rendered, so calling it missing in the
// same payload would contradict the section above it. "Indício a confirmar" is
// already the instruction to check rather than assert.
func researchedValue(e map[string]any, field string) string {
	if raw, ok := e["findings"].([]any); ok {
		for _, item := range raw {
			f, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if normaliseKey(enrichmentString(f, "field")) != field {
				continue
			}
			if v := enrichmentString(f, "value"); v != "" {
				return v
			}
		}
	}
	return enrichmentString(e, field)
}

// joinItems renders what a customer buys as one readable list, without
// repeating a label that differs only in case or accent.
func joinItems(items []string) string {
	seen := map[string]struct{}{}
	var out []string
	for _, label := range items {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key := domain.NormalizeItemLabel(label)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, label)
	}
	return strings.Join(out, ", ")
}

// collectedValueOf reads one answered field. The stored shape is either the
// bare value or an object carrying the value plus when it was collected; both
// exist in live data, so both are read.
func collectedValueOf(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%.2f", v), ".00")
	case bool:
		if v {
			return "sim"
		}
		return "não"
	case map[string]any:
		if inner, ok := v["value"]; ok {
			return collectedValueOf(inner)
		}
	}
	return ""
}

// reorderPhrase says when, in the words a person would use. A negative figure
// means it already ran out, which is a different sentence and a better reason to
// write today.
func reorderPhrase(days int) string {
	switch {
	case days < 0:
		return fmt.Sprintf("já passou do previsto há %d dia(s)", -days)
	case days == 0:
		return "hoje"
	case days == 1:
		return "amanhã"
	default:
		return fmt.Sprintf("em cerca de %d dias", days)
	}
}

// enrichmentSources renders the enrichment's source URLs as one citation, so a
// researched fact can name where it was read.
func enrichmentSources(e map[string]any) string {
	raw, ok := e["sources"].([]any)
	if !ok {
		return ""
	}
	var out []string
	for _, s := range raw {
		if str, ok := s.(string); ok && strings.HasPrefix(str, "http") {
			out = append(out, sourceHost(str))
		}
	}
	return strings.Join(out, ", ")
}

// sourceHost keeps the domain, which is what answers "can I trust this?". A raw
// search URL takes three lines and says nothing more.
func sourceHost(url string) string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	if slash := strings.IndexByte(trimmed, '/'); slash > 0 {
		trimmed = trimmed[:slash]
	}
	return strings.TrimPrefix(trimmed, "www.")
}

func enrichmentString(e map[string]any, key string) string {
	if v, ok := e[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// humaniseFieldKey turns a storage key into something a model reads as a
// question rather than as a column: `orcamento_previsto` becomes
// "Orcamento previsto".
func humaniseFieldKey(key string) string {
	words := strings.TrimSpace(strings.NewReplacer("_", " ", "-", " ").Replace(key))
	if words == "" {
		return key
	}
	return strings.ToUpper(words[:1]) + words[1:]
}
