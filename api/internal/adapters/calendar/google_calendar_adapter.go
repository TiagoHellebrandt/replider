package calendar

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleCalendarAdapter reads availability and books events with the
// PLATFORM service account (GOOGLE_CREDENTIALS_BASE64). It serves as
// the fallback for accounts that haven't connected their own Google
// account via OAuth (see AccountCalendarAdapter).
type GoogleCalendarAdapter struct {
	credentialsBase64 string
}

// NewGoogleCalendarAdapter creates a new Google Calendar adapter
func NewGoogleCalendarAdapter(credentialsBase64 string) *GoogleCalendarAdapter {
	return &GoogleCalendarAdapter{
		credentialsBase64: credentialsBase64,
	}
}

func (a *GoogleCalendarAdapter) service(ctx context.Context, scope string) (*calendar.Service, error) {
	if a.credentialsBase64 == "" {
		return nil, fmt.Errorf("Google Calendar credentials not configured")
	}
	decodedJSON, err := base64.StdEncoding.DecodeString(a.credentialsBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Google Calendar credentials: %w", err)
	}
	srv, err := calendar.NewService(ctx, option.WithCredentialsJSON(decodedJSON), option.WithScopes(scope))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %w", err)
	}
	return srv, nil
}

// ListFreeSlots finds available time slots on a given date for a specific Google Calendar.
// The calendarID should be a Google Calendar email/ID (e.g., "doctor.a@gmail.com").
func (a *GoogleCalendarAdapter) ListFreeSlots(ctx context.Context, calendarID string, date time.Time) ([]time.Time, error) {
	srv, err := a.service(ctx, calendar.CalendarEventsReadonlyScope)
	if err != nil {
		return nil, err
	}
	return listFreeSlotsWith(srv, calendarID, date)
}

// CreateAppointment books an event in the calendar
func (a *GoogleCalendarAdapter) CreateAppointment(ctx context.Context, calendarID string, start, end time.Time, attendeeEmail string) error {
	srv, err := a.service(ctx, calendar.CalendarEventsScope)
	if err != nil {
		return err
	}
	return createAppointmentWith(srv, calendarID, start, end, attendeeEmail)
}

// listFreeSlotsWith is the shared availability logic, used by both the
// service-account path and the per-account OAuth path.
//
// Simplified slot model: business hours 09:00–18:00 in the requested
// date's location, 1-hour gaps between existing events.
func listFreeSlotsWith(srv *calendar.Service, calendarID string, date time.Time) ([]time.Time, error) {
	dateUTC := date.UTC()
	startOfDay := time.Date(dateUTC.Year(), dateUTC.Month(), dateUTC.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	endOfDay := time.Date(dateUTC.Year(), dateUTC.Month(), dateUTC.Day(), 23, 59, 59, 0, time.UTC).Format(time.RFC3339)

	events, err := srv.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(startOfDay).
		TimeMax(endOfDay).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list calendar events: %w", err)
	}

	var freeSlots []time.Time

	loc := date.Location()
	current := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, loc)
	endWork := time.Date(date.Year(), date.Month(), date.Day(), 18, 0, 0, 0, loc)

	for current.Before(endWork) {
		slotEnd := current.Add(1 * time.Hour)
		isFree := true
		for _, event := range events.Items {
			if event.Start.DateTime == "" || event.End.DateTime == "" {
				continue
			}
			eStart, _ := time.Parse(time.RFC3339, event.Start.DateTime)
			eEnd, _ := time.Parse(time.RFC3339, event.End.DateTime)

			// Overlap check
			if current.Before(eEnd) && slotEnd.After(eStart) {
				isFree = false
				break
			}
		}

		if isFree {
			freeSlots = append(freeSlots, current)
		}
		current = slotEnd
	}

	return freeSlots, nil
}

func createAppointmentWith(srv *calendar.Service, calendarID string, start, end time.Time, attendeeEmail string) error {
	event := &calendar.Event{
		Summary:     "Replider Appointment",
		Location:    "Online",
		Description: "Scheduled via Replider AI",
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: start.Location().String(),
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: end.Location().String(),
		},
		Attendees: []*calendar.EventAttendee{
			{Email: attendeeEmail},
		},
	}

	_, err := srv.Events.Insert(calendarID, event).Do()
	if err != nil {
		return fmt.Errorf("unable to create event: %w", err)
	}

	return nil
}
