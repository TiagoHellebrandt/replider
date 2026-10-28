package calendar

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/replider/api/internal/core/domain"
)

// AccountCalendarAdapter implements domain.CalendarAdapter with
// per-account credential resolution: an account that connected its own
// Google Calendar (OAuth) gets API calls under its own token; everyone
// else falls back to the platform service account, preserving the
// pre-OAuth behavior (calendar shared with the SA e-mail).
type AccountCalendarAdapter struct {
	fallback     *GoogleCalendarAdapter
	integrations domain.GoogleCalendarIntegrationRepository // nil-able
	oauth        *GoogleOAuth                               // nil-able (feature not configured)
}

func NewAccountCalendarAdapter(fallback *GoogleCalendarAdapter, integrations domain.GoogleCalendarIntegrationRepository, oauth *GoogleOAuth) *AccountCalendarAdapter {
	return &AccountCalendarAdapter{fallback: fallback, integrations: integrations, oauth: oauth}
}

// resolveService returns an OAuth-authenticated client for accounts
// with an integration, or nil when the account should use the
// service-account fallback. Resolution errors degrade to the fallback
// (logged) — an availability lookup must not hard-fail on a repo blip.
func (a *AccountCalendarAdapter) resolveService(ctx context.Context, accountID string) *calendar.Service {
	if a.oauth == nil || a.integrations == nil {
		return nil
	}
	integ, err := a.integrations.Get(ctx, accountID)
	if err != nil {
		slog.Error("gcal integration lookup failed; falling back to service account", "account_id", accountID, "error", err)
		return nil
	}
	if integ == nil {
		return nil
	}
	srv, err := a.oauth.serviceForToken(ctx, integ.RefreshToken)
	if err != nil {
		slog.Error("gcal oauth client build failed; falling back to service account", "account_id", accountID, "error", err)
		return nil
	}
	return srv
}

func (a *AccountCalendarAdapter) ListFreeSlots(ctx context.Context, accountID, calendarID string, date time.Time) ([]time.Time, error) {
	if srv := a.resolveService(ctx, accountID); srv != nil {
		return listFreeSlotsWith(srv, calendarID, date)
	}
	return a.fallback.ListFreeSlots(ctx, calendarID, date)
}

func (a *AccountCalendarAdapter) CreateAppointment(ctx context.Context, accountID, calendarID string, start, end time.Time, attendeeEmail string) error {
	if srv := a.resolveService(ctx, accountID); srv != nil {
		return createAppointmentWith(srv, calendarID, start, end, attendeeEmail)
	}
	return a.fallback.CreateAppointment(ctx, calendarID, start, end, attendeeEmail)
}
