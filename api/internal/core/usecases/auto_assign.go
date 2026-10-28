package usecases

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// AutoAssignUseCase implements round-robin conversation assignment
// across active attendants in an account. The rotation cursor is
// persisted (US-021) — rotationRepo claims the next attendant and
// assigns the conversation in one transaction, so the rotation
// survives deploys/replicas and an assignment failure never advances
// the cursor.
type AutoAssignUseCase struct {
	convRepo     domain.ConversationRepository
	rotationRepo domain.AutoAssignRotationRepository
	settingsRepo domain.AutoAssignSettingsRepository // optional: per-account config (US-016)

	// platformAccountID gates the default-on behavior (US-020): only
	// the platform account auto-assigns without an explicit opt-in row.
	platformAccountID string
}

func NewAutoAssignUseCase(convRepo domain.ConversationRepository, rotationRepo domain.AutoAssignRotationRepository) *AutoAssignUseCase {
	return &AutoAssignUseCase{
		convRepo:     convRepo,
		rotationRepo: rotationRepo,
	}
}

// SetSettingsRepo enables per-account auto-assign config.
func (uc *AutoAssignUseCase) SetSettingsRepo(repo domain.AutoAssignSettingsRepository) {
	uc.settingsRepo = repo
}

// SetPlatformAccountID marks the one account that keeps the historical
// default-on rotation without a persisted opt-in (US-020). Every other
// account only auto-assigns after explicitly enabling it in settings.
func (uc *AutoAssignUseCase) SetPlatformAccountID(id string) {
	uc.platformAccountID = id
}

// Assign picks the next attendant in round-robin order and assigns the
// conversation to them. Returns the assigned user ID, or nil if
// auto-assign is not enabled for the account or no eligible attendants
// are available.
//
// Enablement (US-020): a persisted settings row is authoritative; with
// no row, only the platform account defaults to enabled — customer
// accounts keep their pre-feature behavior (no auto-assign) until they
// opt in. A settings read error fails CLOSED (no assignment) so a DB
// hiccup can never silently re-enable the feature for an account that
// turned it off.
func (uc *AutoAssignUseCase) Assign(ctx context.Context, conversationID, accountID string) (*string, error) {
	var settings *domain.AutoAssignSettings
	if uc.settingsRepo != nil {
		s, err := uc.settingsRepo.GetAutoAssignSettings(ctx, accountID)
		if err != nil {
			slog.Error("auto-assign settings load failed, skipping assignment (fail-closed)",
				"account_id", accountID, "conversation_id", conversationID, "error", err)
			return nil, nil
		}
		settings = s
	}

	switch {
	case settings == nil:
		if uc.platformAccountID == "" || accountID != uc.platformAccountID {
			return nil, nil
		}
	case !settings.Enabled:
		return nil, nil
	}

	attendants, err := uc.convRepo.GetActiveAttendants(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if settings != nil && len(settings.ParticipantUserIDs) > 0 {
		// Intersect with active attendants (order preserved) so
		// deactivated/removed users drop out of the rotation
		// without any settings cleanup.
		allowed := make(map[string]struct{}, len(settings.ParticipantUserIDs))
		for _, id := range settings.ParticipantUserIDs {
			allowed[id] = struct{}{}
		}
		filtered := attendants[:0:0]
		for _, a := range attendants {
			if _, ok := allowed[a]; ok {
				filtered = append(filtered, a)
			}
		}
		attendants = filtered
	}

	if len(attendants) == 0 {
		if settings != nil && settings.Enabled {
			// Enabled-without-eligible-participants is a config problem
			// the account admin should see, not a silent no-op.
			slog.Warn("auto-assign enabled but no eligible attendants in the rotation",
				"account_id", accountID, "conversation_id", conversationID,
				"configured_participants", len(settings.ParticipantUserIDs))
		}
		return nil, nil
	}

	assignee, err := uc.rotationRepo.AssignNextRoundRobin(ctx, conversationID, accountID, attendants)
	if err != nil {
		slog.Error("auto-assign failed", "conversation_id", conversationID, "account_id", accountID, "error", err)
		return nil, err
	}

	slog.Info("auto-assigned conversation", "conversation_id", conversationID, "assignee", assignee)
	return &assignee, nil
}
