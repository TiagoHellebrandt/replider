package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// UpdateAccountUseCase handles updating mutable account fields. Today the
// only mutable field exposed is `name` (the company/workspace label shown
// in the dashboard). The first user-facing entry point is the onboarding
// flow's "Welcome" step, which asks for the company name.
type UpdateAccountUseCase struct {
	accountRepo domain.AccountRepository
	userRepo    domain.UserRepository
	// digestOptOut is an optional dependency (set via SetDigestOptOutWriter)
	// so the constructor + existing test call sites stay untouched. Nil
	// when unwired — a weekly_digest_opt_out field in the request is then
	// silently ignored rather than erroring.
	digestOptOut DigestOptOutWriter
}

// DigestOptOutWriter flips the per-account weekly-digest opt-out (US-176).
// Satisfied by postgres.WeeklyDigestRepository.
type DigestOptOutWriter interface {
	SetWeeklyDigestOptOut(ctx context.Context, accountID string, optOut bool) error
}

func NewUpdateAccountUseCase(accountRepo domain.AccountRepository, userRepo domain.UserRepository) *UpdateAccountUseCase {
	return &UpdateAccountUseCase{accountRepo: accountRepo, userRepo: userRepo}
}

// SetDigestOptOutWriter wires the weekly-digest opt-out writer. Optional —
// keeps the constructor signature stable (cf. the referral setter seams).
func (uc *UpdateAccountUseCase) SetDigestOptOutWriter(w DigestOptOutWriter) {
	uc.digestOptOut = w
}

// UpdateAccountDTO is the request body for PATCH /accounts.
//
// All fields are optional — only those provided will be updated.
// LogoURL: nil means "don't touch"; empty string means "clear the stored
// logo"; any other string is persisted verbatim (expected to be a GCS
// object path returned by the media/upload flow).
// Timezone: nil means "don't touch"; otherwise must be a valid IANA name.
type UpdateAccountDTO struct {
	Name                          *string `json:"name,omitempty"`
	OnboardingCompleted           *bool   `json:"onboarding_completed,omitempty"`
	LogoURL                       *string `json:"logo_url,omitempty"`
	Timezone                      *string `json:"timezone,omitempty"`
	CSATEnabled                   *bool   `json:"csat_enabled,omitempty"`
	ReplyReminderEnabled          *bool   `json:"reply_reminder_enabled,omitempty"`
	ReplyReminderThresholdMinutes *int    `json:"reply_reminder_threshold_minutes,omitempty"`
	// WeeklyDigestOptOut toggles the per-account weekly retention digest
	// (US-176). true = stop sending; false = resume. nil = don't touch.
	WeeklyDigestOptOut *bool `json:"weekly_digest_opt_out,omitempty"`
	// BusinessLines: nil means "don't touch"; an empty list clears them.
	// Each entry must be one of sell / rent / service.
	BusinessLines *[]domain.BusinessLine `json:"business_lines,omitempty"`
	// Territory: nil means "don't touch". To clear, send {"uf": ""}.
	// Otherwise UF is required (2 letters), cities optional.
	Territory *domain.Territory `json:"territory,omitempty"`
}

