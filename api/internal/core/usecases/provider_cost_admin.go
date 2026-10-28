package usecases

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ProviderCostAdminUseCase exposes the owner-panel CRUD over the manual
// API-price table (US-005, vendedores v3) plus the USD->BRL rate
// setting. The profit computation (US-006) reads both through the same
// repos; this use case only serves the /owner management endpoints.
type ProviderCostAdminUseCase struct {
	repo     domain.ProviderCostRepository
	settings domain.PlatformSettingRepository
	auditLog domain.AuditLogRepository
}

func NewProviderCostAdminUseCase(repo domain.ProviderCostRepository, settings domain.PlatformSettingRepository, auditLog domain.AuditLogRepository) *ProviderCostAdminUseCase {
	return &ProviderCostAdminUseCase{repo: repo, settings: settings, auditLog: auditLog}
}

// DefaultUSDBRLRateCents backs the getter when the setting row is
// somehow absent (matches the migration seed).
const DefaultUSDBRLRateCents = 550

// ProviderCostListing bundles the table with the rate so the owner
// screen renders in one GET.
type ProviderCostListing struct {
	Items           []*domain.ProviderCost `json:"items"`
	USDBRLRateCents int                    `json:"usd_brl_rate_cents"`
}

func (uc *ProviderCostAdminUseCase) List(ctx context.Context) (*ProviderCostListing, error) {
	rows, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	rate, err := uc.GetUSDBRLRateCents(ctx)
	if err != nil {
		return nil, err
	}
	return &ProviderCostListing{Items: rows, USDBRLRateCents: rate}, nil
}

// GetUSDBRLRateCents returns the manual conversion rate in BRL cents
// per 1 USD. US-006 calls this for the token-cost conversion.
func (uc *ProviderCostAdminUseCase) GetUSDBRLRateCents(ctx context.Context) (int, error) {
	raw, err := uc.settings.GetSetting(ctx, domain.SettingUSDBRLRateCents)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return DefaultUSDBRLRateCents, nil
	}
	rate, err := strconv.Atoi(raw)
	if err != nil || rate <= 0 {
		return DefaultUSDBRLRateCents, nil
	}
	return rate, nil
}

// SetUSDBRLRateCents persists an owner-edited rate. Bounds are sanity
// rails, not forecasts: 100 (R$1.00) .. 10000 (R$100.00) per USD.
func (uc *ProviderCostAdminUseCase) SetUSDBRLRateCents(ctx context.Context, actorEmail string, rateCents int) error {
	if rateCents < 100 || rateCents > 10000 {
		return &domain.AppError{Code: "PROVIDER_COST_INVALID_RATE", Message: "usd_brl_rate_cents must be between 100 and 10000 (BRL cents per 1 USD)"}
	}
	if err := uc.settings.SetSetting(ctx, domain.SettingUSDBRLRateCents, strconv.Itoa(rateCents), actorEmail); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, "platform_setting.set_usd_brl_rate", domain.SettingUSDBRLRateCents)
	slog.Info("owner set usd_brl rate", "actor", actorEmail, "rate_cents", rateCents)
	return nil
}

// ProviderCostDTO carries the operator-facing form fields. Costs are
// integer cents of USD per 1M tokens; EffectiveFrom is "YYYY-MM-DD".
type ProviderCostDTO struct {
	Provider                string
	Model                   string
	InputCostPer1MCentsUSD  int64
	OutputCostPer1MCentsUSD int64
	EffectiveFrom           string
}

