package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// LeadTimelineRepository reads what was exchanged with one contact.
//
// Raw SQL: the read spans conversations and channels to answer a question about
// a CONTACT, which none of the sqlc conversation queries ask.
type LeadTimelineRepository struct {
	db *pgxpool.Pool
}

func NewLeadTimelineRepository(db *pgxpool.Pool) *LeadTimelineRepository {
	return &LeadTimelineRepository{db: db}
}

// ListForContact returns the most recent messages, oldest first.
//
// The account scope rides on the CONVERSATION, which is the row that owns the
// tenancy; filtering on the contact alone would trust an id supplied from
// outside. Ordering descending under the cap and reversing in Go is what makes
// "the last hundred" mean the last hundred — a plain ASC + LIMIT returns the
// oldest hundred, which for a long relationship is a conversation from years ago
// presented as the current one.
func (r *LeadTimelineRepository) ListForContact(
	ctx context.Context, accountID, contactID string, limit int32,
) ([]domain.TimelineMessage, error) {
	if limit <= 0 {
		limit = domain.TimelineLimit
	}

	rows, err := r.db.Query(ctx, `
		SELECT m.id,
		       m.direction,
		       m.sender,
		       COALESCE(m.sender_name, ''),
		       COALESCE(m.content, ''),
		       COALESCE(m.type, 'text'),
		       COALESCE(ch.provider, ''),
		       m.created_at,
		       m.failed_at,
		       COALESCE(m.failed_reason, '')
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		LEFT JOIN channels ch ON ch.id = c.channel_id
		-- contact_ref_id, not contact_id: contactID here is the contacts ULID
		-- the lead carries, while contact_id holds the raw channel sender
		-- (bare digits on WhatsApp, an address on email). The two never
		-- matched, so the timeline was empty for every lead that had one.
		WHERE c.account_id = $1 AND c.contact_ref_id = $2
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT $3
	`, accountID, contactID, limit)
	if err != nil {
		return nil, fmt.Errorf("list lead timeline: %w", err)
	}
	defer rows.Close()

	var newestFirst []domain.TimelineMessage
	for rows.Next() {
		var (
			id, direction, sender, senderName, content, kind, channel string
			msg                                                       domain.TimelineMessage
		)
		if err := rows.Scan(&id, &direction, &sender, &senderName,
			&content, &kind, &channel, &msg.SentAt,
			&msg.FailedAt, &msg.FailedReason); err != nil {
			return nil, err
		}

		msg.ID = id
		msg.Author = domain.ResolveTimelineAuthor(direction, sender)
		msg.Text = content
		msg.Kind = kind
		msg.ChannelType = channel
		// Only a teammate has a name worth showing: the customer's name is on the
		// card above the timeline, and "bot" is not a person.
		if msg.Author == domain.AuthorTeam {
			msg.AuthorName = senderName
		}
		newestFirst = append(newestFirst, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Oldest first, so the timeline reads downward like the conversation did.
	for i, j := 0, len(newestFirst)-1; i < j; i, j = i+1, j-1 {
		newestFirst[i], newestFirst[j] = newestFirst[j], newestFirst[i]
	}
	return newestFirst, nil
}
