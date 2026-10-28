package usecases

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/replider/api/internal/core/domain"
)

// The Radar (Ofertas phase 2): every opportunity the system detected, in one
// feed, each item ending in a next step that lands work in the approval
// queue. The sources all existed — PNCP signals, the CNES park, reorder
// prediction, relationship dates — but each lived on its own surface (a tab,
// an MCP tool, a worker) and the seller had no single place to answer "what
// should I offer today?".
//
// Same posture as the command centre: items are DERIVED, never stored;
// collectors run in parallel and a failed one becomes a named warning, not an
// empty feed pretending nothing exists.

type RadarItemType string

const (
	RadarLicitacao RadarItemType = "licitacao"
	RadarParque    RadarItemType = "parque"
	RadarRecompra  RadarItemType = "recompra"
	RadarData      RadarItemType = "data"
	RadarCrossSell RadarItemType = "cross_sell"
	// The two forward-looking kinds (radar_contracts.go). Everything above
	// reacts to something that already happened; these two are the only rows
	// that exist BEFORE the purchase does.
	RadarContrato RadarItemType = "contrato"
	RadarAta      RadarItemType = "ata"
)

// RadarEdge is why this row can be reached before everyone else — and the two
// values are different claims, not degrees of the same one. Collapsing them
// would print "chegue antes do edital" on a row where no edital is coming.
type RadarEdge string

const (
	// RadarEdgeBeforeTender: the commitment runs out on a published date and
	// the replacement process has not been opened yet.
	RadarEdgeBeforeTender RadarEdge = "antes_do_edital"
	// RadarEdgeNoTender: an ata (or a buyer who rides other bodies' atas) can
	// buy without running a tender at all.
	RadarEdgeNoTender RadarEdge = "sem_licitacao"
)

