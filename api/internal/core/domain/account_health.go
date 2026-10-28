package domain

import (
	"context"
	"time"
)

// AccountHealthMetrics is the raw per-account activity snapshot used to
// derive a retention health score (US-177). All counts are scoped to the
// account; the repository populates them in one cross-account sweep over
// two trailing 7-day windows so the score can be compared week-over-week.
type AccountHealthMetrics struct {
	AccountID string
	Name      string
	PlanID    *string

	// MessagesThisWeek / MessagesPrevWeek are automatic bot replies sent in
	// the trailing windows [now-7d, now] and [now-14d, now-7d].
	MessagesThisWeek int
	MessagesPrevWeek int

	// ConversationsThisWeek / TakenOverThisWeek drive the takeover ratio for
	// the current week: conversations created and how many a human assumed.
	ConversationsThisWeek int
	TakenOverThisWeek     int

	// ConversationsPrevWeek / TakenOverPrevWeek mirror the above for the
	// previous week so the score can be recomputed week-over-week.
	ConversationsPrevWeek int
	TakenOverPrevWeek     int

	// HasActiveAgent is true when the account has at least one active agent.
	HasActiveAgent bool

	// LastSeenAt is the most recent JWT-validated activity across the
	// account's users (login recency). Nil when no user has ever been seen.
	LastSeenAt *time.Time
}

// AccountHealthBreakdown splits a health score into its weighted
// components so the admin column can show why an account scores low.
type AccountHealthBreakdown struct {
	Messages     int `json:"messages"`
	BotActive    int `json:"bot_active"`
	Takeover     int `json:"takeover"`
	LoginRecency int `json:"login_recency"`
}

// AccountHealthScore is the computed retention health for one account
// (US-177). Score is 0-100 (higher = healthier). AtRisk flags a
// week-over-week drop of at least HealthDropAlertThreshold percent.
type AccountHealthScore struct {
	AccountID string  `json:"account_id"`
	Name      string  `json:"name"`
	PlanID    *string `json:"plan_id,omitempty"`

	Score     int                    `json:"score"`
	PrevScore int                    `json:"prev_score"`
	DropPct   int                    `json:"drop_pct"`
	AtRisk    bool                   `json:"at_risk"`
	Breakdown AccountHealthBreakdown `json:"breakdown"`

	// Surfaced raw inputs so the dashboard column can render context
	// without a second roundtrip.
	MessagesThisWeek int  `json:"messages_this_week"`
	TakeoverPct      int  `json:"takeover_pct"`
	HasActiveAgent   bool `json:"has_active_agent"`
	DaysSinceLogin   *int `json:"days_since_login,omitempty"`
}

// AccountHealthRepository sweeps per-account activity for the retention
// health dashboard (US-177). It is a raw-SQL repo (cross-account
// aggregation) deliberately kept off the sqlc AccountRepository.
type AccountHealthRepository interface {
	ListAccountHealthMetrics(ctx context.Context, now time.Time) ([]AccountHealthMetrics, error)
}
