package domain

import "strings"

// Importing the customers a distributor already has.
//
// The other half of pillar 2. The catalogue can now arrive as a file; the
// customer list could not arrive at all — there is no endpoint that creates a
// contact. A contact was born only from an inbound message or from prospecting,
// which means the product asked somebody with two hundred existing customers to
// wait for each of them to write first.
//
// Same two-step shape as the price list, for the same reason: a preview that
// reports what it read, then a create from rows a person confirmed.
//
// It deliberately does NOT create companies. `companies` and `contacts.company_id`
// exist in the schema and nothing in the product writes or reads either
// (cmd/api/main.go discards the repository outright), so filling them would
// produce rows no screen can show — work that looks done and is not. The company
// name is kept where it is visible: on the contact.

// CustomerImportRow is one line of the file, parsed.
type CustomerImportRow struct {
	// Line is the 1-based line in the original file, header included, so a
	// problem points at the row the person is looking at in Excel.
	Line int `json:"line"`
	// Company is the organization; Name is the person at it. Both travel, even
	// though only one is needed, because the preview should say which column it
	// read rather than merging them into a single label.
	Company string `json:"company"`
	Name    string `json:"name"`
	// Phone is E.164 when it could be read at all, otherwise empty with a
	// Problem set. Email is lower-cased.
	Phone string `json:"phone"`
	Email string `json:"email"`
	// Problem is empty when the row may create a contact. When it is not, the
	// row still travels — a silently dropped line is a customer the person
	// believes they imported.
	Problem string `json:"problem,omitempty"`
	// Duplicate marks the "já está na sua base" case structurally, so the
	// client renders it as the fact it is (the existing customer stays)
	// instead of with the same error icon as an unreadable phone.
	Duplicate bool `json:"duplicate,omitempty"`
}

// Importable answers whether this row may create a contact.
func (r CustomerImportRow) Importable() bool {
	return r.Problem == "" && (r.Phone != "" || r.Email != "")
}

// DisplayName is what the contact will be called. The person when there is one,
// the organization otherwise — for a B2B customer list the clinic's name is
// usually the only name in the file, and a contact called "" is unusable in the
// approval screen where somebody has to recognise who they are writing to.
func (r CustomerImportRow) DisplayName() string {
	if n := strings.TrimSpace(r.Name); n != "" {
		return n
	}
	return strings.TrimSpace(r.Company)
}

// Problems a row can have. Text, not codes: these are read by the person who
// wrote the spreadsheet, next to the line number.
const (
	ProblemNoCustomerName = "sem nome de cliente nem de contato"
	// A row with a name and no way to reach them is a note, not a customer.
	ProblemNoAddress = "sem telefone nem e-mail"
	// Refused rather than dropped. Importing the customer while quietly
	// discarding the number they typed leaves a card that says the customer has
	// no phone, which is worse than a row they have to look at.
	ProblemBadPhone         = "telefone não reconhecido (use DDD + número, ex.: 51 98888-7766)"
	ProblemBadEmail         = "e-mail não reconhecido"
	ProblemDuplicatePhone   = "telefone repetido no arquivo"
	ProblemDuplicateEmail   = "e-mail repetido no arquivo"
	ProblemAlreadyInAccount = "já está na sua base"
)

var ErrCustomerImportNoHeader = &AppError{
	Code: "CUSTOMER_IMPORT_NO_HEADER",
	Message: "não encontrei uma coluna de telefone nem de e-mail. A primeira linha precisa ter os " +
		"títulos das colunas (empresa, contato, telefone, e-mail)",
}

var ErrCustomerImportEmpty = &AppError{
	Code:    "CUSTOMER_IMPORT_EMPTY",
	Message: "o arquivo não tem nenhuma linha de cliente",
}

// Header aliases, folded and lower-cased. Portuguese first — the spreadsheet
// belongs to a Brazilian distributor.
var (
	companyHeaders = []string{
		"empresa", "cliente", "razao social", "clinica", "hospital",
		"estabelecimento", "company", "customer", "account", "organization",
	}
	contactNameHeaders = []string{
		"contato", "nome", "nome do contato", "responsavel", "comprador",
		"name", "contact", "contact name",
	}
	phoneHeaders = []string{
		"telefone", "celular", "fone", "whatsapp", "whats", "tel",
		"phone", "mobile", "cell",
	}
	emailHeaders = []string{"email", "e-mail", "e mail", "mail"}
)

