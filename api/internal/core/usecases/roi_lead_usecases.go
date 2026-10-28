package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// roiLeadRateWindow is the minimum interval allowed between ROI-calculator
// submissions for the same email address (1 submission / email / 24h).
const roiLeadRateWindow = 24 * time.Hour

// Savings model constants for the ROI calculator (US-163). The model is
// deliberately transparent so the marketing copy can state the assumptions:
// an AI agent deflects a share of inbound messages, each of which would
// otherwise cost an attendant a few minutes of handling time.
const (
	// roiAutomationRate is the fraction of inbound messages the AI agent
	// resolves without human help.
	roiAutomationRate = 0.7
	// roiMinutesPerMessage is the average attendant handling time per message.
	roiMinutesPerMessage = 3.0
)

// EstimateROISavings returns the estimated monthly labour savings (in the
// caller's currency) from deflecting messagesPerMonth at the given hourlyRate.
// Exported so the handler and tests can assert the canonical figure.
func EstimateROISavings(messagesPerMonth int, hourlyRate float64) float64 {
	if messagesPerMonth <= 0 || hourlyRate <= 0 {
		return 0
	}
	hoursSaved := float64(messagesPerMonth) * roiAutomationRate * (roiMinutesPerMessage / 60.0)
	return hoursSaved * hourlyRate
}

// SubmitROILeadInput is the validated payload for an ROI-calculator submission.
type SubmitROILeadInput struct {
	Email            string
	MessagesPerMonth int
	CurrentCost      float64
	HourlyRate       float64
	// Honeypot is the hidden "website" form field. Legitimate users never
	// fill it; any non-empty value means a bot submitted the form.
	Honeypot string
}

// SubmitROILeadUseCase captures a self-qualified lead from the public ROI
// calculator and stores it with the savings estimate its inputs produced
// (US-163).
type SubmitROILeadUseCase struct {
	repo domain.ROILeadRepository
	now  func() time.Time
}

func NewSubmitROILeadUseCase(repo domain.ROILeadRepository) *SubmitROILeadUseCase {
	return &SubmitROILeadUseCase{repo: repo, now: time.Now}
}

// Execute validates and persists an ROI-calculator lead, returning the
// estimated monthly savings so the caller can reveal it once the email gate is
// cleared.
//
// The honeypot short-circuits to ErrSpamDetected before any DB work (the
// handler maps that to a normal 200 so the trap is undetectable). The same
// email may only submit once per roiLeadRateWindow (ErrTooManyRequests). The
// savings figure is recomputed server-side so a tampered client payload can't
// poison the stored estimate.
func (uc *SubmitROILeadUseCase) Execute(ctx context.Context, in SubmitROILeadInput) (float64, error) {
	if strings.TrimSpace(in.Honeypot) != "" {
		return 0, domain.ErrSpamDetected
	}

	email := strings.TrimSpace(strings.ToLower(in.Email))
	if !emailRegex.MatchString(email) {
		return 0, domain.ErrInvalidEmail
	}

	if in.MessagesPerMonth <= 0 || in.CurrentCost <= 0 || in.HourlyRate <= 0 {
		return 0, domain.ErrMissingROIFields
	}

	recent, err := uc.repo.ExistsRecent(ctx, email, uc.now().Add(-roiLeadRateWindow))
	if err != nil {
		return 0, err
	}
	if recent {
		return 0, domain.ErrTooManyRequests
	}

	savings := EstimateROISavings(in.MessagesPerMonth, in.HourlyRate)

	lead := &domain.ROILead{
		ID:               ulid.Make().String(),
		Email:            email,
		MessagesPerMonth: in.MessagesPerMonth,
		CurrentCost:      in.CurrentCost,
		HourlyRate:       in.HourlyRate,
		EstimatedSavings: savings,
		CreatedAt:        uc.now(),
	}

	if err := uc.repo.Create(ctx, lead); err != nil {
		return 0, err
	}

	return savings, nil
}
