package usecases

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// The opportunity being LOST, on the screen that says what needs you.
//
// The four tiles of the command centre all count work already in hand: leads,
// approvals, meetings, closed. An account with one lead and a whole state in
// front of it therefore looked exactly like one that had worked the state
// through — the number that separates them was nowhere. A real account held
// ninety products and 5.208 establishments of matching equipment in its own
// territory, and the home screen showed "1".
//
// Two shapes, on purpose:
//
//   - The TILE is the size ("480 nunca abordados"), because nobody counts what
//     they cannot see.
//   - The ROWS name the hospital ("Santa Casa de Rondonópolis, 63 aparelhos"),
//     because a statistic is not a phone call — the same rule every other
//     source in this file follows.
//
// Deliberately NOT a money figure. CNES says how many machines an
// establishment has, never what a deal is worth; inventing a ticket to fill a
// currency column would be the product guessing, and the ranking would then be
// sorted by a number nobody could check.

const (
	// Rows offered. The tile carries the total; a home screen listing forty
	// cold hospitals is a home screen nobody reads.
	maxUntouchedRows = 5
	// A guard against a pathological catalogue, not a sample: see
	// catalogueWords.
	maxCatalogueWords = 400
)

// untouchedParkReader is the slice of the CNES repository this needs.
type untouchedParkReader interface {
	ResolveEquipmentCodesAny(words []string) ([]domain.CNESEquipmentType, error)
	CityCodesByName(uf string, names []string) ([]string, error)
	CountEquipmentIn(uf string, cityCodes, equipCodes []string) (establishments, units int, err error)
	EstablishmentsWithEquipmentIn(uf string, cityCodes, equipCodes []string, minCount, limit, offset int) ([]domain.CNESEquipmentHolder, string, error)
}

// UntouchedTargetsUseCase answers "who in my territory has what I sell and has
// never heard from me".
type UntouchedTargetsUseCase struct {
	accounts domain.AccountRepository
	products domain.ProductRepository
	park     untouchedParkReader
	contacts domain.ContactRepository
	// identities is optional (SetIdentityRepo) — the index that says an
	// establishment's phone already belongs to a contact this account has.
	// Without it the answer degrades to the external-id keys, which is what
	// this question used to get wrong on its own.
	identities domain.ContactIdentityRepository
}

// SetIdentityRepo wires the contact-identity index.
func (uc *UntouchedTargetsUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identities = r
}

func NewUntouchedTargetsUseCase(
	accounts domain.AccountRepository,
	products domain.ProductRepository,
	park untouchedParkReader,
	contacts domain.ContactRepository,
) *UntouchedTargetsUseCase {
	return &UntouchedTargetsUseCase{accounts: accounts, products: products, park: park, contacts: contacts}
}

// UntouchedTargets is the tile number and the named rows behind it.
type UntouchedTargets struct {
	Total   int
	Actions []domain.PendingAction
}

// UntouchedPage is one window of the territory: what the park ranking held at
// this offset (Scanned) and which of it is cold ground (Holders).
//
// Scanned is deliberately NOT len(Holders), and the difference is the whole
// reason it exists. A page whose every hospital is already in the funnel is
// empty of WORK and is not the end of the territory — and the first page is
// exactly that page, because it is the ground the sweep already consumed.
// Reading one as the other would stop the walk on page one, which is the bug
// paging was added to fix.
type UntouchedPage struct {
	Holders []domain.CNESEquipmentHolder
	Total   int
	Scanned int
}

