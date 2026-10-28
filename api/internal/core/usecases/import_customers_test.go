package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// In-memory doubles rather than testify mocks: this use case's behaviour IS the
// bookkeeping (what already exists, what gets an identity), and asserting that
// through call expectations reads worse than asserting the resulting state.

type fakeContactStore struct {
	byExternal map[string]*domain.Contact
	byID       map[string]*domain.Contact
	upserts    []*domain.Contact
	updates    []*domain.Contact
	optedOut   []string // external ids SetOptedOut was asked to flag
	failOn     string   // external id whose Upsert returns an error
}

func newFakeContactStore() *fakeContactStore {
	return &fakeContactStore{
		byExternal: map[string]*domain.Contact{},
		byID:       map[string]*domain.Contact{},
	}
}

func (f *fakeContactStore) Upsert(_ context.Context, c *domain.Contact) (*domain.Contact, error) {
	if f.failOn != "" && c.ExternalID == f.failOn {
		return nil, assertErr("upsert exploded")
	}
	f.byExternal[c.ExternalID] = c
	if c.ID != "" {
		f.byID[c.ID] = c
	}
	f.upserts = append(f.upserts, c)
	return c, nil
}

func (f *fakeContactStore) GetByExternalID(_ context.Context, _, externalID string) (*domain.Contact, error) {
	return f.byExternal[externalID], nil
}

// Backed for real, not stubbed to nil: the inbound resolver looks a contact up
// by the id the identity index gave it, and a nil here would let a broken
// resolution look like a working one.
func (f *fakeContactStore) GetByID(_ context.Context, id, _ string) (*domain.Contact, error) {
	return f.byID[id], nil
}

func (f *fakeContactStore) ListByIDs(context.Context, string, []string) ([]*domain.Contact, error) {
	return nil, nil
}

func (f *fakeContactStore) ListByAccountID(context.Context, string, string, int, int) ([]*domain.Contact, error) {
	return nil, nil
}
func (f *fakeContactStore) Update(_ context.Context, c *domain.Contact) error {
	f.updates = append(f.updates, c)
	return nil
}
func (f *fakeContactStore) SetOptedOut(_ context.Context, _, externalID string) (bool, error) {
	f.optedOut = append(f.optedOut, externalID)
	return false, nil
}

func (f *fakeContactStore) ClearOptedOut(context.Context, string, string) (bool, error) {
	return false, nil
}

