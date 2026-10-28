package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// EmailMailboxRepository persists per-mailbox sending caps and warmup state
// (US-M007). Raw SQL: the interesting part is the ATOMIC counter, which is
// exactly what an ORM would obscure.
type EmailMailboxRepository struct {
	db DBTX
}

func NewEmailMailboxRepository(db DBTX) *EmailMailboxRepository {
	return &EmailMailboxRepository{db: db}
}

var _ domain.EmailMailboxRepository = (*EmailMailboxRepository)(nil)

const mailboxColumns = `id, account_id, channel_id, address, domain, daily_cap,
	warmup_started_at, warmup_completed_at, sent_today, sent_date, dns_verified_at,
	smtp_verified_at, bulk_domain_id, bulk_verified_at, created_at, updated_at`

func scanMailbox(row pgx.Row) (*domain.EmailMailbox, error) {
	var m domain.EmailMailbox
	var sentDate *time.Time
	err := row.Scan(&m.ID, &m.AccountID, &m.ChannelID, &m.Address, &m.Domain, &m.DailyCap,
		&m.WarmupStartedAt, &m.WarmupCompletedAt, &m.SentToday, &sentDate, &m.DNSVerifiedAt,
		&m.SMTPVerifiedAt, &m.BulkDomainID, &m.BulkVerifiedAt, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.SentDate = sentDate
	return &m, nil
}

// EnsureForChannel creates the row on first use. Making it implicit is
// deliberate: an explicit "register your mailbox" step is a step someone
// skips, and a skipped guard is no guard.
func (r *EmailMailboxRepository) EnsureForChannel(ctx context.Context, accountID, channelID, address string) (*domain.EmailMailbox, error) {
	addr := strings.ToLower(strings.TrimSpace(address))
	if addr == "" {
		return nil, domain.ErrMailboxNotFound
	}
	dom := domain.EmailDomain(addr)

	row := r.db.QueryRow(ctx,
		`INSERT INTO email_mailboxes (id, account_id, channel_id, address, domain, daily_cap, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		 ON CONFLICT (account_id, address) DO UPDATE SET
		   -- Re-point at the latest channel using this address but keep
		   -- every guard field: re-creating a channel must not reset a
		   -- warmup or hand back a fresh daily budget.
		   channel_id = EXCLUDED.channel_id,
		   updated_at = NOW()
		 RETURNING `+mailboxColumns,
		ulid.Make().String(), accountID, channelID, addr, dom, domain.DefaultDailyCap,
	)
	return scanMailbox(row)
}

func (r *EmailMailboxRepository) GetByAddress(ctx context.Context, accountID, address string) (*domain.EmailMailbox, error) {
	m, err := scanMailbox(r.db.QueryRow(ctx,
		`SELECT `+mailboxColumns+` FROM email_mailboxes WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address))))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

func (r *EmailMailboxRepository) ListByAccount(ctx context.Context, accountID string) ([]*domain.EmailMailbox, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+mailboxColumns+` FROM email_mailboxes WHERE account_id = $1 ORDER BY address`,
		accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.EmailMailbox{}
	for rows.Next() {
		m, err := scanMailbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RecordSend increments today's counter in ONE statement.
//
// The read-then-write version is a real bug, not a theoretical one: two
// approaches approved a second apart both read `sent_today = 24`, both
// decide they are under the cap of 25, and both send. Doing the reset and
// the increment inside a single UPDATE makes the counter authoritative.
func (r *EmailMailboxRepository) RecordSend(ctx context.Context, mailboxID string, now time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`UPDATE email_mailboxes
		 SET sent_today = CASE WHEN sent_date = $2::date THEN sent_today + 1 ELSE 1 END,
		     sent_date  = $2::date,
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING sent_today`,
		mailboxID, now.UTC(),
	).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrMailboxNotFound
	}
	return count, err
}

func (r *EmailMailboxRepository) MarkDNSVerified(ctx context.Context, accountID, address string, at time.Time) error {
	// Verification IS the start of the identity's public life, so the warmup
	// clock starts here — in the same statement, because a separate call is a
	// call somebody eventually forgets (warmup_started_at sat NULL for months
	// exactly that way, and a NULL warmup now refuses every cold send).
	// COALESCE: re-verifying must never restart a ramp already under way.
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes
		 SET dns_verified_at = $3,
		     warmup_started_at = COALESCE(warmup_started_at, $3),
		     updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), at)
	return err
}

// MarkSMTPProven records that the customer's own provider accepted an
// authenticated send from us. This is what a successful test send writes, and
// it is the only thing that lifts the cold-send block for a mailbox whose
// domain carries none of our records.
func (r *EmailMailboxRepository) MarkSMTPProven(ctx context.Context, accountID, address string, at time.Time) error {
	// Same rule as MarkDNSVerified: the proof event starts the warmup clock
	// atomically, and COALESCE keeps a running ramp untouched.
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes
		 SET smtp_verified_at = $3,
		     warmup_started_at = COALESCE(warmup_started_at, $3),
		     updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), at)
	return err
}

// ClearSMTPProof revokes that proof, which is what a credential change means:
// the host, user or password we proved with is not the one we would send with
// now. Scoped BY CHANNEL because that is what changed — an address shared by
// two channels keeps the proof earned through the other one.
//
// Deliberately does not touch dns_verified_at: our records are still published
// regardless of what the customer typed into an SMTP form.
func (r *EmailMailboxRepository) ClearSMTPProof(ctx context.Context, accountID, channelID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes SET smtp_verified_at = NULL, updated_at = NOW()
		 WHERE account_id = $1 AND channel_id = $2`,
		accountID, channelID)
	return err
}

// StartWarmup stamps the ramp start. Idempotent: re-running must not
// restart a warmup that is already half done (that would be a silent
// month-long throttle).
func (r *EmailMailboxRepository) StartWarmup(ctx context.Context, accountID, address string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes
		 SET warmup_started_at = COALESCE(warmup_started_at, $3),
		     warmup_completed_at = NULL,
		     updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), at)
	return err
}

