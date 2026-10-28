package domain

import (
	"context"
	"time"
)

// Case instrumentation (US-M011).
//
// The numbers that prove the product works — leads generated, approaches
// sent, replies received, meetings booked, how fast people answer — are
// recorded as EVENTS at the moment they happen, then aggregated per period.
//
// Events rather than counters on purpose: a counter answers only the
// question it was created for and drifts the first time a write is missed,
// while a log can be re-read for any period and audited afterwards.

// SalesEventKind is what happened. Free-form storage, closed set here: an
// unknown kind in the DB aggregates into nothing rather than crashing a
// report, but nothing in the codebase should be writing one.
type SalesEventKind string

const (
	// SalesEventLeadCreated is one lead entering the pipeline, carrying the
	// source that produced it (prospecting provider, inbound, manual).
	SalesEventLeadCreated SalesEventKind = "lead_created"
	// SalesEventApproachSent is a human-approved outbound approach that
	// actually left, tagged with the channel it went through.
	SalesEventApproachSent SalesEventKind = "approach_sent"
	// SalesEventReplyReceived is the prospect answering an approach. Carries
	// the latency so "average response time" averages over real replies.
	SalesEventReplyReceived SalesEventKind = "reply_received"
	// SalesEventMeetingBooked is an appointment the AI or a human booked.
	SalesEventMeetingBooked SalesEventKind = "meeting_booked"
	// SalesEventEnrichmentRun is one enrichment against the monthly tier
	// allowance. Recorded even when the provider later fails: the AI call
	// was made and paid for either way.
	SalesEventEnrichmentRun SalesEventKind = "enrichment_run"
)

// SalesEvent is one recorded fact.
type SalesEvent struct {
	ID        string         `json:"id"`
	AccountID string         `json:"account_id"`
	Kind      SalesEventKind `json:"kind"`
	// LeadID is nil for events with no lead (or whose lead was deleted —
	// the row survives, because a deleted lead must not erase the fact
	// that an approach went out).
	LeadID *string `json:"lead_id,omitempty"`
	// Source is where a lead came from; ChannelType is what a message went
	// through. Only one is meaningful per kind, both are optional.
	Source      string `json:"source,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`
	// LatencySeconds is set only on replies.
	LatencySeconds *int      `json:"latency_seconds,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// CaseMetrics is the aggregated answer for one account over one period.
type CaseMetrics struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	LeadsCreated int64 `json:"leads_created"`
	// LeadsBySource keys on whatever the pipeline recorded. Empty source
	// (a lead created before instrumentation, or by a path that does not
	// know) is reported under "unknown" rather than silently dropped.
	LeadsBySource map[string]int64 `json:"leads_by_source"`

	ApproachesSent      int64            `json:"approaches_sent"`
	ApproachesByChannel map[string]int64 `json:"approaches_by_channel"`

	RepliesReceived int64 `json:"replies_received"`
	MeetingsBooked  int64 `json:"meetings_booked"`

	// AvgReplySeconds is nil when nobody replied in the period — an average
	// of zero replies is not zero, it is unknown, and reporting 0s as
	// "instant response" would be the most flattering possible lie.
	AvgReplySeconds *int `json:"avg_reply_seconds,omitempty"`

	// ReplyRate is replies/approaches as a 0..1 fraction, nil when nothing
	// was sent (same reasoning as above).
	ReplyRate *float64 `json:"reply_rate,omitempty"`
}

// SalesEventRepository persists and aggregates the log.
type SalesEventRepository interface {
	// Record appends one event. Implementations must be idempotent for the
	// once-per-lead kinds, so a retried webhook cannot inflate a case.
	Record(ctx context.Context, e *SalesEvent) error
	// Metrics aggregates [from, to). Both bounds are required so a report
	// always names the period it covers.
	Metrics(ctx context.Context, accountID string, from, to time.Time) (*CaseMetrics, error)
	// CountSince counts events of one kind since a moment — the monthly
	// tier caps read this instead of keeping a second counter that could
	// disagree with the log.
	CountSince(ctx context.Context, accountID string, kind SalesEventKind, since time.Time) (int64, error)
	// LastApproachAt returns when the most recent approach to this lead
	// went out, used to compute reply latency. (zero, nil) when there is
	// none — a reply with no approach before it has no latency to report.
	LastApproachAt(ctx context.Context, leadID string) (time.Time, error)
}