func (f *fakeContactStore) ListOptedOutExternalIDs(context.Context, string, []string) (map[string]bool, error) {
	return nil, nil
}
func (f *fakeContactStore) IsOptedOut(context.Context, string, string) (bool, error) {
	return false, nil
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

type fakeIdentityStore struct {
	byValue map[string]string // "kind|value" -> contactID
	written []*domain.ContactIdentity
}

func newFakeIdentityStore() *fakeIdentityStore {
	return &fakeIdentityStore{byValue: map[string]string{}}
}

func identityKey(kind domain.IdentityKind, value string) string {
	return string(kind) + "|" + domain.NormalizeIdentityValue(kind, value)
}

func (f *fakeIdentityStore) Upsert(_ context.Context, i *domain.ContactIdentity) (*domain.ContactIdentity, error) {
	// The real repository validates, and validation normalizes. A double that
	// skips it would let the test pass on values production rejects.
	if err := i.Validate(); err != nil {
		return nil, err
	}
	// Same contract as the real repository: an address that already belongs
	// to somebody is NOT re-pointed; the standing row (its owner) is returned.
	key := identityKey(i.Kind, i.Value)
	if owner, ok := f.byValue[key]; ok && owner != i.ContactID {
		standing := *i
		standing.ContactID = owner
		return &standing, nil
	}
	f.byValue[key] = i.ContactID
	f.written = append(f.written, i)
	return i, nil
}

func (f *fakeIdentityStore) ContactIDByValue(_ context.Context, _ string, kind domain.IdentityKind, value string) (string, error) {
	return f.byValue[identityKey(kind, value)], nil
}

func (f *fakeIdentityStore) ListByContact(context.Context, string, string) ([]*domain.ContactIdentity, error) {
	return nil, nil
}
func (f *fakeIdentityStore) MarkVerified(context.Context, string, string) (bool, error) {
	return false, nil
}
func (f *fakeIdentityStore) Delete(context.Context, string, string) error { return nil }

func newCustomerImport() (*ImportCustomersUseCase, *fakeContactStore, *fakeIdentityStore) {
	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	return NewImportCustomersUseCase(contacts, identities), contacts, identities
}

func rowsFrom(t *testing.T, uc *ImportCustomersUseCase, csv string) *CustomerImportPreview {
	t.Helper()
	preview, err := uc.Preview(context.Background(), "acc", strings.NewReader(csv))
	require.NoError(t, err)
	return preview
}

// The file a real customer list arrives as: exported from a pt-BR Excel, so
// semicolon-delimited, with phones typed the way a person types them.
func TestImportCustomers_ReadsAPortugueseSpreadsheet(t *testing.T) {
	uc, _, _ := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa;contato;telefone;e-mail\n"+
		"Clínica Aurora;Marina;(51) 98888-7766;marina@aurora.com.br\n"+
		"Hospital Vale;;51 3333-4444;\n")

	require.Len(t, preview.Rows, 2)
	assert.Equal(t, 2, preview.Importable)
	assert.Equal(t, 0, preview.Problems)

	assert.Equal(t, "+5551988887766", preview.Rows[0].Phone)
	assert.Equal(t, "marina@aurora.com.br", preview.Rows[0].Email)
	// The person's name when there is one...
	assert.Equal(t, "Marina", preview.Rows[0].DisplayName())
	// ...the organization when there is not. A contact called "" is unusable in
	// the screen where somebody approves a message to them.
	assert.Equal(t, "Hospital Vale", preview.Rows[1].DisplayName())
}

// The row a person has to look at, rather than one the parser guesses at.
func TestImportCustomers_RefusesAPhoneItCannotRead(t *testing.T) {
	uc, contacts, _ := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa,telefone\nClínica Sol,pergunta pra Ana\n")

	require.Len(t, preview.Rows, 1)
	assert.Equal(t, domain.ProblemBadPhone, preview.Rows[0].Problem)
	assert.Equal(t, 0, preview.Importable)

	// And the refusal survives the client sending the row back anyway: importing
	// the customer while quietly dropping the number leaves a card that claims
	// they have no phone.
	res, err := uc.Import(context.Background(), "acc",
		ImportCustomersInput{Rows: preview.Rows})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Created)
	assert.Equal(t, 1, res.Skipped)
	assert.Empty(t, contacts.upserts)
}

// A name with no way to reach them is a note, not a customer.
func TestImportCustomers_ANameWithNoAddressIsNotACustomer(t *testing.T) {
	uc, _, _ := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa,telefone,email\nClínica Lua,,\n")

	require.Len(t, preview.Rows, 1)
	assert.Equal(t, domain.ProblemNoAddress, preview.Rows[0].Problem)
}

// The header decides whether this is a customer list at all.
func TestImportCustomers_AFileWithNoWayToReachAnybodyIsRefusedAsTheWrongFile(t *testing.T) {
	uc, _, _ := newCustomerImport()

	_, err := uc.Preview(context.Background(), "acc",
		strings.NewReader("empresa,cidade\nClínica Aurora,Porto Alegre\n"))

	// Not "empty" — the file has rows. It is the wrong file, or the right file
	// with different column titles, and those need different answers.
	assert.Equal(t, domain.ErrCustomerImportNoHeader, err)
}