type RadarItem struct {
	Type RadarItemType `json:"type"`
	// ID is stable per underlying fact (signal id, CNES id, contact id,
	// date id) so the client can key rows and a refresh does not reshuffle
	// identity.
	ID      string `json:"id"`
	Title   string `json:"title"`
	Context string `json:"context"`
	// ValueCents is the money attached, when the source publishes one.
	// Zero = not disclosed, never "zero reais".
	ValueCents int64      `json:"value_cents,omitempty"`
	Deadline   *time.Time `json:"deadline,omitempty"`
	// LeadID links to the queue draft the auto-offer already created
	// (licitação) — the CTA is then "open the draft", not "prepare".
	LeadID    string `json:"lead_id,omitempty"`
	ContactID string `json:"contact_id,omitempty"`
	// Matches is what of the catalogue fits what THIS establishment
	// declares — the same crossing the lead panel does, brought forward to
	// the radar so the seller sees the offer before opening anything.
	Matches []ParkProductMatch `json:"matches,omitempty"`
	// CNES identifies a park item's establishment for a future
	// draft-from-park call.
	CNES  string `json:"cnes,omitempty"`
	Phone string `json:"phone,omitempty"`
	URL   string `json:"url,omitempty"`
	// Competencia ("2026-07") is the DATASUS month a park item's counts
	// come from — in September the data is July (45–75 day lag), and the
	// card says so instead of implying "today".
	Competencia string `json:"competencia,omitempty"`

	// City and UF travel apart from Context so the card can put the place in
	// its own chip instead of burying it in the sentence.
	City string `json:"city,omitempty"`
	UF   string `json:"uf,omitempty"`

	// Incumbent is who holds the expiring commitment today, copied verbatim
	// from the published record. The ONLY place a competitor's name may come
	// from: the installed park publishes counts and never a brand, so a name
	// inferred from it would be a guess printed as a fact.
	Incumbent string `json:"incumbent,omitempty"`

	// Edge is the reason this row beats the market to the purchase. Empty for
	// the five reactive kinds, which have no such claim to make.
	Edge RadarEdge `json:"edge,omitempty"`

	// Ready says the offer is already written and waiting on a human. It is
	// what lets this feed absorb the approval queue instead of keeping it as
	// a tab: the row's own verb becomes "read it and send".
	Ready bool `json:"ready,omitempty"`

	// ── What the card cannot hold, and the panel needs ──────────────────
	//
	// The card shows two lines and truncates. That is right for a list you
	// scan, and wrong as the only place the information exists: on
	// 15/09/2026 a tender for "PEÇAS DE REPOSIÇÃO ORIGINAIS PARA ATENDER ÀS
	// NECESSIDADES DO LABORATÓRIO MUNICIPAL" showed up to the word
	// "NECESSIDADES" and was dismissed as out-of-trade on that evidence.
	//
	// It matters more than "nice to have" because 16 of this account's 62
	// tenders carry NO link: PNCP returns linkSistemaOrigem empty when the
	// buyer published none, and some of the links that do exist are broken
	// at the source. For a quarter of the feed this product's own screen is
	// the only place the opportunity can be read at all.

	// Detail is the source's own text, untruncated and uncomposed — the
	// tender's object, the reorder's history, the park's breakdown. Context
	// stays the one-line summary the card shows.
	Detail string `json:"detail,omitempty"`

	// CNPJ identifies the buyer. A public notice publishes no contact, so
	// this is what a seller actually chases: the registry, the phone, who
	// already sells to them.
	CNPJ string `json:"cnpj,omitempty"`

	// Reference is the source's own identifier, shown verbatim so a claim
	// built on it can be checked: the PNCP control number, the CNES id, the
	// sale. It is also the only way to find a notice whose link is missing.
	Reference string `json:"reference,omitempty"`

	// PublishedAt is when the source published it — the other half of the
	// clock. A deadline in five days reads differently if the notice went up
	// yesterday or three weeks ago.
	PublishedAt *time.Time `json:"published_at,omitempty"`

	// Buyer is what we already know about whoever is buying — whether they
	// are already a customer, where their card sits, and what the published
	// record says they have contracted. Nil when nothing is known.
	Buyer *BuyerContext `json:"buyer,omitempty"`

	// Email and Address are how the seller actually reaches this buyer.
	//
	// A published NOTICE carries neither, which is why this panel used to end
	// at the control number. The health registry carries both for most of the
	// territory — 3.208 of Mato Grosso's 5.266 establishments publish an
	// address — and the radar was reading that record already and dropping
	// everything but the phone.
	Email   string `json:"email,omitempty"`
	Address string `json:"address,omitempty"`

	// BuyerEntity is who signs the contracts for an establishment that has no
	// CNPJ of its own — a public hospital is usually a unit of a secretaria,
	// not a legal person. Kept apart from CNPJ on purpose: seven of Mato
	// Grosso's twenty biggest parks share one secretaria, so this identifies
	// the PURCHASER, never the hospital.
	BuyerEntityCNPJ string `json:"buyer_entity_cnpj,omitempty"`
	BuyerEntityName string `json:"buyer_entity_name,omitempty"`

	// Beds is what this hospital is BUILT to treat, beside what it owns.
	//
	// The park says which machines are in the building; the census says how
	// much care the place is set up to deliver, and that is the number that
	// sizes an order. A unit with twenty neonatal cots needs twenty
	// incubators and the monitors to match, on a replacement cycle — six
	// incubators in the park does not contradict that, it quantifies the gap.
	Beds []domain.CNESBedItem `json:"beds,omitempty"`

	// Habilitations is capability the MINISTRY FUNDS, which is a harder
	// signal than capability the hospital declares. A unit habilitated for
	// "UTI II Neonatal" with ten beds is paid against an ordinance and is
	// required to hold the equipment that entitles it. Measured over MT:
	// 548 funded beds across 191 hospitals, 120 of them neonatal.
	Habilitations []domain.CNESHabilitationItem `json:"habilitations,omitempty"`

	// Services is what the establishment says it PERFORMS — the qualifier
	// that separates a hospital running a neonatal service from one that
	// happens to have a cot. Capped at radarServicesPerRow: past a handful
	// it stops being a qualifier and becomes an inventory nobody reads.
	Services []domain.CNESServiceItem `json:"services,omitempty"`

	// AdminSphere is MUNICIPAL / ESTADUAL / FEDERAL / PRIVADA, and it decides
	// the sales motion: a public unit buys through a published tender, on a
	// deadline, against bids anyone can read; a private one buys when it
	// decides to. The product wrote the same approach for both because this
	// was never carried out of the registry.
	AdminSphere string `json:"admin_sphere,omitempty"`
	// Capabilities the buyer declares, named — "centro neonatal", "centro
	// obstétrico". Only what is TRUE is listed: an absent flag means the
	// registry did not say so, never that the hospital lacks it.
	Capabilities []string `json:"capabilities,omitempty"`

	// contactKeys are the external ids this buyer could already be filed
	// under, for the buyer-block lookup. Never serialized: it is how the
	// server recognises the row, not something the client should re-derive.
	contactKeys []string
}

