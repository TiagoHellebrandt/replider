package usecases

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// Importing the sales history a distributor already has.
//
// After the products and the customers come in, the product knows WHO the
// account sells to and WHAT it sells — and nothing about who bought what,
// which is the only thing the portfolio, the reorder engine and the cross-sell
// read. This is the door for that history: a spreadsheet out of the ERP, one
// line per sale, into `sales` with source='imported'.
//
// Same preview→confirm shape as the customer list. The rows this file most
// needs a person to look at are the ones it refuses to guess at: a customer
// the base does not have, a date it could not read, a sale already recorded.

// SaleImportIndex answers "is this sale already recorded?" for one row.
//
// A narrow port rather than a method on domain.SaleRepository, which a dozen
// test doubles implement.
type SaleImportIndex interface {
	HasSaleAt(ctx context.Context, accountID, contactID string, soldAt time.Time, grossCents int64) (bool, error)
}

type ImportSalesUseCase struct {
	sales      domain.SaleRepository
	index      SaleImportIndex
	contacts   domain.ContactRepository
	identities domain.ContactIdentityRepository
	products   domain.ProductRepository
	accounts   domain.AccountRepository
}

func NewImportSalesUseCase(
	sales domain.SaleRepository,
	index SaleImportIndex,
	contacts domain.ContactRepository,
	products domain.ProductRepository,
	accounts domain.AccountRepository,
) *ImportSalesUseCase {
	return &ImportSalesUseCase{
		sales: sales, index: index, contacts: contacts,
		products: products, accounts: accounts,
	}
}

// SetIdentityRepo wires the identity index, so a customer typed as an e-mail
// or a phone resolves to the contact that address belongs to. Optional: without
// it resolution falls back to the channel key.
func (uc *ImportSalesUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identities = r
}

// SaleImportPreview is what the review screen renders.
type SaleImportPreview struct {
	Rows []SaleImportRow `json:"rows"`
	// Counted server-side so the screen states the numbers the import will
	// act on.
	Importable int `json:"importable"`
	Problems   int `json:"problems"`
	// AlreadyRecorded names rows whose sale the account already has.
	AlreadyRecorded int `json:"already_recorded"`
	// UnknownCustomers names rows whose customer is not in the base — the
	// number that tells the person to import the customer list first.
	UnknownCustomers int `json:"unknown_customers"`
}

// Preview parses the file and reports what it found, creating nothing.
func (uc *ImportSalesUseCase) Preview(
	ctx context.Context, accountID string, file io.Reader,
) (*SaleImportPreview, error) {
	records, err := readSaleCSV(file)
	if err != nil {
		return nil, err
	}
	rows, err := buildSaleImportRows(records)
	if err != nil {
		return nil, err
	}
	if len(rows) > maxImportRows {
		return nil, ErrSaleImportTooLarge
	}
	return uc.annotate(ctx, accountID, rows), nil
}

// ImportSalesInput is the confirmed set: the ROWS the preview showed, not the
// file. The person approved what they saw.
type ImportSalesInput struct {
	Rows []SaleImportRow `json:"rows"`
}

type ImportSalesResult struct {
	Created int `json:"created"`
	// Skipped counts rows that arrived unimportable or already recorded.
	Skipped int `json:"skipped"`
}

// Import records a sale per confirmed row.
func (uc *ImportSalesUseCase) Import(
	ctx context.Context, accountID string, in ImportSalesInput,
) (*ImportSalesResult, error) {
	if len(in.Rows) == 0 {
		return nil, ErrSaleImportEmpty
	}
	if len(in.Rows) > maxImportRows {
		return nil, ErrSaleImportTooLarge
	}

	// Everything is re-derived and nothing is trusted from the client: a row
	// arriving with its problem cleared, or with a contact id the preview
	// never resolved, would otherwise file a sale the preview declined to.
	cleaned := make([]SaleImportRow, 0, len(in.Rows))
	for _, row := range in.Rows {
		cleaned = append(cleaned, cleanSaleImportRow(row))
	}
	preview := uc.annotate(ctx, accountID, cleaned)

	currency := uc.accountCurrency(ctx, accountID)
	result := &ImportSalesResult{}
	for _, row := range preview.Rows {
		if !row.Importable() {
			result.Skipped++
			continue
		}
		sale := &domain.Sale{
			ID:         ulid.Make().String(),
			AccountID:  accountID,
			ContactID:  row.ContactID,
			GrossCents: row.GrossCents,
			Currency:   currency,
			SoldAt:     row.SoldAt,
			Source:     domain.SaleImported,
		}
		if item, ok := saleImportItem(row); ok {
			sale.Items = []domain.SaleItem{item}
		}
		if problems := sale.Validate(); len(problems) > 0 {
			result.Skipped++
			continue
		}
		if err := uc.sales.Create(ctx, sale); err != nil {
			// One bad row must not lose the rows already recorded.
			slog.WarnContext(ctx, "sale import: create failed",
				"line", row.Line, "error", err)
			result.Skipped++
			continue
		}
		result.Created++
	}
	return result, nil
}

