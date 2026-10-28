package usecases

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The dedupe used to ask per row: up to three lookups × 2000 rows, once in the
// preview and again on confirm. Each is fast, but they run inside a request
// bounded by a 60s write timeout while holding a pool connection — and the
// price-list import next to it reads its whole catalogue once.
//
// This is a performance refactor, so the tests that matter are the ones proving
// the ANSWERS did not change. The count assertion is the cheap part.

// countingIndex answers from the same in-memory stores the per-row path uses, so
// the two paths cannot disagree by construction — what it adds is a call count
// and a record of the batch sizes.
type countingIndex struct {
	identities *fakeIdentityStore
	contacts   *fakeContactStore

	identityCalls int
	externalCalls int
	batchSizes    []int

	failIdentities bool
	failExternals  bool
}

func (ix *countingIndex) ContactIDsByIdentityValues(
	ctx context.Context, accountID string, kind domain.IdentityKind, values []string,
) (map[string]string, error) {
	ix.identityCalls++
	ix.batchSizes = append(ix.batchSizes, len(values))
	if ix.failIdentities {
		return nil, assertErr("identity index exploded")
	}
	out := map[string]string{}
	for _, v := range values {
		if id, _ := ix.identities.ContactIDByValue(ctx, accountID, kind, v); id != "" {
			out[v] = id
		}
	}
	return out, nil
}

func (ix *countingIndex) ContactIDsByExternalIDs(
	ctx context.Context, accountID string, externalIDs []string,
) (map[string]string, error) {
	ix.externalCalls++
	ix.batchSizes = append(ix.batchSizes, len(externalIDs))
	if ix.failExternals {
		return nil, assertErr("contact index exploded")
	}
	out := map[string]string{}
	for _, e := range externalIDs {
		if c, _ := ix.contacts.GetByExternalID(ctx, accountID, e); c != nil {
			out[e] = c.ID
		}
	}
	return out, nil
}

func newIndexedCustomerImport() (*ImportCustomersUseCase, *fakeContactStore, *fakeIdentityStore, *countingIndex) {
	uc, contacts, identities := newCustomerImport()
	ix := &countingIndex{identities: identities, contacts: contacts}
	uc.SetImportIndex(ix)
	return uc, contacts, identities, ix
}

// bigCustomerCSV is a real-shaped base: 2000 rows, some of them repeats of each
// other inside the same file, which the import has to collapse on its own.
func bigCustomerCSV(rows int) string {
	var b strings.Builder
	b.WriteString("empresa;nome;telefone;email\n")
	for i := 0; i < rows; i++ {
		// Every 50th row repeats the row before it — the same customer listed
		// twice, which a spreadsheet maintained by hand always has.
		n := i
		if i%50 == 49 {
			n = i - 1
		}
		fmt.Fprintf(&b, "Hospital %d;Compras;(65) 9%04d-%04d;compras%d@h%d.com.br\n",
			n, 1000+n%9000, n%10000, n, n)
	}
	return b.String()
}

func TestImportCustomers_TheDedupeReadsOncePerFileNotOncePerRow(t *testing.T) {
	uc, _, _, ix := newIndexedCustomerImport()
	csv := bigCustomerCSV(2000)

	preview := rowsFrom(t, uc, csv)
	require.Len(t, preview.Rows, 2000)

	// Three reads for two thousand rows: phone identities, email identities, and
	// the external-id fallback.
	assert.Equal(t, 2, ix.identityCalls)
	assert.Equal(t, 1, ix.externalCalls)
	// And they were asked in bulk, not one value at a time.
	for _, size := range ix.batchSizes {
		assert.Greater(t, size, 1000, "the whole file's identifiers go in one query")
	}

	ix.identityCalls, ix.externalCalls = 0, 0
	_, err := uc.Import(context.Background(), "acc",
		ImportCustomersInput{Rows: preview.Rows})
	require.NoError(t, err)
	assert.Equal(t, 2, ix.identityCalls)
	assert.Equal(t, 1, ix.externalCalls)
}