func (uc *ProviderCostAdminUseCase) validate(dto *ProviderCostDTO) (time.Time, error) {
	dto.Provider = strings.ToLower(strings.TrimSpace(dto.Provider))
	dto.Model = strings.TrimSpace(dto.Model)
	if dto.Provider == "" || len(dto.Provider) > 64 {
		return time.Time{}, &domain.AppError{Code: "PROVIDER_COST_INVALID_PROVIDER", Message: "Provider is required (max 64 chars)"}
	}
	if dto.Model == "" || len(dto.Model) > 128 {
		return time.Time{}, &domain.AppError{Code: "PROVIDER_COST_INVALID_MODEL", Message: "Model is required (max 128 chars)"}
	}
	if dto.InputCostPer1MCentsUSD < 0 || dto.OutputCostPer1MCentsUSD < 0 {
		return time.Time{}, &domain.AppError{Code: "PROVIDER_COST_INVALID_VALUES", Message: "Costs must be >= 0 cents USD per 1M tokens"}
	}
	from, err := time.Parse("2006-01-02", dto.EffectiveFrom)
	if err != nil {
		return time.Time{}, &domain.AppError{Code: "PROVIDER_COST_INVALID_DATE", Message: "effective_from must be YYYY-MM-DD"}
	}
	return from, nil
}

// Create inserts a new price row. A change of price for an existing
// provider/model should be a new row with a later effective_from —
// that is what keeps history intact for past-cycle recomputes.
func (uc *ProviderCostAdminUseCase) Create(ctx context.Context, actorEmail string, dto ProviderCostDTO) (*domain.ProviderCost, error) {
	from, err := uc.validate(&dto)
	if err != nil {
		return nil, err
	}
	c := &domain.ProviderCost{
		ID:                      ulid.Make().String(),
		Provider:                dto.Provider,
		Model:                   dto.Model,
		InputCostPer1MCentsUSD:  dto.InputCostPer1MCentsUSD,
		OutputCostPer1MCentsUSD: dto.OutputCostPer1MCentsUSD,
		EffectiveFrom:           from,
	}
	// Duplicate (provider, model, effective_from) surfaces as
	// PROVIDER_COST_DUPLICATE — the repo translates the PG unique
	// violation into that domain error.
	if err := uc.repo.Insert(ctx, c); err != nil {
		return nil, err
	}
	uc.audit(ctx, actorEmail, "provider_cost.create", c.ID)
	slog.Info("owner created provider cost", "actor", actorEmail, "provider", c.Provider, "model", c.Model, "effective_from", dto.EffectiveFrom)
	return c, nil
}

// Update rewrites an existing row in place — meant for typo fixes, not
// price changes (those get a new row via Create). Provider/model are
// immutable; only costs and effective_from move.
func (uc *ProviderCostAdminUseCase) Update(ctx context.Context, actorEmail, id string, dto ProviderCostDTO) error {
	if dto.InputCostPer1MCentsUSD < 0 || dto.OutputCostPer1MCentsUSD < 0 {
		return &domain.AppError{Code: "PROVIDER_COST_INVALID_VALUES", Message: "Costs must be >= 0 cents USD per 1M tokens"}
	}
	from, err := time.Parse("2006-01-02", dto.EffectiveFrom)
	if err != nil {
		return &domain.AppError{Code: "PROVIDER_COST_INVALID_DATE", Message: "effective_from must be YYYY-MM-DD"}
	}
	ok, err := uc.repo.Update(ctx, id, dto.InputCostPer1MCentsUSD, dto.OutputCostPer1MCentsUSD, from)
	if err != nil {
		return err
	}
	if !ok {
		return &domain.AppError{Code: "PROVIDER_COST_NOT_FOUND", Message: "Provider cost not found"}
	}
	uc.audit(ctx, actorEmail, "provider_cost.update", id)
	slog.Info("owner updated provider cost", "actor", actorEmail, "id", id)
	return nil
}

func (uc *ProviderCostAdminUseCase) Delete(ctx context.Context, actorEmail, id string) error {
	ok, err := uc.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return &domain.AppError{Code: "PROVIDER_COST_NOT_FOUND", Message: "Provider cost not found"}
	}
	uc.audit(ctx, actorEmail, "provider_cost.delete", id)
	slog.Info("owner deleted provider cost", "actor", actorEmail, "id", id)
	return nil
}

func (uc *ProviderCostAdminUseCase) audit(ctx context.Context, actorEmail, action, entityID string) {
	if uc.auditLog == nil {
		return
	}
	_ = uc.auditLog.Create(ctx, &domain.AuditLog{
		ActorID:    &actorEmail,
		Action:     action,
		EntityType: "provider_cost",
		EntityID:   entityID,
	})
}
