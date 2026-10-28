package usecases

import (
	"sort"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Ranking the catalogue before the prompt sees it.
//
// The matcher sends at most `maxCatalogue` products to the model. It used to
// send the FIRST sixty in `created_at DESC` order and log a warning — so for
// an account with ninety products, thirty were invisible to every offer it
// would ever write, and which thirty depended on import order.
//
// Production showed the effect: 75 picks across the whole account resolved to
// EIGHT distinct products, three of them neonatal, while every hospital park
// in the same account was dominated by infusion pumps (425, 545, 402 units)
// and "Suporte bomba de infusão" was picked four times.
//
// The fix is not a bigger prompt. The dossier already spells out what this
// lead has and what they are buying — "425× Bomba De Infusao", "Aquisição de
// gás oxigênio medicinal e fluxômetros". Ordering the catalogue by overlap
// with that text puts the products the lead actually needs in front of the
// model, and makes the truncation cut the irrelevant tail instead of an
// arbitrary one.
//
// Deterministic, no model call, no new port.

// rankWordMin is the shortest catalogue word worth scoring on. Below four
// runes the words are grammar ("de", "para", "com"), not products.
const rankWordMin = 4

// rankNoiseWords are catalogue words that appear in half the catalogue and in
// any medical text, so matching on them ranks everything equally — which is
// the same as ranking nothing.
var rankNoiseWords = map[string]bool{
	"para": true, "com": true, "sem": true, "modelo": true, "linha": true,
	"tipo": true, "kit": true, "unidade": true, "hospitalar": true,
	"hospitalares": true, "medico": true, "medica": true, "medicinal": true,
	"equipamento": true, "equipamentos": true, "produto": true, "produtos": true,
	"adulto": true, "infantil": true, "grande": true, "pequeno": true,
	"medio": true, "novo": true, "nova": true,
}

// rankCatalogueByDossier orders products by how much of their name the
// dossier actually says, strongest first.
//
// A STABLE sort: products that score the same keep the order they arrived in,
// so an account whose dossier says nothing useful sees exactly the behaviour
// it saw before — this can reorder, never shuffle.
func rankCatalogueByDossier(products []*domain.Product, dossier string) []*domain.Product {
	hay := strings.ToLower(domain.FoldAccents(dossier))
	if strings.TrimSpace(hay) == "" || len(products) < 2 {
		return products
	}

	score := make(map[string]int, len(products))
	for _, p := range products {
		if p == nil {
			continue
		}
		score[p.ID] = productDossierScore(p.Name, hay)
	}

	out := make([]*domain.Product, len(products))
	copy(out, products)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i] == nil || out[j] == nil {
			return out[j] == nil
		}
		return score[out[i].ID] > score[out[j].ID]
	})
	return out
}

// productDossierScore counts the DISTINCT meaningful words of a product name
// the dossier says, as whole words.
//
// Whole-word via domain.ContainsWord, the same matcher the tender watch uses:
// a looser contains() would score "bomba" into "bombardeio" and rank a
// product in on a substring.
func productDossierScore(name, foldedDossier string) int {
	seen := map[string]bool{}
	n := 0
	for _, w := range strings.FieldsFunc(
		strings.ToLower(domain.FoldAccents(name)),
		func(r rune) bool { return !isRankWordRune(r) },
	) {
		if len([]rune(w)) < rankWordMin || rankNoiseWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		if domain.ContainsWord(foldedDossier, w) {
			n++
		}
	}
	return n
}

func isRankWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
