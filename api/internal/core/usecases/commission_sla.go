package usecases

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// US-009 (vendedores v3): the mechanical SLA ladder. The metric is the
// % of the seller's ASSIGNED platform-account conversations in the
// cycle whose first HUMAN response took more than the threshold in
// BUSINESS time (default 4h inside the 8h-18h America/Cuiaba window,
// weekends excluded; all owner-editable). Breach when that % exceeds
// the configured cut (default 20%). The ladder is enforced by
// ComputeCycle with zero judgment calls:
//
//	1st breach cycle        -> warning            (ledger untouched)
//	2nd consecutive breach  -> profit share at 20% (reason 'sla_reduced')
//	3rd+ consecutive breach -> flag_reallocation  (owner acts manually;
//	                           the 20% reduction stays while the streak
//	                           lasts — nothing else is automatic)
//
// A breach-free cycle resets the counter. Detractor NPS is surfaced as
// an informational flag only and NEVER touches a rate.
const (
	// SLAReducedCommissionRate is the ladder's step-2 profit share. It
	// only ever lowers pay: a track rate already below it is kept.
	SLAReducedCommissionRate = 20.0

	DefaultSLAFirstResponseMinutes = 240
	DefaultSLABreachPct            = 20
	DefaultSLABusinessStartHour    = 8
	DefaultSLABusinessEndHour      = 18
	DefaultSLATimezone             = "America/Cuiaba"
)

// SLASettings is the resolved runtime SLA parameters.
type SLASettings struct {
	FirstResponseMinutes int64  `json:"first_response_minutes"`
	BreachPct            int64  `json:"breach_pct"`
	BusinessStartHour    int64  `json:"business_start_hour"`
	BusinessEndHour      int64  `json:"business_end_hour"`
	Timezone             string `json:"timezone"`

	loc *time.Location
}

func (uc *ComputeCommissionsUseCase) loadSLAConfig(ctx context.Context) (*SLASettings, error) {
	return loadSLASettings(ctx, uc.settings)
}

// loadSLASettings resolves the runtime SLA parameters from the
// platform-settings KV (nil falls back to every default) — shared by
// the payroll compute and the seller panel's live widget (US-012).
func loadSLASettings(ctx context.Context, settings domain.PlatformSettingRepository) (*SLASettings, error) {
	cfg := &SLASettings{Timezone: DefaultSLATimezone}
	var err error
	if cfg.FirstResponseMinutes, err = getIntSetting(ctx, settings, domain.SettingSLAFirstResponseMinutes, DefaultSLAFirstResponseMinutes); err != nil {
		return nil, err
	}
	if cfg.BreachPct, err = getIntSetting(ctx, settings, domain.SettingSLABreachPct, DefaultSLABreachPct); err != nil {
		return nil, err
	}
	if cfg.BusinessStartHour, err = getIntSetting(ctx, settings, domain.SettingSLABusinessStartHour, DefaultSLABusinessStartHour); err != nil {
		return nil, err
	}
	if cfg.BusinessEndHour, err = getIntSetting(ctx, settings, domain.SettingSLABusinessEndHour, DefaultSLABusinessEndHour); err != nil {
		return nil, err
	}
	if settings != nil {
		if tz, err := settings.GetSetting(ctx, domain.SettingSLATimezone); err != nil {
			return nil, err
		} else if tz != "" {
			cfg.Timezone = tz
		}
	}
	cfg.loc, err = time.LoadLocation(cfg.Timezone)
	if err != nil {
		// A corrupted tz setting must not stall the whole payroll run:
		// fall back to Cuiabá's fixed offset (no DST in Brazil since
		// 2019) and let the owner fix the setting.
		slog.Warn("invalid SLA timezone setting, using -04 fallback", "tz", cfg.Timezone)
		cfg.loc = time.FixedZone("-04", -4*3600)
	}
	return cfg, nil
}

