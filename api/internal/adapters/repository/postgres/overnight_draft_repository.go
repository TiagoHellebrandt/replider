package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Overnight drafting (§6.17, K2). Raw SQL: a claim that two pods can race on,
// and a per-account-per-local-date uniqueness the ORM would not express.
type OvernightDraftRepository struct {
	db *pgxpool.Pool
}

func NewOvernightDraftRepository(db *pgxpool.Pool) *OvernightDraftRepository {
	return &OvernightDraftRepository{db: db}
}

var _ domain.OvernightDraftRepository = (*OvernightDraftRepository)(nil)

func (r *OvernightDraftRepository) Settings(ctx context.Context, accountID string) (*domain.OvernightDraftSettings, error) {
	var s domain.OvernightDraftSettings
	var funnelID, enabledBy *string
	var lastRun *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT account_id, enabled, funnel_id, nightly_cap, run_hour, last_run_on, enabled_by
		 FROM overnight_draft_settings WHERE account_id = $1`,
		accountID,
	).Scan(&s.AccountID, &s.Enabled, &funnelID, &s.NightlyCap, &s.RunHour, &lastRun, &enabledBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.FunnelID = stringPtrValue(funnelID)
	s.EnabledBy = stringPtrValue(enabledBy)
	s.LastRunOn = lastRun
	return &s, nil
}

func (r *OvernightDraftRepository) SaveSettings(ctx context.Context, s *domain.OvernightDraftSettings) error {
	var funnelID *string
	if s.FunnelID != "" {
		funnelID = &s.FunnelID
	}
	var enabledBy *string
	if s.EnabledBy != "" {
		enabledBy = &s.EnabledBy
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO overnight_draft_settings
		     (account_id, enabled, funnel_id, nightly_cap, run_hour, enabled_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (account_id) DO UPDATE SET
		     enabled     = EXCLUDED.enabled,
		     funnel_id   = EXCLUDED.funnel_id,
		     nightly_cap = EXCLUDED.nightly_cap,
		     run_hour    = EXCLUDED.run_hour,
		     enabled_by  = EXCLUDED.enabled_by,
		     updated_at  = NOW()`,
		s.AccountID, s.Enabled, funnelID, s.NightlyCap, s.RunHour, enabledBy)
	return err
}

