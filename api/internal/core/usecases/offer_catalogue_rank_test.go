package usecases

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
)

func rankProducts(names ...string) []*domain.Product {
	out := make([]*domain.Product, 0, len(names))
	for i, n := range names {
		out = append(out, &domain.Product{ID: string(rune('a' + i)), Name: n, Active: true})
	}
	return out
}

func rankedNames(ps []*domain.Product) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

// The production case: a tender for medical oxygen got an offer for
// multiparameter monitors, because the catalogue reached the model in import
// order and the model picked the most generic thing it saw.
func TestRankCatalogue_TheTenderPutsItsOwnProductsFirst(t *testing.T) {
	products := rankProducts(
		"Monitor Multiparamétrico Logicare",
		"Berço Aquecido Modelo Matrix",
		"Mangueira para oxigênio",
		"Válvula reguladora",
		"Cilindro para gás",
	)
	dossier := "Edital publicado (objeto): Aquisição de gás oxigênio medicinal e fluxômetros " +
		"para atender as necessidades da secretaria municipal de Saúde"

	got := rankedNames(rankCatalogueByDossier(products, dossier))

	// "oxigênio" and "gás" are said by the tender; the monitor and the
	// warming crib are not.
	if got[0] != "Mangueira para oxigênio" && got[0] != "Cilindro para gás" {
		t.Fatalf("first = %q, want a product the tender actually asks for: %v", got[0], got)
	}
	if got[len(got)-1] == "Mangueira para oxigênio" {
		t.Fatalf("an oxygen product ranked last for an oxygen tender: %v", got)
	}
}

// The park case: the dossier names what the hospital declared to the ministry.
func TestRankCatalogue_TheParkPutsItsOwnProductsFirst(t *testing.T) {
	products := rankProducts(
		"Incubadora Neonatal Modelo Line 4",
		"Suporte bomba de infusão",
		"Bilirrubinômetro Transcutâneo BM-100C",
	)
	dossier := "Parque instalado no CNES: 425x Bomba De Infusao, 215x Monitor De Ecg"

	got := rankedNames(rankCatalogueByDossier(products, dossier))

	if got[0] != "Suporte bomba de infusão" {
		t.Fatalf("first = %q, want the product matching 425 infusion pumps: %v", got[0], got)
	}
}

// A dossier that says nothing useful must leave the order EXACTLY as it was.
// This can reorder; it must never shuffle.
func TestRankCatalogue_SaysNothingChangesNothing(t *testing.T) {
	names := []string{"Alfa Um", "Beta Dois", "Gama Três"}
	got := rankedNames(rankCatalogueByDossier(rankProducts(names...), "Endereço: Rua X, Cuiabá"))

	for i := range names {
		if got[i] != names[i] {
			t.Fatalf("order changed with nothing to rank on: %v", got)
		}
	}
}

// Whole-word, like the tender watch: a substring must not rank a product in.
func TestRankCatalogue_MatchesWholeWordsOnly(t *testing.T) {
	if productDossierScore("Bomba de infusão", "houve um bombardeio na cidade") != 0 {
		t.Fatal("\"bomba\" scored inside \"bombardeio\"")
	}
	if productDossierScore("Bomba de infusão", "425x bomba de infusao declaradas") == 0 {
		t.Fatal("a real whole-word match scored zero")
	}
}

// Words that appear in half of any medical catalogue rank everything equally,
// which is the same as ranking nothing.
func TestRankCatalogue_IgnoresWordsThatMatchEverything(t *testing.T) {
	if productDossierScore("Equipamento hospitalar para adulto", "equipamentos hospitalares para adulto") != 0 {
		t.Fatal("noise words scored — every product would tie at the top")
	}
}

// An empty or whitespace dossier is the no-op path and must not panic.
func TestRankCatalogue_EmptyDossierIsANoOp(t *testing.T) {
	in := rankProducts("Alfa", "Beta")
	if got := rankCatalogueByDossier(in, "   "); len(got) != 2 || got[0].Name != "Alfa" {
		t.Fatal("an empty dossier must leave the list untouched")
	}
	if got := rankCatalogueByDossier(nil, "qualquer coisa"); got != nil {
		t.Fatal("nil in, nil out")
	}
}