// The assertion this refactor lives or dies on.
func TestImportCustomers_TheIndexedPathAnswersExactlyLikeThePerRowPath(t *testing.T) {
	// A file with every case in it: new customers, a customer already in the
	// base by identity, one known only by external id (the WhatsApp case), an
	// unreadable phone, a row with no way to reach anybody, and an in-file repeat.
	csv := strings.Join([]string{
		"empresa;nome;telefone;email",
		"Hospital Santa Rosa;Compras;(65) 99999-0001;compras@santarosa.com.br",
		"Clínica Aurora;Ana;(11) 98888-0002;ana@aurora.com.br",
		"Já Conhecido;Zé;(65) 99999-0003;",
		"Só WhatsApp;Bia;(65) 99999-0004;",
		"Telefone Ruim;Léo;123;",
		"Sem Nada;Ninguém;;",
		"Hospital Santa Rosa;Compras;(65) 99999-0001;compras@santarosa.com.br",
	}, "\n")

	seed := func(contacts *fakeContactStore, identities *fakeIdentityStore) {
		// Known by identity.
		known := &domain.Contact{ID: "known-1", AccountID: "acc", ExternalID: "+5565999990003"}
		_, err := contacts.Upsert(context.Background(), known)
		require.NoError(t, err)
		_, err = identities.Upsert(context.Background(), &domain.ContactIdentity{
			AccountID: "acc", ContactID: "known-1",
			Kind: domain.IdentityKindPhone, Value: "+5565999990003",
		})
		require.NoError(t, err)
		// Known only the way WhatsApp keys it: bare digits as external_id, and
		// no identity row at all.
		_, err = contacts.Upsert(context.Background(), &domain.Contact{
			ID: "wa-1", AccountID: "acc", ExternalID: "5565999990004",
		})
		require.NoError(t, err)
	}

	perRow, perRowContacts, perRowIdentities := newCustomerImport()
	seed(perRowContacts, perRowIdentities)
	indexed, indexedContacts, indexedIdentities, _ := newIndexedCustomerImport()
	seed(indexedContacts, indexedIdentities)

	a := rowsFrom(t, perRow, csv)
	b := rowsFrom(t, indexed, csv)

	assert.Equal(t, a.Importable, b.Importable)
	assert.Equal(t, a.Problems, b.Problems)
	assert.Equal(t, a.AlreadyInBase, b.AlreadyInBase)
	require.Len(t, b.Rows, len(a.Rows))
	for i := range a.Rows {
		assert.Equal(t, a.Rows[i].Problem, b.Rows[i].Problem,
			"row %d (line %d) must be judged the same either way", i, a.Rows[i].Line)
	}
	// Both must have RECOGNISED both known customers, or the equality above is
	// two identical wrong answers.
	assert.Equal(t, 2, b.AlreadyInBase)

	resA, err := perRow.Import(context.Background(), "acc", ImportCustomersInput{Rows: a.Rows})
	require.NoError(t, err)
	resB, err := indexed.Import(context.Background(), "acc", ImportCustomersInput{Rows: b.Rows})
	require.NoError(t, err)

	assert.Equal(t, resA.Created, resB.Created)
	assert.Equal(t, resA.Skipped, resB.Skipped)
	assert.Equal(t, 2, resB.Created, "the two genuinely new customers, and only those")

	// And the same rows landed, keyed identically.
	var keysA, keysB []string
	for _, c := range perRowContacts.upserts {
		keysA = append(keysA, c.ExternalID)
	}
	for _, c := range indexedContacts.upserts {
		keysB = append(keysB, c.ExternalID)
	}
	assert.Equal(t, keysA, keysB)
}

// The posture the per-row lookup always had, kept: a read failure and a miss are
// treated alike, which errs toward creating a duplicate rather than skipping a
// customer the account does not have. A duplicate is visible and fixable; a
// customer silently not imported is discovered when a proposal never goes out.
func TestImportCustomers_AnIndexReadFailureStillImports(t *testing.T) {
	uc, contacts, identities, ix := newIndexedCustomerImport()
	_, err := contacts.Upsert(context.Background(), &domain.Contact{
		ID: "known-1", AccountID: "acc", ExternalID: "+5565999990003",
	})
	require.NoError(t, err)
	_, err = identities.Upsert(context.Background(), &domain.ContactIdentity{
		AccountID: "acc", ContactID: "known-1",
		Kind: domain.IdentityKindPhone, Value: "+5565999990003",
	})
	require.NoError(t, err)

	ix.failIdentities = true
	ix.failExternals = true

	preview := rowsFrom(t, uc, "empresa;nome;telefone;email\nJá Conhecido;Zé;(65) 99999-0003;")
	require.Len(t, preview.Rows, 1)
	assert.Equal(t, 1, preview.Importable,
		"a failed dedupe read must not turn into 'skip this customer'")
	assert.Zero(t, preview.AlreadyInBase)
}