// computeSLAStates measures the cycle for every seller with assigned
// platform-account conversations, walks the ladder against the
// PREVIOUS cycle's persisted state (so a recompute of the same cycle is
// deterministic) and upserts one row per seller. Returns the states
// keyed by seller for the rate decision. An unset PLATFORM_ACCOUNT_ID
// means no metric source — no states, ladder inert.
func (uc *ComputeCommissionsUseCase) computeSLAStates(ctx context.Context, cycle string, from, to time.Time) (map[string]*domain.SellerSLAState, error) {
	states := map[string]*domain.SellerSLAState{}
	if uc.platformAccountID == "" {
		return states, nil
	}
	cfg, err := uc.loadSLAConfig(ctx)
	if err != nil {
		return nil, err
	}

	convs, err := uc.repo.ListSLAConversations(ctx, uc.platformAccountID, from, to)
	if err != nil {
		return nil, err
	}

	// An open cycle ("prévia") measures unanswered conversations up to
	// now; a closed one up to the cycle's end.
	unansweredCutoff := uc.now()
	if to.Before(unansweredCutoff) {
		unansweredCutoff = to
	}

	totals := map[string]int{}
	breached := map[string]int{}
	for _, u := range measureSLAUnits(convs, cfg, unansweredCutoff) {
		totals[u.AssignedTo]++
		if u.Breached {
			breached[u.AssignedTo]++
		}
	}

	prevCycle := from.AddDate(0, -1, 0).Format("2006-01")
	for seller, total := range totals {
		s := &domain.SellerSLAState{
			ID:                    ulid.Make().String(),
			SellerUserID:          seller,
			Cycle:                 cycle,
			TotalConversations:    total,
			BreachedConversations: breached[seller],
		}
		// Strict >: exactly at the cut is NOT a breach.
		s.Breach = int64(s.BreachedConversations)*100 > int64(total)*cfg.BreachPct
		if s.Breach {
			s.ConsecutiveBreaches = 1
			if prev, err := uc.repo.GetSLAState(ctx, seller, prevCycle); err != nil {
				return nil, err
			} else if prev != nil && prev.Breach {
				s.ConsecutiveBreaches = prev.ConsecutiveBreaches + 1
			}
		}
		s.Action = slaActionForStreak(s.ConsecutiveBreaches)
		if err := uc.repo.UpsertSLAState(ctx, s); err != nil {
			return nil, err
		}
		states[seller] = s
	}
	return states, nil
}

// slaUnit is one first-response measurement unit of the SLA metric. A
// regular platform conversation is its own unit; ALL of a customer
// account's internal support threads (thread-per-user, canal-interno
// US-009) collapse into one unit per (assignee, customer account) so a
// many-user account never multiplies its weight in the ladder — two
// threads with one blown count as ONE breached unit, never two.
type slaUnit struct {
	AssignedTo string
	Breached   bool
	// BreachedThreads carries every blown conversation inside the unit
	// (a grouped account unit can hold several) so the seller panel can
	// still link each late thread individually.
	BreachedThreads []SLABreachedConversation
}

// measureSLAUnits measures each conversation against the business
// window and groups internal threads by (assignee, customer account).
// Shared by the payroll compute and the seller panel's live widget so
// the two can never disagree on the aggregation rule.
func measureSLAUnits(convs []domain.SLAConversation, cfg *SLASettings, unansweredCutoff time.Time) []slaUnit {
	units := []slaUnit{}
	grouped := map[string]*slaUnit{}
	groupOrder := []string{}
	for _, c := range convs {
		end := unansweredCutoff
		if c.FirstHumanResponseAt != nil {
			end = *c.FirstHumanResponseAt
		}
		minutes := businessMinutesBetween(c.CreatedAt, end, cfg)
		blown := minutes > cfg.FirstResponseMinutes
		var threads []SLABreachedConversation
		if blown {
			threads = []SLABreachedConversation{{
				ConversationID:       c.ConversationID,
				ContactName:          c.ContactName,
				CreatedAt:            c.CreatedAt,
				FirstHumanResponseAt: c.FirstHumanResponseAt,
				BusinessMinutes:      minutes,
			}}
		}
		if c.CustomerAccountID == "" {
			units = append(units, slaUnit{AssignedTo: c.AssignedTo, Breached: blown, BreachedThreads: threads})
			continue
		}
		key := c.AssignedTo + "|" + c.CustomerAccountID
		u, ok := grouped[key]
		if !ok {
			u = &slaUnit{AssignedTo: c.AssignedTo}
			grouped[key] = u
			groupOrder = append(groupOrder, key)
		}
		u.Breached = u.Breached || blown
		u.BreachedThreads = append(u.BreachedThreads, threads...)
	}
	for _, key := range groupOrder {
		units = append(units, *grouped[key])
	}
	return units
}

