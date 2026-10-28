package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ContractCalendarUseCase answers the one question a salesperson in this
// market cannot answer for themselves: what is about to be bought again.
//
// A public contract has a published end date. The purchase that replaces it is
// therefore a known event, months before any tender exists — and by the time
// an edital IS published the specification has usually been written around
// whoever supplied the reference quote. Arriving at the expiry date instead of
// at the publication date is the difference between shaping a spec and
// answering one.
//
// Nothing here is invented: every row is a contract the government published,
// with a link back to it.
type ContractCalendarUseCase struct {
	contracts domain.PublicContractRepository
	accounts  domain.AccountRepository
	// watches supplies the words. The calendar matches the SAME keywords the
	// radar matches, because two screens disagreeing about "is this mine?"
	// is how a feed stops being trusted.
	watches calendarWatchLister
	now     func() time.Time
}

type calendarWatchLister interface {
	ListWatches(ctx context.Context, accountID string) ([]*domain.SignalWatch, error)
}

func NewContractCalendarUseCase(
	contracts domain.PublicContractRepository,
	accounts domain.AccountRepository,
	watches calendarWatchLister,
) *ContractCalendarUseCase {
	return &ContractCalendarUseCase{
		contracts: contracts, accounts: accounts, watches: watches,
		now: time.Now,
	}
}

// CalendarWindowDays is how far ahead the calendar looks by default.
//
// Six months, because that is the head start the instrument actually gives: a
// replacement process for a contract ending in January starts being specified
// around October. Looking further returns rows nobody can act on yet; looking
// less throws away the whole advantage.
const CalendarWindowDays = 180

// calendarGraceDays keeps the recently-expired visible.
//
// A contract that ran out last week is not history — the buyer is very
// probably mid-process right now, and an extension or an emergency purchase is
// the most reachable moment there is.
const calendarGraceDays = 30

// ContractCalendarEntry is one buying window, in the words the screen shows.
type ContractCalendarEntry struct {
	PNCPID string `json:"pncp_id"`
	// URL is the public record, so every claim built on this can be checked
	// by the person who receives it.
	URL          string `json:"url"`
	BuyerName    string `json:"buyer_name"`
	BuyerCNPJ    string `json:"buyer_cnpj"`
	SupplierName string `json:"supplier_name,omitempty"`
	Object       string `json:"object"`
	ValueCents   int64  `json:"value_cents"`
	City         string `json:"city,omitempty"`
	UF           string `json:"uf,omitempty"`

	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// DaysLeft is negative for a window that has just closed. Null never
	// happens here — the query only returns dated rows — but the pointer
	// keeps the contract honest if that ever changes.
	DaysLeft *int `json:"days_left,omitempty"`

	// PriceRegistry says this was bought under an ata: a twelve-month licence
	// to sell at a registered price, not a single closed purchase. Worth more
	// to a supplier than the contract itself.
	PriceRegistry bool `json:"price_registry"`
	// RidesOthers is PNCP's `frutoAdesao` — this buyer took a ride on somebody
	// else's registry. The most actionable bit in the record: a buyer who
	// rides can buy from you without running a tender at all.
	RidesOthers bool `json:"rides_others"`
}

// ContractCalendar is the answer plus what it was narrowed by, so the screen
// can explain an empty list instead of just showing one.
type ContractCalendar struct {
	Entries []ContractCalendarEntry `json:"entries"`
	// WindowDays is how far ahead this looked.
	WindowDays int `json:"window_days"`
	// States and Keywords are what the account is watching. An empty list with
	// no keywords is a configuration answer, not "nothing is happening".
	States   []string `json:"states"`
	Keywords []string `json:"keywords"`
	// Scanning is true while the sweep has not covered this territory yet —
	// an empty calendar on day one means "still reading", not "nothing found",
	// and a screen that cannot tell those apart teaches people to distrust it.
	Scanning bool `json:"scanning"`
}

// Execute builds the calendar for one account.
func (uc *ContractCalendarUseCase) Execute(ctx context.Context, accountID string, windowDays int) (*ContractCalendar, error) {
	if windowDays <= 0 || windowDays > 720 {
		windowDays = CalendarWindowDays
	}
	out := &ContractCalendar{
		Entries: []ContractCalendarEntry{}, WindowDays: windowDays,
		States: []string{}, Keywords: []string{},
	}

	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}
	if account.Territory == nil {
		// No territory, no calendar. Honest empty: the screen says where to
		// set it rather than implying the state has no contracts.
		return out, nil
	}
	uf := domain.NormalizeUF(account.Territory.UF)
	if uf == "" {
		return out, nil
	}
	out.States = []string{uf}

	keywords := uc.keywordsFor(ctx, accountID)
	out.Keywords = keywords
	if len(keywords) == 0 {
		return out, nil
	}

	now := uc.now()
	rows, err := uc.contracts.ListExpiring(ctx, domain.ContractCalendarFilter{
		States:   out.States,
		Keywords: keywords,
		// The window opens BEHIND today on purpose: see calendarGraceDays.
		From:  now.AddDate(0, 0, -calendarGraceDays),
		To:    now.AddDate(0, 0, windowDays),
		Limit: 60,
	})
	if err != nil {
		return nil, err
	}
	for _, c := range rows {
		out.Entries = append(out.Entries, ContractCalendarEntry{
			PNCPID:        c.PNCPID,
			URL:           c.URL(),
			BuyerName:     c.BuyerName,
			BuyerCNPJ:     c.BuyerCNPJ,
			SupplierName:  c.SupplierName,
			Object:        c.Object,
			ValueCents:    c.ValueCents,
			City:          c.City,
			UF:            c.UF,
			ExpiresAt:     c.ExpiresAt,
			DaysLeft:      c.DaysUntilExpiry(now),
			PriceRegistry: c.IsPriceRegistry(),
			RidesOthers:   c.FromAdhesion,
		})
	}
	// "Still reading" rather than "nothing found". The sweep covers a state
	// over days, so an empty answer in week one is a progress report.
	out.Scanning = len(out.Entries) == 0
	return out, nil
}

// keywordsFor reuses the account's radar words.
//
// Deliberately NOT a second list to configure. The account already said what
// it sells once — asking again, in another screen, is how two lists drift and
// one of them silently stops matching.
func (uc *ContractCalendarUseCase) keywordsFor(ctx context.Context, accountID string) []string {
	if uc.watches == nil {
		return nil
	}
	watches, err := uc.watches.ListWatches(ctx, accountID)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, 32)
	for _, w := range watches {
		if w == nil || !w.IsActive {
			continue
		}
		for _, kw := range w.Keywords {
			k := strings.TrimSpace(kw)
			if k == "" || seen[strings.ToLower(k)] {
				continue
			}
			seen[strings.ToLower(k)] = true
			out = append(out, k)
		}
	}
	return out
}
