package usecases

import (
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Per-account daily cap on Google Places searches (PLACES_DAILY_CAP_PER_ACCOUNT).
//
// Every search is a paid request on the PLATFORM's Places key — there is no
// BYOK for it — and nothing bounded how many an account could fire: the
// preview endpoint runs a full search per keystroke-and-enter, and the plan's
// lead cap only limits what gets IMPORTED, not what gets looked up. This is a
// platform cost rail, so the day is UTC (not the account's timezone — the
// bill it protects is ours), and the counter is in-process: a restart resets
// it, which errs toward the customer, and a Places bill is bounded by the
// deploy cadence in the worst case rather than by nothing.

// ErrPlacesDailyCapExceeded refuses a search past the account's daily cap.
var ErrPlacesDailyCapExceeded = &domain.AppError{
	Code:    "PLACES_DAILY_CAP_EXCEEDED",
	Message: "limite diário de buscas de prospecção atingido — tente novamente amanhã",
}

// DefaultPlacesDailyCap applies when the config value is unset.
const DefaultPlacesDailyCap = 50

// placesDailyCounter counts searches per account per UTC day.
type placesDailyCounter struct {
	mu     sync.Mutex
	day    string
	counts map[string]int
	now    func() time.Time
}

func newPlacesDailyCounter() *placesDailyCounter {
	return &placesDailyCounter{counts: map[string]int{}, now: time.Now}
}

// take consumes one search for the account, refusing once `cap` is reached
// today. cap <= 0 means no cap.
func (c *placesDailyCounter) take(accountID string, cap int) error {
	if c == nil || cap <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	today := c.now().UTC().Format("2006-01-02")
	if c.day != today {
		c.day = today
		c.counts = map[string]int{}
	}
	if c.counts[accountID] >= cap {
		return ErrPlacesDailyCapExceeded
	}
	c.counts[accountID]++
	return nil
}

// SetPlacesDailyCap wires the per-account daily Places cap (<= 0 disables).
func (uc *ProspectingUseCase) SetPlacesDailyCap(cap int) {
	uc.placesDailyCap = cap
	if uc.placesQuota == nil {
		uc.placesQuota = newPlacesDailyCounter()
	}
}
