package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// SetAccountAPIKeysUseCase persists per-account BYOK keys for OpenAI
// and Gemini. Only callable by admins on contract plans (Partner /
// Enterprise) — the handler enforces the role + the plan check.
//
// Empty strings clear the slot. Trailing whitespace is trimmed because
// copy/paste from the OpenAI dashboard frequently appends a newline,
// and a single space would otherwise produce a "key not found" 401
// from the upstream API at first AI call.
type SetAccountAPIKeysUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
}

func NewSetAccountAPIKeysUseCase(accountRepo domain.AccountRepository, planRepo domain.PlanRepository) *SetAccountAPIKeysUseCase {
	return &SetAccountAPIKeysUseCase{accountRepo: accountRepo, planRepo: planRepo}
}

type SetAccountAPIKeysDTO struct {
	AccountID string
	OpenAIKey string // empty = clear
	GeminiKey string // empty = clear
}

// ErrAPIKeysNotAvailable is returned when an admin tries to set BYOK
// keys on a self-serve plan. We refuse rather than silently storing
// keys that would never be used — the dashboard's UI is supposed to
// hide the form on those plans, but server-side enforcement keeps a
// stale client from corrupting state.
var ErrAPIKeysNotAvailable = &domain.AppError{Code: "BYOK_NOT_AVAILABLE", Message: "BYOK API keys are only available on contract plans"}

func (uc *SetAccountAPIKeysUseCase) Execute(ctx context.Context, dto SetAccountAPIKeysDTO) error {
	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return domain.ErrAccountNotFound
	}
	if account.PlanID == nil {
		return ErrAPIKeysNotAvailable
	}
	plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
	if err != nil || plan == nil {
		return domain.ErrPlanNotFound
	}
	if !account.IsContractPlan(plan) {
		return ErrAPIKeysNotAvailable
	}
	openKey := strings.TrimSpace(dto.OpenAIKey)
	geminiKey := strings.TrimSpace(dto.GeminiKey)
	return uc.accountRepo.SetAPIKeys(ctx, dto.AccountID, openKey, geminiKey)
}
