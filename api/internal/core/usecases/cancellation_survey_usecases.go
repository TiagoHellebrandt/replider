package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// cancellationCommentMaxLen caps the free-text comment so a runaway client
// can't store unbounded text.
const cancellationCommentMaxLen = 2000

// SubmitCancellationSurveyUseCase persists a churn-reason survey response
// (US-180). The survey is presented on the Stripe Portal cancel return URL;
// this validates the reason against the whitelist and stores it.
type SubmitCancellationSurveyUseCase struct {
	repo domain.CancellationSurveyRepository
	now  func() time.Time
}

func NewSubmitCancellationSurveyUseCase(repo domain.CancellationSurveyRepository) *SubmitCancellationSurveyUseCase {
	return &SubmitCancellationSurveyUseCase{repo: repo, now: time.Now}
}

// Execute validates the structured reason and persists the response scoped to
// the authenticated account + user.
func (uc *SubmitCancellationSurveyUseCase) Execute(ctx context.Context, accountID, userID, reason, comment string) error {
	reason = strings.TrimSpace(reason)
	if !domain.IsValidCancellationReason(reason) {
		return domain.ErrInvalidCancellationReason
	}
	comment = strings.TrimSpace(comment)
	if len(comment) > cancellationCommentMaxLen {
		comment = comment[:cancellationCommentMaxLen]
	}
	return uc.repo.Create(ctx, &domain.CancellationSurvey{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		UserID:    userID,
		Reason:    reason,
		Comment:   comment,
		CreatedAt: uc.now(),
	})
}

// CancellationSurveyMetricsResult is the admin churn-breakdown payload
// (US-180): the per-reason counts over the requested window plus a handful of
// recent verbatims.
type CancellationSurveyMetricsResult struct {
	WindowDays int                                `json:"window_days"`
	Total      int                                `json:"total"`
	Breakdown  []domain.CancellationReasonCount   `json:"breakdown"`
	Recent     []domain.CancellationSurveyComment `json:"recent_comments"`
}

// CancellationSurveyMetricsUseCase computes the churn-reason breakdown and
// recent comments for the admin dashboard (US-180).
type CancellationSurveyMetricsUseCase struct {
	repo domain.CancellationSurveyRepository
	now  func() time.Time
}

func NewCancellationSurveyMetricsUseCase(repo domain.CancellationSurveyRepository) *CancellationSurveyMetricsUseCase {
	return &CancellationSurveyMetricsUseCase{repo: repo, now: time.Now}
}

// Execute aggregates surveys over the trailing windowDays (defaults to 90 —
// one quarter — when non-positive) and returns up to 50 recent comments.
func (uc *CancellationSurveyMetricsUseCase) Execute(ctx context.Context, windowDays int) (CancellationSurveyMetricsResult, error) {
	if windowDays <= 0 {
		windowDays = 90
	}
	since := uc.now().AddDate(0, 0, -windowDays)

	breakdown, err := uc.repo.Breakdown(ctx, since)
	if err != nil {
		return CancellationSurveyMetricsResult{}, err
	}
	recent, err := uc.repo.RecentComments(ctx, since, 50)
	if err != nil {
		return CancellationSurveyMetricsResult{}, err
	}
	total := 0
	for _, b := range breakdown {
		total += b.Count
	}
	return CancellationSurveyMetricsResult{
		WindowDays: windowDays,
		Total:      total,
		Breakdown:  breakdown,
		Recent:     recent,
	}, nil
}
