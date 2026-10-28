package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// Daily digest storage (US-M010). Raw SQL: the digest is one aggregate
// over leads plus a due-query over accounts, and both read columns
// (sdr_digest_*) that no sqlc query touches.
type SDRDigestRepository struct {
	db *pgxpool.Pool
}

func NewSDRDigestRepository(db *pgxpool.Pool) *SDRDigestRepository {
	return &SDRDigestRepository{db: db}
}

var _ domain.SDRDigestRepository = (*SDRDigestRepository)(nil)

// ListDue returns opted-in accounts whose own local clock has reached
// sendHour and that have not been sent a digest since their local midnight.
//
// Both windows resolve in the ACCOUNT's timezone, in SQL. Doing it in Go
// would mean reading every opted-in account on every tick just to discard
// most of them; doing it against the server's clock is the bug this
// replaced — the digest fired at 08:00 UTC, which is 05:28 in São Paulo,
// exactly the "arrives while they sleep" case the worker set out to avoid.
//
// The `LEFT JOIN pg_timezone_names` is load-bearing: `AT TIME ZONE` raises
// on an unrecognised name, so one account with a typo'd timezone would
// abort the query for EVERY account. Unknown or empty degrades to UTC, the
// same fallback `domain.LoadLocation` uses.
//
// The NULL case is included on purpose: an account that has never been
// sent one is due. Suspended accounts are excluded — telling someone whose
// service is cut off what needs their attention is worse than silence.
func (r *SDRDigestRepository) ListDue(ctx context.Context, sendHour, limit int) ([]domain.DigestCandidate, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.Query(ctx,
		`WITH resolved AS (
		   SELECT a.id,
		          COALESCE(tz.name, 'UTC') AS tzname,
		          a.sdr_digest_last_sent_at
		     FROM accounts a
		     LEFT JOIN pg_timezone_names tz ON tz.name = a.timezone
		    WHERE a.sdr_digest_opt_out = FALSE
		      AND a.suspended_at IS NULL
		 )
		 SELECT id, tzname
		   FROM resolved
		  WHERE EXTRACT(HOUR FROM now() AT TIME ZONE tzname) >= $1
		    AND (sdr_digest_last_sent_at IS NULL
		         OR sdr_digest_last_sent_at
		            < date_trunc('day', now() AT TIME ZONE tzname) AT TIME ZONE tzname)
		  ORDER BY id
		  LIMIT $2`, sendHour, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DigestCandidate
	for rows.Next() {
		var c domain.DigestCandidate
		if err := rows.Scan(&c.AccountID, &c.Timezone); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Build computes the four counts in ONE pass over the account's leads.
//
// Every count is a FILTER on the same scan rather than four queries: the
// digest runs for every account on every tick, and four round trips per
// account is how a daily job becomes a nightly incident.
func (r *SDRDigestRepository) Build(ctx context.Context, accountID string, repliesSince, stalledBefore time.Time) (*domain.SDRDigest, error) {
	d := &domain.SDRDigest{AccountID: accountID}
	err := r.db.QueryRow(ctx,
		`SELECT
		   COUNT(*) FILTER (WHERE status = 'pending_approval'),
		   COUNT(*) FILTER (WHERE status = 'responded' AND updated_at >= $2),
		   COUNT(*) FILTER (WHERE status = 'contacted' AND updated_at < $3),
		   COUNT(*) FILTER (WHERE (owner_user_id IS NULL OR owner_user_id = '')
		                      AND status NOT IN ('won', 'lost'))
		 FROM leads
		 WHERE account_id = $1`,
		accountID, repliesSince, stalledBefore).
		Scan(&d.PendingApprovals, &d.NewReplies, &d.StalledLeads, &d.UnownedLeads)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *SDRDigestRepository) MarkSent(ctx context.Context, accountID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET sdr_digest_last_sent_at = $2 WHERE id = $1`, accountID, at)
	return err
}
