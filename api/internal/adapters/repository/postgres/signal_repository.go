package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type SignalRepository struct{ db DBTX }

func NewSignalRepository(db DBTX) *SignalRepository {
	return &SignalRepository{db: db}
}

const signalCols = `id, account_id, source, external_id, organization, cnpj,
	title, description, url, value_cents, city, state, published_at,
	closes_at, lead_id, dismissed_at, created_at`

func scanSignal(row pgx.Row) (*domain.Signal, error) {
	s := &domain.Signal{}
	var source string
	var leadID *string
	err := row.Scan(&s.ID, &s.AccountID, &source, &s.ExternalID, &s.Organization,
		&s.CNPJ, &s.Title, &s.Description, &s.URL, &s.ValueCents, &s.City,
		&s.State, &s.PublishedAt, &s.ClosesAt, &leadID, &s.DismissedAt, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.Source = domain.SignalSource(source)
	if leadID != nil {
		s.LeadID = *leadID
	}
	return s, nil
}

// signalGateOverFetch is how much wider the query reads than the caller asked
// for, so the read-time gate below can drop rows and still fill the page.
//
// Four, because that is roughly the noise ratio measured in production on
// 15/09/2026: of 62 stored signals, 45 failed the health gate — rows collected
// while a pre-gate image was live, because every automated API deploy had been
// broken since 01/09 and the gate reached production days after it merged.
//
// This is NOT deep pagination: OFFSET is applied before the filter, so paging
// far into the list can repeat or skip. Nothing does — the radar asks for 20 at
// offset 0 and the signals screen asks with no params at all — and a list whose
// whole value is "the most urgent first" has no business being paged into.
const signalGateOverFetch = 4

// scanSignalWithFlag is scanSignal plus the derived-watch marker the read-time
// gate needs. Separate function rather than a parameter on scanSignal because
// the single-row reads (Get) select no such column.
func scanSignalWithFlag(row pgx.Row, derived *bool) (*domain.Signal, error) {
	s := &domain.Signal{}
	var source string
	var leadID *string
	err := row.Scan(&s.ID, &s.AccountID, &source, &s.ExternalID, &s.Organization,
		&s.CNPJ, &s.Title, &s.Description, &s.URL, &s.ValueCents, &s.City,
		&s.State, &s.PublishedAt, &s.ClosesAt, &leadID, &s.DismissedAt,
		&s.CreatedAt, derived)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.Source = domain.SignalSource(source)
	if leadID != nil {
		s.LeadID = *leadID
	}
	return s, nil
}

// Upsert is keyed on (account_id, source, external_id).
//
// DO UPDATE rather than DO NOTHING: PNCP republishes a tender as it is
// amended, and the amended version is the one that matters — a deadline that
// moved is the whole point of reading it again. But lead_id and dismissed_at
// are NOT touched: a human's decision outranks the source's redelivery.
// Upsert writes the signal and reads back the COMMITTED row's id, lead
// link and dismissal (atomic upsert-and-read; a republished tender keeps
// its original row id, and the auto-offer path must see the real one or it
// would re-offer a tender somebody already worked).
func (r *SignalRepository) Upsert(ctx context.Context, s *domain.Signal) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO signals (id, account_id, source, external_id, organization,
		    cnpj, title, description, url, value_cents, city, state,
		    published_at, closes_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (account_id, source, external_id) DO UPDATE
		    SET organization = EXCLUDED.organization,
		        title        = EXCLUDED.title,
		        description  = EXCLUDED.description,
		        url          = EXCLUDED.url,
		        value_cents  = EXCLUDED.value_cents,
		        closes_at    = EXCLUDED.closes_at
		 RETURNING id, COALESCE(lead_id, ''), dismissed_at`,
		s.ID, s.AccountID, string(s.Source), s.ExternalID, s.Organization,
		s.CNPJ, s.Title, s.Description, s.URL, s.ValueCents, s.City, s.State,
		s.PublishedAt, s.ClosesAt).Scan(&s.ID, &s.LeadID, &s.DismissedAt)
	return err
}

// List returns the feed: nothing dismissed, nothing already worked, and
// nothing whose deadline has passed.
//
// The expiry filter runs in SQL rather than in Go for the usual reason — a
// page filtered afterwards would show ten rows, hide eight expired ones, and
// leave the salesperson wondering where the feed went.
func (r *SignalRepository) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.Signal, error) {
	return r.listOpen(ctx, accountID, limit, offset, false)
}

// ListForRadar is List INCLUDING signals already linked to a lead. The Radar
// must keep showing a tender after the auto-offer drafted it — that is the
// moment it became work (the card's CTA turns into "open the draft"), not
// the moment it stopped mattering. Same order as List.
func (r *SignalRepository) ListForRadar(ctx context.Context, accountID string, limit, offset int) ([]*domain.Signal, error) {
	return r.listOpen(ctx, accountID, limit, offset, true)
}

// CountByAccount counts every row the account ever stored — dismissed,
// worked and expired included. The poller reads zero as "fresh: backfill".
func (r *SignalRepository) CountByAccount(ctx context.Context, accountID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM signals WHERE account_id = $1`, accountID).Scan(&n)
	return n, err
}