// Execute is the home-screen shape: the tile number plus a handful of named
// rows.
func (uc *UntouchedTargetsUseCase) Execute(ctx context.Context, accountID string) (UntouchedTargets, error) {
	page, err := uc.ListPage(ctx, accountID, 0, maxUntouchedRows*4)
	if err != nil || page.Total == 0 {
		return UntouchedTargets{}, err
	}
	out := UntouchedTargets{Total: page.Total}
	for i, h := range page.Holders {
		if i >= maxUntouchedRows {
			break
		}
		name := ""
		if h.Establishment != nil {
			name = strings.TrimSpace(h.Establishment.TradeName)
		}
		if name == "" {
			// Never a guessed name: the raw registry id, exactly as every
			// other CNES surface renders an unnamed establishment.
			name = "Estabelecimento CNES " + h.CNES
		}
		out.Actions = append(out.Actions, domain.PendingAction{
			Kind:         domain.PendingUntouchedTarget,
			TargetID:     "cnes:" + h.CNES,
			CustomerName: name,
			Detail: fmt.Sprintf("%d %s do que você vende, e nunca foi abordado",
				h.Total, plural(h.Total, "aparelho", "aparelhos")),
			// No SinceAt: this ground has not been waiting on anybody. Giving
			// it an age would let it climb past a customer who is.
			DeepLink: "/prospect?tab=hunt",
		})
	}
	return out, nil
}

// List is the question itself: establishments in the territory holding what
// this account sells, that it has never approached — biggest park first — and
// how many there are in total.
//
// One implementation, two callers: this screen, and the nightly hunt that
// turns the same list into leads. They were separate once and the hunt had its
// own answer, built by asking the CNES dictionary about the six LONGEST words
// in the catalogue. On a real catalogue the longest words are
// "antitransbordamento", "multiparamétricos", "bilirrubinômetro" — none of
// which the registry knows — so the sweep asked six questions nothing could
// answer and reported, truthfully and uselessly, that it had found nothing.
//
// Bounded to three park queries whatever the catalogue's size: every word
// resolves to codes in ONE call, then one count and one top-N. It also runs on
// every home-screen load, so an N-query shape would make the screen slower the
// more products somebody has — punishing exactly the accounts that use the
// product most.
//
// `offset` is where in the ranking to read from. The home screen always reads
// from the top; the nightly hunt walks deeper, because a window fixed at the
// top is a ceiling. Measured in production on 2026-09-17: the hunt read the
// same 60 biggest parks every night out of 2.818 matching establishments in
// the territory, so once those 60 were in the funnel it could never create
// another lead — 97,9% of the ground was unreachable by construction.
func (uc *UntouchedTargetsUseCase) ListPage(
	ctx context.Context, accountID string, offset, limit int,
) (UntouchedPage, error) {
	if uc.accounts == nil || uc.products == nil || uc.park == nil {
		return UntouchedPage{}, nil
	}
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil {
		return UntouchedPage{}, err
	}
	// No territory is not an error and not a zero: it is a question the
	// account has not answered, and the onboarding asks it.
	if account == nil || account.Territory == nil {
		return UntouchedPage{}, nil
	}
	uf := strings.ToUpper(strings.TrimSpace(account.Territory.UF))
	if len(uf) != 2 {
		return UntouchedPage{}, nil
	}

	products, err := uc.products.ListByAccountID(ctx, accountID)
	if err != nil {
		return UntouchedPage{}, err
	}
	// EVERY word, not a longest-first sample.
	//
	// deriveWatchKeywords — the PNCP watch's distiller — keeps the fifteen
	// LONGEST, which is right for a stored keyword list a person reads and
	// wrong here. On a real catalogue of 133 words, "incubadora" ranks 33rd by
	// length, "ventilador" 41st and "monitor" 86th, behind
	// "antitransbordamento" and "multiparamétricos", which the CNES registry
	// has never heard of. Sampling by length cut every word that resolves and
	// the sweep found nothing, twice.
	//
	// There is no reason to sample: ResolveEquipmentCodesAny answers for the
	// whole list in ONE query.
	words := catalogueWords(products)
	if len(words) == 0 {
		return UntouchedPage{}, nil
	}

	types, err := uc.park.ResolveEquipmentCodesAny(words)
	if err != nil {
		return UntouchedPage{}, err
	}
	if len(types) == 0 {
		// A catalogue whose names are not equipment names. Honest zero.
		return UntouchedPage{}, nil
	}
	codes := make([]string, 0, len(types))
	for _, t := range types {
		codes = append(codes, t.Code)
	}

	// The territory's cities narrow it; an unresolvable list means the whole
	// state, which is the same fallback the hunt screen uses.
	var cityCodes []string
	if len(account.Territory.Cities) > 0 {
		if cc, err := uc.park.CityCodesByName(uf, account.Territory.Cities); err == nil {
			cityCodes = cc
		}
	}

	establishments, _, err := uc.park.CountEquipmentIn(uf, cityCodes, codes)
	if err != nil {
		return UntouchedPage{}, err
	}

	holders, _, err := uc.park.EstablishmentsWithEquipmentIn(uf, cityCodes, codes, 1, limit, offset)
	if err != nil {
		return UntouchedPage{}, err
	}

	untouched := make([]domain.CNESEquipmentHolder, 0, len(holders))
	for _, h := range holders {
		if uc.alreadyKnown(ctx, accountID, h) {
			continue
		}
		untouched = append(untouched, h)
	}
	// Biggest park first: the count is the only ordering signal CNES gives,
	// and it is a real one — a hospital with sixty machines of your kind buys
	// differently from one with two.
	sort.SliceStable(untouched, func(i, j int) bool { return untouched[i].Total > untouched[j].Total })

	// The total counts establishments holding the equipment; the ones already
	// in the funnel are not "untouched", so subtract what we could see. The
	// sample is bounded, so this is a floor rather than an exact difference —
	// which is the honest direction: never claim more lost ground than there is.
	total := establishments - (len(holders) - len(untouched))
	if total < 0 {
		total = 0
	}
	return UntouchedPage{Holders: untouched, Total: total, Scanned: len(holders)}, nil
}

