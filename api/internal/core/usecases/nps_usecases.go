package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// npsCommentMaxLen caps the free-text comment so a runaway client can't
// store unbounded text.
const npsCommentMaxLen = 2000

// npsMinAccountAge is how long an account must exist before it's eligible
// for the in-app NPS prompt — so a brand-new user is never asked to rate
// the product on their first login.
const npsMinAccountAge = 7 * 24 * time.Hour

// npsQuarter formats a time as the calendar-quarter key (e.g. "2026-Q2").
// MUST match the Flutter client's quarter format so a quarter stamped by
// either side lines up.
func npsQuarter(t time.Time) string {
	q := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("%d-Q%d", t.Year(), q)
}

// SubmitNPSUseCase persists an in-app NPS response (US-178) and stamps the
// account's quarter so the prompt is suppressed until next quarter.
type SubmitNPSUseCase struct {
	repo domain.NPSRepository
	now  func() time.Time
}

func NewSubmitNPSUseCase(repo domain.NPSRepository) *SubmitNPSUseCase {
	return &SubmitNPSUseCase{repo: repo, now: time.Now}
}

// Execute validates the 0-10 score and persists the response scoped to the
// authenticated account + user, then stamps the NPS prompt as handled for
// the current quarter.
func (uc *SubmitNPSUseCase) Execute(ctx context.Context, accountID, userID string, score int, comment string) error {
	if score < 0 || score > 10 {
		return domain.ErrInvalidNPSScore
	}
	comment = strings.TrimSpace(comment)
	if len(comment) > npsCommentMaxLen {
		comment = comment[:npsCommentMaxLen]
	}
	now := uc.now()
	if err := uc.repo.Create(ctx, &domain.NPSResponse{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		UserID:    userID,
		Score:     score,
		Comment:   comment,
		CreatedAt: now,
	}); err != nil {
		return err
	}
	// Best-effort cadence stamp — a failure here only risks re-prompting,
	// never loses the response, so don't fail the submit on it.
	_ = uc.repo.StampPromptHandled(ctx, accountID, npsQuarter(now))
	return nil
}

// NPSPromptUseCase backs the server-side "show the NPS prompt?" decision and
// the dismiss action (US-178 follow-up), replacing the old client-only
// FlutterSecureStorage cadence flag.
type NPSPromptUseCase struct {
	repo domain.NPSRepository
	now  func() time.Time
}

func NewNPSPromptUseCase(repo domain.NPSRepository) *NPSPromptUseCase {
	return &NPSPromptUseCase{repo: repo, now: time.Now}
}

// Eligible reports whether the in-app NPS prompt should be shown to the
// account: account older than npsMinAccountAge AND not handled this quarter.
func (uc *NPSPromptUseCase) Eligible(ctx context.Context, accountID string) (bool, error) {
	now := uc.now()
	return uc.repo.IsPromptEligible(ctx, accountID, npsQuarter(now), now.Add(-npsMinAccountAge))
}

// Dismiss records a "maybe later" so the prompt stays hidden until next
// quarter without recording a score.
func (uc *NPSPromptUseCase) Dismiss(ctx context.Context, accountID string) error {
	return uc.repo.StampPromptHandled(ctx, accountID, npsQuarter(uc.now()))
}

// NPSMetricsResult is the admin NPS dashboard payload (US-178): the summary
// over the requested window plus a handful of recent verbatims.
type NPSMetricsResult struct {
	WindowDays int                 `json:"window_days"`
	Summary    domain.NPSSummary   `json:"summary"`
	Recent     []domain.NPSComment `json:"recent_comments"`
}

// NPSMetricsUseCase computes the aggregated NPS and recent comments for the
// admin dashboard (US-178).
type NPSMetricsUseCase struct {
	repo domain.NPSRepository
	now  func() time.Time
}

func NewNPSMetricsUseCase(repo domain.NPSRepository) *NPSMetricsUseCase {
	return &NPSMetricsUseCase{repo: repo, now: time.Now}
}

// Execute aggregates responses over the trailing windowDays (defaults to 90 —
// one quarter — when non-positive) and returns up to 50 recent comments.
func (uc *NPSMetricsUseCase) Execute(ctx context.Context, windowDays int) (NPSMetricsResult, error) {
	if windowDays <= 0 {
		windowDays = 90
	}
	since := uc.now().AddDate(0, 0, -windowDays)

	summary, err := uc.repo.Summarize(ctx, since)
	if err != nil {
		return NPSMetricsResult{}, err
	}
	recent, err := uc.repo.RecentComments(ctx, since, 50)
	if err != nil {
		return NPSMetricsResult{}, err
	}
	return NPSMetricsResult{
		WindowDays: windowDays,
		Summary:    summary,
		Recent:     recent,
	}, nil
}