func (r *SignalRepository) listOpen(ctx context.Context, accountID string, limit, offset int, includeWorked bool) ([]*domain.Signal, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+signalCols+`,
		    -- Was this row collected by a watch the SYSTEM derived from the
		    -- catalogue, rather than one a human typed? The read-time health
		    -- gate below applies only to the derived kind, exactly as the
		    -- poller's ingest gate does — somebody who typed those words on
		    -- purpose is not second-guessed.
		    EXISTS (
		        SELECT 1 FROM signal_watches dw
		         WHERE dw.account_id = s.account_id
		           AND dw.source = s.source
		           AND dw.is_active
		           AND dw.derived_from_catalog
		    ) AS from_derived_watch
		  FROM signals s
		  WHERE s.account_id = $1
		    AND s.dismissed_at IS NULL
		    AND ($4 OR s.lead_id IS NULL)
		    AND (s.closes_at IS NULL OR s.closes_at > NOW())
		    -- A signal is mine only while some rule of mine still claims it.
		    --
		    -- Deleting the watch that collected them does not delete what it
		    -- collected: migration 000234 removed one unscoped watch and its
		    -- eighteen national hits — PVC reducers, a garden sprinkler —
		    -- stayed on a hospital-equipment radar. Emptying the tap is not
		    -- emptying the bucket.
		    --
		    -- Matched on SOURCE rather than a stored watch id: a watch that was
		    -- edited is still the same standing interest, and re-collecting
		    -- every tender to re-stamp it would cost a full poll.
		    --
		    -- Only for sources we POLL. A signal pushed in by an integration
		    -- the account pays for was claimed by the act of pushing it, and
		    -- it will never have a watch row; filtering those would hide every
		    -- row a vendor feed ever delivered.
		    AND (
		        s.source <> ANY($5::text[])
		        OR EXISTS (
		            SELECT 1 FROM signal_watches w
		             WHERE w.account_id = s.account_id
		               AND w.source = s.source
		               AND w.is_active
		        )
		    )
		  ORDER BY
		    -- Closest deadline first, and signals with no deadline after the
		    -- dated ones: urgency is the entire value of a tender.
		    s.closes_at ASC NULLS LAST,
		    s.published_at DESC
		  LIMIT $2 OFFSET $3`,
		accountID, limit*signalGateOverFetch, offset, includeWorked, polledSources())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.Signal, 0, limit)
	for rows.Next() {
		var derived bool
		s, err := scanSignalWithFlag(rows, &derived)
		if err != nil {
			return nil, err
		}
		if s == nil {
			continue
		}
		if !domain.SignalPassesDerivedGate(s, derived) {
			continue
		}
		out = append(out, s)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

func (r *SignalRepository) Get(ctx context.Context, id, accountID string) (*domain.Signal, error) {
	return scanSignal(r.db.QueryRow(ctx,
		`SELECT `+signalCols+` FROM signals WHERE id = $1 AND account_id = $2`,
		id, accountID))
}

// Dismiss records "not for us". A stamp rather than a delete, so the next
// poll does not bring the same notice straight back.
func (r *SignalRepository) Dismiss(ctx context.Context, id, accountID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE signals SET dismissed_at = $3
		  WHERE id = $1 AND account_id = $2 AND dismissed_at IS NULL`,
		id, accountID, at)
	return err
}

// LinkLead marks a signal as worked so two people do not chase the same
// tender. Only the first claim sticks.
func (r *SignalRepository) LinkLead(ctx context.Context, id, accountID, leadID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE signals SET lead_id = $3
		  WHERE id = $1 AND account_id = $2 AND lead_id IS NULL`,
		id, accountID, leadID)
	return err
}

