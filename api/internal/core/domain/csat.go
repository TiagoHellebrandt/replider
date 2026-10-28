package domain

import (
	"context"
	"time"
)

// CSATRating captures a single customer-satisfaction answer for a
// conversation. Kept minimal on purpose: 1-5 stars plus optional free
// text is what SMB customers actually read; anything richer gets
// ignored in dashboards.
type CSATRating struct {
	ID             string     `json:"id"`
	AccountID      string     `json:"account_id"`
	ConversationID string     `json:"conversation_id"`
	ContactID      string     `json:"contact_id"`
	Rating         int        `json:"rating"` // 0 = pending, 1-5 = answered
	Feedback       *string    `json:"feedback,omitempty"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	AnsweredAt     *time.Time `json:"answered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CSATStats is the aggregated view for dashboards. AvgRating excludes
// pending (rating=0) rows so averages don't get pulled down by unanswered
// surveys. Response rate is answered/sent.
type CSATStats struct {
	AvgRating float32 `json:"avg_rating"`
	Sent      int64   `json:"sent"`
	Answered  int64   `json:"answered"`
}

// CSATRepository persists satisfaction ratings.
type CSATRepository interface {
	CreatePending(ctx context.Context, r *CSATRating) error
	RecordAnswer(ctx context.Context, conversationID string, rating int, feedback *string) error
	Stats(ctx context.Context, accountID string) (*CSATStats, error)
	// GetPending returns the in-flight CSAT row for a conversation
	// (sent but not answered), or (nil, nil) when there isn't one.
	// Used by the inbound pipeline to intercept a bare "1".."5"
	// reply before the AI responds to it.
	GetPending(ctx context.Context, conversationID string) (*CSATRating, error)
}
