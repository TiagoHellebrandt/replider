package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-F004: the identity tables are the foundation the email channel stands
// on, so their constraints are pinned against a REAL Postgres — the unique
// indexes and the ON CONFLICT behaviour are the contract, and neither shows
// up in a mocked repo test.

// seedIdentityAccount inserts a bare account row (the identity tables FK to
// accounts) and returns its id.
func seedIdentityAccount(t *testing.T) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO accounts (id, name, subscription_status, created_at, updated_at)
		 VALUES ($1, $2, 'trialing', NOW(), NOW())`,
		id, "Identity Test "+id[:6])
	require.NoError(t, err)
	return id
}

// seedIdentityContact inserts a contact in the pre-US-F005 shape (id == the
// phone), which is exactly the state this migration has to coexist with.
func seedIdentityContact(t *testing.T, accountID, externalID string) string {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $1, NOW(), NOW(), NOW(), NOW())`,
		externalID, accountID)
	require.NoError(t, err)
	return externalID
}

func TestCompanyRepository_UpsertIsIdempotentPerAccountCNPJ(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyRepository(testPool)

	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)

	first, err := repo.Upsert(ctx, &domain.Company{
		AccountID: accountA,
		Name:      "Hospital São Lucas",
		CNPJ:      ptr("12.345.678/0001-99"), // masked on the way in
		Website:   ptr("https://saolucas.example.com"),
		Metadata:  json.RawMessage(`{"source":"speedio"}`),
	})
	require.NoError(t, err)
	require.NotNil(t, first.CNPJ)
	assert.Equal(t, "12345678000199", *first.CNPJ, "CNPJ must be stored digits-only")

	// Same CNPJ, same account → same row, enriched not duplicated.
	second, err := repo.Upsert(ctx, &domain.Company{
		AccountID: accountA,
		Name:      "Hospital Sao Lucas LTDA",
		CNPJ:      ptr("12345678000199"),
		Metadata:  json.RawMessage(`{"cnae":"4645-1/01"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "same account + CNPJ must reuse the row")
	assert.Equal(t, "Hospital Sao Lucas LTDA", second.Name)
	require.NotNil(t, second.Website)
	assert.Equal(t, "https://saolucas.example.com", *second.Website,
		"an update without a website must not blank the known one")
	assert.JSONEq(t, `{"source":"speedio","cnae":"4645-1/01"}`, string(second.Metadata),
		"metadata merges instead of replacing")

	// Another ACCOUNT prospecting the same company gets its own row: the
	// uniqueness is per tenant, never global.
	other, err := repo.Upsert(ctx, &domain.Company{
		AccountID: accountB,
		Name:      "Hospital São Lucas",
		CNPJ:      ptr("12345678000199"),
	})
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, other.ID)
}

func TestCompanyRepository_NoCNPJAlwaysInserts(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyRepository(testPool)
	accountID := seedIdentityAccount(t)

	a, err := repo.Upsert(ctx, &domain.Company{AccountID: accountID, Name: "Clínica Bem Estar"})
	require.NoError(t, err)
	b, err := repo.Upsert(ctx, &domain.Company{AccountID: accountID, Name: "Clínica Bem Estar"})
	require.NoError(t, err)

	// Two same-named CNPJ-less companies may genuinely be two clinics;
	// merging them would silently destroy one pipeline.
	assert.NotEqual(t, a.ID, b.ID)
}

func TestCompanyRepository_GetIsAccountScoped(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyRepository(testPool)
	owner := seedIdentityAccount(t)
	stranger := seedIdentityAccount(t)

	c, err := repo.Upsert(ctx, &domain.Company{
		AccountID: owner, Name: "Distribuidora Med", CNPJ: ptr("11222333000181"),
	})
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, owner, c.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	// A foreign account's id must be indistinguishable from a missing one.
	leaked, err := repo.GetByID(ctx, stranger, c.ID)
	require.NoError(t, err)
	assert.Nil(t, leaked)

	byCNPJ, err := repo.GetByCNPJ(ctx, owner, "11.222.333/0001-81")
	require.NoError(t, err)
	require.NotNil(t, byCNPJ, "a masked CNPJ must find the digits-only row")
	assert.Equal(t, c.ID, byCNPJ.ID)

	none, err := repo.GetByCNPJ(ctx, stranger, "11222333000181")
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestCompanyRepository_RejectsMalformedCNPJ(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyRepository(testPool)
	accountID := seedIdentityAccount(t)

	_, err := repo.Upsert(ctx, &domain.Company{
		AccountID: accountID, Name: "X", CNPJ: ptr("123"),
	})
	assert.ErrorIs(t, err, domain.ErrInvalidCNPJ)
}

func TestContactIdentityRepository_ResolvesContactByAnyIdentifier(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewContactIdentityRepository(testPool)
	accountID := seedIdentityAccount(t)
	contactID := seedIdentityContact(t, accountID, "+5511999990001")

	phone, err := repo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountID, ContactID: contactID,
		Kind: domain.IdentityKindPhone, Value: "+5511999990001",
	})
	require.NoError(t, err)

	_, err = repo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountID, ContactID: contactID,
		Kind: domain.IdentityKindEmail, Value: "  Compras@SaoLucas.COM  ",
	})
	require.NoError(t, err)

	// Same person, reachable from either identifier — the whole point of
	// the model.
	byPhone, err := repo.ContactIDByValue(ctx, accountID, domain.IdentityKindPhone, "+5511999990001")
	require.NoError(t, err)
	assert.Equal(t, contactID, byPhone)

	// A differently-cased/spaced lookup must still hit: normalization runs
	// on both write and read.
	byEmail, err := repo.ContactIDByValue(ctx, accountID, domain.IdentityKindEmail, "compras@saolucas.com")
	require.NoError(t, err)
	assert.Equal(t, contactID, byEmail)

	byMessyEmail, err := repo.ContactIDByValue(ctx, accountID, domain.IdentityKindEmail, " COMPRAS@saolucas.com ")
	require.NoError(t, err)
	assert.Equal(t, contactID, byMessyEmail)

	// Unknown identifier is the first-inbound case: empty, not an error.
	unknown, err := repo.ContactIDByValue(ctx, accountID, domain.IdentityKindEmail, "ninguem@nowhere.com")
	require.NoError(t, err)
	assert.Empty(t, unknown)

	identities, err := repo.ListByContact(ctx, accountID, contactID)
	require.NoError(t, err)
	assert.Len(t, identities, 2)

	// Verification is one-way and idempotent.
	flipped, err := repo.MarkVerified(ctx, accountID, phone.ID)
	require.NoError(t, err)
	assert.True(t, flipped)
	again, err := repo.MarkVerified(ctx, accountID, phone.ID)
	require.NoError(t, err)
	assert.False(t, again, "already-verified identity keeps its original stamp")

	reloaded, err := repo.ListByContact(ctx, accountID, contactID)
	require.NoError(t, err)
	var verifiedAt *time.Time
	for _, i := range reloaded {
		if i.ID == phone.ID {
			verifiedAt = i.VerifiedAt
		}
	}
	require.NotNil(t, verifiedAt)
}

func TestContactIdentityRepository_IsolatesAccountsAndNeverRepoints(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewContactIdentityRepository(testPool)

	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)
	contactA := seedIdentityContact(t, accountA, "+5511888880001")
	contactA2 := seedIdentityContact(t, accountA, "+5511888880002")
	contactB := seedIdentityContact(t, accountB, "+5511888880003")

	_, err := repo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountA, ContactID: contactA,
		Kind: domain.IdentityKindEmail, Value: "compras@cliente.com",
	})
	require.NoError(t, err)

	// The SAME email in another account is a different person — a global
	// unique would make one tenant's data block another tenant's write.
	_, err = repo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountB, ContactID: contactB,
		Kind: domain.IdentityKindEmail, Value: "compras@cliente.com",
	})
	require.NoError(t, err)

	inA, err := repo.ContactIDByValue(ctx, accountA, domain.IdentityKindEmail, "compras@cliente.com")
	require.NoError(t, err)
	assert.Equal(t, contactA, inA)
	inB, err := repo.ContactIDByValue(ctx, accountB, domain.IdentityKindEmail, "compras@cliente.com")
	require.NoError(t, err)
	assert.Equal(t, contactB, inB)

	// Re-registering an identity for a different contact of the same account
	// does NOT re-point it: the row that stands comes back with its owner,
	// and the caller reuses that contact. The older contact is the one with
	// the sales and the conversations; a later door must never take its
	// address away (it used to, and the history split between twins).
	standing, err := repo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountA, ContactID: contactA2,
		Kind: domain.IdentityKindEmail, Value: "compras@cliente.com",
	})
	require.NoError(t, err)
	assert.Equal(t, contactA, standing.ContactID, "the returned row must name the owner that stands")
	kept, err := repo.ContactIDByValue(ctx, accountA, domain.IdentityKindEmail, "compras@cliente.com")
	require.NoError(t, err)
	assert.Equal(t, contactA, kept, "the identity was stolen by the newer contact")
}

func TestContactIdentityRepository_RejectsInvalidValues(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewContactIdentityRepository(testPool)
	accountID := seedIdentityAccount(t)
	contactID := seedIdentityContact(t, accountID, "+5511777770001")

	cases := []struct {
		name string
		in   domain.ContactIdentity
		want error
	}{
		{"unknown kind", domain.ContactIdentity{AccountID: accountID, ContactID: contactID, Kind: "fax", Value: "x"}, domain.ErrInvalidIdentityKind},
		{"non-E164 phone", domain.ContactIdentity{AccountID: accountID, ContactID: contactID, Kind: domain.IdentityKindPhone, Value: "11999990000"}, domain.ErrInvalidPhone},
		{"malformed email", domain.ContactIdentity{AccountID: accountID, ContactID: contactID, Kind: domain.IdentityKindEmail, Value: "not-an-email"}, domain.ErrInvalidIdentityValue},
		{"empty value", domain.ContactIdentity{AccountID: accountID, ContactID: contactID, Kind: domain.IdentityKindEmail, Value: "   "}, domain.ErrInvalidIdentityValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			_, err := repo.Upsert(ctx, &in)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// Deleting the contact must take its identifiers with it (ON DELETE CASCADE)
// — an orphaned identity would resolve inbound mail to a contact that no
// longer exists.
func TestContactIdentityRepository_CascadesWithContact(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewContactIdentityRepository(testPool)
	accountID := seedIdentityAccount(t)
	contactID := seedIdentityContact(t, accountID, "+5511666660001")

	_, err := repo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountID, ContactID: contactID,
		Kind: domain.IdentityKindEmail, Value: "some@one.com",
	})
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `DELETE FROM contacts WHERE id = $1`, contactID)
	require.NoError(t, err)

	orphan, err := repo.ContactIDByValue(ctx, accountID, domain.IdentityKindEmail, "some@one.com")
	require.NoError(t, err)
	assert.Empty(t, orphan)
}

func ptr(s string) *string { return &s }
