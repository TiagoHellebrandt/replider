package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- AnalyticsRepository ---

type AnalyticsRepository struct {
	queries *Queries
}

func NewAnalyticsRepository(db DBTX) *AnalyticsRepository {
	return &AnalyticsRepository{queries: New(db)}
}

func (r *AnalyticsRepository) GetConversationStats(ctx context.Context, accountID string) (*domain.ConversationStats, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetConversationStats(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &domain.ConversationStats{
		Total:        row.TotalConversations,
		Open:         row.OpenConversations,
		PendingHuman: row.PendingHuman,
		Closed:       row.ClosedConversations,
		Last24h:      row.Last24h,
		Last7d:       row.Last7d,
	}, nil
}

func (r *AnalyticsRepository) GetMessageStats(ctx context.Context, accountID string) (*domain.MessageStats, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetMessageStats(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &domain.MessageStats{
		Total:   row.TotalMessages,
		Bot:     row.BotMessages,
		Human:   row.HumanMessages,
		Contact: row.ContactMessages,
		Last24h: row.Last24h,
	}, nil
}

func (r *AnalyticsRepository) GetDailyConversationCounts(ctx context.Context, accountID string) ([]domain.DailyCount, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.GetDailyConversationCounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	counts := make([]domain.DailyCount, 0, len(rows))
	for _, row := range rows {
		counts = append(counts, domain.DailyCount{
			Date:  row.Date.Time,
			Count: row.Count,
		})
	}
	return counts, nil
}

func (r *AnalyticsRepository) GetSLAMetrics(ctx context.Context, accountID string) (*domain.SLAMetrics, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetSLAMetrics(ctx, accountID)
	if err != nil {
		return nil, err
	}
	toInt := func(v interface{}) int {
		switch n := v.(type) {
		case int32:
			return int(n)
		case int64:
			return int(n)
		case float64:
			return int(n)
		default:
			return 0
		}
	}
	return &domain.SLAMetrics{
		AvgFirstResponseSeconds: toInt(row.AvgFirstResponseSeconds),
		AvgResolutionSeconds:    toInt(row.AvgResolutionSeconds),
		PctRespondedWithin5Min:  int(row.PctRespondedWithin5min),
		TotalResolved:           int(row.TotalResolved),
		TotalResponded:          int(row.TotalResponded),
	}, nil
}

func (r *AnalyticsRepository) GetAgentPerformance(ctx context.Context, accountID string) ([]domain.AgentPerformance, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.GetAgentPerformance(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AgentPerformance, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.AgentPerformance{
			AgentID:              row.AgentID,
			ConversationsHandled: row.ConversationsHandled,
			MessagesSent:         row.MessagesSent,
		})
	}
	return result, nil
}

func (r *AnalyticsRepository) GetAttendantPerformance(ctx context.Context, f domain.AttendantPerformanceFilter) ([]domain.AttendantPerformance, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.GetAttendantPerformance(ctx, GetAttendantPerformanceParams{
		AccountID:   f.AccountID,
		CreatedAt:   pgtype.Timestamptz{Time: f.From, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: f.To, Valid: true},
		UserID:      f.UserID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AttendantPerformance, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AttendantPerformance{
			UserID:                  row.UserID,
			Name:                    row.Name,
			Email:                   row.Email,
			Role:                    stringPtrValue(row.Role),
			LastSeenAt:              timestamptzPtr(row.LastSeenAt),
			ConversationsAssigned:   row.ConversationsAssigned,
			ConversationsResolved:   row.ConversationsResolved,
			MessagesSent:            row.MessagesSent,
			AvgFirstResponseSeconds: int(row.AvgFirstResponseSeconds),
			AvgResolutionSeconds:    int(row.AvgResolutionSeconds),
			CSATAvg:                 row.CsatAvg,
			CSATCount:               row.CsatCount,
		})
	}
	return out, nil
}

func (r *AnalyticsRepository) GetAttendantLiveSnapshot(ctx context.Context, accountID string) ([]domain.AttendantLiveRow, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.GetAttendantLiveSnapshot(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.AttendantLiveRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AttendantLiveRow{
			UserID:             row.UserID,
			Name:               row.Name,
			Role:               stringPtrValue(row.Role),
			LastSeenAt:         timestamptzPtr(row.LastSeenAt),
			OpenConvs:          row.OpenConvs,
			OldestWaitingSince: timestamptzPtr(row.OldestWaitingSince),
		})
	}
	return out, nil
}

