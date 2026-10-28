package usecases

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// Importing the customers a distributor already has.
//
// Preview reads the file and reports what it found; Import creates a contact per
// row a PERSON confirmed. The split is not ceremony — this file decides who the
// account will be writing to, and the rows it most needs looked at are the ones
// it refuses to guess at (a phone it cannot read, a customer already in the base
// under a different spelling).

var ErrCustomerImportTooLarge = &domain.AppError{
	Code:    "CUSTOMER_IMPORT_TOO_LARGE",
	Message: "o arquivo é grande demais. Importe até 2000 clientes por vez",
}

var ErrCustomerImportUnreadable = &domain.AppError{
	Code:    "CUSTOMER_IMPORT_UNREADABLE",
	Message: "não consegui ler o arquivo como CSV. Salve a planilha como CSV e tente de novo",
}

// CustomerImportIndex answers the dedupe question for a WHOLE file at once.
//
// A narrow port, and optional: without it the import falls back to asking per
// row, which is correct and slow. With it a 2000-row file costs three queries
// instead of up to six thousand, inside a request bounded by a 60s write
// timeout that is holding a pool connection the whole time.
type CustomerImportIndex interface {
	// ContactIDsByIdentityValues maps every value the account already has to
	// its contact. Values nobody owns are absent from the map.
	ContactIDsByIdentityValues(
		ctx context.Context, accountID string, kind domain.IdentityKind, values []string,
	) (map[string]string, error)
	// ContactIDsByExternalIDs is the fallback for contacts that predate the
	// identity index — every contact the account got from WhatsApp.
	ContactIDsByExternalIDs(
		ctx context.Context, accountID string, externalIDs []string,
	) (map[string]string, error)
}

type ImportCustomersUseCase struct {
	contacts   domain.ContactRepository
	identities domain.ContactIdentityRepository
	index      CustomerImportIndex
	leadMaker  firstOfferLeadMaker
	funnels    domain.SalesFunnelRepository
}

func NewImportCustomersUseCase(
	contacts domain.ContactRepository,
	identities domain.ContactIdentityRepository,
) *ImportCustomersUseCase {
	return &ImportCustomersUseCase{contacts: contacts, identities: identities}
}

// SetImportIndex wires the bulk dedupe lookup. Optional setter rather than a
// constructor parameter so the existing call sites and tests stay as they are.
func (uc *ImportCustomersUseCase) SetImportIndex(ix CustomerImportIndex) {
	uc.index = ix
}

// SetLeadMaker makes every imported customer a lead in the default funnel.
//
// Before 2026-09-01 an import produced contacts only; the one door from a
// contact to the pipeline was a button on the contacts screen (merged into
// the Carteira since). A customer she brought IS her pipeline — the Kanban
// and the queue must see them without a second gesture. Optional: unwired
// keeps the contacts-only import (tests, older callers).
func (uc *ImportCustomersUseCase) SetLeadMaker(maker firstOfferLeadMaker, funnels domain.SalesFunnelRepository) {
	uc.leadMaker = maker
	uc.funnels = funnels
}

// CustomerImportPreview is what the review screen renders.
type CustomerImportPreview struct {
	Rows []domain.CustomerImportRow `json:"rows"`
	// Counted server-side so the screen states the same numbers the import
	// will act on.
	Importable int `json:"importable"`
	Problems   int `json:"problems"`
	// AlreadyInBase names rows whose customer the account already has.
	// Reported rather than skipped in silence: somebody re-importing an
	// updated list needs to know which rows the import will not touch.
	AlreadyInBase int `json:"already_in_base"`
}

// Preview parses the file and reports what it found, creating nothing.
func (uc *ImportCustomersUseCase) Preview(
	ctx context.Context, accountID string, file io.Reader,
) (*CustomerImportPreview, error) {
	records, err := readCustomerCSV(file)
	if err != nil {
		return nil, err
	}

	rows, err := domain.BuildCustomerImportRows(records)
	if err != nil {
		return nil, err
	}
	if len(rows) > maxImportRows {
		return nil, ErrCustomerImportTooLarge
	}

	dedupe := uc.buildDedupe(ctx, accountID, rows)

	out := &CustomerImportPreview{Rows: rows}
	for i := range rows {
		switch {
		case !rows[i].Importable():
			out.Problems++
		case dedupe.existing(ctx, accountID, rows[i]) != "":
			out.AlreadyInBase++
			// A fact, not a fault — but it makes the row unimportable so a
			// second run of the same file cannot double the base.
			out.Rows[i].Problem = domain.ProblemAlreadyInAccount
			out.Rows[i].Duplicate = true
			out.Problems++
		default:
			out.Importable++
		}
	}
	return out, nil
}

// ImportCustomersInput is the confirmed set. The client sends back the ROWS the
// preview showed, not the file: the person approved what they saw, and
// re-parsing would let a file that changed underneath be imported instead.
type ImportCustomersInput struct {
	Rows []domain.CustomerImportRow `json:"rows"`
}

