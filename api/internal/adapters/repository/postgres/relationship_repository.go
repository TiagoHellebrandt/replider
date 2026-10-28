package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type RelationshipRepository struct{ db DBTX }

func NewRelationshipRepository(db DBTX) *RelationshipRepository {
	return &RelationshipRepository{db: db}
}

const relationshipCols = `id, account_id, kind, contact_id, month, day, due_at,
	label, notes, last_fired_year, created_at, updated_at`

func scanRelationshipDate(row pgx.Row) (*domain.RelationshipDate, error) {
	d := &domain.RelationshipDate{}
	var kind string
	var contactID *string
	err := row.Scan(&d.ID, &d.AccountID, &kind, &contactID, &d.Month, &d.Day,
		&d.DueAt, &d.Label, &d.Notes, &d.LastFiredYear, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	d.Kind = domain.RelationshipDateKind(kind)
	if contactID != nil {
		d.ContactID = *contactID
	}
	return d, nil
}

func (r *RelationshipRepository) List(ctx context.Context, accountID string) ([]*domain.RelationshipDate, error) {
	return r.query(ctx,
		`SELECT `+relationshipCols+` FROM relationship_dates
		  WHERE account_id = $1
		  ORDER BY due_at NULLS LAST, month, day`, accountID)
}

func (r *RelationshipRepository) query(ctx context.Context, sql string, args ...any) ([]*domain.RelationshipDate, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.RelationshipDate
	for rows.Next() {
		d, err := scanRelationshipDate(rows)
		if err != nil {
			return nil, err
		}
		if d != nil {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

func (r *RelationshipRepository) Upsert(ctx context.Context, d *domain.RelationshipDate) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO relationship_dates (id, account_id, kind, contact_id,
		    month, day, due_at, label, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (id) DO UPDATE
		    SET kind       = EXCLUDED.kind,
		        contact_id = EXCLUDED.contact_id,
		        month      = EXCLUDED.month,
		        day        = EXCLUDED.day,
		        due_at     = EXCLUDED.due_at,
		        label      = EXCLUDED.label,
		        notes      = EXCLUDED.notes,
		        -- last_fired_year is NOT reset by an edit: fixing a typo in a
		        -- label must not send this year's birthday greeting twice.
		        updated_at = NOW()
		  WHERE relationship_dates.account_id = EXCLUDED.account_id`,
		d.ID, d.AccountID, string(d.Kind), nilIfEmpty(d.ContactID),
		d.Month, d.Day, d.DueAt, d.Label, d.Notes)
	return err
}

func (r *RelationshipRepository) Delete(ctx context.Context, id, accountID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM relationship_dates WHERE id = $1 AND account_id = $2`,
		id, accountID)
	return err
}

// DueDates is what the worker acts on, across accounts.
//
// "Today" is each ACCOUNT's today, not the server's. A birthday greeting has
// to arrive on the day the customer's own calendar says — this product is not
// sold in one country, and a date evaluated in a fixed timezone fires a day
// early or a day late for everybody outside it.
//
// The window is computed in SQL rather than by loading every date and
// filtering in Go: a deployment with a thousand accounts has tens of
// thousands of dates and two of them matter today.
func (r *RelationshipRepository) DueDates(ctx context.Context, now time.Time, limit int) ([]*domain.RelationshipDate, error) {
	lead := domain.GreetingLeadDays
	return r.query(ctx,
		`WITH due AS (
		   SELECT d.*,
		          -- The account's own calendar date. LEFT JOIN against
		          -- pg_timezone_names because AT TIME ZONE RAISES on an
		          -- unknown name, and one account with a typo must not take
		          -- down the query for every other account.
		          (($1::timestamptz) AT TIME ZONE COALESCE(tz.name, 'UTC'))::date AS local_date
		     FROM relationship_dates d
		     JOIN accounts a ON a.id = d.account_id
		     LEFT JOIN pg_timezone_names tz ON tz.name = a.timezone
		 )
		 SELECT `+relationshipCols+` FROM due
		  WHERE
		    -- One-off: due once its date arrives, and STAYS due. A missed
		    -- maintenance does not stop mattering because the day passed.
		    (due_at IS NOT NULL AND due_at <= $1 + ($2::int || ' days')::interval)
		    OR
		    -- Recurring: inside the lead-time window on the account's own
		    -- calendar, and not already fired this year.
		    --
		    -- to_date, NOT make_date: somebody born on 29 February has a
		    -- birthday, and make_date(2027, 2, 29) RAISES — which would take
		    -- down this query for every account in the deployment. to_date
		    -- rolls it to 1 March in a non-leap year, which is what a human
		    -- would do anyway.
		    (due_at IS NULL
		     AND last_fired_year <> EXTRACT(YEAR FROM local_date)
		     AND to_date(EXTRACT(YEAR FROM local_date)::int
		                 || '-' || LPAD(month::text, 2, '0')
		                 || '-' || LPAD(day::text, 2, '0'), 'YYYY-MM-DD')
		         BETWEEN local_date AND (local_date + $2::int))
		  ORDER BY due_at NULLS LAST, month, day
		  LIMIT $3`, now, lead, limit)
}

// MarkFired stamps a recurring date, or clears a one-off.
//
// The year is derived IN SQL from the account's own timezone, so a greeting
// sent on 31 December in São Paulo is not stamped with the next year because
// the server had already rolled over.
//
// A one-off has its due date cleared rather than the row deleted: the
// maintenance history is what tells the next salesperson this equipment has
// been serviced before.
func (r *RelationshipRepository) MarkFired(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE relationship_dates d
		    SET last_fired_year = EXTRACT(YEAR FROM
		          (($2::timestamptz) AT TIME ZONE COALESCE(
		             (SELECT tz.name FROM pg_timezone_names tz
		               JOIN accounts a ON a.timezone = tz.name
		              WHERE a.id = d.account_id), 'UTC'))),
		        due_at = CASE WHEN due_at IS NOT NULL THEN NULL ELSE due_at END,
		        updated_at = NOW()
		  WHERE d.id = $1`, id, at)
	return err
}

// DueForCheckIn lists customers nobody has spoken to in a while.
//
// Only customers — somebody who has BOUGHT. A prospect who went quiet is the
// cadence's job, and messaging every silent contact would turn a relationship
// routine into a second cold-outreach channel.
func (r *RelationshipRepository) DueForCheckIn(ctx context.Context, silentSince time.Time, limit int) ([]*domain.CheckInCandidate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT s.account_id, c.id,
		        COALESCE(NULLIF(c.name, ''), c.external_id),
		        (SELECT MAX(m.created_at)
		           FROM messages m
		           JOIN conversations cv ON cv.id = m.conversation_id
		          WHERE cv.account_id = c.account_id AND cv.contact_id = c.id)
		   FROM contacts c
		   JOIN sales s ON s.contact_id = c.id AND s.account_id = c.account_id
		  GROUP BY s.account_id, c.id, c.account_id, c.name, c.external_id
		 HAVING COALESCE((SELECT MAX(m.created_at)
		                    FROM messages m
		                    JOIN conversations cv ON cv.id = m.conversation_id
		                   WHERE cv.account_id = c.account_id AND cv.contact_id = c.id),
		                 MAX(s.sold_at)) < $1
		    -- Not asked recently. A customer who stays quiet must not be
		    -- messaged every day they stay quiet.
		    AND NOT EXISTS (
		      SELECT 1 FROM check_in_nudges n
		       WHERE n.account_id = s.account_id
		         AND n.contact_id = c.id
		         AND n.created_at > $1)
		  LIMIT $2`, silentSince, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.CheckInCandidate
	for rows.Next() {
		c := &domain.CheckInCandidate{}
		if err := rows.Scan(&c.AccountID, &c.ContactID, &c.ContactName,
			&c.LastContactAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *RelationshipRepository) RecordCheckIn(ctx context.Context, id, accountID, contactID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO check_in_nudges (id, account_id, contact_id, created_at)
		 VALUES ($1,$2,$3,$4)`, id, accountID, contactID, at)
	return err
}

var _ domain.RelationshipRepository = (*RelationshipRepository)(nil)
