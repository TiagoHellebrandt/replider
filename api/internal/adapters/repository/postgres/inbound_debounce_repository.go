package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// Inbound debounce queue (§6.15, K1).
//
// Raw SQL, and every statement here is doing something sqlc would not express:
// an upsert that CLAMPS the new value against a stored ceiling, and a claim that
// two pods can race safely.
type InboundDebounceRepository struct {
	db *pgxpool.Pool
}

func NewInboundDebounceRepository(db *pgxpool.Pool) *InboundDebounceRepository {
	return &InboundDebounceRepository{db: db}
}

var _ domain.InboundDebounceRepository = (*InboundDebounceRepository)(nil)

const pendingInboundColumns = `conversation_id, account_id, channel_id, agent_id,
	respond_at, first_queued_at, message_count, content, claimed_at`

func scanPendingInbound(row pgx.Row) (*domain.PendingInbound, error) {
	var p domain.PendingInbound
	err := row.Scan(&p.ConversationID, &p.AccountID, &p.ChannelID, &p.AgentID,
		&p.RespondAt, &p.FirstQueuedAt, &p.MessageCount, &p.Content, &p.ClaimedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Enqueue creates the pending run or extends the one already waiting.
//
// Three properties, all of them in the ON CONFLICT clause because doing them in
// Go would be a read-then-write race between two inbounds of the same burst:
//
//   - The new deadline is CLAMPED to `first_queued_at + ceiling`, so a customer
//     who keeps typing cannot defer the answer forever.
//   - `respond_at` only ever moves FORWARD (GREATEST). A late-arriving webhook
//     for an older message must not pull the answer earlier than the newest
//     message justifies.
//   - A row a worker has already CLAIMED is left alone: that run is in flight,
//     and this message belongs to the next burst. It gets a fresh row as soon as
//     the worker deletes the claimed one, which is the honest outcome — the
//     alternative is silently folding a new question into a reply that was
//     already generated without it.
func (r *InboundDebounceRepository) Enqueue(ctx context.Context, pending *domain.PendingInbound) (*domain.PendingInbound, error) {
	// Seconds as an int32 for make_interval: pgx cannot infer a type for a
	// parameter used as a text-concatenation operand, and `$n || ' seconds'`
	// fails at encode time rather than at compile time.
	ceilingSeconds := int32(domain.DebounceMaxTotalWait / time.Second) //nolint:gosec // G115: a compile-time constant of minutes

	row := r.db.QueryRow(ctx,
		`INSERT INTO inbound_debounce
		     (conversation_id, account_id, channel_id, agent_id,
		      respond_at, first_queued_at, message_count, content)
		 VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
		 ON CONFLICT (conversation_id) DO UPDATE SET
		     respond_at = LEAST(
		         GREATEST(inbound_debounce.respond_at, EXCLUDED.respond_at),
		         inbound_debounce.first_queued_at + make_interval(secs => $8)),
		     message_count = inbound_debounce.message_count + 1,
		     content       = EXCLUDED.content,
		     agent_id      = EXCLUDED.agent_id,
		     updated_at    = NOW()
		 WHERE inbound_debounce.claimed_at IS NULL
		 RETURNING `+pendingInboundColumns,
		pending.ConversationID, pending.AccountID, pending.ChannelID, pending.AgentID,
		pending.RespondAt, pending.FirstQueuedAt, pending.Content, ceilingSeconds,
	)

	out, err := scanPendingInbound(row)
	if err != nil {
		return nil, err
	}
	if out == nil {
		// The WHERE on the DO UPDATE filtered the row out: a claimed run is in
		// flight. Report the row as it stands so the caller can still tell the
		// dashboard what is happening.
		return r.Get(ctx, pending.ConversationID)
	}
	return out, nil
}

func (r *InboundDebounceRepository) Get(ctx context.Context, conversationID string) (*domain.PendingInbound, error) {
	return scanPendingInbound(r.db.QueryRow(ctx,
		`SELECT `+pendingInboundColumns+`
		 FROM inbound_debounce WHERE conversation_id = $1`,
		conversationID))
}

// ClaimDue atomically claims up to limit due, unclaimed rows.
//
// `FOR UPDATE SKIP LOCKED` inside the CTE is what makes two pods safe: each
// claims a disjoint set instead of both waiting on the same row and then both
// answering the same burst.
func (r *InboundDebounceRepository) ClaimDue(ctx context.Context, now time.Time, limit int) ([]*domain.PendingInbound, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx,
		`WITH due AS (
		     SELECT conversation_id
		     FROM inbound_debounce
		     WHERE claimed_at IS NULL AND respond_at <= $1
		     ORDER BY respond_at
		     LIMIT $2
		     FOR UPDATE SKIP LOCKED
		 )
		 UPDATE inbound_debounce d
		 SET claimed_at = $1, updated_at = NOW()
		 FROM due
		 WHERE d.conversation_id = due.conversation_id
		 RETURNING d.conversation_id, d.account_id, d.channel_id, d.agent_id,
		           d.respond_at, d.first_queued_at, d.message_count, d.content, d.claimed_at`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.PendingInbound{}
	for rows.Next() {
		var p domain.PendingInbound
		if err := rows.Scan(&p.ConversationID, &p.AccountID, &p.ChannelID, &p.AgentID,
			&p.RespondAt, &p.FirstQueuedAt, &p.MessageCount, &p.Content, &p.ClaimedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *InboundDebounceRepository) Release(ctx context.Context, conversationID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE inbound_debounce SET claimed_at = NULL, updated_at = NOW()
		 WHERE conversation_id = $1`,
		conversationID)
	return err
}

func (r *InboundDebounceRepository) Delete(ctx context.Context, conversationID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM inbound_debounce WHERE conversation_id = $1`, conversationID)
	return err
}

// AnswerNow pulls a pending run forward — the attendant's "responder agora".
//
// Account-scoped in the statement itself, not checked beforehand: this is
// reachable from an HTTP route, and a conversation id from another tenant must
// match zero rows rather than depend on a caller remembering to verify.
//
// A CLAIMED row is left alone and reported as "nothing pending": the run is
// already going, so there is nothing to hurry, and clearing the claim would let a
// second worker answer the same burst.
func (r *InboundDebounceRepository) AnswerNow(ctx context.Context, accountID, conversationID string, now time.Time) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE inbound_debounce
		 SET respond_at = $3, updated_at = NOW()
		 WHERE conversation_id = $1 AND account_id = $2 AND claimed_at IS NULL`,
		conversationID, accountID, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ReleaseStaleClaims re-queues what a dead worker left claimed.
//
// Without this, a pod that dies between the claim and the reply leaves a
// conversation permanently unanswered — the worst possible failure for this
// feature, because the customer's message was accepted and then silently
// dropped. Re-queuing risks a duplicate reply in the rare case the original run
// actually finished but failed to delete its row; an occasional double answer is
// a far better failure than a swallowed one.
func (r *InboundDebounceRepository) ReleaseStaleClaims(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE inbound_debounce
		 SET claimed_at = NULL, respond_at = NOW(), updated_at = NOW()
		 WHERE claimed_at IS NOT NULL AND claimed_at < $1`,
		olderThan)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
