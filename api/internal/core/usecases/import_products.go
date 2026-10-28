package usecases

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Importing the price list a distributor already has.
//
// Two steps on purpose. `Preview` reads the file and reports what it found,
// including every row it could not read; `Import` creates products from the rows
// a PERSON confirmed. A one-shot "upload and create" would put four hundred rows
// into a catalogue on the strength of a parser nobody checked — and the row this
// parser most needs checked is a price it refuses to guess.

// maxImportBytes bounds the read. A price list is text; anything past this is
// either the wrong file or an attempt to make us hold it in memory.
const maxImportBytes = 2 << 20 // 2 MiB

// maxImportRows caps one import. Past this the preview stops being reviewable,
// which is the point of the preview.
const maxImportRows = 2000

var ErrProductImportTooLarge = &domain.AppError{
	Code:    "PRODUCT_IMPORT_TOO_LARGE",
	Message: "o arquivo é grande demais. Importe até 2000 produtos por vez",
}

var ErrProductImportUnreadable = &domain.AppError{
	Code:    "PRODUCT_IMPORT_UNREADABLE",
	Message: "não consegui ler o arquivo como CSV. Salve a planilha como CSV e tente de novo",
}

type ImportProductsUseCase struct {
	products domain.ProductRepository
	accounts domain.AccountRepository
	// sources verifies a brochure id the CLIENT sent back.
	//
	// The preview hands the browser a source id and the confirm posts it
	// back — the only way the reviewed rows keep their link to the file they
	// were read from. That makes it a foreign key arriving from outside, and
	// a foreign key from outside is checked or it is a way to point one
	// account's product at another account's document. Optional: unwired, the
	// link is simply not made.
	sources domain.ProductSourceRepository
}

// SetSourceRepo wires the brochure-ownership check.
func (uc *ImportProductsUseCase) SetSourceRepo(r domain.ProductSourceRepository) {
	uc.sources = r
}

// ownedSourceID returns the brochure id only when it belongs to this account.
//
// Silent on rejection rather than failing the import: a row that cannot prove
// its origin is still a product the person reviewed and asked for. It arrives
// without a document instead of not arriving.
func (uc *ImportProductsUseCase) ownedSourceID(ctx context.Context, accountID, sourceID string) string {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return ""
	}
	if uc.sources == nil {
		return ""
	}
	src, err := uc.sources.GetByID(ctx, sourceID, accountID)
	if err != nil || src == nil {
		slog.WarnContext(ctx, "product import: ignoring a brochure id this account does not own",
			"account_id", accountID, "source_id", sourceID)
		return ""
	}
	return sourceID
}

func NewImportProductsUseCase(
	products domain.ProductRepository,
	accounts domain.AccountRepository,
) *ImportProductsUseCase {
	return &ImportProductsUseCase{products: products, accounts: accounts}
}

// ProductImportPreview is what the review screen renders.
type ProductImportPreview struct {
	Rows []domain.ProductImportRow `json:"rows"`
	// Importable and Problems are counted server-side so the screen states the
	// same numbers the import will act on.
	Importable int `json:"importable"`
	Problems   int `json:"problems"`
	// AlreadyInCatalogue names rows whose product already exists. Reported rather
	// than skipped silently: somebody re-importing an updated price list needs to
	// know the old row is the one that stays.
	AlreadyInCatalogue int `json:"already_in_catalogue"`
}

// Preview parses the file and reports what it found, creating nothing.
func (uc *ImportProductsUseCase) Preview(
	ctx context.Context, accountID string, file io.Reader,
) (*ProductImportPreview, error) {
	records, err := readCSV(file)
	if err != nil {
		return nil, err
	}

	rows, err := domain.BuildImportRows(records, uc.accountCurrency(ctx, accountID))
	if err != nil {
		return nil, err
	}
	if len(rows) > maxImportRows {
		return nil, ErrProductImportTooLarge
	}

	return uc.previewFromRows(ctx, accountID, rows)
}

