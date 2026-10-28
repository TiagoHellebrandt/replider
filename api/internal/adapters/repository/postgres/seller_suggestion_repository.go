package postgres

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// SellerSuggestionRepository is a raw-SQL repo (NOT sqlc, same rationale
// as SellerRefRepository) feeding the seller co-pilot engine (US-014):
// read-only signal aggregates over data other features already write,
// plus the tiny seller_suggestion_states table for dismissals.
type SellerSuggestionRepository struct {
	db DBTX
}

func NewSellerSuggestionRepository(db DBTX) *SellerSuggestionRepository {
	return &SellerSuggestionRepository{db: db}
}

// ListConversationStats aggregates, per carteira account, the current
// month's conversations (total + how many had at least one human reply
// — an OUTBOUND message from a non-bot sender, same direction+sender
// convention as the SLA and attendant analytics queries; inbound rows
// store the contact's phone/id in sender, never 'customer') and the
// previous month's total. The LEFT JOIN keeps zero-conversation
// accounts in the result.
func (r *SellerSuggestionRepository) ListConversationStats(ctx context.Context, sellerUserID string, curFrom, curTo, prevFrom, prevTo time.Time) ([]domain.SellerConvStats, error) {
	rows, err := r.db.Query(ctx,
		`SELECT a.id,
		        COUNT(c.id) FILTER (WHERE c.created_at >= $2 AND c.created_at < $3),
		        COUNT(c.id) FILTER (WHERE c.created_at >= $2 AND c.created_at < $3
		            AND EXISTS (SELECT 1 FROM messages m
		                         WHERE m.conversation_id = c.id
		                           AND m.direction = 'outbound'
		                           AND m.sender NOT IN ('bot', 'customer'))),
		        COUNT(c.id) FILTER (WHERE c.created_at >= $4 AND c.created_at < $5)
		 FROM accounts a
		 LEFT JOIN conversations c
		        ON c.account_id = a.id AND c.created_at >= $4 AND c.created_at < $3
		 WHERE a.seller_user_id = $1
		 GROUP BY a.id`,
		sellerUserID, curFrom, curTo, prevFrom, prevTo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerConvStats{}
	for rows.Next() {
		var s domain.SellerConvStats
		if err := rows.Scan(&s.AccountID, &s.CurrentTotal, &s.CurrentHuman, &s.PrevTotal); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListWhatsAppRisk returns carteira WhatsApp channels whose cached
// quality_rating (written by the WhatsAppQualityWorker onto
// channels.credentials) is RED — the ban-risk flag.
func (r *SellerSuggestionRepository) ListWhatsAppRisk(ctx context.Context, sellerUserID string) ([]domain.SellerChannelRisk, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ch.account_id, ch.id, COALESCE(ch.name, ''),
		        ch.credentials->>'quality_rating'
		 FROM channels ch
		 JOIN accounts a ON a.id = ch.account_id
		 WHERE a.seller_user_id = $1
		   AND ch.provider = 'whatsapp'
		   AND ch.credentials->>'quality_rating' = 'RED'
		 ORDER BY ch.account_id, ch.id`,
		sellerUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerChannelRisk{}
	for rows.Next() {
		var c domain.SellerChannelRisk
		if err := rows.Scan(&c.AccountID, &c.ChannelID, &c.ChannelName, &c.QualityRating); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListLatestNPS returns the newest NPS response per carteira account
// inside [from, to).
func (r *SellerSuggestionRepository) ListLatestNPS(ctx context.Context, sellerUserID string, from, to time.Time) ([]domain.SellerNPSSignal, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT ON (n.account_id) n.account_id, n.score, n.created_at
		 FROM nps_responses n
		 JOIN accounts a ON a.id = n.account_id
		 WHERE a.seller_user_id = $1
		   AND n.created_at >= $2 AND n.created_at < $3
		 ORDER BY n.account_id, n.created_at DESC`,
		sellerUserID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerNPSSignal{}
	for rows.Next() {
		var n domain.SellerNPSSignal
		if err := rows.Scan(&n.AccountID, &n.Score, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *SellerSuggestionRepository) ListSuggestionStates(ctx context.Context, sellerUserID, cycle string) ([]domain.SellerSuggestionState, error) {
	rows, err := r.db.Query(ctx,
		`SELECT account_id, suggestion_type, channel_id, cycle, status
		 FROM seller_suggestion_states
		 WHERE seller_user_id = $1 AND cycle = $2`,
		sellerUserID, cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerSuggestionState{}
	for rows.Next() {
		var s domain.SellerSuggestionState
		if err := rows.Scan(&s.AccountID, &s.SuggestionType, &s.ChannelID, &s.Cycle, &s.Status); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertSuggestionState marks a suggestion dismissed/done for the
// cycle; a second call for the same key just overwrites the status.
// channelID scopes channel-level types (whatsapp_ban_risk); "" for
// account-level ones (US-022).
func (r *SellerSuggestionRepository) UpsertSuggestionState(ctx context.Context, sellerUserID, accountID, suggestionType, channelID, cycle, status string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO seller_suggestion_states
		     (id, seller_user_id, account_id, suggestion_type, channel_id, cycle, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (seller_user_id, account_id, suggestion_type, channel_id, cycle)
		 DO UPDATE SET status = EXCLUDED.status, updated_at = NOW()`,
		ulid.Make().String(), sellerUserID, accountID, suggestionType, channelID, cycle, status,
	)
	return err
}