// saleImportItem is the line the portfolio and the cross-sell will read.
//
// The product id when the catalogue matched, so the reorder engine can key
// on the product; the description either way, so a sale of something not in
// the catalogue still names what was sold.
func saleImportItem(row SaleImportRow) (domain.SaleItem, bool) {
	if row.Product == "" && row.ProductID == "" {
		return domain.SaleItem{}, false
	}
	item := domain.SaleItem{
		Description: row.Product, Quantity: row.Quantity, ProductID: row.ProductID,
	}
	if row.Quantity > 0 {
		item.UnitCents = row.GrossCents / int64(row.Quantity)
	}
	return item, true
}

// cleanSaleImportRow re-derives one confirmed row from scratch: the typed
// cells stay, everything the server decides is decided again.
func cleanSaleImportRow(row SaleImportRow) SaleImportRow {
	clean := SaleImportRow{
		Line:       row.Line,
		Customer:   strings.TrimSpace(row.Customer),
		Product:    strings.TrimSpace(row.Product),
		Date:       strings.TrimSpace(row.Date),
		Quantity:   row.Quantity,
		GrossCents: row.GrossCents,
	}
	if clean.Quantity < 0 {
		clean.Quantity = 0
	}
	// The date the preview parsed is accepted back when the typed cell is
	// gone or unreadable — an API client may send only sold_at.
	if parsed, ok := ParseSaleDate(clean.Date); ok {
		clean.SoldAt = parsed
	} else if !row.SoldAt.IsZero() {
		clean.SoldAt = row.SoldAt
	}
	clean.Problem = firstSaleRowProblem(clean, "x")
	if clean.Problem == "" && clean.GrossCents <= 0 {
		clean.Problem = ProblemSaleBadAmount
	}
	return clean
}

// annotate resolves every row against the base and marks the duplicates.
//
// Shared by Preview and Import so "cliente não encontrado" can never mean
// different things on the two calls.
func (uc *ImportSalesUseCase) annotate(
	ctx context.Context, accountID string, rows []SaleImportRow,
) *SaleImportPreview {
	catalogue := uc.catalogueByName(ctx, accountID)
	resolver := newSaleContactResolver(uc.contacts, uc.identities)
	seen := make(map[string]bool, len(rows))

	out := &SaleImportPreview{Rows: rows}
	for i := range out.Rows {
		row := &out.Rows[i]
		if row.Problem != "" {
			out.Problems++
			continue
		}

		if p := catalogue[strings.ToLower(strings.TrimSpace(row.Product))]; p != nil {
			row.ProductID = p.ID
		}

		contact := resolver.resolve(ctx, accountID, row.Customer)
		if contact == nil {
			row.Problem = ProblemSaleCustomerUnknown
			out.UnknownCustomers++
			out.Problems++
			continue
		}
		row.ContactID = contact.ID
		row.ContactName = contactDisplayName(contact)

		key := saleImportKey(contact.ID, row.SoldAt, row.GrossCents)
		recorded := seen[key]
		if !recorded && uc.index != nil {
			// A lookup failure reads as "not recorded", erring toward a
			// duplicate the person can delete rather than a sale silently
			// not imported.
			if has, err := uc.index.HasSaleAt(ctx, accountID, contact.ID, row.SoldAt, row.GrossCents); err == nil && has {
				recorded = true
			}
		}
		if recorded {
			row.Problem = ProblemSaleAlreadyRecorded
			row.Duplicate = true
			out.AlreadyRecorded++
			out.Problems++
			continue
		}
		seen[key] = true
		out.Importable++
	}
	return out
}