func (r *EmailMailboxRepository) CompleteWarmup(ctx context.Context, accountID, address string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes SET warmup_completed_at = $3, updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), at)
	return err
}

// SetBulkDomain records the bulk provider's id for this mailbox's domain.
// Re-enabling with the same domain overwrites harmlessly (the provider call
// is create-or-reuse); it never touches bulk_verified_at — a re-register of
// an already-verified domain keeps the verification.
func (r *EmailMailboxRepository) SetBulkDomain(ctx context.Context, accountID, address, domainID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes SET bulk_domain_id = $3, updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), domainID)
	return err
}

// MarkBulkVerified stamps the provider reporting the domain's records
// verified. Forward-only like MarkDNSVerified: COALESCE keeps the first
// verification instant when a status poll re-reports it.
func (r *EmailMailboxRepository) MarkBulkVerified(ctx context.Context, accountID, address string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes
		 SET bulk_verified_at = COALESCE(bulk_verified_at, $3),
		     updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), at)
	return err
}

// SetDailyCap overrides the ceiling. Bounded: a cap of 5000 would not raise
// throughput, it would burn the domain — and a cap of 0 would silently stop
// all sending with no error anywhere.
func (r *EmailMailboxRepository) SetDailyCap(ctx context.Context, accountID, address string, cap int) error {
	if cap < 1 {
		cap = 1
	}
	if cap > 200 {
		cap = 200
	}
	_, err := r.db.Exec(ctx,
		`UPDATE email_mailboxes SET daily_cap = $3, updated_at = NOW()
		 WHERE account_id = $1 AND address = $2`,
		accountID, strings.ToLower(strings.TrimSpace(address)), cap)
	return err
}
