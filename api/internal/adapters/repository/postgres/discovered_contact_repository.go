package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// DiscoveredContactRepository persists the customer-list candidates learned from
// a Sent folder.
//
// Raw SQL rather than sqlc: the accumulate-on-conflict upsert below is the whole
// point of the table and reads better written out than assembled from a
// generated querier.
type DiscoveredContactRepository struct {
	db DBTX
}

func NewDiscoveredContactRepository(db DBTX) *DiscoveredContactRepository {
	return &DiscoveredContactRepository{db: db}
}

var _ domain.DiscoveredContactRepository = (*DiscoveredContactRepository)(nil)

const discoveredColumns = `id, account_id, COALESCE(channel_id, ''), address,
	COALESCE(name, ''), sent_count, first_sent_at, last_sent_at,
	COALESCE(last_subject, ''), imported_at, dismissed_at, created_at, updated_at`

func scanDiscovered(row pgx.Row) (*domain.DiscoveredContact, error) {
	var d domain.DiscoveredContact
	err := row.Scan(&d.ID, &d.AccountID, &d.ChannelID, &d.Address, &d.Name,
		&d.SentCount, &d.FirstSentAt, &d.LastSentAt, &d.LastSubject,
		&d.ImportedAt, &d.DismissedAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Upsert accumulates one sighting.
//
// Every clause here defends something:
//
//   - sent_count adds, so a rescan strengthens the ranking instead of resetting
//     it to 1 and making a two-year customer look like a stranger.
//   - the date window uses LEAST/GREATEST, because pages arrive newest-first and
//     a plain assignment would leave first_sent_at holding the most recent date.
//   - COALESCE on the name fills a blank one without overwriting a good one: the
//     same person appears with a display name in some messages and bare in
//     others, and the named version is the one a human recognises.
//   - imported_at and dismissed_at are untouched. A rescan must never re-offer
//     somebody she already refused, nor un-import somebody already in the CRM.
func (r *DiscoveredContactRepository) Upsert(ctx context.Context, accountID, channelID string, rec domain.SentRecipient) error {
	address := strings.ToLower(strings.TrimSpace(rec.Address))
	if address == "" {
		return nil
	}
	var channel *string
	if c := strings.TrimSpace(channelID); c != "" {
		channel = &c
	}
	var sentAt *time.Time
	if !rec.SentAt.IsZero() {
		t := rec.SentAt.UTC()
		sentAt = &t
	}
	var name *string
	if n := strings.TrimSpace(rec.Name); n != "" {
		name = &n
	}
	var subject *string
	if s := strings.TrimSpace(rec.Subject); s != "" {
		subject = &s
	}

	// last_subject follows last_sent_at: it only moves when THIS sighting is
	// the newest one, so a backwards history walk (newest page first) cannot
	// overwrite the current thread's subject with an older one.
	_, err := r.db.Exec(ctx,
		`INSERT INTO discovered_contacts
		     (id, account_id, channel_id, address, name, sent_count, first_sent_at, last_sent_at, last_subject)
		 VALUES ($1, $2, $3, $4, $5, 1, $6, $6, $7)
		 ON CONFLICT (account_id, address) DO UPDATE SET
		     sent_count    = discovered_contacts.sent_count + 1,
		     first_sent_at = LEAST(
		         COALESCE(discovered_contacts.first_sent_at, EXCLUDED.first_sent_at),
		         COALESCE(EXCLUDED.first_sent_at, discovered_contacts.first_sent_at)),
		     last_sent_at  = GREATEST(
		         COALESCE(discovered_contacts.last_sent_at, EXCLUDED.last_sent_at),
		         COALESCE(EXCLUDED.last_sent_at, discovered_contacts.last_sent_at)),
		     last_subject  = CASE
		         WHEN EXCLUDED.last_subject IS NOT NULL
		              AND (discovered_contacts.last_sent_at IS NULL
		                   OR (EXCLUDED.last_sent_at IS NOT NULL
		                       AND EXCLUDED.last_sent_at >= discovered_contacts.last_sent_at))
		         THEN EXCLUDED.last_subject
		         ELSE discovered_contacts.last_subject
		     END,
		     name          = COALESCE(NULLIF(discovered_contacts.name, ''), EXCLUDED.name),
		     channel_id    = COALESCE(discovered_contacts.channel_id, EXCLUDED.channel_id),
		     updated_at    = NOW()`,
		ulid.Make().String(), accountID, channel, address, name, sentAt, subject)
	return err
}

// ListPending returns undecided candidates, most-emailed first.
func (r *DiscoveredContactRepository) ListPending(ctx context.Context, accountID string, limit, offset int32) ([]*domain.DiscoveredContact, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+discoveredColumns+`
		   FROM discovered_contacts
		  WHERE account_id = $1 AND imported_at IS NULL AND dismissed_at IS NULL
		  ORDER BY sent_count DESC, last_sent_at DESC NULLS LAST, address ASC
		  LIMIT $2 OFFSET $3`,
		accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.DiscoveredContact
	for rows.Next() {
		d, err := scanDiscovered(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetByAddresses resolves a selection, account-scoped so a request cannot reach
// another tenant's candidate by naming its address.
func (r *DiscoveredContactRepository) GetByAddresses(ctx context.Context, accountID string, addresses []string) ([]*domain.DiscoveredContact, error) {
	if len(addresses) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+discoveredColumns+`
		   FROM discovered_contacts
		  WHERE account_id = $1 AND address = ANY($2)`,
		accountID, lowerAll(addresses))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.DiscoveredContact
	for rows.Next() {
		d, err := scanDiscovered(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DiscoveredContactRepository) MarkImported(ctx context.Context, accountID string, addresses []string, at time.Time) error {
	return r.stampDecision(ctx, accountID, addresses, "imported_at", at)
}

func (r *DiscoveredContactRepository) MarkDismissed(ctx context.Context, accountID string, addresses []string, at time.Time) error {
	return r.stampDecision(ctx, accountID, addresses, "dismissed_at", at)
}

// stampDecision writes one decision column.
//
// Only ever set once (`IS NULL` guard): re-running an import must not move the
// timestamp, because that date is the answer to "when did this come in?" and a
// second click would rewrite history.
func (r *DiscoveredContactRepository) stampDecision(ctx context.Context, accountID string, addresses []string, column string, at time.Time) error {
	if len(addresses) == 0 {
		return nil
	}
	// column is one of two literals chosen above, never caller input.
	_, err := r.db.Exec(ctx,
		`UPDATE discovered_contacts
		    SET `+column+` = $3, updated_at = NOW()
		  WHERE account_id = $1 AND address = ANY($2) AND `+column+` IS NULL`,
		accountID, lowerAll(addresses), at)
	return err
}

func (r *DiscoveredContactRepository) CountPending(ctx context.Context, accountID string) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM discovered_contacts
		  WHERE account_id = $1 AND imported_at IS NULL AND dismissed_at IS NULL`,
		accountID).Scan(&total)
	return total, err
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if v := strings.ToLower(strings.TrimSpace(s)); v != "" {
			out = append(out, v)
		}
	}
	return out
}