// previewFromRows applies the dedup and the server-side counting to rows that
// came from ANY origin — the CSV parser above, or the PDF extraction. One
// implementation so "já está no catálogo" can never mean different things on
// two screens, and the existing-name preload stays a single query, not N+1.
func (uc *ImportProductsUseCase) previewFromRows(
	ctx context.Context, accountID string, rows []domain.ProductImportRow,
) (*ProductImportPreview, error) {
	existing, err := uc.existingProducts(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := &ProductImportPreview{Rows: rows}
	for i := range rows {
		current := existing[productMatchKey(rows[i].Name)]
		switch {
		case !rows[i].Importable():
			out.Problems++
		case current != nil && !rowChangesProduct(current, rows[i]):
			out.AlreadyInCatalogue++
			// Not a problem — a fact. Nothing in this row would move, so
			// importing it would only rewrite a row with its own values.
			out.Rows[i].Problem = "já está no catálogo"
			out.Rows[i].Duplicate = true
			out.Problems++
		case current != nil:
			// The row the person re-imported the list FOR: a new price, a
			// fuller description, or — the case the price-only check used to
			// miss — a technical sheet the stored product does not have yet.
			// Marked as existing so the screen says "will update", and left
			// importable so the import actually does.
			out.AlreadyInCatalogue++
			out.Rows[i].Duplicate = true
			out.Importable++
		default:
			out.Importable++
		}
	}
	return out, nil
}

// ImportProductsInput is the confirmed set. The client sends back the ROWS it
// showed, not the file: the person approved what they saw, and re-parsing would
// let a file that changed underneath be imported instead.
type ImportProductsInput struct {
	Rows []domain.ProductImportRow `json:"rows"`
}

type ImportProductsResult struct {
	Created int `json:"created"`
	// Updated counts rows whose product already existed under the same name
	// and got the file's price (and description, when it carried one). A
	// re-imported price list is a list of new prices, and skipping them was
	// the whole reason the catalogue drifted from the spreadsheet.
	Updated int `json:"updated"`
	// Skipped counts rows that arrived unimportable or were already identical.
	// Reported so a count that does not match the selection has an explanation.
	Skipped int `json:"skipped"`
}

// Import creates a product per confirmed row.
func (uc *ImportProductsUseCase) Import(
	ctx context.Context, accountID string, in ImportProductsInput,
) (*ImportProductsResult, error) {
	if len(in.Rows) == 0 {
		return nil, domain.ErrProductImportEmpty
	}
	if len(in.Rows) > maxImportRows {
		return nil, ErrProductImportTooLarge
	}

	existing, err := uc.existingProducts(ctx, accountID)
	if err != nil {
		return nil, err
	}

	fallback := uc.accountCurrency(ctx, accountID)
	result := &ImportProductsResult{}

	for _, row := range in.Rows {
		name := strings.TrimSpace(row.Name)
		// Re-checked here, never trusted from the client: a row that arrives with
		// its problem cleared would otherwise create the product the preview
		// refused.
		if name == "" || row.PriceCents < 0 {
			result.Skipped++
			continue
		}
		if current := existing[productMatchKey(name)]; current != nil {
			if uc.updateExisting(ctx, current, row) {
				result.Updated++
			} else {
				result.Skipped++
			}
			continue
		}

		currency := strings.ToUpper(strings.TrimSpace(row.Currency))
		if currency == "" {
			currency = fallback
		}

		p := &domain.Product{
			AccountID:   accountID,
			Name:        name,
			Description: strings.TrimSpace(row.Description),
			PriceCents:  row.PriceCents,
			Currency:    currency,
			Attributes:  importRowAttributes(row),
			Active:      true,
			// The pointer back to the brochure, carried from the preview the
			// person reviewed. Empty on CSV rows — a spreadsheet has no page
			// to open.
			SourceID:   uc.ownedSourceID(ctx, accountID, row.SourceID),
			SourcePage: row.Page,
		}
		created, err := uc.createProduct(ctx, p)
		if err != nil {
			// One bad row must not lose the rows already created. Counted as
			// skipped and reported in the total.
			result.Skipped++
			continue
		}
		// Within one file too: a second row with the same name now UPDATES
		// the one just created rather than creating a twin.
		existing[productMatchKey(name)] = created
		result.Created++
	}

	return result, nil
}

// updateExisting writes the file's price (and description, when it carries
// one) onto the product already in the catalogue. Reports whether anything
// changed: a row identical to what is there is a skip, not an update.
func (uc *ImportProductsUseCase) updateExisting(
	ctx context.Context, current *domain.Product, row domain.ProductImportRow,
) bool {
	description := strings.TrimSpace(row.Description)
	attrs := mergeProductAttributes(current.Attributes, importRowAttributes(row))
	// Same check as the create path: the id came back from a browser, so it
	// is verified before it is stored, or a re-import is a way to repoint a
	// product at another account's document.
	ownedSource := uc.ownedSourceID(ctx, current.AccountID, row.SourceID)
	linked := ownedSource != "" &&
		(current.SourceID != ownedSource || current.SourcePage != row.Page)

	// "Nothing changed" used to mean price and description only, so
	// re-importing a brochure to pick up its technical sheet was judged
	// identical and skipped — the row carried a whole datasheet the check
	// could not see. It now asks whether ANY of what we store would move.
	if current.PriceCents == row.PriceCents &&
		(description == "" || description == current.Description) &&
		!linked && bytes.Equal(attrs, current.Attributes) {
		return false
	}
	current.PriceCents = row.PriceCents
	if description != "" {
		current.Description = description
	}
	// MERGE, never blank. A CSV row that says nothing about specs must not
	// erase the sheet a brochure gave; the row overwrites what it states and
	// leaves the rest standing.
	current.Attributes = attrs
	if linked {
		current.SourceID = ownedSource
		current.SourcePage = row.Page
	}
	if err := uc.products.Update(ctx, current); err != nil {
		slog.WarnContext(ctx, "product import: update failed",
			"product_id", current.ID, "error", err)
		return false
	}
	return true
}

// rowChangesProduct reports whether importing this row would move anything we
// store. It is the preview's half of the same question updateExisting asks, and
// the two must agree: a row the preview called "já está no catálogo" that the
// import then rewrites (or the reverse) makes the review screen a liar.
func rowChangesProduct(current *domain.Product, row domain.ProductImportRow) bool {
	description := strings.TrimSpace(row.Description)
	if current.PriceCents != row.PriceCents {
		return true
	}
	if description != "" && description != current.Description {
		return true
	}
	if src := strings.TrimSpace(row.SourceID); src != "" &&
		(current.SourceID != src || current.SourcePage != row.Page) {
		return true
	}
	return !bytes.Equal(mergeProductAttributes(current.Attributes, importRowAttributes(row)), current.Attributes)
}

// mergeProductAttributes overlays the row's brochure fields onto whatever the
// product already carries. Keys the row does not mention survive: attributes
// is a shared bag (tags, warranty, things set elsewhere), and an import that
// replaced it wholesale would delete data it never read.
func mergeProductAttributes(current, incoming json.RawMessage) json.RawMessage {
	if len(incoming) == 0 {
		if len(current) == 0 {
			return nil
		}
		return current
	}
	merged := map[string]any{}
	if len(current) > 0 {
		// An unparseable bag is not a reason to lose the new data; start clean
		// and let the row's fields land.
		_ = json.Unmarshal(current, &merged)
	}
	add := map[string]any{}
	if err := json.Unmarshal(incoming, &add); err != nil {
		return current
	}
	for k, v := range add {
		merged[k] = v
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return current
	}
	return out
}

// createProduct goes through the same construction the single-product form uses,
// so ids, timestamps and the attributes default are decided in one place.
func (uc *ImportProductsUseCase) createProduct(ctx context.Context, p *domain.Product) (*domain.Product, error) {
	return NewProductUseCase(uc.products).Create(ctx, CreateProductDTO{
		AccountID:   p.AccountID,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
		Currency:    p.Currency,
		Attributes:  p.Attributes,
		SourceID:    p.SourceID,
		SourcePage:  p.SourcePage,
	})
}

// importRowAttributes folds the brochure-only fields into the product's
// attributes JSONB. CSV rows have none of them and keep the nil → "{}"
// default the product constructor already applies.
func importRowAttributes(row domain.ProductImportRow) json.RawMessage {
	attrs := map[string]any{}
	for key, value := range map[string]string{
		"sku":   strings.TrimSpace(row.SKU),
		"brand": strings.TrimSpace(row.Brand),
		"model": strings.TrimSpace(row.Model),
		"unit":  strings.TrimSpace(row.Unit),
	} {
		if value != "" {
			attrs[key] = value
		}
	}
	if specs := strings.TrimSpace(row.Specs); specs != "" {
		attrs["specs"] = specs
	}
	if len(row.Accessories) > 0 {
		attrs["accessories"] = row.Accessories
	}
	if len(row.Standards) > 0 {
		attrs["standards"] = row.Standards
	}
	if row.Page > 0 {
		attrs["source_page"] = row.Page
	}
	if len(attrs) == 0 {
		return nil
	}
	raw, err := json.Marshal(attrs)
	if err != nil {
		return nil
	}
	return raw
}

// existingProducts is the dedup key: a catalogue with "Gaze estéril" twice is
// two products nobody can tell apart in a proposal. Keyed by lower-cased,
// trimmed name; the value is what a re-import updates.
func (uc *ImportProductsUseCase) existingProducts(
	ctx context.Context, accountID string,
) (map[string]*domain.Product, error) {
	products, err := uc.products.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*domain.Product, len(products))
	for _, p := range products {
		// Keyed on the NORMALISED name — see productMatchKey. The first
		// spelling of a name wins, so a catalogue that already carries two
		// variants keeps updating the older one rather than flip-flopping.
		key := productMatchKey(p.Name)
		if _, seen := out[key]; !seen {
			out[key] = p
		}
	}
	return out, nil
}