// alreadyKnown answers, for one establishment, whether this account already
// holds the contact behind it — under EXACTLY the rules that create it.
//
// It used to ask only "is there a contact keyed cnes:<id>?", which had been
// wrong since the day the writer started keying reachable establishments by
// phone. Production on 2026-09-17: 51 of 60 hunted hospitals carried a phone
// key, so this reported 52 of the top 60 as never approached — the home
// screen offered hospitals already sitting in the funnel, and the nightly
// sweep spent every run re-deciding the same ones (`found=52 created=0`).
//
// Sharing cnesContactDraft with the writer is the fix and the guarantee: the
// two cannot answer differently again without someone changing one function.
func (uc *UntouchedTargetsUseCase) alreadyKnown(
	ctx context.Context, accountID string, h domain.CNESEquipmentHolder,
) bool {
	if uc.contacts == nil {
		return false
	}
	draft, opts := cnesContactDraft(accountID, h.CNES, h.Establishment)
	return newContactResolver(uc.contacts, uc.identities).
		find(ctx, accountID, derefStr(draft.Phone), derefStr(draft.Email), draft.ExternalID, opts) != nil
}

// catalogueWords is every distinct significant word in the product names.
//
// Same tokenising and same stopwords as the watch distiller — a word that is
// noise in a watch is noise here — but no length ordering and no sample.
func catalogueWords(products []*domain.Product) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 64)
	for _, p := range products {
		if p == nil {
			continue
		}
		for _, tok := range strings.FieldsFunc(strings.ToLower(p.Name), func(r rune) bool {
			return !(r >= 'a' && r <= 'z') && !(r >= 'à' && r <= 'ÿ')
		}) {
			if len([]rune(tok)) < 5 || watchStopwords[tok] || seen[tok] {
				continue
			}
			seen[tok] = true
			out = append(out, tok)
			// A ceiling only against the pathological: a catalogue with
			// thousands of distinct words would build a query nobody wants.
			// Far above any real one (133 on the catalogue this was built
			// for), so it never decides which words get asked.
			if len(out) >= maxCatalogueWords {
				return out
			}
		}
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
