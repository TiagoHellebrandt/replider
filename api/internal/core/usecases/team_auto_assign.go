package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// AutoAssignSettingsUseCase reads/writes the per-account round-robin
// config exposed under Settings → Team (US-016). Every account can
// configure it, but only the platform account defaults to enabled
// without a persisted row (US-020) — customer accounts must opt in.
type AutoAssignSettingsUseCase struct {
	settingsRepo      domain.AutoAssignSettingsRepository
	userRepo          domain.UserRepository
	platformAccountID string
}

func NewAutoAssignSettingsUseCase(
	settingsRepo domain.AutoAssignSettingsRepository,
	userRepo domain.UserRepository,
) *AutoAssignSettingsUseCase {
	return &AutoAssignSettingsUseCase{settingsRepo: settingsRepo, userRepo: userRepo}
}

// SetPlatformAccountID mirrors AutoAssignUseCase's default gate so
// GET /team/auto-assign reports the same default the assigner applies.
func (uc *AutoAssignSettingsUseCase) SetPlatformAccountID(id string) {
	uc.platformAccountID = id
}

// AutoAssignSettingsView is the wire shape for GET/PUT /team/auto-assign.
// An empty participant list means "all active attendants participate".
type AutoAssignSettingsView struct {
	Enabled            bool       `json:"enabled"`
	ParticipantUserIDs []string   `json:"participant_user_ids"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

// Get returns the persisted settings. When the account never saved a
// row, the default is enabled only for the platform account (US-020) —
// customer accounts report disabled until they explicitly opt in.
func (uc *AutoAssignSettingsUseCase) Get(ctx context.Context, accountID string) (*AutoAssignSettingsView, error) {
	settings, err := uc.settingsRepo.GetAutoAssignSettings(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		defaultEnabled := uc.platformAccountID != "" && accountID == uc.platformAccountID
		return &AutoAssignSettingsView{Enabled: defaultEnabled, ParticipantUserIDs: []string{}}, nil
	}
	updatedAt := settings.UpdatedAt
	return &AutoAssignSettingsView{
		Enabled:            settings.Enabled,
		ParticipantUserIDs: settings.ParticipantUserIDs,
		UpdatedAt:          &updatedAt,
	}, nil
}

// Update validates every participant against the account's active
// members whose role is in the GetActiveAttendants pool (owner/admin/
// gestor/attendant) — EXACTLY the pool
// GetActiveAttendants rotates over (US-021), so a saved participant can
// never be silently unrotatable (e.g. an active super_admin would pass
// an is-active-only check but never receive a conversation).
func (uc *AutoAssignSettingsUseCase) Update(ctx context.Context, accountID string, enabled bool, participantUserIDs []string) (*AutoAssignSettingsView, error) {
	participants := dedupeStrings(participantUserIDs)

	if len(participants) > 0 {
		members, err := uc.userRepo.ListByAccountID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		eligible := make(map[string]struct{}, len(members))
		for _, m := range members {
			if m.IsActive && m.Role != nil && (*m.Role == "owner" || *m.Role == "admin" || *m.Role == "gestor" || *m.Role == "attendant") {
				eligible[m.ID] = struct{}{}
			}
		}
		for _, id := range participants {
			if _, ok := eligible[id]; !ok {
				return nil, domain.ErrInvalidAutoAssignParticipant
			}
		}
	}

	err := uc.settingsRepo.UpsertAutoAssignSettings(ctx, &domain.AutoAssignSettings{
		AccountID:          accountID,
		Enabled:            enabled,
		ParticipantUserIDs: participants,
	})
	if err != nil {
		return nil, err
	}

	slog.Info("auto-assign settings updated",
		"account_id", accountID,
		"enabled", enabled,
		"participants", len(participants),
	)

	return uc.Get(ctx, accountID)
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
