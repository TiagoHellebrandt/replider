package usecases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// salesLeadRateWindow is the minimum interval allowed between sales-lead
// submissions for the same email address (1 submission / email / 24h).
const salesLeadRateWindow = 24 * time.Hour

// SalesLeadNotifier is the narrow port the sales use case needs: route a
// qualification lead to the sales inbox. The concrete *email.SMTPAdapter
// satisfies it. Kept narrow (not the full EmailService) so the lead-capture
// flow doesn't depend on the dozen customer-facing email composers.
type SalesLeadNotifier interface {
	SendSalesLeadEmail(to string, lead domain.SalesLead) error
}

// SubmitSalesLeadInput is the validated payload for a qualification request.
type SubmitSalesLeadInput struct {
	Company       string
	Email         string
	CompanySize   string
	Channels      string
	MonthlyVolume string
	Message       string
	// Honeypot is the hidden "website" form field. Legitimate users never
	// fill it; any non-empty value means a bot submitted the form.
	Honeypot string
}

// SubmitSalesLeadUseCase captures an Enterprise-tier qualification lead and
// routes a notification email to the sales inbox (US-157).
type SubmitSalesLeadUseCase struct {
	repo     domain.SalesLeadRepository
	notifier SalesLeadNotifier
	salesTo  string
	now      func() time.Time
}

func NewSubmitSalesLeadUseCase(repo domain.SalesLeadRepository, notifier SalesLeadNotifier, salesTo string) *SubmitSalesLeadUseCase {
	return &SubmitSalesLeadUseCase{repo: repo, notifier: notifier, salesTo: salesTo, now: time.Now}
}

// Execute validates, persists, and routes a sales-qualification lead.
//
// The honeypot short-circuits to ErrSpamDetected before any DB work (the
// handler maps that to a normal 200 so the trap is undetectable). The same
// email may only submit once per salesLeadRateWindow (ErrTooManyRequests).
//
// Email delivery is best-effort: the lead is the durable record, so a failed
// SMTP send is logged but does NOT fail the request — sales can still recover
// the lead from the table.
func (uc *SubmitSalesLeadUseCase) Execute(ctx context.Context, in SubmitSalesLeadInput) error {
	if strings.TrimSpace(in.Honeypot) != "" {
		return domain.ErrSpamDetected
	}

	email := strings.TrimSpace(strings.ToLower(in.Email))
	if !emailRegex.MatchString(email) {
		return domain.ErrInvalidEmail
	}

	company := strings.TrimSpace(in.Company)
	size := strings.TrimSpace(in.CompanySize)
	channels := strings.TrimSpace(in.Channels)
	volume := strings.TrimSpace(in.MonthlyVolume)
	if company == "" || size == "" || channels == "" || volume == "" {
		return domain.ErrMissingSalesFields
	}

	recent, err := uc.repo.ExistsRecent(ctx, email, uc.now().Add(-salesLeadRateWindow))
	if err != nil {
		return err
	}
	if recent {
		return domain.ErrTooManyRequests
	}

	lead := domain.SalesLead{
		ID:            ulid.Make().String(),
		Company:       company,
		Email:         email,
		CompanySize:   size,
		Channels:      channels,
		MonthlyVolume: volume,
		Message:       strings.TrimSpace(in.Message),
		CreatedAt:     uc.now(),
	}

	if err := uc.repo.Create(ctx, &lead); err != nil {
		return err
	}

	// Best-effort routing — the persisted lead is the source of truth.
	if uc.notifier != nil && uc.salesTo != "" {
		if err := uc.notifier.SendSalesLeadEmail(uc.salesTo, lead); err != nil {
			slog.Error("failed to route sales lead email", "error", err, "lead_id", lead.ID)
		}
	}

	return nil
}