type RadarResponse struct {
	Items []RadarItem `json:"items"`
	// Warnings names the collectors that failed — an empty feed is a claim,
	// and a query that blew up has not earned it.
	Warnings []string `json:"warnings"`
	// Park is how big the park opportunity actually is, beyond the page of
	// items above. Nil when the account has no territory or no catalogue
	// word that resolves to CNES equipment.
	Park *RadarParkUniverse `json:"park,omitempty"`
}

// RadarParkUniverse answers "how much am I missing?" — the whole point of
// the screen. Shown is what this response carries; Establishments is every
// hospital in the territory that declares equipment the catalogue matches.
type RadarParkUniverse struct {
	Establishments int    `json:"establishments"`
	Units          int    `json:"units"`
	Shown          int    `json:"shown"`
	Scope          string `json:"scope"`
	Competencia    string `json:"competencia,omitempty"`
	// Equipment names what the catalogue matched in the CNES dictionary,
	// so the number above is auditable instead of a bare claim.
	Equipment []string `json:"equipment,omitempty"`
}

// Narrow ports: the radar reads five sources and must not drag any of their
// full repositories (and mock churn) along.

// radarSignalReader is the worked-inclusive feed: a tender that the
// auto-offer already drafted stays on the Radar with an "open the draft"
// CTA, so the plain List (which hides worked rows for the /signals screen)
// is the wrong port here.
type radarSignalReader interface {
	ListForRadar(ctx context.Context, accountID string, limit, offset int) ([]*domain.Signal, error)
}

type radarParkReader interface {
	ResolveEquipmentCodes(nameQuery string) ([]domain.CNESEquipmentType, error)
	ResolveEquipmentCodesAny(words []string) ([]domain.CNESEquipmentType, error)
	CityCodesByName(uf string, names []string) ([]string, error)
	EstablishmentsWithEquipmentIn(uf string, cityCodes, equipCodes []string, minCount, limit, offset int) ([]domain.CNESEquipmentHolder, string, error)
	BedsForCNESIn(cnesIDs []string) (map[string][]domain.CNESBedItem, string, error)
	HabilitationsForCNESIn(cnesIDs []string) (map[string][]domain.CNESHabilitationItem, string, error)
	ServicesForCNESIn(cnesIDs []string) (map[string][]domain.CNESServiceItem, string, error)
	// ParkForCNESIn gives each card the park of ITS establishment, and
	// CountEquipmentIn gives the page the size of what it is not showing.
	ParkForCNESIn(cnesIDs, equipCodes []string) (map[string][]domain.CNESParkItem, string, error)
	CountEquipmentIn(uf string, cityCodes, equipCodes []string) (establishments, units int, err error)
}

type radarPortfolioReader interface {
	List(ctx context.Context, accountID string, limit, offset int) ([]*domain.PortfolioCustomer, error)
}

type radarRelationshipReader interface {
	List(ctx context.Context, accountID string) ([]*domain.RelationshipDate, error)
}

type radarCrossSellReader interface {
	ItemsBoughtBy(ctx context.Context, accountID, contactID string) ([]string, error)
	Pairs(ctx context.Context, accountID string) ([]domain.CoPurchasePair, error)
}

type RadarUseCase struct {
	signals      radarSignalReader
	park         radarParkReader
	portfolio    radarPortfolioReader
	relationship radarRelationshipReader
	crossSell    radarCrossSellReader
	products     domain.ProductRepository
	accountRepo  domain.AccountRepository
	// Both optional (see radar_contracts.go): an unwired radar keeps the five
	// original sources instead of failing, and no constructor call site moved.
	contracts radarContractReader
	leads     radarLeadReader
	// Both optional (see radar_buyer.go): what this account already knows
	// about the buyer, and what the published record says they have bought.
	buyerContacts radarContactReader
	buyerLeads    radarBuyerLeadReader
	buyerSales    radarSaleChecker
	buyerHistory  radarBuyerHistory
	now           func() time.Time
}

