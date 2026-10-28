package usecases

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// SignalWatchSeeder gives the autonomous path its eyes: an account that has
// products and a territory but never opened the watch settings still gets a
// PNCP watch, derived from its own catalogue. Without this, "o agente acha
// licitações sozinho" is only true for whoever configured a form.
//
// The derived watch is a normal row the customer can edit or turn off —
// seeded ONCE (its presence, active or not, stops re-seeding), so turning
// it off is respected forever, same posture as overnight drafting.
type SignalWatchSeeder struct {
	candidates watchCandidateLister
	products   domain.ProductRepository
	watches    watchUpserter
}

// watchCandidateLister finds accounts that should have a derived watch.
// Satisfied by *postgres.SignalRepository (raw SQL — the join spans three
// tables no domain repo owns together).
type watchCandidateLister interface {
	AccountsNeedingDefaultWatch(ctx context.Context) ([]domain.SignalWatchCandidate, error)
}

type watchUpserter interface {
	UpsertWatch(ctx context.Context, w *domain.SignalWatch) error
}

func NewSignalWatchSeeder(
	candidates watchCandidateLister,
	products domain.ProductRepository,
	watches watchUpserter,
) *SignalWatchSeeder {
	return &SignalWatchSeeder{candidates: candidates, products: products, watches: watches}
}

// EnsureDefaultWatches runs before a poll. Best-effort throughout: a seeding
// failure only means that account keeps living without a watch until the
// next tick.
func (s *SignalWatchSeeder) EnsureDefaultWatches(ctx context.Context) {
	cands, err := s.candidates.AccountsNeedingDefaultWatch(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "watch seeder: candidate scan failed", "error", err)
		return
	}
	for _, c := range cands {
		products, err := s.products.ListByAccountID(ctx, c.AccountID)
		if err != nil || len(products) == 0 {
			continue
		}
		keywords := deriveWatchKeywords(products)
		if len(keywords) == 0 {
			continue
		}
		// Reuse the existing derived row's id so UpsertWatch REFRESHES it
		// instead of adding a second watch beside it.
		id := c.DerivedWatchID
		refresh := id != ""
		if !refresh {
			id = ulid.Make().String()
		}
		w := &domain.SignalWatch{
			ID:        id,
			AccountID: c.AccountID,
			Source:    domain.SignalPNCP,
			Keywords:  keywords,
			States:    []string{c.UF},
			IsActive:  true,
			// The one place this flag is ever set: these keywords really
			// did come from the catalogue, so the screen may say so.
			DerivedFromCatalog: true,
		}
		// The candidate query already demands a territory UF, so failing
		// here means the stored one is not a state at all. Seeding anyway
		// would mint the unscoped watch that drags every OTHER account's
		// poll national — better no watch than that one.
		if err := w.NormalizeAndValidate(); err != nil {
			slog.WarnContext(ctx, "watch seeder: refusing unscoped watch",
				"account_id", c.AccountID, "uf", c.UF, "error", err)
			continue
		}
		if err := s.watches.UpsertWatch(ctx, w); err != nil {
			slog.WarnContext(ctx, "watch seeder: upsert failed", "account_id", c.AccountID, "error", err)
			continue
		}
		action := "created"
		if refresh {
			action = "refreshed"
		}
		slog.InfoContext(ctx, "watch seeder: derived PNCP watch "+action,
			"account_id", c.AccountID, "keywords", len(keywords))
	}
}

// maxDerivedKeywords bounds the watch. Matching is any-keyword, so more
// words only widen it — but fifteen was too few AND ranked wrong (see
// deriveWatchKeywords), which left a 103-product catalogue watching for
// "antitransbordamento" and blind to "incubadora".
const maxDerivedKeywords = 40

// watchStopwords are catalogue words that would match every tender.
//
// The second block is evidence, not guesswork: matching the seeded watch
// against 414 real MT tenders (2026-08-09..09-08) returned ten hits, and
// every one of them came from "acessórios" or "instrumentos" — musical
// instruments, clothing accessories, IT supplies. A word that names the
// packaging rather than the product is noise in a tender title.
var watchStopwords = map[string]bool{
	"para": true, "com": true, "sem": true, "tipo": true, "unidade": true,
	"caixa": true, "pacote": true, "kit": true, "adulto": true,
	"infantil": true, "descartavel": true, "descartável": true,
	"hospitalar": true, "equipamento": true, "equipamentos": true,
	"aparelho": true, "modelo": true, "nacional": true, "importado": true,

	"acessorio": true, "acessorios": true, "acessório": true, "acessórios": true,
	"instrumento": true, "instrumentos": true, "material": true, "materiais": true,
	"produto": true, "produtos": true, "componente": true, "componentes": true,
	"dispositivo": true, "dispositivos": true, "suprimento": true, "suprimentos": true,
	"conjunto": true, "conjuntos": true, "peça": true, "peças": true,
	"reutilizavel": true, "reutilizável": true, "reutilizaveis": true, "reutilizáveis": true,
	"pequeno": true, "medio": true, "médio": true, "grande": true,
	"linha": true, "serie": true, "série": true, "marca": true,

	// Measured the same way, after the frequency ranking put them in: these
	// are frequent in a health catalogue AND frequent in tender language for
	// something else entirely — "coleta, transporte e destinação",
	// "serviços de segurança", "suporte técnico". Whole-word matching alone
	// does not save them; they are simply the wrong search terms.
	"transporte": true, "seguranca": true, "segurança": true,
	"capacidade": true, "suporte": true, "duplo": true, "leito": true,
}

// deriveWatchKeywords distils the catalogue into search words: significant
// tokens from product names, lowercased (the matcher is a lowercase
// substring), deduped, longest first — "ventilador" earns its place,
// "cx" and "azul" do not.
func deriveWatchKeywords(products []*domain.Product) []string {
	// Ranked by how many products carry the word, NOT by how long it is.
	//
	// Length was a proxy for "specific" and it is a bad one: the longest
	// words in a catalogue are its rarest compound adjectives, so a
	// 103-product account was watching for "antitransbordamento" and
	// "bilirrubinômetro" while "incubadora" — the thing it actually sells,
	// 33rd by length — never made the cut. Frequency says what the company
	// is IN THE BUSINESS OF, which is what a tender will name.
	count := map[string]int{}
	var order []string
	for _, p := range products {
		perProduct := map[string]bool{}
		for _, tok := range strings.FieldsFunc(strings.ToLower(p.Name), func(r rune) bool {
			return !(r >= 'a' && r <= 'z') && !(r >= 'à' && r <= 'ÿ')
		}) {
			// One vote per product: "capuz de oxigênio pequeno/médio/grande"
			// is three products, not three votes for "capuz" from one.
			if len([]rune(tok)) < 5 || watchStopwords[tok] || perProduct[tok] {
				continue
			}
			perProduct[tok] = true
			if count[tok] == 0 {
				order = append(order, tok)
			}
			count[tok]++
		}
	}
	out := append([]string(nil), order...)
	sort.SliceStable(out, func(i, j int) bool {
		if count[out[i]] != count[out[j]] {
			return count[out[i]] > count[out[j]]
		}
		// Ties go to the longer word: between two words appearing once,
		// the longer one is the more specific search term.
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	if len(out) > maxDerivedKeywords {
		out = out[:maxDerivedKeywords]
	}
	return out
}