// The whole point of the import: the customer she brings in and the WhatsApp
// message they send next week must be ONE contact.
func TestImportCustomers_KeysTheContactTheWayWhatsAppKeysIt(t *testing.T) {
	uc, contacts, identities := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa,telefone\nClínica Aurora,51988887766\n")
	res, err := uc.Import(context.Background(), "acc", ImportCustomersInput{Rows: preview.Rows})
	require.NoError(t, err)
	require.Equal(t, 1, res.Created)

	saved := contacts.upserts[0]
	// Meta writes a sender as bare digits, and that is what lands in
	// external_id. Keying this row as "+5551…" would make the customer's first
	// message create a second contact for the same person.
	assert.Equal(t, "5551988887766", saved.ExternalID)
	require.NotNil(t, saved.Phone)
	assert.Equal(t, "+5551988887766", *saved.Phone, "the readable form lives on the field")

	// And the identity is what makes the resolution work in the other
	// direction, on any channel.
	require.Len(t, identities.written, 1)
	assert.Equal(t, "+5551988887766", identities.written[0].Value)
	assert.Nil(t, identities.written[0].VerifiedAt,
		"a number out of a spreadsheet is a claim; the stamp is what an inbound message earns")
}

// Re-importing an updated list must not double the base.
func TestImportCustomers_ACustomerAlreadyInTheBaseIsReportedNotDuplicated(t *testing.T) {
	uc, contacts, _ := newCustomerImport()
	csv := "empresa,telefone\nClínica Aurora,51988887766\n"

	first := rowsFrom(t, uc, csv)
	_, err := uc.Import(context.Background(), "acc", ImportCustomersInput{Rows: first.Rows})
	require.NoError(t, err)

	second := rowsFrom(t, uc, csv)
	assert.Equal(t, 1, second.AlreadyInBase)
	assert.Equal(t, domain.ProblemAlreadyInAccount, second.Rows[0].Problem)

	res, err := uc.Import(context.Background(), "acc", ImportCustomersInput{Rows: second.Rows})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Created)
	assert.Len(t, contacts.upserts, 1)
}

// The account's existing WhatsApp contacts have no identity row — they predate
// the index. Asking only the index would report a base full of new customers.
func TestImportCustomers_FindsAContactThatOnlyEverCameFromWhatsApp(t *testing.T) {
	uc, contacts, _ := newCustomerImport()
	contacts.byExternal["5551988887766"] = &domain.Contact{
		ID: "existing", AccountID: "acc", ExternalID: "5551988887766",
	}

	preview := rowsFrom(t, uc, "empresa,telefone\nClínica Aurora,(51) 98888-7766\n")

	assert.Equal(t, 1, preview.AlreadyInBase)
	assert.Equal(t, 0, preview.Importable)
}