// BuildCustomerImportRows turns parsed CSV records into rows, keeping every
// problem attached to the line that has it.
func BuildCustomerImportRows(records [][]string) ([]CustomerImportRow, error) {
	if len(records) == 0 {
		return nil, ErrCustomerImportEmpty
	}

	header := records[0]
	phoneCol := MatchHeader(header, phoneHeaders)
	emailCol := MatchHeader(header, emailHeaders)
	// Without one of these the file cannot produce a reachable customer, and
	// saying so beats importing a list of names nobody can write to. A file
	// whose header we do not recognise is the wrong file, or the right file
	// with different column titles — both need saying, and neither is "empty".
	if phoneCol < 0 && emailCol < 0 {
		return nil, ErrCustomerImportNoHeader
	}
	companyCol := MatchHeader(header, companyHeaders)
	nameCol := MatchHeader(header, contactNameHeaders)

	if len(records) == 1 {
		return nil, ErrCustomerImportEmpty
	}

	seenPhone := make(map[string]bool, len(records))
	seenEmail := make(map[string]bool, len(records))
	rows := make([]CustomerImportRow, 0, len(records)-1)

	for i, record := range records[1:] {
		row := CustomerImportRow{Line: i + 2}
		row.Company = strings.TrimSpace(cell(record, companyCol))
		row.Name = strings.TrimSpace(cell(record, nameCol))

		// A trailing blank line is the norm in a spreadsheet, not an error.
		if blank(record) {
			continue
		}

		rawPhone := strings.TrimSpace(cell(record, phoneCol))
		rawEmail := strings.TrimSpace(cell(record, emailCol))

		if rawPhone != "" {
			// A human typed this — accept their country's format. The channel
			// rule (bare digits are already country-coded) is wrong here: "51
			// 98888-7766" from a Brazilian is not a foreign number.
			if phone, ok := NormalizeTypedPhone(rawPhone); ok {
				row.Phone = phone
			} else {
				row.Problem = ProblemBadPhone
			}
		}
		if row.Problem == "" && rawEmail != "" {
			email := strings.ToLower(rawEmail)
			if emailPattern.MatchString(email) {
				row.Email = email
			} else {
				row.Problem = ProblemBadEmail
			}
		}

		switch {
		case row.Problem != "":
			// Already reported; don't stack a second complaint on one row.
		case row.DisplayName() == "":
			row.Problem = ProblemNoCustomerName
		case row.Phone == "" && row.Email == "":
			row.Problem = ProblemNoAddress
		case row.Phone != "" && seenPhone[row.Phone]:
			// Two lines with one number would create one contact and let the
			// person believe they imported two customers.
			row.Problem = ProblemDuplicatePhone
		case row.Phone == "" && row.Email != "" && seenEmail[row.Email]:
			row.Problem = ProblemDuplicateEmail
		default:
			if row.Phone != "" {
				seenPhone[row.Phone] = true
			}
			if row.Email != "" {
				seenEmail[row.Email] = true
			}
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, ErrCustomerImportEmpty
	}
	return rows, nil
}

// ChannelKey is the address this contact is reached at, in the form the
// channel itself uses.
//
// WhatsApp is the product's main channel and Meta writes a sender as bare
// digits, which is what lands in contacts.external_id and what
// conversations are looked up by. An imported customer keyed as "+5511…"
// would therefore NOT be the same row as the one their first WhatsApp
// message creates — the same person, twice, with the history split between
// them. So a phone-bearing import is keyed exactly the way the channel keys
// it, and the '+' form lives on Phone and on the identity index.
func (r CustomerImportRow) ChannelKey() (key, channelType string) {
	key, ok := ContactKey(r.Phone, r.Email)
	if !ok {
		return "", ""
	}
	if r.Phone != "" {
		return key, "whatsapp"
	}
	return key, "email"
}