// catalogueByName is the product match: lower-cased, trimmed name.
func (uc *ImportSalesUseCase) catalogueByName(ctx context.Context, accountID string) map[string]*domain.Product {
	out := map[string]*domain.Product{}
	if uc.products == nil {
		return out
	}
	products, err := uc.products.ListByAccountID(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "sale import: catalogue read failed", "error", err)
		return out
	}
	for _, p := range products {
		key := strings.ToLower(strings.TrimSpace(p.Name))
		if key != "" {
			out[key] = p
		}
	}
	return out
}

func (uc *ImportSalesUseCase) accountCurrency(ctx context.Context, accountID string) string {
	if uc.accounts == nil {
		return "brl"
	}
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil || account.Currency == "" {
		return "brl"
	}
	return account.Currency
}

func contactDisplayName(c *domain.Contact) string {
	if c.Name != nil && strings.TrimSpace(*c.Name) != "" {
		return strings.TrimSpace(*c.Name)
	}
	return c.ExternalID
}

// saleContactResolver finds the contact a "cliente" cell names.
//
// An address (e-mail or phone) goes through the shared contactResolver — the
// one place that knows every spelling a door ever wrote for a person. A bare
// name is this import's own question: the whole name, case-insensitively,
// and only when exactly one contact carries it. Cached per file: a history
// has the same customer on hundreds of lines.
type saleContactResolver struct {
	contacts domain.ContactRepository
	shared   *contactResolver
	cache    map[string]*domain.Contact
}

func newSaleContactResolver(contacts domain.ContactRepository, identities domain.ContactIdentityRepository) *saleContactResolver {
	return &saleContactResolver{
		contacts: contacts,
		shared:   newContactResolver(contacts, identities),
		cache:    map[string]*domain.Contact{},
	}
}

func (r *saleContactResolver) resolve(ctx context.Context, accountID, raw string) *domain.Contact {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" || r.contacts == nil {
		return nil
	}
	if c, ok := r.cache[key]; ok {
		return c
	}
	c := r.lookup(ctx, accountID, strings.TrimSpace(raw))
	r.cache[key] = c
	return c
}

func (r *saleContactResolver) lookup(ctx context.Context, accountID, raw string) *domain.Contact {
	switch {
	case strings.Contains(raw, "@"):
		email := strings.ToLower(raw)
		return r.shared.lookup(ctx, accountID, "", email, email)
	case looksLikePhone(raw):
		phone, ok := domain.NormalizeTypedPhone(raw)
		if !ok {
			return nil
		}
		return r.shared.lookup(ctx, accountID, phone, "", "")
	}
	// A ULID from an API client resolves directly.
	if len(raw) == 26 {
		if c, err := r.contacts.GetByID(ctx, raw, accountID); err == nil && c != nil {
			return c
		}
	}
	return r.byName(ctx, accountID, raw)
}

// byName matches the whole name, case-insensitively. ONE match only: two
// "Clínica Aurora" in the base is a question for the person, not a coin toss.
func (r *saleContactResolver) byName(ctx context.Context, accountID, name string) *domain.Contact {
	list, err := r.contacts.ListByAccountID(ctx, accountID, name, 20, 0)
	if err != nil {
		return nil
	}
	want := strings.ToLower(strings.TrimSpace(name))
	var found *domain.Contact
	for _, c := range list {
		if c == nil || c.Name == nil || strings.ToLower(strings.TrimSpace(*c.Name)) != want {
			continue
		}
		if found != nil {
			return nil
		}
		found = c
	}
	return found
}

// looksLikePhone: digits with the punctuation a person types around them,
// and at least a DDD's worth of them.
func looksLikePhone(s string) bool {
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == '-' || r == ' ' || r == '(' || r == ')' || r == '.':
		default:
			return false
		}
	}
	return digits >= 8
}

// readSaleCSV reads the upload with the same tolerances the price list needs
// and reports failures in this import's own words.
func readSaleCSV(file io.Reader) ([][]string, error) {
	records, err := readCSV(file)
	switch err {
	case nil:
		return records, nil
	case ErrProductImportTooLarge:
		return nil, ErrSaleImportTooLarge
	case ErrProductImportUnreadable:
		return nil, ErrSaleImportUnreadable
	case domain.ErrProductImportEmpty:
		return nil, ErrSaleImportEmpty
	default:
		return nil, err
	}
}
