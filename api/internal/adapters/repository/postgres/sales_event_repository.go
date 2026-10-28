package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Case instrumentation storage (US-M011).
//
// Raw SQL rather than sqlc: this repo is one insert and one aggregation
// that returns a shape sqlc cannot express well (several GROUP BYs folded
// into a single struct), and keeping it self-contained means adding a new
// event kind never regenerates the whole query layer.
type SalesEventRepository struct {
	db *pgxpool.Pool
}

func NewSalesEventRepository(db *pgxpool.Pool) *SalesEventRepository {
	return &SalesEventRepository{db: db}
}

var _ domain.SalesEventRepository = (*SalesEventRepository)(nil)

// Record appends one event, ignoring a duplicate of a once-per-lead kind.
// A redelivered webhook or a retried job must not inflate a case report.
func (r *SalesEventRepository) Record(ctx context.Context, e *domain.SalesEvent) error {
	if e.ID == "" {
		e.ID = ulid.Make().String()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO sales_events (id, account_id, kind, lead_id, source, channel_type, latency_seconds, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT DO NOTHING`,
		e.ID, e.AccountID, string(e.Kind), e.LeadID, e.Source, e.ChannelType, e.LatencySeconds, e.OccurredAt)
	return err
}

// CountSince backs the monthly tier caps (US-M013).
func (r *SalesEventRepository) CountSince(ctx context.Context, accountID string, kind domain.SalesEventKind, since time.Time) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM sales_events
		 WHERE account_id = $1 AND kind = $2 AND occurred_at >= $3`,
		accountID, string(kind), since).Scan(&n)
	return n, err
}

// LastApproachAt is how reply latency is derived: the newest approach to
// this lead. Zero time (no error) when the lead was never approached.
func (r *SalesEventRepository) LastApproachAt(ctx context.Context, leadID string) (time.Time, error) {
	var at *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT MAX(occurred_at) FROM sales_events
		 WHERE lead_id = $1 AND kind = $2`,
		leadID, string(domain.SalesEventApproachSent)).Scan(&at)
	if err != nil || at == nil {
		if err != nil {
			return time.Time{}, err
		}
		return time.Time{}, nil
	}
	return *at, nil
}

// Metrics aggregates [from, to) in ONE pass over the period's rows. The
// breakdowns are folded in Go rather than run as three more queries: the
// row set is already scoped to one account and one period, and three round
// trips to produce one screen is how a report becomes slow enough that
// nobody opens it.
func (r *SalesEventRepository) Metrics(ctx context.Context, accountID string, from, to time.Time) (*domain.CaseMetrics, error) {
	rows, err := r.db.Query(ctx,
		`SELECT kind, source, channel_type, latency_seconds
		 FROM sales_events
		 WHERE account_id = $1 AND occurred_at >= $2 AND occurred_at < $3`,
		accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &domain.CaseMetrics{
		From:                from,
		To:                  to,
		LeadsBySource:       map[string]int64{},
		ApproachesByChannel: map[string]int64{},
	}
	var latencySum, latencyCount int64

	for rows.Next() {
		var (
			kind        string
			source      string
			channelType string
			latency     *int
		)
		if err := rows.Scan(&kind, &source, &channelType, &latency); err != nil {
			return nil, err
		}
		switch domain.SalesEventKind(kind) {
		case domain.SalesEventLeadCreated:
			out.LeadsCreated++
			// A lead whose source nobody recorded still happened; hiding it
			// would make the totals disagree with the breakdown.
			if source == "" {
				source = "unknown"
			}
			out.LeadsBySource[source]++
		case domain.SalesEventApproachSent:
			out.ApproachesSent++
			if channelType == "" {
				channelType = "unknown"
			}
			out.ApproachesByChannel[channelType]++
		case domain.SalesEventReplyReceived:
			out.RepliesReceived++
			if latency != nil {
				latencySum += int64(*latency)
				latencyCount++
			}
		case domain.SalesEventMeetingBooked:
			out.MeetingsBooked++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Both derived numbers stay nil when their denominator is zero: an
	// average over no replies is unknown, not instant, and a reply rate
	// with nothing sent is undefined, not 0%.
	if latencyCount > 0 {
		avg := int(latencySum / latencyCount)
		out.AvgReplySeconds = &avg
	}
	if out.ApproachesSent > 0 {
		rate := float64(out.RepliesReceived) / float64(out.ApproachesSent)
		out.ReplyRate = &rate
	}
	return out, nil
}
