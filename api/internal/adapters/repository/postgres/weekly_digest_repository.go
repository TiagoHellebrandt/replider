package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// WeeklyDigestRepository is a raw-SQL repo (NOT sqlc) over the same pool.
// It owns the accounts.weekly_digest_opt_out / weekly_digest_last_sent_at
// columns and the read-only activity aggregations that feed the weekly
// digest email (US-176). Raw SQL keeps these aggregate queries off the
// expensive-to-extend sqlc AccountRepository.
type WeeklyDigestRepository struct {
	db DBTX
}

func NewWeeklyDigestRepository(db DBTX) *WeeklyDigestRepository {
	return &WeeklyDigestRepository{db: db}
}

// ListAccountsDueForDigest returns accounts that should receive the weekly
// digest now: not opted out, not suspended, and either never sent or last
// sent before `before` (the once-a-week throttle cutoff). Ordered by id so
// the sweep is deterministic.
func (r *WeeklyDigestRepository) ListAccountsDueForDigest(ctx context.Context, before time.Time) ([]domain.WeeklyDigestAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, plan_id, weekly_digest_last_sent_at
		FROM accounts
		WHERE weekly_digest_opt_out = false
		  AND suspended_at IS NULL
		  AND (weekly_digest_last_sent_at IS NULL OR weekly_digest_last_sent_at < $1)
		ORDER BY id`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WeeklyDigestAccount
	for rows.Next() {
		var a domain.WeeklyDigestAccount
		if err := rows.Scan(&a.AccountID, &a.Name, &a.PlanID, &a.LastSentAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ComputeWeeklyStats aggregates one account's activity since `since`:
// bot replies sent, bot resolution rate, and the most-asked customer
// questions. All three are scoped to the account via the conversations
// join (messages have no direct account_id) — tenant-safe by construction.
func (r *WeeklyDigestRepository) ComputeWeeklyStats(ctx context.Context, accountID string, since time.Time) (domain.WeeklyDigestStats, error) {
	var stats domain.WeeklyDigestStats

	// Messages handled = automatic bot replies sent in the window.
	if err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.account_id = $1
		  AND m.direction = 'outbound'
		  AND m.sender = 'bot'
		  AND m.created_at >= $2`, accountID, since).Scan(&stats.MessagesHandled); err != nil {
		return stats, err
	}
	stats.TimeSavedMinutes = stats.MessagesHandled * domain.MinutesSavedPerBotReply

	// Conversion = bot resolution rate: conversations opened in the
	// window that closed without a human ever taking over.
	var resolved, total int
	if err := r.db.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'closed' AND is_bot_paused = false),
			count(*)
		FROM conversations
		WHERE account_id = $1
		  AND created_at >= $2`, accountID, since).Scan(&resolved, &total); err != nil {
		return stats, err
	}
	if total > 0 {
		stats.ConversionPct = resolved * 100 / total
	}

	// Top FAQs = most frequent inbound text questions in the window.
	// Group by the normalized (lower/trimmed) content; bound the length
	// so one-word acks and pasted blobs don't dominate the ranking.
	rows, err := r.db.Query(ctx, `
		SELECT lower(btrim(m.content)) AS q, count(*) AS n
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.account_id = $1
		  AND m.direction = 'inbound'
		  AND m.type = 'text'
		  AND m.content IS NOT NULL
		  AND char_length(btrim(m.content)) BETWEEN 3 AND 140
		  AND m.created_at >= $2
		GROUP BY lower(btrim(m.content))
		ORDER BY n DESC, q ASC
		LIMIT 5`, accountID, since)
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	for rows.Next() {
		var faq domain.WeeklyDigestFAQ
		if err := rows.Scan(&faq.Question, &faq.Count); err != nil {
			return stats, err
		}
		stats.TopFAQs = append(stats.TopFAQs, faq)
	}
	return stats, rows.Err()
}

// MarkDigestSent stamps the account so it isn't re-sent until the next
// week's cutoff passes.
func (r *WeeklyDigestRepository) MarkDigestSent(ctx context.Context, accountID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET weekly_digest_last_sent_at = $2, updated_at = now() WHERE id = $1`,
		accountID, at)
	return err
}

// SetWeeklyDigestOptOut flips the per-account opt-out (US-176). Exposed
// through the existing PATCH /accounts settings endpoint.
func (r *WeeklyDigestRepository) SetWeeklyDigestOptOut(ctx context.Context, accountID string, optOut bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET weekly_digest_opt_out = $2, updated_at = now() WHERE id = $1`,
		accountID, optOut)
	return err
}