func NewRadarUseCase(
	signals radarSignalReader,
	park radarParkReader,
	portfolio radarPortfolioReader,
	relationship radarRelationshipReader,
	crossSell radarCrossSellReader,
	products domain.ProductRepository,
	accounts domain.AccountRepository,
) *RadarUseCase {
	return &RadarUseCase{
		signals: signals, park: park, portfolio: portfolio,
		relationship: relationship, crossSell: crossSell,
		products: products, accountRepo: accounts,
		now: time.Now,
	}
}

func (uc *RadarUseCase) WithClock(now func() time.Time) *RadarUseCase {
	uc.now = now
	return uc
}

// How far each source looks. Deliberately different: a tender has a legal
// deadline, a reorder has a lead time, a date is a calendar fact.
const (
	radarSignalLimit = 20
	// radarParkLimit was 5, which quietly answered "you have five
	// opportunities" for an account whose territory holds thousands. The
	// page now shows a real page of them and says how many there are in
	// total (ParkUniverse), because the size of what is being MISSED is
	// the reason to open this screen at all.
	radarParkLimit     = 25
	radarReorderWindow = 7 * 24 * time.Hour
	radarDateWindow    = 14 * 24 * time.Hour
	// radarCrossSellCustomers bounds the per-customer purchase reads; the
	// co-purchase PAIRS are read once for the whole account.
	radarCrossSellCustomers = 10
	radarCrossSellLimit     = 5
)

// Execute builds the feed from an account id alone. Rows come back unmarked
// (see markReady) because "the offer is already written" is a claim about a
// PERSON's work, and this entry point does not know which person is asking.
func (uc *RadarUseCase) Execute(ctx context.Context, accountID string) (*RadarResponse, error) {
	return uc.ExecuteFor(ctx, domain.Actor{AccountID: accountID})
}

// ExecuteFor is the same feed for a known caller, which is what the screen
// uses: the marks are what let one list replace the approval queue's tab.
func (uc *RadarUseCase) ExecuteFor(ctx context.Context, actor domain.Actor) (*RadarResponse, error) {
	accountID := actor.AccountID
	res := &RadarResponse{Items: []RadarItem{}, Warnings: []string{}}
	var mu sync.Mutex
	var wg sync.WaitGroup

	collect := func(name string, f func() ([]RadarItem, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := f()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Warnings = append(res.Warnings, name)
				return
			}
			res.Items = append(res.Items, items...)
		}()
	}

	collect("licitacao", func() ([]RadarItem, error) { return uc.collectSignals(ctx, accountID) })
	collect("parque", func() ([]RadarItem, error) {
		items, universe, err := uc.collectPark(ctx, accountID)
		if err == nil && universe != nil {
			// collect() calls this BEFORE taking its lock, so the write
			// to the shared response needs the mutex here.
			mu.Lock()
			res.Park = universe
			mu.Unlock()
		}
		return items, err
	})
	collect("recompra", func() ([]RadarItem, error) { return uc.collectReorders(ctx, accountID) })
	collect("data", func() ([]RadarItem, error) { return uc.collectDates(ctx, accountID) })
	collect("cross_sell", func() ([]RadarItem, error) { return uc.collectCrossSell(ctx, accountID) })
	// One collector, two kinds: the calendar returns contracts and atas
	// together and the split is a property of the row, not of the query.
	collect("contrato", func() ([]RadarItem, error) { return uc.collectContracts(ctx, accountID) })
	wg.Wait()

	// After the fan-in, because it answers a question ABOUT the collected
	// rows rather than producing any.
	uc.markReady(ctx, actor, res.Items)
	uc.annotateBuyers(ctx, accountID, res.Items)
	sortRadar(res.Items)
	sort.Strings(res.Warnings)
	return res, nil
}

