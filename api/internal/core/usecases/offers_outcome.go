package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Weekly outcome line for the Ofertas screen (Hoje, phase 1).
//
// The queue shows input (40 waiting) and never output, so approving is an
// act of faith. This answers the only question that justifies the ritual:
// did the sends turn into anything this week?
type OffersOutcome struct {
	// WeekStart is Monday 00:00 in the account's timezone.
	WeekStart time.Time `json:"week_start"`
	Sent      int64     `json:"sent"`
	Replies   int64     `json:"replies"`
	Proposals int64     `json:"proposals"`
}

// ProposalCounter is the one slice of the proposal repository this needs.
type ProposalCounter interface {
	CountCreatedSince(ctx context.Context, accountID string, since time.Time) (int64, error)
}

type OffersOutcomeUseCase struct {
	events      domain.SalesEventRepository
	proposals   ProposalCounter
	accountRepo domain.AccountRepository
	now         func() time.Time
}

func NewOffersOutcomeUseCase(events domain.SalesEventRepository, proposals ProposalCounter, accounts domain.AccountRepository) *OffersOutcomeUseCase {
	return &OffersOutcomeUseCase{events: events, proposals: proposals, accountRepo: accounts, now: time.Now}
}

// WithClock pins the clock (tests).
func (uc *OffersOutcomeUseCase) WithClock(now func() time.Time) *OffersOutcomeUseCase {
	uc.now = now
	return uc
}

// Execute counts the current week, Monday-anchored in the account's
// timezone — "esta semana" on the screen must mean the seller's week, not
// UTC's (a Sunday-evening send in Cuiabá is still this week there).
func (uc *OffersOutcomeUseCase) Execute(ctx context.Context, accountID string) (*OffersOutcome, error) {
	loc := time.UTC
	if account, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && account != nil && account.Timezone != "" {
		if l, err := time.LoadLocation(account.Timezone); err == nil {
			loc = l
		}
	}
	now := uc.now().In(loc)
	// Monday 00:00 local. Go's Weekday has Sunday=0; shift so Monday=0.
	daysSinceMonday := (int(now.Weekday()) + 6) % 7
	weekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).
		AddDate(0, 0, -daysSinceMonday)

	out := &OffersOutcome{WeekStart: weekStart}
	var err error
	if out.Sent, err = uc.events.CountSince(ctx, accountID, domain.SalesEventApproachSent, weekStart); err != nil {
		return nil, err
	}
	if out.Replies, err = uc.events.CountSince(ctx, accountID, domain.SalesEventReplyReceived, weekStart); err != nil {
		return nil, err
	}
	if uc.proposals != nil {
		if out.Proposals, err = uc.proposals.CountCreatedSince(ctx, accountID, weekStart); err != nil {
			// The two counts above still answer the screen's question; a
			// broken proposal count degrades to zero rather than to no line.
			out.Proposals = 0
		}
	}
	return out, nil
}
