package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Case instrumentation recorder (US-M011).
//
// Every pipeline point calls this instead of touching the repository, for
// one reason: instrumentation must NEVER be able to break the thing it
// measures. A failed insert here logs and returns; an approach that was
// sent is still sent, an inbound message is still saved.
type CaseRecorder struct {
	repo domain.SalesEventRepository
	// now is injectable so latency math is testable without sleeping.
	now func() time.Time
}

func NewCaseRecorder(repo domain.SalesEventRepository) *CaseRecorder {
	return &CaseRecorder{repo: repo, now: time.Now}
}

// WithClock pins the clock (tests).
func (c *CaseRecorder) WithClock(now func() time.Time) *CaseRecorder {
	c.now = now
	return c
}

// enabled reports whether there is anywhere to write. A nil recorder is a
// valid state — instrumentation is optional wiring, and every call site
// must work without it.
func (c *CaseRecorder) enabled() bool { return c != nil && c.repo != nil }

func (c *CaseRecorder) record(ctx context.Context, e *domain.SalesEvent) {
	if !c.enabled() {
		return
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = c.now()
	}
	if err := c.repo.Record(ctx, e); err != nil {
		slog.WarnContext(ctx, "case instrumentation: failed to record event",
			"kind", e.Kind, "account_id", e.AccountID, "error", err)
	}
}

// LeadCreated records one lead entering the pipeline. Idempotent per lead
// via the DB's partial unique index, so a re-imported prospect counts once.
func (c *CaseRecorder) LeadCreated(ctx context.Context, accountID, leadID, source string) {
	c.record(ctx, &domain.SalesEvent{
		AccountID: accountID,
		Kind:      domain.SalesEventLeadCreated,
		LeadID:    &leadID,
		Source:    source,
	})
}

// ApproachSent records an approach that actually LEFT — call it after the
// provider accepted the message, never before, or the case report counts
// sends that failed.
func (c *CaseRecorder) ApproachSent(ctx context.Context, accountID, leadID, channelType string) {
	c.record(ctx, &domain.SalesEvent{
		AccountID:   accountID,
		Kind:        domain.SalesEventApproachSent,
		LeadID:      &leadID,
		ChannelType: channelType,
	})
}

// ReplyReceived records a prospect answering, with the latency since the
// approach when there was one. A reply to a conversation nobody approached
// (pure inbound) is still a reply — it just has no latency to report.
func (c *CaseRecorder) ReplyReceived(ctx context.Context, accountID, leadID, channelType string) {
	if !c.enabled() {
		return
	}
	e := &domain.SalesEvent{
		AccountID:   accountID,
		Kind:        domain.SalesEventReplyReceived,
		LeadID:      &leadID,
		ChannelType: channelType,
		OccurredAt:  c.now(),
	}
	if sentAt, err := c.repo.LastApproachAt(ctx, leadID); err == nil && !sentAt.IsZero() {
		if secs := int(e.OccurredAt.Sub(sentAt).Seconds()); secs >= 0 {
			e.LatencySeconds = &secs
		}
	}
	c.record(ctx, e)
}

// MeetingBooked records an appointment. Idempotent per lead, so a client
// retrying the booking tool does not double-count the case's best metric.
func (c *CaseRecorder) MeetingBooked(ctx context.Context, accountID, leadID string) {
	c.record(ctx, &domain.SalesEvent{
		AccountID: accountID,
		Kind:      domain.SalesEventMeetingBooked,
		LeadID:    &leadID,
	})
}

// EnrichmentRun records one enrichment against the tier's monthly
// allowance (US-M013). Not idempotent per lead on purpose: enriching the
// same company twice really is two AI calls.
func (c *CaseRecorder) EnrichmentRun(ctx context.Context, accountID, leadID string) {
	c.record(ctx, &domain.SalesEvent{
		AccountID: accountID,
		Kind:      domain.SalesEventEnrichmentRun,
		LeadID:    &leadID,
	})
}

// CaseMetricsUseCase serves the aggregated report.
type CaseMetricsUseCase struct {
	repo domain.SalesEventRepository
}

func NewCaseMetricsUseCase(repo domain.SalesEventRepository) *CaseMetricsUseCase {
	return &CaseMetricsUseCase{repo: repo}
}

// CaseReport is the one-page story: this period, the one before it, and
// the difference — plus an explicitly-labelled estimate of the manual work
// avoided.
type CaseReport struct {
	Current  *domain.CaseMetrics `json:"current"`
	Previous *domain.CaseMetrics `json:"previous"`

	// Deltas are absolute differences (current − previous). Percentages are
	// deliberately absent: a jump from 1 to 3 is not "+200% growth", and a
	// report that says so is the kind of thing that gets quoted back.
	ApproachesDelta int64 `json:"approaches_delta"`
	RepliesDelta    int64 `json:"replies_delta"`
	MeetingsDelta   int64 `json:"meetings_delta"`
	LeadsDelta      int64 `json:"leads_delta"`

	// HoursSaved estimates the manual work avoided, using the assumption
	// below. It is reported WITH its inputs so the reader can disagree with
	// the arithmetic instead of being asked to trust a number.
	HoursSaved float64 `json:"hours_saved"`
	// MinutesPerApproach is the assumption behind HoursSaved: how long a
	// human takes to research a company, write a personalised approach and
	// send it. Stated, not hidden.
	MinutesPerApproach int `json:"minutes_per_approach"`
}

// DefaultMinutesPerApproach is the assumption behind the "work avoided"
// estimate: ~8 minutes to open a company, read enough to say something
// specific, write it and send it. Conservative on purpose — an inflated
// assumption is what makes a case report unbelievable.
const DefaultMinutesPerApproach = 8

// Report builds the one-page case (US-M012): the period, the SAME-LENGTH
// period immediately before it, and the difference. Comparing against an
// equal-length window is what makes the delta mean anything.
func (uc *CaseMetricsUseCase) Report(ctx context.Context, accountID string, from, to time.Time, minutesPerApproach int) (*CaseReport, error) {
	current, err := uc.Execute(ctx, accountID, from, to)
	if err != nil {
		return nil, err
	}
	window := to.Sub(from)
	previous, err := uc.Execute(ctx, accountID, from.Add(-window), from)
	if err != nil {
		return nil, err
	}
	if minutesPerApproach <= 0 {
		minutesPerApproach = DefaultMinutesPerApproach
	}

	return &CaseReport{
		Current:            current,
		Previous:           previous,
		ApproachesDelta:    current.ApproachesSent - previous.ApproachesSent,
		RepliesDelta:       current.RepliesReceived - previous.RepliesReceived,
		MeetingsDelta:      current.MeetingsBooked - previous.MeetingsBooked,
		LeadsDelta:         current.LeadsCreated - previous.LeadsCreated,
		HoursSaved:         float64(current.ApproachesSent) * float64(minutesPerApproach) / 60,
		MinutesPerApproach: minutesPerApproach,
	}, nil
}

// Execute aggregates [from, to). Both bounds are required by the repo, so
// the caller resolves defaults and the report always names its period.
func (uc *CaseMetricsUseCase) Execute(ctx context.Context, accountID string, from, to time.Time) (*domain.CaseMetrics, error) {
	if uc == nil || uc.repo == nil {
		return nil, &domain.AppError{
			Code:    "CASE_METRICS_UNAVAILABLE",
			Message: "instrumentação de case não configurada",
		}
	}
	if !to.After(from) {
		return nil, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "o fim do período deve ser depois do início",
		}
	}
	return uc.repo.Metrics(ctx, accountID, from, to)
}
