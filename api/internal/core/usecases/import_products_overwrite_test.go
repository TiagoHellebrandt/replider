package usecases

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// Re-importing a brochure has to REFRESH the product, and the old rule could
// not see the reason to: it compared price and description only, so a row
// carrying a full technical sheet was judged identical to a row that had none
// and was skipped as "já está no catálogo". These pin the new rule and the
// line it must not cross — an import overwrites what the row STATES and leaves
// everything else standing.

type overwriteRepo struct {
	domain.ProductRepository
	items   []*domain.Product
	updated []*domain.Product
	created []*domain.Product
}

func (r *overwriteRepo) ListByAccountID(context.Context, string) ([]*domain.Product, error) {
	return r.items, nil
}

func (r *overwriteRepo) Update(_ context.Context, p *domain.Product) error {
	r.updated = append(r.updated, p)
	return nil
}

func (r *overwriteRepo) Create(_ context.Context, p *domain.Product) error {
	r.created = append(r.created, p)
	r.items = append(r.items, p)
	return nil
}

// ownedSources is the brochure store, and these tests need it wired: the
// import only keeps a source id it can prove the account owns, so a use case
// built without this drops every link and the assertion below fails for a
// reason that has nothing to do with what it is testing.
type ownedSources struct {
	domain.ProductSourceRepository
	owner map[string]string // sourceID → accountID
}

func (s *ownedSources) GetByID(_ context.Context, id, accountID string) (*domain.ProductSource, error) {
	if s.owner[id] != accountID {
		return nil, nil
	}
	return &domain.ProductSource{ID: id, AccountID: accountID}, nil
}

