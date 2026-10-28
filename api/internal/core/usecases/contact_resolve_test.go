package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// One customer, one key. The same hospital used to become two, three, four
// contacts because every door keyed contacts.external_id its own way and
// only one of them asked the identity index first.

func e164(s string) *string { return &s }

// A manual lead written before the canonical key stored "+556536188000". The
// same number arriving through any door now must land on THAT row.
func TestContactResolver_FindsAContactKeyedTheOldWay(t *testing.T) {
	ctx := context.Background()
	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	legacy, _ := contacts.Upsert(ctx, &domain.Contact{
		ID: "A", AccountID: "acc", ExternalID: "+556536188000",
	})

	draft := &domain.Contact{
		ID: "B", AccountID: "acc", ExternalID: "556536188000",
		Phone: e164("+556536188000"), Name: e164("Hospital Santa Rosa"),
	}
	got, created, err := newContactResolver(contacts, identities).resolve(ctx, draft, contactResolveOpts{})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, legacy.ID, got.ID, "the '+'-keyed contact was not found; a twin was minted")
	assert.Len(t, contacts.upserts, 1, "only the seed")

	// Found means completed, never overwritten: the blank name is filled…
	require.NotNil(t, got.Name)
	assert.Equal(t, "Hospital Santa Rosa", *got.Name)
	assert.Len(t, contacts.updates, 1)
	// …and the number is now indexed, so the NEXT door finds it in one hop.
	assert.Equal(t, "A", identities.byValue[identityKey(domain.IdentityKindPhone, "+556536188000")])
}

// A person the base knows by e-mail writes in with a phone for the first
// time: same contact, and the phone joins their identities.
func TestContactResolver_FindsByTheOtherAddress(t *testing.T) {
	ctx := context.Background()
	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	known, _ := contacts.Upsert(ctx, &domain.Contact{
		ID: "A", AccountID: "acc", ExternalID: "compras@hospital.br",
		Email: e164("compras@hospital.br"), Name: e164("Compras"),
	})
	_, err := identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: "acc", ContactID: "A", Kind: domain.IdentityKindEmail, Value: "compras@hospital.br",
	})
	require.NoError(t, err)

	draft := &domain.Contact{
		ID: "B", AccountID: "acc", ExternalID: "556536188000",
		Phone: e164("+556536188000"), Email: e164("Compras@Hospital.BR"),
		Name: e164("Somebody Else"),
	}
	got, created, err := newContactResolver(contacts, identities).resolve(ctx, draft, contactResolveOpts{})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, known.ID, got.ID)
	assert.Equal(t, "Compras", *got.Name, "a name the base already had must not be overwritten")
	assert.Equal(t, "+556536188000", *got.Phone, "the new address fills the blank")
	assert.Equal(t, "A", identities.byValue[identityKey(domain.IdentityKindPhone, "+556536188000")])
}

func TestContactResolver_CreatesAndIndexesBothAddresses(t *testing.T) {
	ctx := context.Background()
	contacts, identities := newFakeContactStore(), newFakeIdentityStore()

	draft := &domain.Contact{
		ID: "N", AccountID: "acc", ExternalID: "556536188000",
		Phone: e164("+556536188000"), Email: e164("compras@hospital.br"),
	}
	got, created, err := newContactResolver(contacts, identities).resolve(ctx, draft, contactResolveOpts{})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "N", got.ID)
	require.Len(t, identities.written, 2)
	assert.Equal(t, "N", identities.byValue[identityKey(domain.IdentityKindPhone, "+556536188000")])
	assert.Equal(t, "N", identities.byValue[identityKey(domain.IdentityKindEmail, "compras@hospital.br")])
}

// The older contact is the one with the sales and the conversations. A later
// door reaching the same address must never take it away.
func TestContactResolver_NeverStealsAnAddress(t *testing.T) {
	ctx := context.Background()
	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	_, _ = contacts.Upsert(ctx, &domain.Contact{ID: "A", AccountID: "acc", ExternalID: "556536188000"})
	_, err := identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: "acc", ContactID: "A", Kind: domain.IdentityKindPhone, Value: "+556536188000",
	})
	require.NoError(t, err)
	later, _ := contacts.Upsert(ctx, &domain.Contact{ID: "B", AccountID: "acc", ExternalID: "cnes:123"})

	newContactResolver(contacts, identities).registerIdentities(ctx, later, "+556536188000", "", contactResolveOpts{})

	assert.Equal(t, "A", identities.byValue[identityKey(domain.IdentityKindPhone, "+556536188000")],
		"the identity was re-pointed to the newer contact")
}

// A door whose old key was its own id ("cnes:<id>") still finds the contact
// it made before it started keying by phone.
func TestContactResolver_TriesTheDoorsLegacyKey(t *testing.T) {
	ctx := context.Background()
	contacts := newFakeContactStore()
	old, _ := contacts.Upsert(ctx, &domain.Contact{ID: "A", AccountID: "acc", ExternalID: "cnes:2655411"})

	draft := &domain.Contact{ID: "B", AccountID: "acc", ExternalID: "556536188000", Phone: e164("+556536188000")}
	got, created, err := newContactResolver(contacts, nil).resolve(ctx, draft,
		contactResolveOpts{LegacyKeys: []string{"cnes:2655411"}})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, old.ID, got.ID)
}

func TestContactResolver_LookupAnswersNilForAStranger(t *testing.T) {
	r := newContactResolver(newFakeContactStore(), newFakeIdentityStore())
	assert.Nil(t, r.lookup(context.Background(), "acc", "+556536188000", "x@y.com", "556536188000"))
}
