package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// Importing a price list. The preview is where a person catches what the parser
// could not read, so these pin that it reports rather than guesses — and that a
// confirmed row cannot smuggle past the checks the preview applied.

type importProductRepo struct {
	existing []*domain.Product
	created  []*domain.Product
	updated  []*domain.Product
	failOn   string
}

func (r *importProductRepo) Create(_ context.Context, p *domain.Product) error {
	if r.failOn != "" && p.Name == r.failOn {
		return domain.ErrProductNotFound // any repo failure
	}
	r.created = append(r.created, p)
	return nil
}

func (r *importProductRepo) GetByID(context.Context, string) (*domain.Product, error) {
	return nil, domain.ErrProductNotFound
}

func (r *importProductRepo) ListByAccountID(context.Context, string) ([]*domain.Product, error) {
	return r.existing, nil
}

func (r *importProductRepo) Update(_ context.Context, p *domain.Product) error {
	clone := *p
	r.updated = append(r.updated, &clone)
	return nil
}
func (r *importProductRepo) Delete(context.Context, string) error { return nil }

const importAccount = "acc-1"

func importPreviewOf(t *testing.T, repo *importProductRepo, csv string) *ProductImportPreview {
	t.Helper()
	uc := NewImportProductsUseCase(repo, nil)
	out, err := uc.Preview(context.Background(), importAccount, strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	return out
}

func TestImportProducts_PreviewCountsWhatTheImportWillDo(t *testing.T) {
	repo := &importProductRepo{}
	out := importPreviewOf(t, repo, "Nome,Preço\nGaze estéril,49.90\nLuva,125.50\n")

	if out.Importable != 2 || out.Problems != 0 {
		t.Fatalf("counts wrong: %+v", out)
	}
	if out.Rows[0].PriceCents != 4990 || out.Rows[1].PriceCents != 12550 {
		t.Fatalf("prices wrong: %+v", out.Rows)
	}
	// Preview creates nothing.
	if len(repo.created) != 0 {
		t.Fatal("Preview created products")
	}
}

// Excel in a Brazilian locale writes `;` because the comma is the decimal
// separator. Getting this wrong makes every price unreadable.
func TestImportProducts_ReadsSemicolonExports(t *testing.T) {
	out := importPreviewOf(t, &importProductRepo{},
		"Nome;Preço;Moeda\nGaze estéril;1.234,56;BRL\n")

	if out.Importable != 1 {
		t.Fatalf("counts wrong: %+v", out)
	}
	if out.Rows[0].PriceCents != 123456 {
		t.Fatalf("price wrong: %d", out.Rows[0].PriceCents)
	}
}

func TestImportProducts_TolerantOfTheBOMExcelWrites(t *testing.T) {
	out := importPreviewOf(t, &importProductRepo{},
		"\ufeffNome,Preço\nGaze,49.90\n")

	if out.Importable != 1 || out.Rows[0].Name != "Gaze" {
		t.Fatalf("BOM broke the header: %+v", out.Rows)
	}
}

// A product already in the catalogue is reported, not silently skipped: somebody
// re-importing an updated list needs to know the old row is the one that stays.
func TestImportProducts_PreviewFlagsWhatIsAlreadyThere(t *testing.T) {
	repo := &importProductRepo{existing: []*domain.Product{
		{ID: "p1", AccountID: importAccount, Name: "Gaze estéril", PriceCents: 4990},
	}}
	out := importPreviewOf(t, repo, "Nome,Preço\nGaze estéril,49.90\nLuva,125.50\n")

	if out.AlreadyInCatalogue != 1 {
		t.Fatalf("already-there count wrong: %+v", out)
	}
	if out.Importable != 1 {
		t.Fatalf("importable wrong: %+v", out)
	}
	// And the row says so where the person is looking.
	if out.Rows[0].Problem == "" {
		t.Fatalf("the existing row carries no explanation: %+v", out.Rows[0])
	}
}

// The parser refuses "1,234" rather than guessing. The preview must carry that
// refusal to the person instead of importing a price 1000x off.
func TestImportProducts_AmbiguousPriceReachesThePreview(t *testing.T) {
	// Semicolon-delimited, because a comma-delimited file cannot even
	// EXPRESS "1,234" — the comma would split the column.
	out := importPreviewOf(t, &importProductRepo{}, "Nome;Preço\nGaze;1,234\n")

	if out.Importable != 0 || out.Problems != 1 {
		t.Fatalf("counts wrong: %+v", out)
	}
	if out.Rows[0].Problem != domain.ProblemAmbiguousPrice {
		t.Fatalf("problem wrong: %q", out.Rows[0].Problem)
	}
}

func TestImportProducts_CreatesTheConfirmedRows(t *testing.T) {
	repo := &importProductRepo{}
	uc := NewImportProductsUseCase(repo, nil)

	res, err := uc.Import(context.Background(), importAccount, ImportProductsInput{
		Rows: []domain.ProductImportRow{
			{Line: 2, Name: "Gaze estéril", Description: "Pacote", PriceCents: 4990, Currency: "BRL"},
			{Line: 3, Name: "Luva", PriceCents: 12550, Currency: "BRL"},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 2 || res.Skipped != 0 {
		t.Fatalf("result wrong: %+v", res)
	}
	if len(repo.created) != 2 {
		t.Fatalf("created %d products", len(repo.created))
	}
	if repo.created[0].Name != "Gaze estéril" || repo.created[0].PriceCents != 4990 {
		t.Fatalf("product wrong: %+v", repo.created[0])
	}
	if repo.created[0].Currency != "BRL" || !repo.created[0].Active {
		t.Fatalf("product wrong: %+v", repo.created[0])
	}
}

// A row that arrives with its problem cleared must not create the product the
// preview refused. The checks belong to the server, not to what the client sends.
func TestImportProducts_RechecksInsteadOfTrustingTheClient(t *testing.T) {
	repo := &importProductRepo{existing: []*domain.Product{
		{ID: "p1", AccountID: importAccount, Name: "Gaze estéril", PriceCents: 4990},
	}}
	uc := NewImportProductsUseCase(repo, nil)

	res, err := uc.Import(context.Background(), importAccount, ImportProductsInput{
		Rows: []domain.ProductImportRow{
			// Already in the catalogue at the SAME price, arriving with no
			// problem attached: nothing to do.
			{Line: 2, Name: "Gaze estéril", PriceCents: 4990},
			// Nameless.
			{Line: 3, Name: "   ", PriceCents: 100},
			// A negative price is not a discount, it is a parse that went wrong.
			{Line: 4, Name: "Luva", PriceCents: -100},
			{Line: 5, Name: "Swab", PriceCents: 500},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || res.Updated != 0 || res.Skipped != 3 {
		t.Fatalf("result wrong: %+v", res)
	}
	if len(repo.created) != 1 || repo.created[0].Name != "Swab" {
		t.Fatalf("wrong product created: %+v", repo.created)
	}
	if len(repo.updated) != 0 {
		t.Fatalf("an identical row must not rewrite the product: %+v", repo.updated)
	}
}

// A re-imported price list is a list of NEW prices. The row whose name is
// already in the catalogue updates it — price, and description when the file
// carries one — instead of being skipped, which is how the catalogue drifted
// from the spreadsheet.
func TestImportProducts_ReimportUpdatesTheExistingPrice(t *testing.T) {
	repo := &importProductRepo{existing: []*domain.Product{
		{ID: "p1", AccountID: importAccount, Name: "Gaze estéril", PriceCents: 4990, Description: "10x10"},
	}}
	uc := NewImportProductsUseCase(repo, nil)

	// The preview says the row is already there AND still counts it as
	// importable, so the screen can say "will update".
	preview := importPreviewOf(t, repo, "Nome,Preço,Descrição\nGaze estéril,52.90,\nLuva,125.50,\n")
	if preview.AlreadyInCatalogue != 1 || preview.Importable != 2 || preview.Problems != 0 {
		t.Fatalf("preview counts wrong: %+v", preview)
	}
	if !preview.Rows[0].Duplicate || preview.Rows[0].Problem != "" {
		t.Fatalf("the changed row must be flagged as existing and stay importable: %+v", preview.Rows[0])
	}

	res, err := uc.Import(context.Background(), importAccount, ImportProductsInput{Rows: preview.Rows})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || res.Updated != 1 || res.Skipped != 0 {
		t.Fatalf("result wrong: %+v", res)
	}
	if len(repo.updated) != 1 || repo.updated[0].ID != "p1" || repo.updated[0].PriceCents != 5290 {
		t.Fatalf("existing product not updated: %+v", repo.updated)
	}
	if repo.updated[0].Description != "10x10" {
		t.Fatalf("an empty description in the file must not erase the current one: %+v", repo.updated[0])
	}
}

// Two identical names in one confirmed set would create two products nobody can
// tell apart in a proposal: the second row updates the first, the file's last
// price wins.
func TestImportProducts_DedupesWithinTheConfirmedSet(t *testing.T) {
	repo := &importProductRepo{}
	uc := NewImportProductsUseCase(repo, nil)

	res, err := uc.Import(context.Background(), importAccount, ImportProductsInput{
		Rows: []domain.ProductImportRow{
			{Line: 2, Name: "Gaze", PriceCents: 100},
			{Line: 3, Name: "gaze", PriceCents: 200},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || res.Updated != 1 || len(repo.created) != 1 {
		t.Fatalf("result wrong: %+v", res)
	}
}

// One failing row must not lose the rows already created.
func TestImportProducts_OneBadRowDoesNotLoseTheRest(t *testing.T) {
	repo := &importProductRepo{failOn: "Luva"}
	uc := NewImportProductsUseCase(repo, nil)

	res, err := uc.Import(context.Background(), importAccount, ImportProductsInput{
		Rows: []domain.ProductImportRow{
			{Line: 2, Name: "Gaze", PriceCents: 100},
			{Line: 3, Name: "Luva", PriceCents: 200},
			{Line: 4, Name: "Swab", PriceCents: 300},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 2 || res.Skipped != 1 {
		t.Fatalf("result wrong: %+v", res)
	}
}

// The wrong file and an empty one are different answers.
func TestImportProducts_PreviewRefusalsAreDistinct(t *testing.T) {
	uc := NewImportProductsUseCase(&importProductRepo{}, nil)
	ctx := context.Background()

	_, err := uc.Preview(ctx, importAccount, strings.NewReader(""))
	if err != domain.ErrProductImportEmpty {
		t.Fatalf("empty: err = %v", err)
	}

	_, err = uc.Preview(ctx, importAccount, strings.NewReader("a,b\n1,2\n"))
	if err != domain.ErrProductImportNoHeader {
		t.Fatalf("no header: err = %v", err)
	}

	_, err = uc.Import(ctx, importAccount, ImportProductsInput{})
	if err != domain.ErrProductImportEmpty {
		t.Fatalf("no rows: err = %v", err)
	}
}

// A file past the cap stops being reviewable, which is the point of the preview.
func TestImportProducts_RefusesAFileTooBigToReview(t *testing.T) {
	var b strings.Builder
	b.WriteString("Nome,Preço\n")
	for i := 0; i < maxImportRows+1; i++ {
		b.WriteString("Produto ")
		b.WriteString(strings.Repeat("x", 3))
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(",10,00\n")
	}

	uc := NewImportProductsUseCase(&importProductRepo{}, nil)
	_, err := uc.Preview(context.Background(), importAccount, strings.NewReader(b.String()))
	if err != ErrProductImportTooLarge {
		t.Fatalf("err = %v, want ErrProductImportTooLarge", err)
	}
}

// A hyphen is not a different product.
//
// The match key was the exact lowercased name, and a PDF import is an AI
// reading a page: run it twice and one run writes "Analisador de Bilirrubina -
// Bilirrubinômetro AG" while the other writes "Analisador de Bilirrubina
// BILIRRUBINÔMETRO AG". Under the old rule those are two products, and a live
// customer catalogue has exactly that pair sitting in it — which is what makes
// "just re-import the brochure" dangerous advice.
func TestProductMatchKey_FoldsWhatIsNotADifferentProduct(t *testing.T) {
	same := [][2]string{
		{
			"Analisador de Bilirrubina - Bilirrubinômetro AG",
			"Analisador de Bilirrubina BILIRRUBINÔMETRO AG",
		},
		{"Bilirrubinômetro BM-100C", "Bilirrubinometro BM 100C"},
		{"Capuz de Oxigênio - Pequeno", "capuz de oxigenio   pequeno"},
		{"  Berço Aquecido Matrix ", "Berco Aquecido Matrix"},
	}
	for _, pair := range same {
		if a, b := productMatchKey(pair[0]), productMatchKey(pair[1]); a != b {
			t.Errorf("%q and %q must be one product\n  %q != %q",
				pair[0], pair[1], a, b)
		}
	}
}

// And it must not fold what IS a different product. Merging two real entries
// silently overwrites one of them, which is worse than a visible duplicate —
// a duplicate is obvious and fixable, a merge loses a price nobody can recover.
func TestProductMatchKey_KeepsRealProductsApart(t *testing.T) {
	different := [][2]string{
		{"Aspirador Cirúrgico", "Aspirador Cirúrgico A-45"},
		{"Aspirador Cirúrgico A-45", "Aspirador Cirúrgico A-45 MAX"},
		{"Bilirrubinômetro AG", "Bilirrubinômetro AG MAX"},
		{"Berço Modelo Afeto", "Berço Modelo Ninar"},
		{"Capuz de Oxigênio - Pequeno", "Capuz de Oxigênio - Grande"},
		// Plurals are deliberately NOT folded: a catalogue may legitimately
		// carry a family entry beside a model, and guessing costs an entry.
		{"Berço Aquecido", "Berços Aquecidos"},
	}
	for _, pair := range different {
		if a, b := productMatchKey(pair[0]), productMatchKey(pair[1]); a == b {
			t.Errorf("%q and %q are different products, both keyed %q",
				pair[0], pair[1], a)
		}
	}
}

// Re-importing the same brochure must UPDATE the product it already made, even
// when the model spells the name slightly differently the second time. This is
// the whole reason a customer can be told "re-import to relink the PDF".
func TestImportProducts_ARespelledNameUpdatesInsteadOfDuplicating(t *testing.T) {
	repo := &importProductRepo{existing: []*domain.Product{{
		ID:        "p1",
		AccountID: importAccount,
		Name:      "Analisador de Bilirrubina - Bilirrubinômetro AG",
		Currency:  "BRL",
	}}}
	uc := NewImportProductsUseCase(repo, nil)

	res, err := uc.Import(context.Background(), importAccount, ImportProductsInput{
		Rows: []domain.ProductImportRow{{
			Name:       "Analisador de Bilirrubina BILIRRUBINÔMETRO AG",
			PriceCents: 129900,
			Currency:   "BRL",
		}},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if res.Created != 0 {
		t.Fatalf("created %d; a respelled name must not mint a twin", res.Created)
	}
	if res.Updated != 1 {
		t.Fatalf("updated %d, want 1", res.Updated)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created %d products, want none", len(repo.created))
	}
	if len(repo.updated) != 1 || repo.updated[0].PriceCents != 129900 {
		t.Fatalf("the existing product must get the new price: %+v", repo.updated)
	}
	// The stored spelling is left alone: the catalogue is the person's, and an
	// import that quietly renames their entries is an import they cannot trust.
	if got := repo.updated[0].Name; got != "Analisador de Bilirrubina - Bilirrubinômetro AG" {
		t.Fatalf("name was rewritten to %q", got)
	}
}