// ListEnabled joins the account's timezone in, so the worker answers "is it 3am
// there?" for every account from one query instead of one lookup each.
//
// Expired trials are excluded in SQL (same rule as usecases.accountMayUseAI):
// trial expiry used to live only in the HTTP middleware, so an expired trial
// kept drafting every night on whichever key it had.
func (r *OvernightDraftRepository) ListEnabled(ctx context.Context) ([]domain.OvernightDraftTarget, error) {
	// Two populations, one list:
	//  1. Accounts with a settings row — the row is a decision and rules,
	//     whichever way it points (an explicit enabled=false stays off).
	//  2. Accounts with NO row — on by default since 2026-08-19, with the
	//     column defaults (every active funnel, 40/night, 3am local). The
	//     product's promise is "open the app in the morning to work already
	//     done", and a default that breaks the promise until somebody finds
	//     a setting is the wrong default. The spend is still bounded twice:
	//     the nightly cap here and the plan's monthly AI-draft allowance
	//     (K4) inside DraftBatch. Only accounts with at least one lead are
	//     defaulted in — a night's tick over every empty signup would be
	//     churn for nothing (the batch would no-op, but why wake it).
	rows, err := r.db.Query(ctx,
		`SELECT s.account_id, s.enabled, s.funnel_id, s.nightly_cap, s.run_hour,
		        s.last_run_on, COALESCE(a.timezone, '')
		 FROM overnight_draft_settings s
		 JOIN accounts a ON a.id = s.account_id
		 WHERE s.enabled
		   AND a.suspended_at IS NULL
		   AND NOT (a.subscription_status = 'trialing'
		            AND a.trial_expires_at IS NOT NULL
		            AND a.trial_expires_at < NOW()
		            AND NOT a.trial_bypass)
		 UNION ALL
		 SELECT a.id, TRUE, NULL, 40, 3, NULL, COALESCE(a.timezone, '')
		 FROM accounts a
		 WHERE a.suspended_at IS NULL
		   AND NOT (a.subscription_status = 'trialing'
		            AND a.trial_expires_at IS NOT NULL
		            AND a.trial_expires_at < NOW()
		            AND NOT a.trial_bypass)
		   AND NOT EXISTS (
		       SELECT 1 FROM overnight_draft_settings s WHERE s.account_id = a.id
		   )
		   AND EXISTS (SELECT 1 FROM leads l WHERE l.account_id = a.id)
		 ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.OvernightDraftTarget{}
	for rows.Next() {
		var t domain.OvernightDraftTarget
		var funnelID *string
		var lastRun *time.Time
		if err := rows.Scan(&t.Settings.AccountID, &t.Settings.Enabled, &funnelID,
			&t.Settings.NightlyCap, &t.Settings.RunHour, &lastRun, &t.Timezone); err != nil {
			return nil, err
		}
		t.Settings.FunnelID = stringPtrValue(funnelID)
		t.Settings.LastRunOn = lastRun
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimNight stamps the local date and reports whether THIS caller got it.
//
// The conditional UPDATE is the claim: two pods ticking in the same minute both
// run the query, exactly one matches the `IS DISTINCT FROM` predicate, and the
// other drafts nothing. Doing this as read-then-write in Go would let both pods
// spend a night's allowance twice.
func (r *OvernightDraftRepository) ClaimNight(ctx context.Context, accountID string, localDate time.Time) (bool, error) {
	// INSERT-or-claim, because a default-on account (2026-08-19) has no row
	// until its first night: the insert materializes the row with the
	// column defaults and claims it in the same statement, so from the
	// second night on the account is the ordinary row-backed case. The
	// conditional DO UPDATE keeps the claim semantics — two pods ticking in
	// the same minute conflict on account_id and exactly one passes the
	// WHERE; an explicitly disabled row never matches it.
	tag, err := r.db.Exec(ctx,
		`INSERT INTO overnight_draft_settings (account_id, enabled, last_run_on)
		 VALUES ($1, TRUE, $2)
		 ON CONFLICT (account_id) DO UPDATE
		 SET last_run_on = $2, updated_at = NOW()
		 WHERE overnight_draft_settings.enabled
		   AND overnight_draft_settings.last_run_on IS DISTINCT FROM $2`,
		accountID, localDate)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// SaveRun records the night.
//
// ON CONFLICT DO UPDATE rather than plain insert: the claim already guarantees
// one run per night, so a conflict here means a retry after a crash, and
// overwriting the partial row with the complete one is the honest outcome.
func (r *OvernightDraftRepository) SaveRun(ctx context.Context, run *domain.OvernightDraftRun) error {
	if run.ID == "" {
		run.ID = ulid.Make().String()
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO overnight_draft_runs
		     (id, account_id, ran_on, drafted, skipped_opted_out, skipped_budget,
		      skipped_error, used_batch, started_at, finished_at, stopped_reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (account_id, ran_on) DO UPDATE SET
		     drafted           = EXCLUDED.drafted,
		     skipped_opted_out = EXCLUDED.skipped_opted_out,
		     skipped_budget    = EXCLUDED.skipped_budget,
		     skipped_error     = EXCLUDED.skipped_error,
		     used_batch        = EXCLUDED.used_batch,
		     finished_at       = EXCLUDED.finished_at,
		     stopped_reason    = EXCLUDED.stopped_reason`,
		run.ID, run.AccountID, run.RanOn, run.Drafted, run.SkippedOptedOut,
		run.SkippedBudget, run.SkippedError, run.UsedBatch,
		run.StartedAt, run.FinishedAt, nullableString(run.StoppedReason))
	return err
}

// LatestUnseen is the morning briefing.
//
// Only runs that DREW something: a night that drafted nothing is not news, and
// "0 abordagens prontas" every morning is how a good habit becomes a dismissed
// banner.
func (r *OvernightDraftRepository) LatestUnseen(ctx context.Context, accountID string) (*domain.OvernightDraftRun, error) {
	var run domain.OvernightDraftRun
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, ran_on, drafted, skipped_opted_out, skipped_budget,
		        skipped_error, used_batch, seen_at, started_at, finished_at,
		        COALESCE(stopped_reason, '')
		 FROM overnight_draft_runs
		 WHERE account_id = $1 AND seen_at IS NULL
		   AND (drafted > 0 OR stopped_reason IS NOT NULL)
		 ORDER BY ran_on DESC
		 LIMIT 1`,
		accountID,
	).Scan(&run.ID, &run.AccountID, &run.RanOn, &run.Drafted, &run.SkippedOptedOut,
		&run.SkippedBudget, &run.SkippedError, &run.UsedBatch, &run.SeenAt,
		&run.StartedAt, &run.FinishedAt, &run.StoppedReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// MarkSeen is account-scoped in the statement: this is reachable from an HTTP
// route, and another tenant's run id must match zero rows rather than depend on
// a caller having checked.
func (r *OvernightDraftRepository) MarkSeen(ctx context.Context, accountID, runID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE overnight_draft_runs SET seen_at = $3
		 WHERE id = $1 AND account_id = $2 AND seen_at IS NULL`,
		runID, accountID, at)
	return err
}
