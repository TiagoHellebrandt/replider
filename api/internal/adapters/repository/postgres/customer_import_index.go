package postgres

import (
	"context"
	"fmt"

	"github.com/replider/api/internal/core/domain"
)

// CustomerImportIndexRepository answers the customer import's dedupe question
// for a WHOLE file in three queries.
//
// The import used to ask per row: up to three lookups × 2000 rows, twice (once
// in the preview, once on confirm). Each one is fast — the indexes are there —
// but they run inside a request bounded by a 60s write timeout while holding a
// pool connection, and the price-list import next to it already read its whole
// catalogue once and looked at nothing else.
//
// Raw SQL rather than sqlc, and its own struct rather than methods on the
// contact repos: the reads span two tables, and putting them on
// domain.ContactRepository / domain.ContactIdentityRepository would make every
// mock of those interfaces grow a method it never uses.
type CustomerImportIndexRepository struct {
	db DBTX
}

func NewCustomerImportIndexRepository(db DBTX) *CustomerImportIndexRepository {
	return &CustomerImportIndexRepository{db: db}
}

// ContactIDsByIdentityValues maps every value the account already has to the
// contact that owns it. Values nobody owns are simply absent from the map.
func (r *CustomerImportIndexRepository) ContactIDsByIdentityValues(
	ctx context.Context, accountID string, kind domain.IdentityKind, values []string,
) (map[string]string, error) {
	out := make(map[string]string, len(values))
	if len(values) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT value, contact_id
		   FROM contact_identities
		  WHERE account_id = $1 AND kind = $2 AND value = ANY($3)`,
		accountID, string(kind), values)
	if err != nil {
		return nil, fmt.Errorf("customer import identity index: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value, contactID string
		if err := rows.Scan(&value, &contactID); err != nil {
			return nil, err
		}
		out[value] = contactID
	}
	return out, rows.Err()
}

// ContactIDsByExternalIDs is the fallback the identity index cannot cover: every
// contact created before that index existed — which is every contact the account
// got from WhatsApp — has no identity row, and without this the import would
// report a base full of new customers and create all of them a second time.
func (r *CustomerImportIndexRepository) ContactIDsByExternalIDs(
	ctx context.Context, accountID string, externalIDs []string,
) (map[string]string, error) {
	out := make(map[string]string, len(externalIDs))
	if len(externalIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT external_id, id
		   FROM contacts
		  WHERE account_id = $1 AND external_id = ANY($2)`,
		accountID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("customer import contact index: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var externalID, id string
		if err := rows.Scan(&externalID, &id); err != nil {
			return nil, err
		}
		out[externalID] = id
	}
	return out, rows.Err()
}
