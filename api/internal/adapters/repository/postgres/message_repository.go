package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- MessageRepository ---

type MessageRepository struct {
	queries *Queries
}

func NewMessageRepository(db DBTX) *MessageRepository {
	return &MessageRepository{queries: New(db)}
}

func (r *MessageRepository) Create(ctx context.Context, msg *domain.Message) (bool, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.CreateMessage(ctx, CreateMessageParams{
		ID:                msg.ID,
		ChannelID:         msg.ChannelID,
		ConversationID:    msg.ConversationID,
		MediaUrl:          msg.MediaURL,
		Sender:            msg.From,
		SenderName:        msg.SenderName,
		Type:              msg.Type,
		Content:           msg.Content,
		Metadata:          toBytes(msg.Metadata),
		Direction:         msg.Direction,
		ExternalMessageID: msg.ExternalMessageID,
		InputTokens:       msg.InputTokens,
		OutputTokens:      msg.OutputTokens,
		ProviderCostUsd:   msg.ProviderCostUSD,
		TokenEstimated:    msg.TokenEstimated,
	})
	return rows > 0, err
}

// GetByExternalID looks up an already-persisted inbound message by its
// provider-side id. Used by the webhook dedupe path before we spend work
// on parsing, AI, and WebSocket fanout.
func (r *MessageRepository) GetByExternalID(ctx context.Context, channelID, externalID string) (*domain.Message, error) {
	if externalID == "" {
		return nil, nil
	}
	q := getQueries(ctx, r.queries)
	row, err := q.GetMessageByExternalID(ctx, GetMessageByExternalIDParams{
		ChannelID:         channelID,
		ExternalMessageID: &externalID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.Message{ID: row}, nil
}

func (r *MessageRepository) GetByID(ctx context.Context, messageID, accountID string) (*domain.Message, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetMessageForAccount(ctx, GetMessageForAccountParams{
		ID:        messageID,
		AccountID: accountID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.Message{
		ID:             row.ID,
		ChannelID:      row.ChannelID,
		ConversationID: row.ConversationID,
		From:           row.Sender,
		SenderName:     row.SenderName,
		Type:           row.Type,
		Content:        row.Content,
		MediaURL:       row.MediaUrl,
		Metadata:       metadataJSON(row.Metadata),
		Direction:      row.Direction,
		Transcript:     row.Transcript,
		Timestamp:      row.CreatedAt.Time,
	}, nil
}

func (r *MessageRepository) ApplyWhatsAppStatus(ctx context.Context, s domain.WhatsAppStatusUpdate) error {
	q := getQueries(ctx, r.queries)
	var pricing pgtype.Bool
	if s.PricingBillable != nil {
		pricing = pgtype.Bool{Bool: *s.PricingBillable, Valid: true}
	}
	id := s.ExternalMessageID
	return q.ApplyWhatsAppStatus(ctx, ApplyWhatsAppStatusParams{
		ChannelID:               s.ChannelID,
		ExternalMessageID:       &id,
		SentAt:                  timestampOrNull(s.SentAt),
		DeliveredAt:             timestampOrNull(s.DeliveredAt),
		ReadAt:                  timestampOrNull(s.ReadAt),
		FailedAt:                timestampOrNull(s.FailedAt),
		WaConversationExpiresAt: timestampOrNull(s.WAConversationExpiresAt),
		FailedReason:            s.FailedReason,
		WaConversationID:        s.WAConversationID,
		ConversationOrigin:      s.ConversationOrigin,
		PricingCategory:         s.PricingCategory,
		// sqlc's :exec generator collapses NULL booleans to a plain
		// bool zero — false here means "no opinion", which matches
		// the COALESCE semantics in the query (existing value wins).
		PricingBillable: pricing.Bool,
	})
}

// timestampOrNull converts an optional time.Time pointer into the
// pgtype.Timestamptz the sqlc-generated args expect. Avoids hand-writing
// the same `if t != nil { Valid: true }` wrapper at every call site.
func timestampOrNull(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// CreateMessageAttempt records a single provider send attempt against
// the parent messages row. Failed sends still get a row (with a NULL
// external_message_id) so ops can count attempts without scanning logs.
func (r *MessageRepository) CreateMessageAttempt(ctx context.Context, a *domain.MessageAttempt) error {
	q := getQueries(ctx, r.queries)
	external := ""
	if a.ExternalMessageID != nil {
		external = *a.ExternalMessageID
	}
	failedReason := ""
	if a.FailedReason != nil {
		failedReason = *a.FailedReason
	}
	return q.CreateMessageAttempt(ctx, CreateMessageAttemptParams{
		ID:                a.ID,
		MessageID:         a.MessageID,
		ChannelID:         a.ChannelID,
		ExternalMessageID: external,
		BubbleIndex:       a.BubbleIndex,
		SentAt:            timestampOrNull(a.SentAt),
		FailedAt:          timestampOrNull(a.FailedAt),
		FailedReason:      failedReason,
	})
}

// ApplyMessageAttemptStatus mirrors ApplyWhatsAppStatus on the child
// attempts table.
func (r *MessageRepository) ApplyMessageAttemptStatus(ctx context.Context, s domain.WhatsAppStatusUpdate) error {
	q := getQueries(ctx, r.queries)
	return q.ApplyMessageAttemptStatus(ctx, ApplyMessageAttemptStatusParams{
		ChannelID:         s.ChannelID,
		ExternalMessageID: s.ExternalMessageID,
		SentAt:            timestampOrNull(s.SentAt),
		DeliveredAt:       timestampOrNull(s.DeliveredAt),
		ReadAt:            timestampOrNull(s.ReadAt),
		FailedAt:          timestampOrNull(s.FailedAt),
		FailedReason:      s.FailedReason,
	})
}

func (r *MessageRepository) GetMessageAttemptByExternalID(ctx context.Context, channelID, externalID string) (*domain.MessageAttempt, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetMessageAttemptByExternalID(ctx, GetMessageAttemptByExternalIDParams{
		ChannelID:         channelID,
		ExternalMessageID: externalID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a := &domain.MessageAttempt{
		ID:          row.ID,
		MessageID:   row.MessageID,
		ChannelID:   row.ChannelID,
		BubbleIndex: row.BubbleIndex,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
	if row.ExternalMessageID != nil {
		a.ExternalMessageID = row.ExternalMessageID
	}
	if row.SentAt.Valid {
		t := row.SentAt.Time
		a.SentAt = &t
	}
	if row.DeliveredAt.Valid {
		t := row.DeliveredAt.Time
		a.DeliveredAt = &t
	}
	if row.ReadAt.Valid {
		t := row.ReadAt.Time
		a.ReadAt = &t
	}
	if row.FailedAt.Valid {
		t := row.FailedAt.Time
		a.FailedAt = &t
	}
	if row.FailedReason != nil {
		a.FailedReason = row.FailedReason
	}
	return a, nil
}

func (r *MessageRepository) SetTranscript(ctx context.Context, messageID, transcript string) error {
	q := getQueries(ctx, r.queries)
	return q.SetMessageTranscript(ctx, SetMessageTranscriptParams{
		ID:         messageID,
		Transcript: &transcript,
	})
}

func (r *MessageRepository) MarkSendFailed(ctx context.Context, messageID, reason string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkMessageSendFailed(ctx, MarkMessageSendFailedParams{
		ID:           messageID,
		FailedReason: &reason,
	})
}

// LastInboundInConversation returns the customer's most recent inbound
// message in the conversation, or (nil, nil) when there is none yet. Email
// replies read its threading headers and Reply-To from Metadata.
func (r *MessageRepository) LastInboundInConversation(ctx context.Context, conversationID string) (*domain.Message, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetLastInboundMessageByConversation(ctx, &conversationID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.Message{
		ID:                row.ID,
		ChannelID:         row.ChannelID,
		ConversationID:    row.ConversationID,
		From:              row.Sender,
		SenderName:        row.SenderName,
		Type:              row.Type,
		Content:           row.Content,
		MediaURL:          row.MediaUrl,
		Metadata:          metadataJSON(row.Metadata),
		Direction:         row.Direction,
		ExternalMessageID: row.ExternalMessageID,
		Timestamp:         row.CreatedAt.Time,
	}, nil
}

func (r *MessageRepository) ListByConversationID(ctx context.Context, conversationID string, limit, offset int32) ([]*domain.Message, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListMessagesByConversation(ctx, ListMessagesByConversationParams{
		ConversationID: &conversationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]*domain.Message, len(rows))
	for i, row := range rows {
		m := &domain.Message{
			ID:             row.ID,
			ChannelID:      row.ChannelID,
			ConversationID: row.ConversationID,
			From:           row.Sender,
			SenderName:     row.SenderName,
			Type:           row.Type,
			Content:        row.Content,
			MediaURL:       row.MediaUrl,
			Metadata:       metadataJSON(row.Metadata),
			Direction:      row.Direction,
			Timestamp:      row.CreatedAt.Time,
			FailedReason:   row.FailedReason,
		}
		if row.FailedAt.Valid {
			t := row.FailedAt.Time
			m.FailedAt = &t
		}
		msgs[i] = m
	}
	return msgs, nil
}

// ListByConversationBefore is the transcript's load-more: up to `limit`
// messages OLDER than `before`, returned in chronological order (the SQL
// walks newest-first so LIMIT takes the ones nearest the cursor). Same
// read-constructor shape as ListByConversationID — the dashboard reads
// failed_at/failed_reason from both.
func (r *MessageRepository) ListByConversationBefore(ctx context.Context, conversationID string, before time.Time, limit int32) ([]*domain.Message, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListMessagesByConversationBefore(ctx, ListMessagesByConversationBeforeParams{
		ConversationID: &conversationID,
		CreatedAt:      pgtype.Timestamptz{Time: before, Valid: true},
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]*domain.Message, len(rows))
	for i, row := range rows {
		m := &domain.Message{
			ID:             row.ID,
			ChannelID:      row.ChannelID,
			ConversationID: row.ConversationID,
			From:           row.Sender,
			SenderName:     row.SenderName,
			Type:           row.Type,
			Content:        row.Content,
			MediaURL:       row.MediaUrl,
			Metadata:       metadataJSON(row.Metadata),
			Direction:      row.Direction,
			Timestamp:      row.CreatedAt.Time,
			FailedReason:   row.FailedReason,
		}
		if row.FailedAt.Valid {
			t := row.FailedAt.Time
			m.FailedAt = &t
		}
		// Oldest first: index 0 of the DESC result is the newest.
		msgs[len(rows)-1-i] = m
	}
	return msgs, nil
}

func (r *MessageRepository) ListConversationMedia(ctx context.Context, conversationID string, limit, offset int32) ([]*domain.Message, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListConversationMedia(ctx, ListConversationMediaParams{
		ConversationID: &conversationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]*domain.Message, len(rows))
	for i, row := range rows {
		msgs[i] = &domain.Message{
			ID:             row.ID,
			ChannelID:      row.ChannelID,
			ConversationID: row.ConversationID,
			From:           row.Sender,
			SenderName:     row.SenderName,
			Type:           row.Type,
			Content:        row.Content,
			MediaURL:       row.MediaUrl,
			Metadata:       metadataJSON(row.Metadata),
			Direction:      row.Direction,
			Timestamp:      row.CreatedAt.Time,
		}
	}
	return msgs, nil
}

func (r *MessageRepository) ListByConversationSince(ctx context.Context, conversationID string, since time.Time) ([]*domain.Message, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListMessagesByConversationSince(ctx, ListMessagesByConversationSinceParams{
		ConversationID: &conversationID,
		CreatedAt:      pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]*domain.Message, len(rows))
	for i, row := range rows {
		msgs[i] = &domain.Message{
			ID:             row.ID,
			ChannelID:      row.ChannelID,
			ConversationID: row.ConversationID,
			From:           row.Sender,
			SenderName:     row.SenderName,
			Type:           row.Type,
			Content:        row.Content,
			MediaURL:       row.MediaUrl,
			Metadata:       metadataJSON(row.Metadata),
			Direction:      row.Direction,
			Timestamp:      row.CreatedAt.Time,
		}
	}
	return msgs, nil
}

func (r *MessageRepository) MarkConversationRead(ctx context.Context, conversationID string, readAt time.Time) error {
	_, err := r.queries.db.Exec(ctx,
		`UPDATE messages SET read_at = $1 WHERE conversation_id = $2 AND direction = 'inbound' AND read_at IS NULL`,
		readAt, conversationID,
	)
	return err
}

func (r *MessageRepository) ListUnreadInbound(ctx context.Context, conversationID string) ([]*domain.Message, error) {
	rows, err := r.queries.db.Query(ctx,
		`SELECT id, channel_id, conversation_id, sender, sender_name, type, content, media_url, metadata, direction, read_at, created_at
		 FROM messages WHERE conversation_id = $1 AND direction = 'inbound' AND read_at IS NULL
		 ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil so empty result serialises to `[]` not `null`.
	msgs := make([]*domain.Message, 0)
	for rows.Next() {
		msg := &domain.Message{}
		if err := rows.Scan(&msg.ID, &msg.ChannelID, &msg.ConversationID, &msg.From, &msg.SenderName,
			&msg.Type, &msg.Content, &msg.MediaURL, &msg.Metadata, &msg.Direction, &msg.ReadAt, &msg.Timestamp); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

func (r *MessageRepository) SearchMessages(ctx context.Context, accountID, query string, limit, offset int) ([]*domain.MessageSearchResult, error) {
	rows, err := r.queries.db.Query(ctx,
		`SELECT m.id, m.conversation_id, m.content, c.contact_name, m.created_at,
		        ts_rank(m.search_vector, plainto_tsquery('portuguese', $2)) as rank
		 FROM messages m
		 JOIN conversations c ON c.id = m.conversation_id
		 WHERE c.account_id = $1 AND m.search_vector @@ plainto_tsquery('portuguese', $2)
		 ORDER BY rank DESC, m.created_at DESC
		 LIMIT $3 OFFSET $4`,
		accountID, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil so empty result serialises to `[]` not `null`.
	results := make([]*domain.MessageSearchResult, 0)
	for rows.Next() {
		r := &domain.MessageSearchResult{}
		if err := rows.Scan(&r.MessageID, &r.ConversationID, &r.Content, &r.ContactName, &r.CreatedAt, &r.Rank); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// metadataJSON returns the JSONB column as json.RawMessage, never []byte.
//
// The generated row types carry metadata as []byte, and a []byte inside an
// interface{} field marshals as a BASE64 STRING — so every message re-read
// from the list arrived in the dashboard with its metadata unreadable, and
// the email sender-auth verdict (email_auth.trustworthy) silently fell back
// to "remetente não verificado" on refresh, even for mail whose DKIM/DMARC
// the receiving server had already passed. Live WebSocket delivery carried a
// map and serialized fine, which is exactly why nobody caught it: the bug
// only existed after a reload.
func metadataJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