// sortRadar: the thing that expires first comes first; among the deadline-less,
// source order is value order (a customer about to run out beats a cold park
// hit). Stable so each collector's own ordering survives ties.
func sortRadar(items []RadarItem) {
	rank := map[RadarItemType]int{
		RadarRecompra: 0, RadarLicitacao: 1, RadarAta: 2, RadarContrato: 3,
		RadarData: 4, RadarCrossSell: 5, RadarParque: 6,
	}
	sort.SliceStable(items, func(i, j int) bool {
		di, dj := items[i].Deadline, items[j].Deadline
		if di != nil && dj != nil && !di.Equal(*dj) {
			return di.Before(*dj)
		}
		if (di != nil) != (dj != nil) {
			return di != nil
		}
		return rank[items[i].Type] < rank[items[j].Type]
	})
}

func (uc *RadarUseCase) collectSignals(ctx context.Context, accountID string) ([]RadarItem, error) {
	signals, err := uc.signals.ListForRadar(ctx, accountID, radarSignalLimit, 0)
	if err != nil {
		return nil, err
	}
	now := uc.now()
	out := []RadarItem{}
	for _, s := range signals {
		if s.DismissedAt != nil || s.Expired(now) {
			continue
		}
		context := s.Description
		if context == "" {
			context = s.Title
		}
		place := strings.TrimSpace(strings.Trim(s.City+"/"+s.State, "/"))
		if place != "" {
			context = place + " · " + context
		}
		out = append(out, RadarItem{
			Type: RadarLicitacao, ID: s.ID,
			Title:      s.Organization,
			Context:    context,
			ValueCents: s.ValueCents,
			Deadline:   s.ClosesAt,
			LeadID:     s.LeadID,
			URL:        s.URL,
			City:       s.City,
			UF:         s.State,
			// The object as the buyer wrote it. Context is a composed summary
			// and truncates on the card; this is what the panel reads.
			Detail:      strings.TrimSpace(s.Title + "\n\n" + s.Description),
			CNPJ:        s.CNPJ,
			Reference:   s.ExternalID,
			PublishedAt: publishedOrNil(s.PublishedAt),
		})
	}
	return out, nil
}

