package postgres

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ReferralRepository is a raw-SQL repo (NOT sqlc) over the same pool. It
// owns the accounts.referral_code / referred_by_account_id columns and
// the `referrals` table for the referral program (US-174). Raw SQL keeps
// the referral feature off the expensive-to-extend sqlc AccountRepository.
type ReferralRepository struct {
	db DBTX
}

func NewReferralRepository(db DBTX) *ReferralRepository {
	return &ReferralRepository{db: db}
}

// referralCodeAlphabet excludes visually ambiguous characters (0/O, 1/I/L)
// so a code can be read aloud or copied from a screenshot without error.
const referralCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

const referralCodeLength = 8

// pgUniqueViolation is the Postgres SQLSTATE for a unique-constraint
// violation — used to retry a code collision rather than erroring out.
const pgUniqueViolation = "23505"

// generateReferralCode mints a random, unambiguous, uppercase code.
func generateReferralCode() (string, error) {
	buf := make([]byte, referralCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, referralCodeLength)
	for i, b := range buf {
		out[i] = referralCodeAlphabet[int(b)%len(referralCodeAlphabet)]
	}
	return string(out), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// GetOrCreateCode returns the account's referral code, minting one lazily
// on first call. Concurrent first-calls (or a code collision) converge on
// a single persisted code via the partial UNIQUE index + re-read.
func (r *ReferralRepository) GetOrCreateCode(ctx context.Context, accountID string) (string, error) {
	var existing *string
	err := r.db.QueryRow(ctx, `SELECT referral_code FROM accounts WHERE id = $1`, accountID).Scan(&existing)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", domain.ErrAccountNotFound
		}
		return "", err
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	// Mint a code, retrying on the rare collision or a concurrent claim.
	for attempt := 0; attempt < 5; attempt++ {
		code, err := generateReferralCode()
		if err != nil {
			return "", err
		}
		var written *string
		err = r.db.QueryRow(ctx,
			`UPDATE accounts SET referral_code = $2, updated_at = now()
			 WHERE id = $1 AND referral_code IS NULL
			 RETURNING referral_code`,
			accountID, code,
		).Scan(&written)
		if err == nil && written != nil {
			return *written, nil
		}
		if err == pgx.ErrNoRows {
			// A concurrent caller already set it — read the winner.
			if rerr := r.db.QueryRow(ctx, `SELECT referral_code FROM accounts WHERE id = $1`, accountID).Scan(&written); rerr == nil && written != nil {
				return *written, nil
			}
			continue
		}
		if isUniqueViolation(err) {
			continue // code collision — try a fresh one
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not allocate a unique referral code")
}

// ResolveReferrer maps a code to its owning account id, or "" if unknown.
func (r *ReferralRepository) ResolveReferrer(ctx context.Context, code string) (string, error) {
	if code == "" {
		return "", nil
	}
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM accounts WHERE referral_code = $1`, code).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

// LinkReferral stamps the referee's referred_by and inserts a pending
// referral row. Idempotent and self-referral-safe.
func (r *ReferralRepository) LinkReferral(ctx context.Context, refereeAccountID, referrerAccountID string) error {
	if refereeAccountID == "" || referrerAccountID == "" || refereeAccountID == referrerAccountID {
		return nil
	}
	if _, err := r.db.Exec(ctx,
		`UPDATE accounts SET referred_by_account_id = $2, updated_at = now()
		 WHERE id = $1 AND referred_by_account_id IS NULL`,
		refereeAccountID, referrerAccountID,
	); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO referrals (id, referrer_account_id, referee_account_id, status, created_at)
		 VALUES ($1, $2, $3, 'pending', now())
		 ON CONFLICT (referee_account_id) DO NOTHING`,
		ulid.Make().String(), referrerAccountID, refereeAccountID,
	)
	return err
}

// GetPendingByReferee returns the referee's unrewarded referral, or nil.
func (r *ReferralRepository) GetPendingByReferee(ctx context.Context, refereeAccountID string) (*domain.Referral, error) {
	var ref domain.Referral
	var rewardedAt *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT id, referrer_account_id, referee_account_id, status,
		        reward_amount_cents, reward_currency, created_at, rewarded_at
		 FROM referrals
		 WHERE referee_account_id = $1 AND status = $2`,
		refereeAccountID, domain.ReferralStatusPending,
	).Scan(&ref.ID, &ref.ReferrerAccountID, &ref.RefereeAccountID, &ref.Status,
		&ref.RewardAmountCents, &ref.RewardCurrency, &ref.CreatedAt, &rewardedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ref.RewardedAt = rewardedAt
	return &ref, nil
}

// MarkRewarded flips a pending referral to rewarded exactly once.
func (r *ReferralRepository) MarkRewarded(ctx context.Context, id string, amountCents int64, currency string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE referrals
		 SET status = $2, reward_amount_cents = $3, reward_currency = $4, rewarded_at = $5
		 WHERE id = $1 AND status = $6`,
		id, domain.ReferralStatusRewarded, amountCents, currency, at, domain.ReferralStatusPending,
	)
	return err
}

// StatsByReferrer counts referrals and rewards for a referrer account.
func (r *ReferralRepository) StatsByReferrer(ctx context.Context, referrerAccountID string) (*domain.ReferralStats, error) {
	var stats domain.ReferralStats
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = $2)
		 FROM referrals WHERE referrer_account_id = $1`,
		referrerAccountID, domain.ReferralStatusRewarded,
	).Scan(&stats.TotalReferred, &stats.TotalRewarded)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}