// Execute updates the requesting user's account. The user MUST be an admin
// of the account they're updating — verified via the JWT claims passed in
// from the handler. Returns ErrUnauthorized if the role check fails or
// ErrInvalidRequest if no actionable fields are present.
func (uc *UpdateAccountUseCase) Execute(ctx context.Context, accountID, userID string, dto UpdateAccountDTO) error {
	// 1. Verify role
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.AccountID != accountID {
		return domain.ErrForbidden
	}
	// RoleSatisfies, not string equality: an OWNER is not literally "admin", and
	// was left out of what is theirs by definition. And the refusal is 403 — with
	// a 401 the app concludes the credential died and clears the session.
	if !domain.RoleSatisfies(roleOf(user), domain.RoleAdminRole) {
		return domain.ErrForbidden
	}

	// 2. Apply each field independently
	hasUpdate := false

	if dto.Name != nil {
		name := strings.TrimSpace(*dto.Name)
		if name == "" {
			return &domain.AppError{Code: "INVALID_REQUEST", Message: "name cannot be empty"}
		}
		if err := uc.accountRepo.UpdateName(ctx, accountID, name); err != nil {
			return err
		}
		hasUpdate = true
	}

	if dto.BusinessLines != nil || dto.Territory != nil {
		acc, err := uc.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return err
		}
		if acc == nil {
			return domain.ErrAccountNotFound
		}
		lines := acc.BusinessLines
		if dto.BusinessLines != nil {
			lines = make([]domain.BusinessLine, 0, len(*dto.BusinessLines))
			seen := map[domain.BusinessLine]bool{}
			for _, l := range *dto.BusinessLines {
				if !l.Valid() {
					return &domain.AppError{Code: "INVALID_REQUEST", Message: "unknown business line: " + string(l)}
				}
				if !seen[l] {
					seen[l] = true
					lines = append(lines, l)
				}
			}
		}
		territory := acc.Territory
		if dto.Territory != nil {
			territory = normalizeTerritory(dto.Territory)
			if territory != nil && len(territory.UF) != 2 {
				return &domain.AppError{Code: "INVALID_REQUEST", Message: "territory.uf must be a two-letter state code"}
			}
		}
		if err := uc.accountRepo.UpdateBusinessProfile(ctx, accountID, lines, territory); err != nil {
			return err
		}
		hasUpdate = true
	}

	if dto.OnboardingCompleted != nil && *dto.OnboardingCompleted {
		if err := uc.accountRepo.CompleteOnboarding(ctx, accountID); err != nil {
			return err
		}
		hasUpdate = true
	}

	if dto.LogoURL != nil || dto.Timezone != nil {
		acc, err := uc.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return err
		}
		if acc == nil {
			return domain.ErrAccountNotFound
		}

		newLogo := acc.LogoURL
		if dto.LogoURL != nil {
			trimmed := strings.TrimSpace(*dto.LogoURL)
			if trimmed == "" {
				newLogo = nil
			} else {
				newLogo = &trimmed
			}
		}

		newTZ := acc.Timezone
		if dto.Timezone != nil {
			tz := strings.TrimSpace(*dto.Timezone)
			if tz == "" {
				return &domain.AppError{Code: "INVALID_REQUEST", Message: "timezone cannot be empty"}
			}
			if _, err := time.LoadLocation(tz); err != nil {
				return &domain.AppError{Code: "INVALID_REQUEST", Message: "invalid IANA timezone"}
			}
			newTZ = tz
		}

		if err := uc.accountRepo.UpdateBranding(ctx, accountID, newLogo, newTZ); err != nil {
			return err
		}
		hasUpdate = true
	}

	if dto.CSATEnabled != nil {
		if err := uc.accountRepo.UpdateCSATEnabled(ctx, accountID, *dto.CSATEnabled); err != nil {
			return err
		}
		hasUpdate = true
	}

	if dto.ReplyReminderEnabled != nil || dto.ReplyReminderThresholdMinutes != nil {
		acc, err := uc.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return err
		}
		if acc == nil {
			return domain.ErrAccountNotFound
		}
		enabled := acc.ReplyReminderEnabled
		if dto.ReplyReminderEnabled != nil {
			enabled = *dto.ReplyReminderEnabled
		}
		threshold := acc.ReplyReminderThresholdMinutes
		if dto.ReplyReminderThresholdMinutes != nil {
			t := *dto.ReplyReminderThresholdMinutes
			if t < 1 || t > 1440 {
				return &domain.AppError{Code: "INVALID_REQUEST", Message: "reply_reminder_threshold_minutes must be 1..1440"}
			}
			threshold = t
		}
		if err := uc.accountRepo.SetReplyReminderSettings(ctx, accountID, enabled, threshold); err != nil {
			return err
		}
		hasUpdate = true
	}

	if dto.WeeklyDigestOptOut != nil && uc.digestOptOut != nil {
		if err := uc.digestOptOut.SetWeeklyDigestOptOut(ctx, accountID, *dto.WeeklyDigestOptOut); err != nil {
			return err
		}
		hasUpdate = true
	}

	if !hasUpdate {
		return &domain.AppError{Code: "INVALID_REQUEST", Message: "no fields to update"}
	}

	return nil
}

// normalizeTerritory trims and upper-cases the state, drops blank cities,
// and turns "no state" into nil — which is how the caller clears it.
func normalizeTerritory(t *domain.Territory) *domain.Territory {
	uf := strings.ToUpper(strings.TrimSpace(t.UF))
	if uf == "" {
		return nil
	}
	cities := make([]string, 0, len(t.Cities))
	seen := map[string]bool{}
	for _, c := range t.Cities {
		c = strings.TrimSpace(c)
		if c == "" || seen[strings.ToLower(c)] {
			continue
		}
		seen[strings.ToLower(c)] = true
		cities = append(cities, c)
	}
	return &domain.Territory{UF: uf, Cities: cities}
}
