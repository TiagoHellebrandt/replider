package usecases

import (
	"context"
	"sort"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// The lead panel's CNES card (Clientes redesign): what the public registry
// says this establishment HOLDS — counts, never age. Reuses the dossier's
// bridge rules verbatim: phone digits are an identity; a name only counts
// when the search returns exactly one hit, because a guessed hospital's
// equipment on screen is an invented fact.

type LeadParkResult struct {
	CNES      string `json:"cnes"`
	TradeName string `json:"trade_name,omitempty"`
	// MatchedBy says how the bridge landed ("phone" | "name") so the UI can
	// qualify the claim honestly.
	MatchedBy string `json:"matched_by"`
	// Competencia ("2026-07") is the DATASUS month the counts come from —
	// the card says "dados CNES de 07/2026", never "hoje".
	Competencia string                `json:"competencia,omitempty"`
	Park        []domain.CNESParkItem `json:"park"`
	// Matches are the account's OWN products that fit this park.
	//
	// The answer to "what can I offer this hospital", available before
	// spending anything: the crossing is a string match between the
	// catalogue's words and the equipment the registry lists, so it needs no
	// model at all. The AI was only ever needed to WRITE the message; picking
	// what fits is arithmetic, and hiding it behind a paid call meant nobody
	// could see the opportunity without first paying for a draft.
	Matches []ParkProductMatch `json:"matches,omitempty"`
}

// ParkProductMatch is one product of yours against one thing they own.
type ParkProductMatch struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	// Equipment is the registry's own label for what they have, and QtExist
	// how many they declare — a count, never an age.
	Equipment string `json:"equipment"`
	QtExist   int    `json:"qt_exist"`
}

type LeadParkUseCase struct {
	leads    domain.LeadRepository
	contacts domain.ContactRepository
	park     dossierParkReader
	// products crosses the catalogue against the park. Optional.
	products domain.ProductRepository
}

// dossierParkReader is the same slice of the CNES repo the dossier uses.
type dossierParkReader interface {
	// GetEstablishment resolves the id a hunted lead already carries in its
	// contact key — the cheapest and only exact answer.
	GetEstablishment(cnes string) (*domain.CNESEstablishment, error)
	FindEstablishmentByPhoneDigits(digits string) (*domain.CNESEstablishment, error)
	SearchEstablishments(query string, limit int) ([]domain.CNESEstablishment, error)
	ParkByCNES(cnes string) ([]domain.CNESParkItem, string, error)
}

func NewLeadParkUseCase(leads domain.LeadRepository, contacts domain.ContactRepository, park dossierParkReader) *LeadParkUseCase {
	return &LeadParkUseCase{leads: leads, contacts: contacts, park: park}
}

// SetProducts wires the catalogue, so the panel can say what to offer.
// Optional: without it the park still renders, just without the crossing.
func (uc *LeadParkUseCase) SetProducts(r domain.ProductRepository) { uc.products = r }

// Execute answers (nil, nil) when nothing matches — an absent card is the
// honest render, not an error state.
func (uc *LeadParkUseCase) Execute(ctx context.Context, accountID, leadID string) (*LeadParkResult, error) {
	lead, err := uc.leads.GetByID(ctx, leadID)
	if err != nil || lead == nil || lead.AccountID != accountID {
		return nil, domain.ErrLeadNotFound
	}
	// GetByID takes (id, accountID) — in that order. This passed them
	// swapped, so the contact was NEVER loaded: the lookup asked for a
	// contact whose id is an account id, inside an account whose id is a
	// contact id. Both lookups then missed, contact stayed nil, and with no
	// contact there is no key, no phone and no name to match on — so this
	// endpoint answered 204 for every lead that ever asked, and the panel's
	// park section never rendered for anybody.
	//
	// The fallback keeps using lead.ContactID because on some leads it IS the
	// external id (a phone, or "cnes:<id>") rather than a row id.
	var contact *domain.Contact
	if c, err := uc.contacts.GetByID(ctx, lead.ContactID, accountID); err == nil && c != nil {
		contact = c
	} else if c, err := uc.contacts.GetByExternalID(ctx, accountID, lead.ContactID); err == nil && c != nil {
		contact = c
	}

	est, matchedBy := uc.match(lead, contact)
	if est == nil {
		return nil, nil
	}
	items, comp, err := uc.park.ParkByCNES(strings.TrimSpace(est.CNES))
	if err != nil || len(items) == 0 {
		return nil, nil
	}
	return &LeadParkResult{
		CNES:        strings.TrimSpace(est.CNES),
		TradeName:   strings.TrimSpace(est.TradeName),
		MatchedBy:   matchedBy,
		Competencia: domain.CompetenciaYearMonth(comp),
		Park:        items,
		Matches:     uc.crossCatalogue(ctx, accountID, items),
	}, nil
}