// collectPark: establishments in the account's territory holding equipment
// that matches the catalogue by NAME against the CNES dictionary. Counts
// only — CNES publishes how many, never how old, and the item copy must not
// suggest otherwise.
func (uc *RadarUseCase) collectPark(ctx context.Context, accountID string) ([]RadarItem, *RadarParkUniverse, error) {
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, nil, fmt.Errorf("radar park: account: %w", err)
	}
	if account.Territory == nil || len(strings.TrimSpace(account.Territory.UF)) != 2 {
		return []RadarItem{}, nil, nil // no territory, nothing honest to hunt
	}
	// Cities narrow the hunt when the territory names them (and they
	// resolve); otherwise the whole UF, as before.
	scope := resolveTerritoryScope(uc.park, account.Territory)

	products, err := uc.products.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	// Resolved per WORD, not per full name: the dictionary says "Tomógrafo
	// Computadorizado", the catalogue says "Tomógrafo computadorizado GE" —
	// a whole-name ILIKE never matches the longer string. All the words of
	// the whole catalogue go in ONE query (this used to be N×4 ILIKEs per
	// GET).
	seen := map[string]bool{}
	words := []string{}
	for _, p := range products {
		for _, word := range equipmentWords(p.Name) {
			if !seen[word] {
				seen[word] = true
				words = append(words, word)
			}
		}
	}
	if len(words) == 0 {
		return []RadarItem{}, nil, nil
	}
	types, err := uc.park.ResolveEquipmentCodesAny(words)
	if err != nil {
		return nil, nil, err
	}
	if len(types) == 0 {
		// A consumables-only catalogue matches no CNES equipment type —
		// an honest empty, not a failure.
		return []RadarItem{}, nil, nil
	}
	codeSet := map[string]domain.CNESEquipmentType{}
	for _, t := range types {
		codeSet[t.Code] = t
	}
	codes := make([]string, 0, len(codeSet))
	names := make([]string, 0, len(codeSet))
	for code, t := range codeSet {
		codes = append(codes, code)
		names = append(names, t.Name)
	}
	sort.Strings(codes)
	sort.Strings(names)

	holders, comp, err := uc.park.EstablishmentsWithEquipmentIn(scope.UF, scope.CityCodes, codes, 1, radarParkLimit, 0)
	if err != nil {
		return nil, nil, err
	}

	// What each hospital actually holds. Before this, every card repeated
	// the SAME global list of matched equipment names, so "1164 Berço
	// Aquecido, Bomba De Infusao, …" and the next hospital's line were
	// word-for-word identical and told the seller nothing about either.
	ids := make([]string, 0, len(holders))
	for _, h := range holders {
		ids = append(ids, strings.TrimSpace(h.CNES))
	}
	parks, parkComp, err := uc.park.ParkForCNESIn(ids, codes)
	if err != nil {
		// The page is still worth showing with counts only; losing the
		// per-hospital breakdown must not lose the hospitals.
		parks = map[string][]domain.CNESParkItem{}
	}
	if parkComp != "" {
		comp = parkComp
	}
	// The bed census, best-effort and in one query for the page. It is extra
	// context on a row that already stands on its park, so a census that
	// cannot be read must not cost the account its hospitals.
	beds, _, err := uc.park.BedsForCNESIn(ids)
	if err != nil {
		beds = map[string][]domain.CNESBedItem{}
	}
	habs, _, err := uc.park.HabilitationsForCNESIn(ids)
	if err != nil {
		habs = map[string][]domain.CNESHabilitationItem{}
	}
	servs, _, err := uc.park.ServicesForCNESIn(ids)
	if err != nil {
		servs = map[string][]domain.CNESServiceItem{}
	}

	out := []RadarItem{}
	for _, h := range holders {
		cnes := strings.TrimSpace(h.CNES)
		title := "Estabelecimento CNES " + cnes
		phone, email, address, cnpj := "", "", "", ""
		entityCNPJ, entityName, sphere := "", "", ""
		var capabilities []string
		if h.Establishment != nil {
			sphere = strings.TrimSpace(h.Establishment.AdminSphere)
			// Listed only when declared true. A nil flag is the registry
			// staying silent, and rendering that as "no neonatal centre"
			// would be the product answering a question nobody asked it.
			if v := h.Establishment.HasNeonatalCentre; v != nil && *v {
				capabilities = append(capabilities, "centro neonatal")
			}
			if v := h.Establishment.HasObstetricCentre; v != nil && *v {
				capabilities = append(capabilities, "centro obstétrico")
			}
			if v := h.Establishment.HasSurgicalCenter; v != nil && *v {
				capabilities = append(capabilities, "centro cirúrgico")
			}
			if n := strings.TrimSpace(h.Establishment.TradeName); n != "" {
				title = n
			}
			if h.Establishment.Phone != nil {
				phone = strings.TrimSpace(*h.Establishment.Phone)
			}
			if h.Establishment.Email != nil {
				email = strings.ToLower(strings.TrimSpace(*h.Establishment.Email))
			}
			address = strings.TrimSpace(h.Establishment.Address)
			if h.Establishment.CNPJ != nil {
				if d := onlyDigitsOf(*h.Establishment.CNPJ); len(d) == 14 {
					cnpj = d
				}
			}
			// Only when the hospital has none of its own: with both present
			// the entity adds nothing the row cannot already say for itself.
			if cnpj == "" && h.Establishment.EntityCNPJ != nil {
				if d := onlyDigitsOf(*h.Establishment.EntityCNPJ); len(d) == 14 {
					entityCNPJ = d
					entityName = strings.TrimSpace(h.Establishment.LegalName)
				}
			}
		}
		park := parks[cnes]
		item := RadarItem{
			Type: RadarParque, ID: "cnes-" + cnes,
			Title: title,
			Context: fmt.Sprintf("%d aparelhos no CNES: %s · %s · dado público, quantidade — não idade",
				h.Total, describePark(park, names), scope.Label()),
			Matches: CrossCatalogueWithPark(products, park),
			CNES:    cnes,
			Phone:   phone,
			Email:   email,
			Address: address,
			// The registry's CNPJ is what turns a park row into a BUYER: it
			// is the key the published-contract record is indexed by, and the
			// key a tender files the same organization under.
			CNPJ:            cnpj,
			BuyerEntityCNPJ: entityCNPJ,
			BuyerEntityName: entityName,
			UF:              scope.UF,
			Competencia:     domain.CompetenciaYearMonth(comp),
			// Every type this establishment declares, not the four the card
			// has room for.
			Detail:        describeParkFull(park),
			Reference:     "CNES " + cnes,
			Beds:          beds[cnes],
			Habilitations: habs[cnes],
			Services:      capServices(servs[cnes]),
			AdminSphere:   sphere,
			Capabilities:  capabilities,
		}
		// Every spelling the CNES door files a hospital under, so the buyer
		// block recognises a card this account already holds. ONE source of
		// that truth — the same draft the lead writer builds.
		draft, probe := cnesContactDraft("", cnes, h.Establishment)
		item.contactKeys = append([]string{draft.ExternalID}, probe.LegacyKeys...)
		out = append(out, item)
	}

	// The universe behind the page: how many hospitals in the territory
	// this catalogue could reach at all.
	universe := &RadarParkUniverse{
		Establishments: len(out), Shown: len(out),
		Scope: scope.Label(), Competencia: domain.CompetenciaYearMonth(comp),
		Equipment: names,
	}
	if estabs, units, cerr := uc.park.CountEquipmentIn(scope.UF, scope.CityCodes, codes); cerr == nil {
		universe.Establishments, universe.Units = estabs, units
	}
	return out, universe, nil
}