type ImportCustomersResult struct {
	Created int `json:"created"`
	// Skipped counts rows that arrived unimportable or already existed.
	Skipped int `json:"skipped"`
	// LeadsCreated is how many of the created customers landed in the
	// default funnel (see SetLeadMaker). Zero when the maker is unwired.
	LeadsCreated int `json:"leads_created"`
}

// Import creates a contact per confirmed row.
func (uc *ImportCustomersUseCase) Import(
	ctx context.Context, accountID string, in ImportCustomersInput,
) (*ImportCustomersResult, error) {
	if len(in.Rows) == 0 {
		return nil, domain.ErrCustomerImportEmpty
	}
	if len(in.Rows) > maxImportRows {
		return nil, ErrCustomerImportTooLarge
	}

	// Everything is re-derived and nothing is trusted from the client. A row
	// arriving with its problem cleared, or with a phone the parser refused,
	// would otherwise create the contact the preview declined to.
	//
	// Done for the whole file BEFORE the loop so the dedupe index is built from
	// the cleaned values — asking it about the client's raw spelling would miss
	// the rows it exists to catch.
	cleaned := make([]domain.CustomerImportRow, 0, len(in.Rows))
	for _, row := range in.Rows {
		cleaned = append(cleaned, cleanCustomerImportRow(accountID, row))
	}
	dedupe := uc.buildDedupe(ctx, accountID, cleaned)
	resolver := newContactResolver(uc.contacts, uc.identities)

	result := &ImportCustomersResult{}
	var createdIDs []string
	// Within one file too: the client may send rows the preview flagged as
	// duplicates of each other, and the second one must not overwrite the first.
	seen := make(map[string]bool, len(cleaned))

	for _, clean := range cleaned {
		key, channelType := clean.ChannelKey()
		if !clean.Importable() || key == "" || seen[key] {
			result.Skipped++
			continue
		}
		if dedupe.existing(ctx, accountID, clean) != "" {
			result.Skipped++
			continue
		}

		contact := &domain.Contact{
			ID:          ulid.Make().String(),
			AccountID:   accountID,
			ExternalID:  key,
			ChannelType: &channelType,
		}
		if name := clean.DisplayName(); name != "" {
			contact.Name = &name
		}
		// The organization travels even when there is a person: "Clínica
		// Aurora;Marina" used to become Marina at nowhere.
		if company := strings.TrimSpace(clean.Company); company != "" {
			contact.CompanyName = &company
		}
		if clean.Phone != "" {
			phone := clean.Phone
			contact.Phone = &phone
		}
		if clean.Email != "" {
			email := clean.Email
			contact.Email = &email
		}

		// The dedupe above already answered "nobody owns these addresses" for
		// the whole file, so this is the create path: Upsert (which still
		// dedupes on the key, for the rare row two doors race on) and index
		// both addresses through the same resolver every other door uses.
		saved, err := uc.contacts.Upsert(ctx, contact)
		if err != nil {
			// One bad row must not lose the rows already created.
			slog.WarnContext(ctx, "customer import: contact upsert failed",
				"line", clean.Line, "error", err)
			result.Skipped++
			continue
		}

		// The identities are what make the NEXT message from this customer —
		// on WhatsApp, or by email — land on this contact instead of creating
		// a stranger. Without them the import is a list, not a base.
		resolver.registerIdentities(ctx, saved, clean.Phone, clean.Email, contactResolveOpts{})

		seen[key] = true
		result.Created++
		createdIDs = append(createdIDs, saved.ID)
	}

	// The customers she brought go straight into her pipeline. Best effort:
	// the contacts are already saved, and a funnel hiccup must not turn a
	// successful import into an error.
	if uc.leadMaker != nil && uc.funnels != nil && len(createdIDs) > 0 {
		funnel, err := uc.funnels.EnsureDefault(ctx, accountID)
		if err != nil || funnel == nil {
			slog.WarnContext(ctx, "customer import: default funnel unavailable — contacts saved, no leads",
				"account_id", accountID, "error", err)
			return result, nil
		}
		res, err := uc.leadMaker.CreateFromContacts(ctx, ContactsToFunnelInput{
			AccountID: accountID, FunnelID: funnel.ID, ContactIDs: createdIDs,
		})
		if err != nil {
			slog.WarnContext(ctx, "customer import: contacts saved but leads not created",
				"account_id", accountID, "error", err)
			return result, nil
		}
		result.LeadsCreated = res.Created
	}

	return result, nil
}

// cleanCustomerImportRow re-derives one confirmed row from scratch.
func cleanCustomerImportRow(accountID string, row domain.CustomerImportRow) domain.CustomerImportRow {
	clean := domain.CustomerImportRow{
		Line:    row.Line,
		Company: strings.TrimSpace(row.Company),
		Name:    strings.TrimSpace(row.Name),
	}
	if p, ok := domain.NormalizeTypedPhone(row.Phone); ok {
		clean.Phone = p
	}
	if e := strings.ToLower(strings.TrimSpace(row.Email)); e != "" {
		// Validate returns the same refusal the parser would have.
		probe := &domain.ContactIdentity{
			AccountID: accountID, ContactID: "probe",
			Kind: domain.IdentityKindEmail, Value: e,
		}
		if probe.Validate() == nil {
			clean.Email = probe.Value
		}
	}
	return clean
}

