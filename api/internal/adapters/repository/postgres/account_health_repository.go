package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// AccountHealthRepository is a raw-SQL repo (NOT sqlc) that sweeps
// per-account activity for the retention health dashboard (US-177). The
// cross-account aggregation joins messages/conversations/agents/users per
// account in one pass, so it's deliberately kept off the expensive sqlc
// AccountRepository.
type AccountHealthRepository struct {
	db DBTX
}

func NewAccountHealthRepository(db DBTX) *AccountHealthRepository {
	return &AccountHealthRepository{db: db}
}

// ListAccountHealthMetrics returns one row per non-suspended account with
// the raw signals (bot replies, conversations + takeovers, active-agent
// flag, login recency) across the current and previous 7-day windows.
func (r *AccountHealthRepository) ListAccountHealthMetrics(ctx context.Context, now time.Time) ([]domain.AccountHealthMetrics, error) {
	weekStart := now.AddDate(0, 0, -7)
	prevStart := now.AddDate(0, 0, -14)

	rows, err := r.db.Query(ctx, `
		SELECT
			a.id,
			a.name,
			a.plan_id,
			COALESCE(m.msgs_this, 0),
			COALESCE(m.msgs_prev, 0),
			COALESCE(cv.conv_this, 0),
			COALESCE(cv.taken_this, 0),
			COALESCE(cv.conv_prev, 0),
			COALESCE(cv.taken_prev, 0),
			COALESCE(ag.has_active, false),
			u.last_seen
		FROM accounts a
		LEFT JOIN LATERAL (
			SELECT
				count(*) FILTER (WHERE msg.created_at >= $1) AS msgs_this,
				count(*) FILTER (WHERE msg.created_at >= $2 AND msg.created_at < $1) AS msgs_prev
			FROM messages msg
			JOIN conversations c ON c.id = msg.conversation_id
			WHERE c.account_id = a.id
			  AND msg.direction = 'outbound'
			  AND msg.sender = 'bot'
			  AND msg.created_at >= $2
		) m ON true
		LEFT JOIN LATERAL (
			SELECT
				count(*) FILTER (WHERE c.created_at >= $1) AS conv_this,
				count(*) FILTER (WHERE c.created_at >= $1 AND c.assigned_to IS NOT NULL) AS taken_this,
				count(*) FILTER (WHERE c.created_at >= $2 AND c.created_at < $1) AS conv_prev,
				count(*) FILTER (WHERE c.created_at >= $2 AND c.created_at < $1 AND c.assigned_to IS NOT NULL) AS taken_prev
			FROM conversations c
			WHERE c.account_id = a.id
			  AND c.created_at >= $2
		) cv ON true
		LEFT JOIN LATERAL (
			SELECT bool_or(agt.is_active) AS has_active
			FROM agents agt
			WHERE agt.account_id = a.id
		) ag ON true
		LEFT JOIN LATERAL (
			SELECT max(us.last_seen_at) AS last_seen
			FROM users us
			WHERE us.account_id = a.id
		) u ON true
		WHERE a.suspended_at IS NULL
		ORDER BY a.id`, weekStart, prevStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AccountHealthMetrics
	for rows.Next() {
		var m domain.AccountHealthMetrics
		if err := rows.Scan(
			&m.AccountID,
			&m.Name,
			&m.PlanID,
			&m.MessagesThisWeek,
			&m.MessagesPrevWeek,
			&m.ConversationsThisWeek,
			&m.TakenOverThisWeek,
			&m.ConversationsPrevWeek,
			&m.TakenOverPrevWeek,
			&m.HasActiveAgent,
			&m.LastSeenAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