// Two lines, one number: one contact, and the person must not believe they
// imported two customers.
func TestImportCustomers_TwoLinesWithOneNumberIsReported(t *testing.T) {
	uc, contacts, _ := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa,telefone\n"+
		"Clínica Aurora,51988887766\n"+
		"Aurora Matriz,(51) 98888-7766\n")

	require.Len(t, preview.Rows, 2)
	assert.Empty(t, preview.Rows[0].Problem)
	assert.Equal(t, domain.ProblemDuplicatePhone, preview.Rows[1].Problem)

	// And if the client sends both back anyway, the second still creates nothing.
	res, err := uc.Import(context.Background(), "acc", ImportCustomersInput{
		Rows: []domain.CustomerImportRow{
			{Company: "Clínica Aurora", Phone: "51988887766"},
			{Company: "Aurora Matriz", Phone: "(51) 98888-7766"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created)
	assert.Equal(t, 1, res.Skipped)
	assert.Len(t, contacts.upserts, 1)
}

// Nothing in the payload is trusted: a row that arrives with its problem
// cleared must still be re-checked against the same rules.
func TestImportCustomers_ReDerivesEveryFieldFromTheClientPayload(t *testing.T) {
	uc, contacts, _ := newCustomerImport()

	res, err := uc.Import(context.Background(), "acc", ImportCustomersInput{
		Rows: []domain.CustomerImportRow{
			// Problem cleared by hand, phone still unreadable.
			{Company: "Clínica Sol", Phone: "pergunta pra Ana"},
			// A malformed address the parser would have refused.
			{Company: "Clínica Lua", Email: "não-é-email"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Created)
	assert.Equal(t, 2, res.Skipped)
	assert.Empty(t, contacts.upserts)
}

// One failing row must not lose the rows already created.
func TestImportCustomers_OneBadRowDoesNotAbortTheImport(t *testing.T) {
	uc, contacts, _ := newCustomerImport()
	contacts.failOn = "5551988887766"

	res, err := uc.Import(context.Background(), "acc", ImportCustomersInput{
		Rows: []domain.CustomerImportRow{
			{Company: "Clínica Aurora", Phone: "51988887766"},
			{Company: "Hospital Vale", Phone: "51333344445"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created)
	assert.Equal(t, 1, res.Skipped)
}

// An email-only customer is keyed and indexed on the address.
func TestImportCustomers_AnEmailOnlyCustomerIsImported(t *testing.T) {
	uc, contacts, identities := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa,e-mail\nClínica Aurora,COMPRAS@Aurora.com.br\n")
	require.Equal(t, 1, preview.Importable)
	// Case-folded, or the same mailbox becomes two contacts.
	assert.Equal(t, "compras@aurora.com.br", preview.Rows[0].Email)

	_, err := uc.Import(context.Background(), "acc", ImportCustomersInput{Rows: preview.Rows})
	require.NoError(t, err)

	saved := contacts.upserts[0]
	assert.Equal(t, "compras@aurora.com.br", saved.ExternalID)
	require.NotNil(t, saved.ChannelType)
	assert.Equal(t, "email", *saved.ChannelType)
	require.Len(t, identities.written, 1)
	assert.Equal(t, domain.IdentityKindEmail, identities.written[0].Kind)
}

// A customer with both is reachable on both, from one contact.
func TestImportCustomers_BothAddressesAreIndexed(t *testing.T) {
	uc, _, identities := newCustomerImport()

	preview := rowsFrom(t, uc,
		"empresa,telefone,e-mail\nClínica Aurora,51988887766,compras@aurora.com.br\n")
	_, err := uc.Import(context.Background(), "acc", ImportCustomersInput{Rows: preview.Rows})
	require.NoError(t, err)

	require.Len(t, identities.written, 2)
	assert.Equal(t, identities.written[0].ContactID, identities.written[1].ContactID,
		"one person, two ways to reach them — not two contacts")
}

// A trailing blank line is what every spreadsheet ends with.
func TestImportCustomers_TrailingBlankLinesAreNotProblems(t *testing.T) {
	uc, _, _ := newCustomerImport()

	preview := rowsFrom(t, uc, "empresa,telefone\nClínica Aurora,51988887766\n,\n \n")

	require.Len(t, preview.Rows, 1)
	assert.Equal(t, 0, preview.Problems)
}

// Every created customer lands in the default funnel when a lead maker is
// wired; the contacts-only import stays for callers that never wire one.
func TestImportCustomers_CreatesLeadsInTheDefaultFunnel(t *testing.T) {
	contacts := newFakeContactStore()
	uc := NewImportCustomersUseCase(contacts, newFakeIdentityStore())
	maker := &fakeLeadMaker{out: &ContactsToFunnelResult{Created: 2, LeadIDs: []string{"l1", "l2"}}}
	funnels := new(MockSalesFunnelRepo)
	funnels.On("EnsureDefault", mock.Anything, "acc-1").
		Return(&domain.SalesFunnel{ID: "f-def", AccountID: "acc-1", IsDefault: true}, nil)
	uc.SetLeadMaker(maker, funnels)

	res, err := uc.Import(context.Background(), "acc-1", ImportCustomersInput{Rows: []domain.CustomerImportRow{
		{Line: 2, Name: "Hospital A", Phone: "+55 65 3618-8000"},
		{Line: 3, Name: "Hospital B", Email: "compras@b.com.br"},
	}})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Created)
	assert.Equal(t, 2, res.LeadsCreated)
	require.NotNil(t, maker.in)
	assert.Equal(t, "f-def", maker.in.FunnelID)
	assert.Len(t, maker.in.ContactIDs, 2)
}
