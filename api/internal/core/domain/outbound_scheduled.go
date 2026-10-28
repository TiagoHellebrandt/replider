package domain

import (
	"context"
	"time"
)

// OutboundScheduled is a future-dated outbound message staged for
// dispatch by the scheduler worker. The full Cloud-API-shaped payload
// is stored opaque in JSONB so adding a new message type / metadata
// field doesn't require a migration.
type OutboundScheduled struct {
	ID             string
	AccountID      string
	ChannelID      string
	ConversationID *string
	Recipient      string
	Payload        []byte // domain.Message JSON
	SendAt         time.Time
	Status         string // pending / sent / failed / cancelled
	Attempts       int
	LastError      *string
	SentMessageID  *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// OutboundScheduledRepository persists future-dated sends staged by
// the campaign / scheduler use cases and drained by the dispatcher
// worker. Status transitions: pending → sent | failed | cancelled.
type OutboundScheduledRepository interface {
	Create(ctx context.Context, row *OutboundScheduled) error
	ListDue(ctx context.Context, batchSize int) ([]*OutboundScheduled, error)
	MarkSent(ctx context.Context, id, messageID string) error
	MarkFailed(ctx context.Context, id, reason string) error
	TouchAttempt(ctx context.Context, id, reason string) error
	Cancel(ctx context.Context, id, accountID string) error
	ListByAccount(ctx context.Context, accountID string, limit, offset int) ([]*OutboundScheduled, error)
	// RecoverStale rolls back rows stuck in 'processing' for too long
	// so a crashed dispatcher's claims aren't lost forever.
	RecoverStale(ctx context.Context, staleMinutes int) (int64, error)
}