func attrs(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func brochureRow() domain.ProductImportRow {
	return domain.ProductImportRow{
		Name:        "Incubadora Line Duo G",
		Description: "Unidade híbrida neonatal",
		Specs:       "Alimentação 127/220V. Faixa 20–37 °C.",
		Accessories: []string{"Suporte de soro"},
		Standards:   []string{"NBR IEC 60601-2-19"},
		Brand:       "Olidef",
		Model:       "LINE DUO G",
		SourceID:    "src-1",
		Page:        4,
	}
}

// The bug this file exists for: the import creates products THROUGH
// ProductUseCase.Create, whose DTO had no source fields — so the brochure link
// was assembled, carried through the preview, and dropped at the last step.
func TestImport_CreateCarriesTheBrochureLink(t *testing.T) {
	repo := &overwriteRepo{}
	uc := NewImportProductsUseCase(repo, nil)
	uc.SetSourceRepo(&ownedSources{owner: map[string]string{"src-1": "acc1"}})

	res, err := uc.Import(context.Background(), "acc1",
		ImportProductsInput{Rows: []domain.ProductImportRow{brochureRow()}})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || len(repo.created) != 1 {
		t.Fatalf("created = %d", res.Created)
	}
	got := repo.created[0]
	if got.SourceID != "src-1" || got.SourcePage != 4 {
		t.Fatalf("brochure link lost on create: source=%q page=%d", got.SourceID, got.SourcePage)
	}
	var bag map[string]any
	if err := json.Unmarshal(got.Attributes, &bag); err != nil {
		t.Fatalf("attributes: %v", err)
	}
	for _, key := range []string{"specs", "accessories", "standards", "brand", "model"} {
		if _, ok := bag[key]; !ok {
			t.Fatalf("%q did not reach the product: %s", key, got.Attributes)
		}
	}
}

// Same name, same price, and a sheet the stored product does not have. The old
// rule called this "já está no catálogo" and skipped it.
func TestImport_RefreshesAProductThatOnlyGainedItsSheet(t *testing.T) {
	repo := &overwriteRepo{items: []*domain.Product{{
		ID: "p1", AccountID: "acc1", Name: "Incubadora Line Duo G",
		Description: "Unidade híbrida neonatal", PriceCents: 0,
		Attributes: json.RawMessage(`{}`),
	}}}
	uc := NewImportProductsUseCase(repo, nil)
	uc.SetSourceRepo(&ownedSources{owner: map[string]string{"src-1": "acc1"}})

	preview, err := uc.previewFromRows(context.Background(), "acc1",
		[]domain.ProductImportRow{brochureRow()})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Importable != 1 {
		t.Fatalf("preview refused the refresh: importable=%d problem=%q",
			preview.Importable, preview.Rows[0].Problem)
	}
	// It is an update, not a new product — the screen has to say so.
	if !preview.Rows[0].Duplicate {
		t.Fatal("row not marked as already existing")
	}

	res, err := uc.Import(context.Background(), "acc1",
		ImportProductsInput{Rows: []domain.ProductImportRow{brochureRow()}})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Updated != 1 || len(repo.updated) != 1 {
		t.Fatalf("updated=%d skipped=%d", res.Updated, res.Skipped)
	}
	got := repo.updated[0]
	if got.SourceID != "src-1" || got.SourcePage != 4 {
		t.Fatalf("brochure link not written on update: %q p%d", got.SourceID, got.SourcePage)
	}
	if !strings.Contains(string(got.Attributes), "60601") {
		t.Fatalf("the sheet did not land: %s", got.Attributes)
	}
}

// The line the overwrite must not cross. A CSV re-import says nothing about
// specs; erasing the sheet a brochure gave would be the import deleting data
// it never read.
func TestImport_UpdateMergesAndNeverBlanks(t *testing.T) {
	repo := &overwriteRepo{items: []*domain.Product{{
		ID: "p1", AccountID: "acc1", Name: "Incubadora Line Duo G",
		PriceCents: 0,
		Attributes: attrs(t, map[string]any{
			"specs":     "Alimentação 127/220V.",
			"standards": []string{"NBR IEC 60601-2-19"},
			"warranty":  "24 meses", // set elsewhere, by a human
		}),
		SourceID: "src-1", SourcePage: 4,
	}}}
	uc := NewImportProductsUseCase(repo, nil)
	uc.SetSourceRepo(&ownedSources{owner: map[string]string{"src-1": "acc1"}})

	// A spreadsheet row: name and price, nothing else.
	csvRow := domain.ProductImportRow{Name: "Incubadora Line Duo G", PriceCents: 8_500_000}
	res, err := uc.Import(context.Background(), "acc1",
		ImportProductsInput{Rows: []domain.ProductImportRow{csvRow}})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Updated != 1 {
		t.Fatalf("updated=%d", res.Updated)
	}
	got := repo.updated[0]
	if got.PriceCents != 8_500_000 {
		t.Fatalf("price not overwritten: %d", got.PriceCents)
	}
	var bag map[string]any
	if err := json.Unmarshal(got.Attributes, &bag); err != nil {
		t.Fatalf("attributes: %v", err)
	}
	for _, key := range []string{"specs", "standards", "warranty"} {
		if _, ok := bag[key]; !ok {
			t.Fatalf("a CSV row erased %q: %s", key, got.Attributes)
		}
	}
	// And the brochure it came from is still the brochure it came from.
	if got.SourceID != "src-1" || got.SourcePage != 4 {
		t.Fatalf("a CSV row unlinked the brochure: %q p%d", got.SourceID, got.SourcePage)
	}
}

// The preview and the import must answer the same question. A row the review
// screen called "já está no catálogo" that the import then rewrites makes the
// screen a liar.
func TestImport_PreviewAndImportAgreeOnUnchanged(t *testing.T) {
	stored := &domain.Product{
		ID: "p1", AccountID: "acc1", Name: "Incubadora Line Duo G",
		Description: "Unidade híbrida neonatal", PriceCents: 0,
		Attributes: attrs(t, map[string]any{
			"specs":       "Alimentação 127/220V. Faixa 20–37 °C.",
			"accessories": []string{"Suporte de soro"},
			"standards":   []string{"NBR IEC 60601-2-19"},
			"brand":       "Olidef",
			"model":       "LINE DUO G",
			"source_page": float64(4),
		}),
		SourceID: "src-1", SourcePage: 4,
	}
	repo := &overwriteRepo{items: []*domain.Product{stored}}
	uc := NewImportProductsUseCase(repo, nil)
	uc.SetSourceRepo(&ownedSources{owner: map[string]string{"src-1": "acc1"}})

	preview, err := uc.previewFromRows(context.Background(), "acc1",
		[]domain.ProductImportRow{brochureRow()})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Rows[0].Problem != "já está no catálogo" {
		t.Fatalf("preview said the identical row would change: %q", preview.Rows[0].Problem)
	}

	res, err := uc.Import(context.Background(), "acc1",
		ImportProductsInput{Rows: []domain.ProductImportRow{brochureRow()}})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Updated != 0 || len(repo.updated) != 0 {
		t.Fatal("the import rewrote a row the preview called unchanged")
	}
}
