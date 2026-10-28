package usecases

import (
	"context"
	"sort"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Health-score component weights (sum = 100). The split favours
// engagement (messages) while still rewarding a working bot, low human
// takeover, and a recently-active admin.
const (
	healthWeightMessages     = 40
	healthWeightBotActive    = 20
	healthWeightTakeover     = 20
	healthWeightLoginRecency = 20

	// healthMessagesFullScore is the weekly bot-reply volume that earns the
	// full messages weight; volume above it doesn't add more.
	healthMessagesFullScore = 50

	// Login-recency decay bounds: seen within Full days = full weight, not
	// seen for Zero+ days (or never) = zero, linear in between.
	healthLoginRecencyFullDays = 2
	healthLoginRecencyZeroDays = 14

	// HealthDropAlertThreshold is the week-over-week percentage drop that
	// flags an account at risk (AC: "Alert when drops 30%+ wk-over-wk").
	HealthDropAlertThreshold = 30
)

// AccountHealthUseCase computes a retention health score per account from
// the raw activity sweep (US-177). The repository does the SQL; this use
// case turns the counts into a 0-100 score and the week-over-week drop
// alert, so the arithmetic stays unit-testable without a DB.
type AccountHealthUseCase struct {
	repo domain.AccountHealthRepository
	now  func() time.Time
}

func NewAccountHealthUseCase(repo domain.AccountHealthRepository) *AccountHealthUseCase {
	return &AccountHealthUseCase{repo: repo, now: time.Now}
}

// Execute returns every account's health score, worst (and at-risk)
// first so the admin column surfaces churn candidates at the top.
func (uc *AccountHealthUseCase) Execute(ctx context.Context) ([]domain.AccountHealthScore, error) {
	now := uc.now()
	metrics, err := uc.repo.ListAccountHealthMetrics(ctx, now)
	if err != nil {
		return nil, err
	}
	scores := make([]domain.AccountHealthScore, 0, len(metrics))
	for _, m := range metrics {
		scores = append(scores, scoreAccount(m, now))
	}
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].AtRisk != scores[j].AtRisk {
			return scores[i].AtRisk // at-risk accounts float to the top
		}
		return scores[i].Score < scores[j].Score
	})
	return scores, nil
}

// scoreAccount derives the current and previous-week scores and the drop
// alert for one account. Bot-active and login-recency are point-in-time
// (no historical snapshot), so they contribute equally to both weeks and
// the week-over-week delta is driven by message volume and takeover —
// exactly the engagement signals the alert cares about.
func scoreAccount(m domain.AccountHealthMetrics, now time.Time) domain.AccountHealthScore {
	bd, score := healthScore(m.MessagesThisWeek, m.ConversationsThisWeek, m.TakenOverThisWeek, m.HasActiveAgent, m.LastSeenAt, now)
	_, prev := healthScore(m.MessagesPrevWeek, m.ConversationsPrevWeek, m.TakenOverPrevWeek, m.HasActiveAgent, m.LastSeenAt, now)

	drop := 0
	if prev > 0 && score < prev {
		drop = (prev - score) * 100 / prev
	}

	res := domain.AccountHealthScore{
		AccountID:        m.AccountID,
		Name:             m.Name,
		PlanID:           m.PlanID,
		Score:            score,
		PrevScore:        prev,
		DropPct:          drop,
		AtRisk:           drop >= HealthDropAlertThreshold,
		Breakdown:        bd,
		MessagesThisWeek: m.MessagesThisWeek,
		HasActiveAgent:   m.HasActiveAgent,
	}
	if m.ConversationsThisWeek > 0 {
		res.TakeoverPct = m.TakenOverThisWeek * 100 / m.ConversationsThisWeek
	}
	if m.LastSeenAt != nil {
		d := int(now.Sub(*m.LastSeenAt).Hours() / 24)
		if d < 0 {
			d = 0
		}
		res.DaysSinceLogin = &d
	}
	return res
}

// healthScore computes the weighted breakdown and total (0-100) for a
// single week's inputs.
func healthScore(msgs, convs, takenOver int, hasAgent bool, lastSeen *time.Time, now time.Time) (domain.AccountHealthBreakdown, int) {
	var bd domain.AccountHealthBreakdown

	// Messages: linear up to the full-score volume.
	if msgs > 0 {
		capped := msgs
		if capped > healthMessagesFullScore {
			capped = healthMessagesFullScore
		}
		bd.Messages = capped * healthWeightMessages / healthMessagesFullScore
	}

	// Bot active: all-or-nothing.
	if hasAgent {
		bd.BotActive = healthWeightBotActive
	}

	// Takeover: full weight at 0% takeover, scaling to 0 at 100%. No
	// conversations = no negative signal, so award full weight.
	if convs > 0 {
		retained := convs - takenOver
		if retained < 0 {
			retained = 0
		}
		bd.Takeover = retained * healthWeightTakeover / convs
	} else {
		bd.Takeover = healthWeightTakeover
	}

	// Login recency: full weight if seen within Full days, decaying
	// linearly to zero at Zero days; never seen = zero.
	if lastSeen != nil {
		days := now.Sub(*lastSeen).Hours() / 24
		switch {
		case days <= healthLoginRecencyFullDays:
			bd.LoginRecency = healthWeightLoginRecency
		case days >= healthLoginRecencyZeroDays:
			bd.LoginRecency = 0
		default:
			span := float64(healthLoginRecencyZeroDays - healthLoginRecencyFullDays)
			frac := (float64(healthLoginRecencyZeroDays) - days) / span
			bd.LoginRecency = int(frac * float64(healthWeightLoginRecency))
		}
	}

	return bd, bd.Messages + bd.BotActive + bd.Takeover + bd.LoginRecency
}