// ── Watches ───────────────────────────────────────────────────────────────

const watchCols = `id, account_id, source, keywords, cnaes, states,
	min_value_cents, is_active, derived_from_catalog, created_at, updated_at`

func scanWatch(row pgx.Row) (*domain.SignalWatch, error) {
	w := &domain.SignalWatch{}
	var source string
	err := row.Scan(&w.ID, &w.AccountID, &source, &w.Keywords, &w.CNAEs,
		&w.States, &w.MinValueCents, &w.IsActive, &w.DerivedFromCatalog,
		&w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	w.Source = domain.SignalSource(source)
	return w, nil
}

func (r *SignalRepository) ListWatches(ctx context.Context, accountID string) ([]*domain.SignalWatch, error) {
	return r.queryWatches(ctx,
		`SELECT `+watchCols+` FROM signal_watches
		  WHERE account_id = $1 ORDER BY created_at`, accountID)
}

// ListActiveWatches spans every account: the poller fetches the source once
// and fans out, rather than hitting an external API per tenant.
func (r *SignalRepository) ListActiveWatches(ctx context.Context) ([]*domain.SignalWatch, error) {
	return r.queryWatches(ctx,
		`SELECT `+watchCols+` FROM signal_watches WHERE is_active ORDER BY account_id`)
}

func (r *SignalRepository) queryWatches(ctx context.Context, sql string, args ...any) ([]*domain.SignalWatch, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.SignalWatch
	for rows.Next() {
		w, err := scanWatch(rows)
		if err != nil {
			return nil, err
		}
		if w != nil {
			out = append(out, w)
		}
	}
	return out, rows.Err()
}

func (r *SignalRepository) UpsertWatch(ctx context.Context, w *domain.SignalWatch) error {
	// Nil slices reach a NOT NULL TEXT[] column as NULL. Normalised here
	// rather than at every call site.
	keywords, cnaes, states := w.Keywords, w.CNAEs, w.States
	if keywords == nil {
		keywords = []string{}
	}
	if cnaes == nil {
		cnaes = []string{}
	}
	if states == nil {
		states = []string{}
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO signal_watches (id, account_id, source, keywords, cnaes,
		    states, min_value_cents, is_active, derived_from_catalog)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (id) DO UPDATE
		    SET source               = EXCLUDED.source,
		        keywords             = EXCLUDED.keywords,
		        cnaes                = EXCLUDED.cnaes,
		        states               = EXCLUDED.states,
		        min_value_cents      = EXCLUDED.min_value_cents,
		        is_active            = EXCLUDED.is_active,
		        derived_from_catalog = EXCLUDED.derived_from_catalog,
		        updated_at           = NOW()
		  WHERE signal_watches.account_id = EXCLUDED.account_id`,
		w.ID, w.AccountID, string(w.Source), keywords, cnaes, states,
		w.MinValueCents, w.IsActive, w.DerivedFromCatalog)
	return err
}

// AccountsNeedingDefaultWatch finds accounts with a territory UF and at
// least one product whose pncp watch is either ABSENT or still exactly as
// the seeder left it.
//
// The second case is new and is what lets a fix to the keyword derivation
// reach accounts that were seeded by the old one. It is safe because
// derived_from_catalog is set in exactly one place (the seeder) and the
// update handler builds its row without it, so any edit through the UI
// clears the flag and permanently opts that watch out of re-derivation —
// a watch the customer touched is a decision, same posture as one they
// turned off (is_active = false is excluded here too).
func (r *SignalRepository) AccountsNeedingDefaultWatch(ctx context.Context) ([]domain.SignalWatchCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, UPPER(a.territory->>'uf'),
		       COALESCE((
			SELECT w.id FROM signal_watches w
			WHERE w.account_id = a.id AND w.source = 'pncp'
			  AND w.derived_from_catalog AND w.is_active
			ORDER BY w.created_at LIMIT 1
		       ), '') AS derived_watch_id
		FROM accounts a
		WHERE a.territory->>'uf' IS NOT NULL AND a.territory->>'uf' <> ''
		  AND EXISTS (SELECT 1 FROM products p WHERE p.account_id = a.id)
		  AND (
			NOT EXISTS (
			  SELECT 1 FROM signal_watches w
			  WHERE w.account_id = a.id AND w.source = 'pncp'
			)
			OR EXISTS (
			  SELECT 1 FROM signal_watches w
			  WHERE w.account_id = a.id AND w.source = 'pncp'
			    AND w.derived_from_catalog AND w.is_active
			)
		  )`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SignalWatchCandidate
	for rows.Next() {
		var c domain.SignalWatchCandidate
		if err := rows.Scan(&c.AccountID, &c.UF, &c.DerivedWatchID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *SignalRepository) DeleteWatch(ctx context.Context, id, accountID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM signal_watches WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

var _ domain.SignalRepository = (*SignalRepository)(nil)

// ── Source health ─────────────────────────────────────────────────────────

// RecordAttempt stamps one poll pass per source.
//
// The whole row is written in one statement so a crash between "attempt" and
// "outcome" cannot leave a source looking healthier than it is. The failure
// count is computed in SQL from the row's own previous value, which is what
// makes concurrent pods idempotent rather than additive.
func (r *SignalRepository) RecordAttempt(
	ctx context.Context, source domain.SignalSource, at time.Time, cause error,
) error {
	failed := cause != nil
	msg := ""
	if failed {
		msg = cause.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO signal_source_health
		       (source, last_attempt_at, last_success_at, consecutive_failures, last_error, updated_at)
		VALUES ($1, $2::timestamptz,
		        CASE WHEN $3::bool THEN NULL ELSE $2::timestamptz END,
		        CASE WHEN $3::bool THEN 1 ELSE 0 END, $4, now())
		ON CONFLICT (source) DO UPDATE SET
		    last_attempt_at = EXCLUDED.last_attempt_at,
		    -- A failed pass must never move the success stamp: that stamp is
		    -- the only thing the screen can honestly date the feed by.
		    last_success_at = CASE WHEN $3::bool
		        THEN signal_source_health.last_success_at
		        ELSE EXCLUDED.last_attempt_at END,
		    consecutive_failures = CASE WHEN $3::bool
		        THEN signal_source_health.consecutive_failures + 1
		        ELSE 0 END,
		    last_error = EXCLUDED.last_error,
		    updated_at = now()`,
		string(source), at, failed, msg)
	return err
}

func (r *SignalRepository) ListHealth(ctx context.Context) ([]*domain.SignalSourceHealth, error) {
	rows, err := r.db.Query(ctx, `
		SELECT source, last_attempt_at, last_success_at,
		       consecutive_failures, last_error
		  FROM signal_source_health
		 ORDER BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.SignalSourceHealth
	for rows.Next() {
		h := &domain.SignalSourceHealth{}
		var src string
		if err := rows.Scan(&src, &h.LastAttemptAt, &h.LastSuccessAt,
			&h.ConsecutiveFailures, &h.LastError); err != nil {
			return nil, err
		}
		h.Source = domain.SignalSource(src)
		out = append(out, h)
	}
	return out, rows.Err()
}

var _ domain.SignalSourceHealthRepository = (*SignalRepository)(nil)

// polledSources lists the sources the orphan rule applies to.
//
// Derived from the domain predicate rather than written out here, so adding a
// polled source cannot leave this query behind — a filter that silently stops
// covering a source is exactly the shape of the bug it was written to fix.
func polledSources() []string {
	all := []domain.SignalSource{
		domain.SignalPNCP, domain.SignalNewCompany, domain.SignalJobPost,
	}
	out := make([]string, 0, len(all))
	for _, s := range all {
		if domain.IsPolledSource(s) {
			out = append(out, string(s))
		}
	}
	return out
}