func (r *AnalyticsRepository) GetLiveQueueSnapshot(ctx context.Context, accountID string) (*domain.LiveQueue, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetUnassignedQueueSnapshot(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &domain.LiveQueue{
		UnassignedCount:    row.UnassignedCount,
		OldestWaitingSince: timestamptzPtr(row.OldestWaitingSince),
	}, nil
}

func (r *AnalyticsRepository) GetAIPerformance(ctx context.Context, accountID string) (*domain.AIPerformance, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAIPerformance(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := &domain.AIPerformance{
		TotalConversations:      row.TotalConversations,
		AutonomousConversations: row.AutonomousConversations,
		BotMessages:             row.BotMessages,
		HumanMessages:           row.HumanMessages,
		WindowDays:              30,
	}
	if out.TotalConversations > 0 {
		out.AutonomousResolutionPct = float64(out.AutonomousConversations) * 100 / float64(out.TotalConversations)
		out.HandoffPct = 100 - out.AutonomousResolutionPct
	}
	return out, nil
}

// GetOverview returns the Mission 6 operational MVP payload. It fans
// out to four sqlc queries (overview stats, channel breakdown, top
// questions, CSAT distribution) rather than a single mega-CTE so
// each piece can be unit-tested in isolation and a new breakdown can
// be added without rewriting the core.
func (r *AnalyticsRepository) GetOverview(ctx context.Context, accountID string, days int) (*domain.AnalyticsOverview, error) {
	q := getQueries(ctx, r.queries)

	// Core aggregates
	stats, err := q.GetOverviewStats(ctx, GetOverviewStatsParams{
		AccountID: accountID,
		Column2:   int32(days),
	})
	if err != nil {
		return nil, err
	}

	// Channel breakdown
	channelRows, err := q.GetChannelBreakdown(ctx, GetChannelBreakdownParams{
		AccountID: accountID,
		Column2:   int32(days),
	})
	if err != nil {
		return nil, err
	}
	channels := make([]domain.ChannelBreakdown, len(channelRows))
	for i, row := range channelRows {
		channels[i] = domain.ChannelBreakdown{
			Channel: row.Channel,
			Count:   int(row.Count),
		}
	}

	// Top questions
	topRows, err := q.GetTopQuestions(ctx, GetTopQuestionsParams{
		AccountID: accountID,
		Column2:   int32(days),
	})
	if err != nil {
		return nil, err
	}
	top := make([]domain.TopQuestion, len(topRows))
	for i, row := range topRows {
		top[i] = domain.TopQuestion{Text: row.Text, Count: int(row.Count)}
	}

	// CSAT distribution: seed all five buckets so missing scores are
	// "0" in the response rather than absent keys.
	csatRows, err := q.GetCSATDistribution(ctx, GetCSATDistributionParams{
		AccountID: accountID,
		Column2:   int32(days),
	})
	if err != nil {
		return nil, err
	}
	dist := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	var sum, total int
	for _, row := range csatRows {
		dist[strconv.Itoa(int(row.Rating))] = int(row.Count)
		sum += int(row.Rating) * int(row.Count)
		total += int(row.Count)
	}
	avg := 0.0
	if total > 0 {
		avg = float64(sum) / float64(total)
	}

	return &domain.AnalyticsOverview{
		TotalConversations:      int(stats.TotalConversations),
		ResolvedByAIPct:         stats.ResolvedByAiPct,
		AvgFirstResponseSeconds: int(stats.AvgFirstResponseSeconds),
		ChannelBreakdown:        channels,
		TopQuestions:            top,
		CSAT: domain.CSATOverview{
			Responses:    total,
			Average:      avg,
			Distribution: dist,
		},
		PeriodDays: days,
	}, nil
}

func (r *AnalyticsRepository) ListIdleForCSAT(ctx context.Context, idleMinutes, limit int) ([]*domain.IdleConversationForCSAT, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListIdleConversationsForCSAT(ctx, ListIdleConversationsForCSATParams{
		Column1: int32(idleMinutes),
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.IdleConversationForCSAT, len(rows))
	for i, row := range rows {
		lang := ""
		if row.Language != nil {
			lang = *row.Language
		}
		out[i] = &domain.IdleConversationForCSAT{
			ConversationID: row.ID,
			AccountID:      row.AccountID,
			ChannelID:      row.ChannelID,
			ContactID:      row.ContactID,
			Language:       lang,
		}
	}
	return out, nil
}

// GetTopExpensiveConversations returns the account's conversations ranked
// by accumulated estimated AI provider cost (US-030).
func (r *AnalyticsRepository) GetTopExpensiveConversations(ctx context.Context, accountID string, limit int) ([]*domain.ExpensiveConversation, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.GetTopExpensiveConversations(ctx, GetTopExpensiveConversationsParams{
		AccountID: accountID,
		Limit:     int32(limit), //nolint:gosec // G115: limit is pagination-clamped to [1,200]
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.ExpensiveConversation, len(rows))
	for i, row := range rows {
		out[i] = &domain.ExpensiveConversation{
			ConversationID: row.ID,
			ContactID:      row.ContactID,
			ContactName:    row.ContactName,
			TotalCostUSD:   row.TotalCostUsd,
			CreatedAt:      row.CreatedAt.Time,
		}
	}
	return out, nil
}

// Keep the pgtype import used — other functions in this file
// reference it when mapping interval/timestamp columns.
var _ = pgtype.Timestamptz{}
