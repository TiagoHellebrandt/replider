package domain

import (
	"strconv"
	"strings"
)

// Importing the price list a distributor already has.
//
// The second half of pillar 2 — "pegue os produtos que ela já tem". Until now a
// catalogue was built one product at a time in a form, which for somebody with
// four hundred SKUs in a spreadsheet is a reason not to start.
//
// Everything here is a PREVIEW. Nothing is created until a person confirms the
// rows, for the same reason the Sent-folder candidates work that way: a list a
// machine read out of a file cannot act on its own.

// ProductImportRow is one line of the file, parsed.
type ProductImportRow struct {
	// Line is the 1-based line in the original file, header included, so a
	// problem can be pointed at the row the person is looking at in Excel.
	Line        int    `json:"line"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	// PDF-brochure origin fields. Empty on CSV rows — the CSV has no columns
	// for them. They travel to the preview so the person can check what the
	// model read; on confirmation they land in the product's attributes.
	SKU   string `json:"sku,omitempty"`
	Brand string `json:"brand,omitempty"`
	Model string `json:"model,omitempty"`
	Unit  string `json:"unit,omitempty"`
	// Specs is the brochure's technical block, transcribed rather than
	// summarised: dimensions, power, ranges, materials, what ships with it.
	// A distributor answering "cabe na minha UTI?" needs the sheet, and the
	// old extraction threw it away — 126 characters on average survived.
	Specs string `json:"specs,omitempty"`
	// Accessories are the optional items offered FOR this product. They exist
	// so a "Válvula reguladora" stops arriving as a product of its own — an
	// accessory is a row in the catalogue nobody can sell alone.
	Accessories []string `json:"accessories,omitempty"`
	// Standards are the norms and certifications the brochure cites
	// (NBR IEC 60601, ANVISA…). In this market they are half the argument in
	// a bid, and they were being dropped.
	Standards []string `json:"standards,omitempty"`
	// SourceID / SourcePage point back at the archived brochure: which file,
	// which page. Empty on CSV rows.
	SourceID string `json:"source_id,omitempty"`
	// Page is the 1-based brochure page the row was read from, so a human can
	// open the PDF and check the model's source. 0 on CSV rows.
	Page int `json:"page,omitempty"`
	// Problem is empty when the row is importable. When it is not, the row still
	// travels: a silently dropped line is a product the person believes they
	// imported.
	Problem string `json:"problem,omitempty"`
	// Duplicate marks the "já está no catálogo" case STRUCTURALLY. It is a
	// fact, not a mistake — the existing product stays, nothing is
	// overwritten — and the client needs to render it neutrally instead of
	// painting it with the same error icon as an unreadable price. The text
	// in Problem stays for humans; this flag is for the renderer.
	Duplicate bool `json:"duplicate,omitempty"`
}

// Importable answers whether this row may create a product.
func (r ProductImportRow) Importable() bool {
	return r.Problem == "" && strings.TrimSpace(r.Name) != ""
}

// Problems a row can have. Text rather than codes because these are read by the
// person who wrote the spreadsheet, next to the line number.
const (
	ProblemNoName = "sem nome"
	// The one that matters most. See ParseMoney.
	ProblemAmbiguousPrice  = "preço ambíguo: use duas casas decimais (ex.: 1234,00) ou remova o separador de milhar"
	ProblemBadPrice        = "preço não reconhecido"
	ProblemDuplicateInFile = "nome repetido no arquivo"
)

// ErrProductImportNoHeader is what an unreadable file reports. A file whose
// header we cannot recognise is not an empty import — it is the wrong file, or
// the right file with different column names, and both need saying.
var ErrProductImportNoHeader = &AppError{
	Code:    "PRODUCT_IMPORT_NO_HEADER",
	Message: "não encontrei uma coluna de nome no arquivo. A primeira linha precisa ter os títulos das colunas (nome, preço, descrição)",
}

var ErrProductImportEmpty = &AppError{
	Code:    "PRODUCT_IMPORT_EMPTY",
	Message: "o arquivo não tem nenhuma linha de produto",
}

// Header aliases, folded and lowercased. Portuguese first: the spreadsheet
// belongs to a Brazilian distributor, and "preço" with the cedilla-less typo is
// what a real export looks like.
var (
	nameHeaders  = []string{"nome", "produto", "descricao curta", "item", "name", "product"}
	priceHeaders = []string{
		"preco", "preço", "preco unitario", "preço unitário", "valor",
		"price", "unit price", "unit_price",
	}
	descHeaders = []string{
		"descricao", "descrição", "detalhe", "detalhes", "observacao",
		"description", "details",
	}
	currencyHeaders = []string{"moeda", "currency"}
)

// ParseMoney turns a spreadsheet's price into cents, and REFUSES when the string
// could mean two things a thousand apart.
//
// "1.234,56" and "1,234.56" are the same amount; "1,234" is either 1234 or
// 1.234, and no rule recovers which. This codebase has already shipped that bug
// in both directions, so a single separator followed by exactly three digits is
// reported rather than guessed: it costs the person one look at one row, where a
// guess costs them a price list that is wrong by a factor of a thousand.
//
// Returns (cents, ok, ambiguous).
func ParseMoney(raw string) (int64, bool, bool) {
	s := strings.TrimSpace(raw)
	// Currency symbols and spaces are noise around the number, including the
	// non-breaking space Excel likes to emit.
	for _, junk := range []string{"R$", "US$", "$", " ", " ", "BRL", "USD", "brl", "usd"} {
		s = strings.ReplaceAll(s, junk, "")
	}
	if s == "" {
		return 0, false, false
	}

	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	dot, comma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")

	var intPart, fracPart string
	switch {
	case dot >= 0 && comma >= 0:
		// Both present: the LAST one is the decimal separator, the other groups.
		decimal, group := dot, byte(',')
		if comma > dot {
			decimal, group = comma, byte('.')
		}
		var ok bool
		intPart, ok = groupedDigits(s[:decimal], group)
		if !ok {
			return 0, false, false
		}
		fracPart = s[decimal+1:]
	case dot >= 0 || comma >= 0:
		sep := dot
		if comma >= 0 {
			sep = comma
		}
		frac := s[sep+1:]
		switch len(frac) {
		case 1, 2:
			if !allDigits(s[:sep]) {
				return 0, false, false
			}
			intPart, fracPart = s[:sep], frac
		case 3:
			// The ambiguous case. Refusing is the whole point of this function.
			return 0, false, true
		default:
			return 0, false, false
		}
	default:
		if !allDigits(s) {
			return 0, false, false
		}
		intPart, fracPart = s, ""
	}

	if intPart == "" {
		intPart = "0"
	}
	units, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, false, false
	}
	// One decimal digit means tenths: "10,5" is 10.50, not 10.05.
	cents := int64(0)
	if fracPart != "" {
		if !allDigits(fracPart) {
			return 0, false, false
		}
		padded := (fracPart + "00")[:2]
		cents, err = strconv.ParseInt(padded, 10, 64)
		if err != nil {
			return 0, false, false
		}
	}

	total := units*100 + cents
	if negative {
		total = -total
	}
	return total, true, false
}

// groupedDigits removes the grouping separators from an integer part, and
// reports whether the grouping was actually valid.
//
// Stripping without checking accepted "1..2,3" as 123: every separator vanished
// and what was left was digits. A price list is not the place to be forgiving —
// a string nobody can read as one number is a row for the person to look at.
func groupedDigits(s string, separator byte) (string, bool) {
	if s == "" {
		return "0", true
	}
	groups := strings.Split(s, string(separator))
	for i, g := range groups {
		if g == "" || !allDigits(g) {
			return "", false
		}
		// The first group is 1-3 digits; every later one is exactly 3. That is
		// what grouping MEANS, and anything else is a typo or another separator.
		if i == 0 {
			if len(g) > 3 {
				// No grouping at all is fine — "1234,56" is how most exports
				// write it. What is not fine is a long first group FOLLOWED by
				// groups, which cannot be read as one number.
				if len(groups) > 1 {
					return "", false
				}
			}
		} else if len(g) != 3 {
			return "", false
		}
	}
	return strings.Join(groups, ""), true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// MatchHeader returns the index of the first column whose title matches one of
// [aliases], or -1.
func MatchHeader(header []string, aliases []string) int {
	for i, cell := range header {
		normalized := FoldAccents(strings.ToLower(strings.TrimSpace(cell)))
		for _, alias := range aliases {
			if normalized == FoldAccents(alias) {
				return i
			}
		}
	}
	return -1
}

// BuildImportRows turns parsed CSV records into rows, keeping every problem it
// finds attached to the line that has it.
//
// The default currency is the ACCOUNT's, passed in: a spreadsheet with no
// currency column belongs to somebody who sells in one currency, and inferring
// "BRL" for a USD account would relabel their whole price list.
func BuildImportRows(records [][]string, defaultCurrency string) ([]ProductImportRow, error) {
	if len(records) == 0 {
		return nil, ErrProductImportEmpty
	}

	header := records[0]
	nameCol := MatchHeader(header, nameHeaders)
	if nameCol < 0 {
		return nil, ErrProductImportNoHeader
	}
	priceCol := MatchHeader(header, priceHeaders)
	descCol := MatchHeader(header, descHeaders)
	currencyCol := MatchHeader(header, currencyHeaders)

	if len(records) == 1 {
		return nil, ErrProductImportEmpty
	}

	fallback := strings.ToUpper(strings.TrimSpace(defaultCurrency))
	if fallback == "" {
		fallback = "BRL"
	}

	seen := make(map[string]bool, len(records))
	rows := make([]ProductImportRow, 0, len(records)-1)

	for i, record := range records[1:] {
		row := ProductImportRow{Line: i + 2, Currency: fallback}

		row.Name = strings.TrimSpace(cell(record, nameCol))
		row.Description = strings.TrimSpace(cell(record, descCol))
		if c := strings.ToUpper(strings.TrimSpace(cell(record, currencyCol))); c != "" {
			row.Currency = c
		}

		switch {
		case row.Name == "":
			// A blank line at the end of a spreadsheet is the norm, not an error:
			// skip it entirely rather than reporting a problem the person did not
			// cause.
			if blank(record) {
				continue
			}
			row.Problem = ProblemNoName
		case seen[strings.ToLower(row.Name)]:
			// Two lines with one name would create two products the person then
			// has to tell apart.
			row.Problem = ProblemDuplicateInFile
		default:
			seen[strings.ToLower(row.Name)] = true
		}

		if row.Problem == "" && priceCol >= 0 {
			cents, ok, ambiguous := ParseMoney(cell(record, priceCol))
			switch {
			case ambiguous:
				row.Problem = ProblemAmbiguousPrice
			case !ok && strings.TrimSpace(cell(record, priceCol)) != "":
				row.Problem = ProblemBadPrice
			default:
				row.PriceCents = cents
			}
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, ErrProductImportEmpty
	}
	return rows, nil
}

func cell(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func blank(record []string) bool {
	for _, c := range record {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