func (uc *LeadParkUseCase) match(lead *domain.Lead, contact *domain.Contact) (*domain.CNESEstablishment, string) {
	// The id the lead already carries. See cnes_key.go: a hunted lead is keyed
	// "cnes:<id>", and guessing while the answer sits in the key left the
	// panel empty for every lead the product found itself.
	for _, key := range []string{contactExternalID(contact), leadContactID(lead)} {
		if id := cnesIDFromKey(key); id != "" {
			if est, err := uc.park.GetEstablishment(id); err == nil && est != nil {
				return est, "cnes"
			}
		}
	}
	digits := leadPhoneDigits(lead, contact)
	if digits != "" {
		if est, err := uc.park.FindEstablishmentByPhoneDigits(digits); err == nil && est != nil {
			return est, "phone"
		}
	}
	if contact != nil && contact.Name != nil {
		name := strings.TrimSpace(*contact.Name)
		if len(name) >= 6 { // "Santa" alone matches half the country
			if found, err := uc.park.SearchEstablishments(name, 2); err == nil && len(found) == 1 {
				return &found[0], "name"
			}
		}
	}
	return nil, ""
}

func contactExternalID(c *domain.Contact) string {
	if c == nil {
		return ""
	}
	return c.ExternalID
}

func leadContactID(l *domain.Lead) string {
	if l == nil {
		return ""
	}
	return l.ContactID
}

// crossCatalogue answers "what of mine fits what they own".
//
// A word match, not a model call: the registry names the equipment
// ("Incubadora", "Respirador/Ventilador") and the product names carry the same
// words. Done in memory over a park of a dozen items and a catalogue of a few
// hundred — no query beyond the one that already loaded the catalogue.
//
// Deliberately conservative. A product matches only when one of ITS words
// appears in the equipment label, so "Incubadora Line Duo G" fits
// "Incubadora" and "Contrato de manutenção anual" fits nothing. A looser rule
// would pad the panel with products the hospital cannot use, and a list that
// is mostly wrong is worse than an empty one.
func (uc *LeadParkUseCase) crossCatalogue(
	ctx context.Context, accountID string, park []domain.CNESParkItem,
) []ParkProductMatch {
	if uc.products == nil || len(park) == 0 {
		return nil
	}
	products, err := uc.products.ListByAccountID(ctx, accountID)
	if err != nil || len(products) == 0 {
		return nil
	}
	return CrossCatalogueWithPark(products, park)
}

// CrossCatalogueWithPark is the crossing itself, over a catalogue already in
// hand. The radar pages twenty-five hospitals against ONE catalogue read, so
// the loading and the matching had to come apart; the lead panel keeps
// calling it through crossCatalogue.
func CrossCatalogueWithPark(
	products []*domain.Product, park []domain.CNESParkItem,
) []ParkProductMatch {
	if len(products) == 0 || len(park) == 0 {
		return nil
	}

	out := make([]ParkProductMatch, 0, 8)
	seen := map[string]bool{}
	for _, item := range park {
		if item.QtExist <= 0 {
			continue
		}
		// A code the dictionary never named cannot be matched by word — and
		// guessing from "código CNES 0565" would pair a product with an
		// equipment nobody can name.
		label := strings.TrimSpace(item.Name)
		if label == "" || domain.IsCNESRawCodeName(label) {
			continue
		}
		folded := domain.FoldAccents(strings.ToLower(label))
		for _, p := range products {
			if p == nil || !p.Active || seen[p.ID+"|"+item.EquipCode] {
				continue
			}
			if !productFitsEquipment(p.Name, folded) {
				continue
			}
			seen[p.ID+"|"+item.EquipCode] = true
			out = append(out, ParkProductMatch{
				ProductID:   p.ID,
				ProductName: p.Name,
				Equipment:   label,
				QtExist:     item.QtExist,
			})
			if len(out) >= maxParkMatches {
				return out
			}
		}
	}
	// Biggest installed count first: a hospital with sixty of something buys
	// differently from one with two.
	sort.SliceStable(out, func(i, j int) bool { return out[i].QtExist > out[j].QtExist })
	return out
}

// maxParkMatches bounds the panel. A list of forty pairings is a spreadsheet,
// not an answer to "what do I offer them".
const maxParkMatches = 12

// productFitsEquipment reports whether any significant word of the product
// name appears in the (already folded, lowercased) equipment label.
func productFitsEquipment(productName, foldedEquipment string) bool {
	for _, tok := range strings.FieldsFunc(strings.ToLower(productName), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'à' && r <= 'ÿ')
	}) {
		// Five runes is the same floor the watch distiller uses: below it the
		// words are "para", "linha", "duo" — noise that matches anything.
		if len([]rune(tok)) < 5 || watchStopwords[tok] {
			continue
		}
		if strings.Contains(foldedEquipment, domain.FoldAccents(tok)) {
			return true
		}
	}
	return false
}