// radarServicesPerRow bounds the service list. A big hospital declares
// dozens; past a handful the section stops qualifying the buyer and becomes
// an inventory dump the seller scrolls past.
const radarServicesPerRow = 8

func capServices(in []domain.CNESServiceItem) []domain.CNESServiceItem {
	if len(in) <= radarServicesPerRow {
		return in
	}
	return in[:radarServicesPerRow]
}

// radarParkNamesInCard bounds the per-card equipment list. Beyond a handful
// the line stops being a summary and becomes an inventory dump.
const radarParkNamesInCard = 4

// describePark names what THIS establishment holds, biggest count first
// (the query already orders that way), falling back to the catalogue-wide
// match list only when the breakdown could not be read — an honest
// degradation, flagged by the "…" the caller would otherwise not see.
func describePark(park []domain.CNESParkItem, fallback []string) string {
	if len(park) == 0 {
		if len(fallback) > radarParkNamesInCard {
			return strings.Join(fallback[:radarParkNamesInCard], ", ") + "…"
		}
		return strings.Join(fallback, ", ")
	}
	parts := make([]string, 0, radarParkNamesInCard)
	for _, it := range park {
		if len(parts) >= radarParkNamesInCard {
			parts = append(parts, fmt.Sprintf("+%d tipos", len(park)-radarParkNamesInCard))
			break
		}
		parts = append(parts, fmt.Sprintf("%d× %s", it.QtExist, it.Name))
	}
	return strings.Join(parts, ", ")
}

func (uc *RadarUseCase) collectReorders(ctx context.Context, accountID string) ([]RadarItem, error) {
	customers, err := uc.portfolio.List(ctx, accountID, 20, 0)
	if err != nil {
		return nil, err
	}
	now := uc.now()
	horizon := now.Add(radarReorderWindow)
	out := []RadarItem{}
	for _, c := range customers {
		if c.NextDueAt == nil || c.NextDueAt.After(horizon) {
			continue
		}
		due := *c.NextDueAt
		days := int(due.Sub(now).Hours() / 24)
		var when string
		if days < 0 {
			when = fmt.Sprintf("estimativa: acabou há %dd", -days)
		} else {
			when = fmt.Sprintf("estimativa: acaba em %dd", days)
		}
		out = append(out, RadarItem{
			Type: RadarRecompra, ID: "reorder-" + c.ContactID,
			Title:      c.ContactName,
			Context:    when + " · pelo ciclo das compras anteriores",
			ValueCents: 0,
			Deadline:   &due,
			ContactID:  c.ContactID,
			// A customer with no recorded purchase date still has a due
			// date (the cycle can be set by hand), and "01/01/0001" on a
			// card is worse than saying nothing.
			Detail: reorderDetail(c.LastPurchaseAt, due),
		})
	}
	return out, nil
}

func (uc *RadarUseCase) collectDates(ctx context.Context, accountID string) ([]RadarItem, error) {
	dates, err := uc.relationship.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	now := uc.now()
	horizon := now.Add(radarDateWindow)
	out := []RadarItem{}
	for _, d := range dates {
		next := nextOccurrence(d, now)
		if next == nil || next.After(horizon) {
			continue
		}
		title := d.Label
		if title == "" {
			title = string(d.Kind)
		}
		out = append(out, RadarItem{
			Type: RadarData, ID: "date-" + d.ID,
			Title:     title,
			Context:   next.Format("02/01"),
			Deadline:  next,
			ContactID: d.ContactID,
		})
	}
	return out, nil
}

