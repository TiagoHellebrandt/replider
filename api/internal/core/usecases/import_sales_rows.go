package usecases

import (
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Rows of a sales-history import.
//
// The parsing half of import_sales.go: what a spreadsheet column becomes, and
// what stops a line from becoming a sale. Kept apart from the use case so the
// pure parts are testable without a repository, the same split the product
// import has between domain.BuildImportRows and ImportProductsUseCase.

// SaleImportRow is one line of the file, with every problem attached to it.
type SaleImportRow struct {
	// Line is the 1-based line in the original file, header included.
	Line int `json:"line"`
	// Customer is the cell as typed: an e-mail, a phone or a name. Which of
	// the three it is gets decided at resolution time, not here.
	Customer string `json:"customer"`
	// Product is the cell as typed. Matched against the catalogue by name;
	// when nothing matches it becomes the item's free description.
	Product string `json:"product"`
	// Date is the cell as typed; SoldAt is what it parsed to (zero when it
	// did not). Both travel so the preview can show the person the text they
	// wrote next to the date the import will use.
	Date   string    `json:"date"`
	SoldAt time.Time `json:"sold_at"`

	Quantity   int   `json:"quantity"`
	GrossCents int64 `json:"gross_cents"`

	// ContactID / ContactName are the resolution: who in the base this row
	// is. Empty with a Problem when nobody matched.
	ContactID   string `json:"contact_id,omitempty"`
	ContactName string `json:"contact_name,omitempty"`
	// ProductID is set when Product matched the catalogue.
	ProductID string `json:"product_id,omitempty"`

	// Problem is empty when the row may create a sale.
	Problem string `json:"problem,omitempty"`
	// Duplicate marks "already recorded" structurally, so the client renders
	// it as the fact it is rather than with the error icon.
	Duplicate bool `json:"duplicate,omitempty"`
}

// Importable answers whether this row may create a sale.
func (r SaleImportRow) Importable() bool {
	return r.Problem == "" && r.ContactID != "" && !r.SoldAt.IsZero() && r.GrossCents > 0
}

// Problems a row can have. Text, read by the person next to the line number.
const (
	ProblemSaleNoCustomer = "sem cliente"
	// The import does not create customers: a sale filed against a customer
	// the base does not have would be a sale nobody can call about.
	ProblemSaleCustomerUnknown = "cliente não encontrado na base — importe os clientes primeiro"
	ProblemSaleNoDate          = "sem data"
	ProblemSaleBadDate         = "data não reconhecida (use dd/mm/aaaa)"
	ProblemSaleNoAmount        = "sem valor"
	ProblemSaleBadAmount       = "valor não reconhecido"
	ProblemSaleAlreadyRecorded = "venda já registrada"
)

var ErrSaleImportNoHeader = &domain.AppError{
	Code: "SALE_IMPORT_NO_HEADER",
	Message: "não encontrei as colunas de cliente, data e valor. A primeira linha precisa ter os " +
		"títulos das colunas (cliente, produto, data, valor, quantidade)",
}

var ErrSaleImportEmpty = &domain.AppError{
	Code:    "SALE_IMPORT_EMPTY",
	Message: "o arquivo não tem nenhuma linha de venda",
}

var ErrSaleImportTooLarge = &domain.AppError{
	Code:    "SALE_IMPORT_TOO_LARGE",
	Message: "o arquivo é grande demais. Importe até 2000 vendas por vez",
}

var ErrSaleImportUnreadable = &domain.AppError{
	Code:    "SALE_IMPORT_UNREADABLE",
	Message: "não consegui ler o arquivo como CSV. Salve a planilha como CSV e tente de novo",
}

// Header aliases, folded and lower-cased. Portuguese first.
var (
	saleCustomerHeaders = []string{
		"cliente", "customer", "client", "empresa", "company", "contato",
		"contact", "nome", "name", "comprador", "razao social",
	}
	saleProductHeaders = []string{
		"produto", "product", "item", "descricao", "description", "mercadoria",
	}
	saleDateHeaders = []string{
		"data", "date", "data da venda", "vendido em", "emissao", "data de emissao",
	}
	saleAmountHeaders = []string{
		"valor", "total", "valor total", "amount", "gross", "preco", "price",
	}
	saleQuantityHeaders = []string{"quantidade", "qtd", "qtde", "qty", "quantity"}
)

// buildSaleImportRows turns parsed CSV records into rows.
//
// Contact and product resolution happen later, against the base; here a row
// only learns what it could not read.
func buildSaleImportRows(records [][]string) ([]SaleImportRow, error) {
	if len(records) == 0 {
		return nil, ErrSaleImportEmpty
	}

	header := records[0]
	customerCol := domain.MatchHeader(header, saleCustomerHeaders)
	dateCol := domain.MatchHeader(header, saleDateHeaders)
	amountCol := domain.MatchHeader(header, saleAmountHeaders)
	if customerCol < 0 || dateCol < 0 || amountCol < 0 {
		return nil, ErrSaleImportNoHeader
	}
	productCol := domain.MatchHeader(header, saleProductHeaders)
	quantityCol := domain.MatchHeader(header, saleQuantityHeaders)

	var rows []SaleImportRow
	for i, record := range records[1:] {
		if saleRowBlank(record) {
			continue
		}
		row := SaleImportRow{
			Line:     i + 2,
			Customer: strings.TrimSpace(saleCell(record, customerCol)),
			Product:  strings.TrimSpace(saleCell(record, productCol)),
			Date:     strings.TrimSpace(saleCell(record, dateCol)),
		}
		row.Quantity = parseQuantity(saleCell(record, quantityCol))
		row.SoldAt, _ = ParseSaleDate(row.Date)

		amount := strings.TrimSpace(saleCell(record, amountCol))
		if cents, ok, ambiguous := domain.ParseMoney(amount); ok {
			row.GrossCents = cents
		} else if ambiguous {
			row.Problem = domain.ProblemAmbiguousPrice
		}
		row.Problem = firstSaleRowProblem(row, amount)
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, ErrSaleImportEmpty
	}
	return rows, nil
}

// firstSaleRowProblem reports what stops the row, in the order a person would
// fix it. One problem per row: the preview points at a line, not at a list.
func firstSaleRowProblem(row SaleImportRow, rawAmount string) string {
	switch {
	case row.Customer == "":
		return ProblemSaleNoCustomer
	case row.Date == "":
		return ProblemSaleNoDate
	case row.SoldAt.IsZero():
		return ProblemSaleBadDate
	case row.Problem != "":
		// The ambiguous-price refusal set by the parser.
		return row.Problem
	case strings.TrimSpace(rawAmount) == "":
		return ProblemSaleNoAmount
	case row.GrossCents <= 0:
		return ProblemSaleBadAmount
	}
	return ""
}

// saleDateLayouts is what a Brazilian spreadsheet writes, plus ISO.
var saleDateLayouts = []string{
	"02/01/2006", "2006-01-02", "02-01-2006", "02/01/06", "2/1/2006",
	"02/01/2006 15:04", "2006-01-02 15:04:05", time.RFC3339,
}

// ParseSaleDate reads a date the way a person typed it.
//
// Date-only values land at midnight UTC — the import has no time of day to
// offer and the portfolio works in days, so pretending a precision the file
// does not have would only move a sale across a day boundary somewhere.
func ParseSaleDate(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range saleDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseQuantity(raw string) int {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	// "200", "200,00" and "200.0" are all two hundred to a person.
	if i := strings.IndexAny(s, ",."); i >= 0 {
		s = s[:i]
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			return 0
		}
	}
	return n
}

func saleCell(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func saleRowBlank(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// saleImportKey is the idempotency key: one customer, one day, one total.
//
// The day rather than the instant, because a re-export of the same ERP report
// carries the same date and the same amount and nothing else stable.
func saleImportKey(contactID string, soldAt time.Time, grossCents int64) string {
	return contactID + "|" + soldAt.UTC().Format("2006-01-02") + "|" + strconv.FormatInt(grossCents, 10)
}
