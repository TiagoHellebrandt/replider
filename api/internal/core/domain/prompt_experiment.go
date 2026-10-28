package domain

import "time"

// Experiment status lifecycle: draft -> running -> stopped|completed.
const (
	ExperimentStatusDraft     = "draft"
	ExperimentStatusRunning   = "running"
	ExperimentStatusStopped   = "stopped"
	ExperimentStatusCompleted = "completed"
)

const (
	VariantA = "A"
	VariantB = "B"
)

// PromptExperiment is an A/B test bound to a single agent. Routing is
// resolved by AgentRunner: when the agent has a `running` experiment
// and the conversation is bucketed into a variant, that variant's body
// replaces agent.system_prompt for the AI call.
type PromptExperiment struct {
	ID            string     `json:"id"`
	AccountID     string     `json:"account_id"`
	AgentID       string     `json:"agent_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	VariantABody  string     `json:"variant_a_body"`
	VariantBBody  string     `json:"variant_b_body"`
	VariantALabel string     `json:"variant_a_label"`
	VariantBLabel string     `json:"variant_b_label"`
	SplitPctB     int        `json:"split_pct_b"`
	Winner        *string    `json:"winner,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	StoppedAt     *time.Time `json:"stopped_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ExperimentAssignment is the sticky bucket assignment for a single
// conversation. Once written, the conversation stays on this variant
// for its entire lifetime so closed-conversation metrics can still be
// attributed correctly after the experiment is stopped.
type ExperimentAssignment struct {
	ExperimentID   string    `json:"experiment_id"`
	ConversationID string    `json:"conversation_id"`
	Variant        string    `json:"variant"`
	AssignedAt     time.Time `json:"assigned_at"`
}

// VariantMetrics is the per-variant aggregation surfaced by
// GET /experiments/:id/results.
type VariantMetrics struct {
	Variant              string   `json:"variant"`
	Label                string   `json:"label"`
	ConversationCount    int64    `json:"conversation_count"`
	AIResolvedCount      int64    `json:"ai_resolved_count"`
	HandoffCount         int64    `json:"handoff_count"`
	CSATAverage          *float64 `json:"csat_average,omitempty"`
	CSATResponses        int64    `json:"csat_responses"`
	TokensTotal          int64    `json:"tokens_total"`
	AvgFirstResponseSecs *float64 `json:"avg_first_response_secs,omitempty"`
	AvgConversationLen   *float64 `json:"avg_conversation_len,omitempty"`
}

// ExperimentResults bundles both variants plus a "is_significant" flag
// computed from sample-size + delta thresholds (NOT a real Bayesian
// test — labelled "indicative" in the UI).
type ExperimentResults struct {
	Experiment    *PromptExperiment `json:"experiment"`
	VariantA      VariantMetrics    `json:"variant_a"`
	VariantB      VariantMetrics    `json:"variant_b"`
	IsSignificant bool              `json:"is_significant"`
	LikelyWinner  *string           `json:"likely_winner,omitempty"`
}

// ExperimentSample is one row in the per-variant sample bundle the
// improvement suggester sends to the LLM. The repo orders by CSAT
// rating so callers can split into "best" and "worst" slices and
// hand the model the most informative excerpts without including the
// full conversation log.
type ExperimentSample struct {
	ConversationID string
	Rating         *int // CSAT 1..5, NULL when no survey response
	Resolved       bool // true when conversation closed without human takeover
}

// ImprovementSuggestion is the parsed output of the auto-suggester
// (MVP-3). The LLM is asked for JSON shaped exactly like this struct;
// any parsing failure surfaces as ErrSuggestionParseFailed so the UI
// shows a "try again" CTA instead of a half-rendered card.
type ImprovementSuggestion struct {
	Rationale      string   `json:"rationale"`
	SuggestedBody  string   `json:"suggested_body"`
	NextHypotheses []string `json:"next_hypotheses"`
	TokensUsed     int64    `json:"tokens_used"`
	TokensRemain   int64    `json:"tokens_remain"`
}

var (
	ErrExperimentNotFound       = &AppError{Code: "EXPERIMENT_NOT_FOUND", Message: "experiment not found"}
	ErrExperimentAlreadyRunning = &AppError{Code: "EXPERIMENT_ALREADY_RUNNING", Message: "this agent already has a running experiment"}
	ErrExperimentNotRunning     = &AppError{Code: "EXPERIMENT_NOT_RUNNING", Message: "experiment is not running"}
	ErrExperimentStatusInvalid  = &AppError{Code: "EXPERIMENT_STATUS_INVALID", Message: "operation not allowed in this experiment status"}
	ErrPlanFeatureLocked        = &AppError{Code: "PLAN_FEATURE_LOCKED", Message: "this feature requires a higher plan"}
	ErrSuggestionParseFailed    = &AppError{Code: "SUGGESTION_PARSE_FAILED", Message: "the suggester returned an unexpected output — try again"}
	ErrSuggestionRateLimited    = &AppError{Code: "SUGGESTION_RATE_LIMITED", Message: "daily suggestion limit reached — try again tomorrow"}
)