// equipmentWords picks the tokens of a product name worth asking the CNES
// dictionary about: 5+ runes (shorter is "de"/"com"/"GE" noise), capped so a
// long descriptive name does not turn into a dozen queries.
func equipmentWords(name string) []string {
	fields := strings.Fields(strings.ToLower(name))
	out := make([]string, 0, 4)
	for _, f := range fields {
		// "tomógrafo," never matched anything; "equipamento" matched the
		// whole dictionary. Same stopword list the PNCP seeder uses.
		f = strings.TrimFunc(f, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if len([]rune(f)) < 5 || watchStopwords[f] {
			continue
		}
		out = append(out, f)
		if len(out) == 4 {
			break
		}
	}
	return out
}

// collectCrossSell: what a customer does NOT buy yet, anchored on what they
// do — the account's own co-purchase patterns, never a popularity guess.
// Pairs are read once; too few sales simply yields nothing (the honest
// first-month answer).
func (uc *RadarUseCase) collectCrossSell(ctx context.Context, accountID string) ([]RadarItem, error) {
	customers, err := uc.portfolio.List(ctx, accountID, radarCrossSellCustomers, 0)
	if err != nil {
		return nil, err
	}
	if len(customers) == 0 {
		return []RadarItem{}, nil
	}
	pairs, err := uc.crossSell.Pairs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return []RadarItem{}, nil
	}
	out := []RadarItem{}
	for _, c := range customers {
		if len(out) >= radarCrossSellLimit {
			break
		}
		bought, err := uc.crossSell.ItemsBoughtBy(ctx, accountID, c.ContactID)
		if err != nil || len(bought) == 0 {
			continue // one unreadable customer must not kill the collector
		}
		for _, sug := range domain.SuggestCrossSell(bought, pairs, 1) {
			out = append(out, RadarItem{
				Type: RadarCrossSell, ID: "xsell-" + c.ContactID,
				Title: c.ContactName,
				Context: fmt.Sprintf("compra %s e ainda não leva %s — %d clientes seus levam juntos",
					sug.Because, sug.Suggest, sug.Customers),
				ContactID: c.ContactID,
			})
		}
	}
	return out, nil
}

// nextOccurrence resolves a relationship date to its next concrete moment:
// the stored DueAt for a one-off, the next month/day recurrence otherwise.
func nextOccurrence(d *domain.RelationshipDate, now time.Time) *time.Time {
	if d.DueAt != nil {
		if d.DueAt.Before(now) {
			return nil // a lapsed one-off is history, not an opportunity
		}
		return d.DueAt
	}
	if d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > 31 {
		return nil
	}
	next := time.Date(now.Year(), time.Month(d.Month), d.Day, 0, 0, 0, 0, now.Location())
	if next.Before(now.Truncate(24 * time.Hour)) {
		next = next.AddDate(1, 0, 0)
	}
	return &next
}

// publishedOrNil keeps a zero time out of the payload. "Not published" and
// "published at the zero hour of year one" are different claims, and only one
// of them is true.
func publishedOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// describeParkFull lists everything the establishment declares, biggest first.
// describePark caps at four for the card; the panel is where the seller looks
// for the one type the summary cut.
func describeParkFull(park []domain.CNESParkItem) string {
	if len(park) == 0 {
		return ""
	}
	parts := make([]string, 0, len(park))
	for _, it := range park {
		parts = append(parts, fmt.Sprintf("%d× %s", it.QtExist, it.Name))
	}
	return strings.Join(parts, "\n")
}

// reorderDetail writes the history behind the estimate, and omits the half it
// does not have rather than printing a zero date as a fact.
func reorderDetail(last *time.Time, due time.Time) string {
	if last == nil || last.IsZero() {
		return fmt.Sprintf("O ciclo que você registrou para estes itens vence em %s.",
			due.Format("02/01/2006"))
	}
	return fmt.Sprintf("Última compra em %s. O ciclo que você registrou para estes itens vence em %s.",
		last.Format("02/01/2006"), due.Format("02/01/2006"))
}
