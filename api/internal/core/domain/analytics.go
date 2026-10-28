package domain

import "time"

// IdleConversationForCSAT is the slim view returned by the CSAT
// asker worker when it picks up conversations that have gone idle
// long enough to prompt for a rating.
type IdleConversationForCSAT struct {
	ConversationID string
	AccountID      string
	ChannelID      string
	ContactID      string
	// Language is the detected language tag of the conversation
	// ("pt_BR", "en", "es", ...). Empty when detection didn't run
	// (legacy rows, media-only first message); the worker falls
	// back to pt_BR in that case.
	Language string
}

// ExpensiveConversation is one row of the cost-attribution dashboard
// (US-030) — a conversation ranked by accumulated estimated AI provider
// cost. ContactName is nullable (anonymous/unnamed contacts).
type ExpensiveConversation struct {
	ConversationID string    `json:"conversation_id"`
	ContactID      string    `json:"contact_id"`
	ContactName    *string   `json:"contact_name,omitempty"`
	TotalCostUSD   float64   `json:"total_cost_usd"`
	CreatedAt      time.Time `json:"created_at"`
}

// AnalyticsOverview is the payload behind GET /v1/analytics/overview —
// the operational analytics MVP from Mission 6. Shape is fixed by the
// mission spec and the Flutter cards read it verbatim.
type AnalyticsOverview struct {
	TotalConversations      int                `json:"total_conversations"`
	ResolvedByAIPct         float64            `json:"resolved_by_ai_pct"`
	AvgFirstResponseSeconds int                `json:"avg_first_response_seconds"`
	ChannelBreakdown        []ChannelBreakdown `json:"channel_breakdown"`
	TopQuestions            []TopQuestion      `json:"top_questions"`
	CSAT                    CSATOverview       `json:"csat"`
	// PeriodDays mirrors the query param so the client can sanity-check
	// what the server actually ran (values outside 7|30|90 get clamped).
	PeriodDays int `json:"period_days"`
}

// ChannelBreakdown is the per-provider conversation count rendered
// as a horizontal-bar list on the dashboard. Provider matches the
// string the channels table stores ("whatsapp", "telegram",
// "instagram").
type ChannelBreakdown struct {
	Channel string `json:"channel"`
	Count   int    `json:"count"`
}

// TopQuestion is the lowercased/trimmed first inbound message of a
// conversation. MVP clustering (no NLP); copy on the dashboard
// frames it as "mensagens iniciais mais comuns" so users understand
// they're seeing raw text.
type TopQuestion struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// CSATOverview reports the satisfaction summary for the period.
// Distribution always carries keys "1".."5" (zero-valued buckets
// included) so the Flutter chart doesn't need defensive nil-handling.
type CSATOverview struct {
	Responses    int            `json:"responses"`
	Average      float64        `json:"average"`
	Distribution map[string]int `json:"distribution"`
}

// ConversationStats holds aggregate conversation metrics for an account
type ConversationStats struct {
	Total        int64 `json:"total"`
	Open         int64 `json:"open"`
	PendingHuman int64 `json:"pending_human"`
	Closed       int64 `json:"closed"`
	Last24h      int64 `json:"last_24h"`
	Last7d       int64 `json:"last_7d"`
}

// MessageStats holds aggregate message metrics for an account
type MessageStats struct {
	Total   int64 `json:"total"`
	Bot     int64 `json:"bot"`
	Human   int64 `json:"human"`
	Contact int64 `json:"contact"`
	Last24h int64 `json:"last_24h"`
}

// DailyCount represents a single day's conversation count
type DailyCount struct {
	Date  time.Time `json:"date"`
	Count int64     `json:"count"`
}

// SLAMetrics holds service level agreement performance metrics.
type SLAMetrics struct {
	AvgFirstResponseSeconds int `json:"avg_first_response_seconds"`
	AvgResolutionSeconds    int `json:"avg_resolution_seconds"`
	PctRespondedWithin5Min  int `json:"pct_responded_within_5min"`
	TotalResolved           int `json:"total_resolved"`
	TotalResponded          int `json:"total_responded"`
}

// AgentPerformance holds per-agent activity metrics.
type AgentPerformance struct {
	AgentID              string `json:"agent_id"`
	ConversationsHandled int64  `json:"conversations_handled"`
	MessagesSent         int64  `json:"messages_sent"`
}

// AttendantPerformance rolls up metrics rendered per human attendant on
// the Settings → Time screen. Zeros are surfaced (not omitted) so idle
// team members still appear — admins need "who picked up nothing this
// week?" visibility. CSAT fields are zero when the window has no
// answered ratings; the UI renders "—" in that case.
type AttendantPerformance struct {
	UserID                  string     `json:"user_id"`
	Name                    string     `json:"name"`
	Email                   string     `json:"email"`
	Role                    string     `json:"role"`
	LastSeenAt              *time.Time `json:"last_seen_at,omitempty"`
	ConversationsAssigned   int64      `json:"conversations_assigned"`
	ConversationsResolved   int64      `json:"conversations_resolved"`
	MessagesSent            int64      `json:"messages_sent"`
	AvgFirstResponseSeconds int        `json:"avg_first_response_seconds"`
	AvgResolutionSeconds    int        `json:"avg_resolution_seconds"`
	CSATAvg                 float32    `json:"csat_avg"`
	CSATCount               int64      `json:"csat_count"`
}

// AttendantPerformanceFilter is the input to
// AnalyticsRepository.GetAttendantPerformance. UserID is nullable so the
// same call shape powers the team list (all attendants) and the per-
// member drill-down (scoped to one).
type AttendantPerformanceFilter struct {
	AccountID string
	From      time.Time
	To        time.Time
	UserID    *string
}

// AttendantLiveRow is the per-attendant slice of the "who's swamped
// right now?" supervisor panel. Captures load (open conversations) and
// the worst SLA tail (oldest unanswered customer message) without
// requiring the panel to fan out one-query-per-attendant.
type AttendantLiveRow struct {
	UserID             string     `json:"user_id"`
	Name               string     `json:"name"`
	Role               string     `json:"role"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	OpenConvs          int64      `json:"open_convs"`
	OldestWaitingSince *time.Time `json:"oldest_waiting_since,omitempty"`
}

// LiveQueue is the "unassigned pile" summary: how many conversations
// currently have no attendant + the oldest customer wait among them.
// Drives the SLA-at-risk banner.
type LiveQueue struct {
	UnassignedCount    int64      `json:"unassigned_count"`
	OldestWaitingSince *time.Time `json:"oldest_waiting_since,omitempty"`
}

// LiveSupervisionSnapshot is the composite payload the /analytics/
// attendants/live endpoint returns — per-attendant load + queue tail
// in a single round trip so the panel can render atomically.
type LiveSupervisionSnapshot struct {
	Attendants []AttendantLiveRow `json:"attendants"`
	Queue      LiveQueue          `json:"queue"`
}

// AIPerformance is the metric customers actually look at to justify the
// subscription: "did the bot save me time this month?". Surfaces the
// autonomous-resolution rate (closed without human needing to pause the
// bot) and the volume of messages the bot handled end-to-end.
type AIPerformance struct {
	TotalConversations      int64   `json:"total_conversations"`
	AutonomousConversations int64   `json:"autonomous_conversations"`
	AutonomousResolutionPct float64 `json:"autonomous_resolution_pct"` // 0..100
	HandoffPct              float64 `json:"handoff_pct"`               // conversations where a human took over
	BotMessages             int64   `json:"bot_messages"`
	HumanMessages           int64   `json:"human_messages"`
	WindowDays              int     `json:"window_days"` // reporting window, e.g. 30
}