// customerDedupe answers "does the account already have this customer?" for
// every row of one file, from maps read up front.
//
// perRow is the fallback for when no bulk index is wired: same answer, one
// lookup at a time.
type customerDedupe struct {
	phones    map[string]string
	emails    map[string]string
	externals map[string]string
	perRow    func(ctx context.Context, accountID string, row domain.CustomerImportRow) string
}

// buildDedupe reads the whole file's identifiers in three queries.
//
// A read failure logs once and yields an EMPTY map, which is the same posture
// the per-row lookup always had: a lookup failure and a miss are treated alike,
// erring toward creating a duplicate rather than skipping a customer the account
// does not have. A duplicate is visible and fixable; a customer silently not
// imported is discovered when a proposal never goes out.
func (uc *ImportCustomersUseCase) buildDedupe(
	ctx context.Context, accountID string, rows []domain.CustomerImportRow,
) *customerDedupe {
	if uc.index == nil {
		return &customerDedupe{perRow: uc.existingContactID}
	}

	var phones, emails, externals []string
	for _, row := range rows {
		if row.Phone != "" {
			phones = append(phones, row.Phone)
		}
		if row.Email != "" {
			emails = append(emails, row.Email)
		}
		if key, _ := row.ChannelKey(); key != "" {
			externals = append(externals, key)
		}
		if row.Phone != "" {
			// The key older doors (manual lead, Places) wrote for this same
			// number: E.164 with the '+'.
			externals = append(externals, row.Phone)
		}
	}

	d := &customerDedupe{}
	var err error
	if d.phones, err = uc.index.ContactIDsByIdentityValues(
		ctx, accountID, domain.IdentityKindPhone, phones); err != nil {
		slog.WarnContext(ctx, "customer import: phone identity index read failed",
			"error", err)
		d.phones = map[string]string{}
	}
	if d.emails, err = uc.index.ContactIDsByIdentityValues(
		ctx, accountID, domain.IdentityKindEmail, emails); err != nil {
		slog.WarnContext(ctx, "customer import: email identity index read failed",
			"error", err)
		d.emails = map[string]string{}
	}
	if d.externals, err = uc.index.ContactIDsByExternalIDs(
		ctx, accountID, externals); err != nil {
		slog.WarnContext(ctx, "customer import: contact index read failed",
			"error", err)
		d.externals = map[string]string{}
	}
	return d
}

// existing answers for one row, in the same order the per-row lookup used:
// phone identity, then email identity, then the channel key.
func (d *customerDedupe) existing(
	ctx context.Context, accountID string, row domain.CustomerImportRow,
) string {
	if d.perRow != nil {
		return d.perRow(ctx, accountID, row)
	}
	if row.Phone != "" {
		if id := d.phones[row.Phone]; id != "" {
			return id
		}
	}
	if row.Email != "" {
		if id := d.emails[row.Email]; id != "" {
			return id
		}
	}
	if key, _ := row.ChannelKey(); key != "" {
		if id := d.externals[key]; id != "" {
			return id
		}
	}
	if row.Phone != "" {
		return d.externals[row.Phone]
	}
	return ""
}

// existingContactID is the per-row fallback, used only when no bulk index is
// wired.
//
// It asks twice on purpose. The identity index is the right question and the
// one that will keep being right. But every contact created before the index
// existed — which is every contact the account got from WhatsApp — has no
// identity row, so the index alone would report a base full of new customers
// and import all of them a second time. The channel key is the fallback that
// catches those.
//
// The same resolver every other door uses answers it, so "already in the
// base" here and "reuse this contact" on a manual lead cannot disagree.
func (uc *ImportCustomersUseCase) existingContactID(
	ctx context.Context, accountID string, row domain.CustomerImportRow,
) string {
	key, _ := row.ChannelKey()
	if c := newContactResolver(uc.contacts, uc.identities).lookup(
		ctx, accountID, row.Phone, row.Email, key); c != nil {
		return c.ID
	}
	return ""
}

// readCustomerCSV reads the upload with the same tolerances the price list
// needs — a semicolon delimiter from a pt-BR Excel, and a UTF-8 BOM — and
// reports failures in this import's own words.
func readCustomerCSV(file io.Reader) ([][]string, error) {
	records, err := readCSV(file)
	switch err {
	case nil:
		return records, nil
	case ErrProductImportTooLarge:
		return nil, ErrCustomerImportTooLarge
	case ErrProductImportUnreadable:
		return nil, ErrCustomerImportUnreadable
	case domain.ErrProductImportEmpty:
		return nil, domain.ErrCustomerImportEmpty
	default:
		return nil, err
	}
}
