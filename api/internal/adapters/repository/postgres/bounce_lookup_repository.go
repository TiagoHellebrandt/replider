package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// BounceLookupRepository answers one question: what did we last send to this
// address on this channel?
//
// Its own raw-SQL repo rather than a method on the sqlc MessageRepository,
// which dozens of test mocks implement — the narrow-port convention. The
// query cannot be expressed by paging ListByConversationID either: that one
// is ORDER BY created_at ASC, so a page of it holds the OLDEST messages in
// the thread, and a bounce is always about the newest.
type BounceLookupRepository struct{ db DBTX }

func NewBounceLookupRepository(db DBTX) *BounceLookupRepository {
	return &BounceLookupRepository{db: db}
}

// LastOutboundToContact returns the most recent outbound message sent to a
// contact on a channel, or (nil, nil) when there is none.
func (r *BounceLookupRepository) LastOutboundToContact(ctx context.Context, channelID, contactID string) (*domain.Message, error) {
	row := r.db.QueryRow(ctx,
		`SELECT m.id, m.conversation_id, m.direction, m.sender, m.metadata,
		        m.external_message_id
		   FROM messages m
		   JOIN conversations c ON c.id = m.conversation_id
		  WHERE c.channel_id = $1
		    AND c.contact_id = $2
		    AND m.direction = 'outbound'
		  ORDER BY m.created_at DESC
		  LIMIT 1`, channelID, contactID)

	m := &domain.Message{}
	// Metadata is `interface{}` on the domain struct, so it must be scanned
	// into a concrete type first — pgx has nothing to infer from otherwise.
	var meta []byte
	err := row.Scan(&m.ID, &m.ConversationID, &m.Direction, &m.From,
		&meta, &m.ExternalMessageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(meta) > 0 {
		m.Metadata = json.RawMessage(meta)
	}
	return m, nil
}