// productMatchKey is how a row finds the product it is about.
//
// Exact-lowercase was the rule, and a PDF import is an AI reading a page: run
// it twice and one run says "Analisador de Bilirrubina - Bilirrubinômetro AG"
// while the other says "Analisador de Bilirrubina BILIRRUBINÔMETRO AG". Under
// the old rule those are two products, and a customer's live catalogue has
// exactly that pair in it.
//
// So the key folds accents, case, punctuation and repeated spaces — the things
// a person would not call a different product. It deliberately does NOT fold
// plurals or drop words: "Aspirador Cirúrgico" and "Aspirador Cirúrgico A-45"
// are different machines, and merging two real products silently overwrites
// one of them, which is worse than a visible duplicate.
func productMatchKey(name string) string {
	folded := strings.ToLower(domain.FoldAccents(strings.TrimSpace(name)))
	var b strings.Builder
	b.Grow(len(folded))
	lastSpace := false
	for _, r := range folded {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSpace = false
		default:
			// Every separator — space, hyphen, dot, slash — collapses to one
			// space, so "BM-100C" and "BM 100C" are the same product and
			// "A-45" still differs from "A-45 MAX".
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// accountCurrency is the fallback for a spreadsheet with no currency column.
//
// A lookup failure falls back to BRL with no error: the import is worth more than
// the perfect label, and the currency column overrides it anyway. The row-level
// consequence is visible in the preview before anything is created.
func (uc *ImportProductsUseCase) accountCurrency(ctx context.Context, accountID string) string {
	if uc.accounts == nil {
		return "BRL"
	}
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return "BRL"
	}
	if c := strings.ToUpper(strings.TrimSpace(account.Currency)); c != "" {
		return c
	}
	return "BRL"
}

// readCSV reads the upload, tolerating the two things a real export gets wrong:
// a semicolon delimiter (what Excel writes in a pt-BR locale) and a UTF-8 BOM.
func readCSV(file io.Reader) ([][]string, error) {
	raw, err := io.ReadAll(io.LimitReader(file, maxImportBytes+1))
	if err != nil {
		return nil, ErrProductImportUnreadable
	}
	if len(raw) > maxImportBytes {
		return nil, ErrProductImportTooLarge
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, domain.ErrProductImportEmpty
	}

	// Excel in a Brazilian locale writes `;` because the comma is the decimal
	// separator — and this is not a nicety, it is the only way such a file parses
	// at all: a COMMA-delimited file cannot express "49,90", because the comma
	// splits the column. Guessing from the header line rather than asking is the
	// difference between "it worked" and a support thread.
	delimiter := ','
	if firstLine := firstLineOf(raw); strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		delimiter = ';'
	}

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = delimiter
	// Rows with a stray extra column are common in hand-edited files; refusing
	// the whole import over one is worse than reading what is there.
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		var parseErr *csv.ParseError
		if errors.As(err, &parseErr) {
			return nil, ErrProductImportUnreadable
		}
		return nil, ErrProductImportUnreadable
	}
	return records, nil
}

func firstLineOf(raw []byte) string {
	if i := bytes.IndexAny(raw, "\r\n"); i >= 0 {
		return string(raw[:i])
	}
	return string(raw)
}