// slaActionForStreak maps a consecutive-breach count to the ladder
// step: 0 → none, 1 → warning, 2 → reduced, 3+ → flag_reallocation.
func slaActionForStreak(n int) string {
	switch {
	case n == 0:
		return domain.SLAActionNone
	case n == 1:
		return domain.SLAActionWarning
	case n == 2:
		return domain.SLAActionReduced
	default:
		return domain.SLAActionFlagReallocation
	}
}

// slaReduced reports whether the cycle's profit share drops to the
// reduced rate: 2+ consecutive breaches (step 3 keeps the reduction
// while flagging reallocation).
func slaReduced(s *domain.SellerSLAState) bool {
	return s != nil && s.Breach && s.ConsecutiveBreaches >= 2
}

// businessMinutesBetween counts whole minutes of business time between
// from and to: the daily window [start, end) hours in the configured
// timezone, weekends excluded.
func businessMinutesBetween(from, to time.Time, cfg *SLASettings) int64 {
	if !to.After(from) {
		return 0
	}
	from, to = from.In(cfg.loc), to.In(cfg.loc)
	var total int64
	for day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, cfg.loc); day.Before(to); day = day.AddDate(0, 0, 1) {
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		winStart := day.Add(time.Duration(cfg.BusinessStartHour) * time.Hour)
		winEnd := day.Add(time.Duration(cfg.BusinessEndHour) * time.Hour)
		s, e := winStart, winEnd
		if from.After(s) {
			s = from
		}
		if to.Before(e) {
			e = to
		}
		if e.After(s) {
			total += int64(e.Sub(s) / time.Minute)
		}
	}
	return total
}

// SetSLASettings persists the owner-edited SLA parameters (US-009).
func (uc *ComputeCommissionsUseCase) SetSLASettings(ctx context.Context, actorEmail string, in SLASettings) error {
	if uc.settings == nil {
		return &domain.AppError{Code: "COMMISSION_SETTINGS_UNAVAILABLE", Message: "platform settings are not configured"}
	}
	if in.FirstResponseMinutes < 1 || in.FirstResponseMinutes > 7*24*60 {
		return &domain.AppError{Code: "COMMISSION_INVALID_SLA", Message: "first_response_minutes must be between 1 and 10080"}
	}
	if in.BreachPct < 1 || in.BreachPct > 100 {
		return &domain.AppError{Code: "COMMISSION_INVALID_SLA", Message: "breach_pct must be between 1 and 100"}
	}
	if in.BusinessStartHour < 0 || in.BusinessEndHour > 24 || in.BusinessStartHour >= in.BusinessEndHour {
		return &domain.AppError{Code: "COMMISSION_INVALID_SLA", Message: "business window must satisfy 0 <= start < end <= 24"}
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return &domain.AppError{Code: "COMMISSION_INVALID_SLA", Message: "timezone must be a valid IANA name"}
	}
	values := map[string]string{
		domain.SettingSLAFirstResponseMinutes: strconv.FormatInt(in.FirstResponseMinutes, 10),
		domain.SettingSLABreachPct:            strconv.FormatInt(in.BreachPct, 10),
		domain.SettingSLABusinessStartHour:    strconv.FormatInt(in.BusinessStartHour, 10),
		domain.SettingSLABusinessEndHour:      strconv.FormatInt(in.BusinessEndHour, 10),
		domain.SettingSLATimezone:             in.Timezone,
	}
	for key, value := range values {
		if err := uc.settings.SetSetting(ctx, key, value, actorEmail); err != nil {
			return err
		}
	}
	uc.audit(ctx, actorEmail, "commission.set_sla_settings", domain.SettingSLABreachPct)
	slog.Info("owner set SLA settings", "actor", actorEmail,
		"first_response_minutes", in.FirstResponseMinutes, "breach_pct", in.BreachPct,
		"window", []int64{in.BusinessStartHour, in.BusinessEndHour}, "tz", in.Timezone)
	return nil
}
