package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// AutoAssignRotationRepository is a raw-SQL repo (NOT sqlc) over the
// auto_assign_rotation table (US-021 review fixes) — the persisted
// round-robin cursor. Same rationale as AutoAssignSettingsRepository:
// isolated feature, cheap to keep off the sqlc repos.
type AutoAssignRotationRepository struct {
	db DBTX
}

func NewAutoAssignRotationRepository(db DBTX) *AutoAssignRotationRepository {
	return &AutoAssignRotationRepository{db: db}
}

// AssignNextRoundRobin claims the next attendant and assigns the
// conversation in ONE atomic statement, so the cursor only ever
// advances together with a successful assignment:
//
//   - conv locks the target conversation row (and proves it exists);
//   - claim upserts the cursor ONLY when conv matched — the ON CONFLICT
//     row lock serializes concurrent claims across replicas, and the
//     DO UPDATE expression sees the latest committed cursor, so two
//     pods never hand out the same turn;
//   - assigned stamps conversations.assigned_to from the claim (same
//     routing-only semantics as AssignConversation: no is_bot_paused,
//     no assigned_at).
//
// A missing conversation returns domain.ErrConversationNotFound with the cursor
// untouched. The next pick is the smallest pool id greater than the
// stored cursor, wrapping to the smallest — a stale cursor (user left
// the pool) degrades to "next in ULID order", never an error.
func (r *AutoAssignRotationRepository) AssignNextRoundRobin(ctx context.Context, conversationID, accountID string, pool []string) (string, error) {
	if len(pool) == 0 {
		return "", fmt.Errorf("auto-assign rotation: empty attendant pool")
	}

	var assignee string
	err := r.db.QueryRow(ctx, `
		WITH conv AS (
			SELECT id FROM conversations
			WHERE id = $1 AND account_id = $2
			FOR UPDATE
		),
		claim AS (
			INSERT INTO auto_assign_rotation (account_id, last_assigned_user_id, updated_at)
			SELECT $2, (SELECT min(p) FROM unnest($3::text[]) AS p), NOW()
			WHERE EXISTS (SELECT 1 FROM conv)
			ON CONFLICT (account_id) DO UPDATE SET
				last_assigned_user_id = COALESCE(
					(SELECT min(p) FROM unnest($3::text[]) AS p
					 WHERE p > auto_assign_rotation.last_assigned_user_id),
					(SELECT min(p) FROM unnest($3::text[]) AS p)
				),
				updated_at = NOW()
			RETURNING last_assigned_user_id
		)
		UPDATE conversations c
		SET assigned_to = claim.last_assigned_user_id, updated_at = NOW()
		FROM claim
		WHERE c.id = $1 AND c.account_id = $2
		RETURNING c.assigned_to`,
		conversationID, accountID, pool,
	).Scan(&assignee)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", domain.ErrConversationNotFound
		}
		return "", err
	}
	return assignee, nil
}
