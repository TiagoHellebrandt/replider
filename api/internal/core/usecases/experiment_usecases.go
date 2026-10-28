package usecases

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ExperimentCacheInvalidator drops the in-process cache the AgentRunner
// keeps for "is there a running experiment for this agent?". Optional
// dependency — when nil we just rely on the natural cache TTL.
type ExperimentCacheInvalidator interface {
	InvalidateExperimentCache(agentID string)
}

// ExperimentUseCase covers create/start/stop/list/get-results for the
// A/B testing flow. Routing of in-flight conversations is handled by
// AgentRunner; this UC only manages the experiment lifecycle and the
// metrics aggregation.
type ExperimentUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	agentRepo   domain.AgentRepository
	expRepo     domain.PromptExperimentRepository
	auditRepo   domain.AuditLogRepository
	cacheInv    ExperimentCacheInvalidator
}

func NewExperimentUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	agentRepo domain.AgentRepository,
	expRepo domain.PromptExperimentRepository,
	auditRepo domain.AuditLogRepository,
) *ExperimentUseCase {
	return &ExperimentUseCase{
		accountRepo: accountRepo,
		planRepo:    planRepo,
		agentRepo:   agentRepo,
		expRepo:     expRepo,
		auditRepo:   auditRepo,
	}
}

// SetCacheInvalidator wires the AgentRunner cache so Start/Stop drop
// stale entries immediately. Safe to leave unset in tests.
func (uc *ExperimentUseCase) SetCacheInvalidator(inv ExperimentCacheInvalidator) {
	uc.cacheInv = inv
}

type CreateExperimentDTO struct {
	AccountID     string
	AgentID       string
	Name          string
	VariantABody  string
	VariantBBody  string
	VariantALabel string
	VariantBLabel string
	SplitPctB     int
}

func (uc *ExperimentUseCase) Create(ctx context.Context, dto CreateExperimentDTO) (*domain.PromptExperiment, error) {
	if err := uc.assertPlanAllows(ctx, dto.AccountID); err != nil {
		return nil, err
	}
	if err := uc.assertAgentOwnership(ctx, dto.AccountID, dto.AgentID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(dto.Name) == "" {
		return nil, &domain.AppError{Code: "EXPERIMENT_NAME_REQUIRED", Message: "experiment name is required"}
	}
	if strings.TrimSpace(dto.VariantABody) == "" || strings.TrimSpace(dto.VariantBBody) == "" {
		return nil, &domain.AppError{Code: "EXPERIMENT_BODIES_REQUIRED", Message: "both variant bodies are required"}
	}
	if dto.VariantABody == dto.VariantBBody {
		return nil, &domain.AppError{Code: "EXPERIMENT_BODIES_IDENTICAL", Message: "variants must differ"}
	}
	if len(dto.VariantABody) > domain.MaxPromptBodyBytes || len(dto.VariantBBody) > domain.MaxPromptBodyBytes {
		return nil, domain.ErrPromptBodyTooLarge
	}
	split := dto.SplitPctB
	if split <= 0 || split >= 100 {
		split = 50
	}

	e := &domain.PromptExperiment{
		ID:            ulid.Make().String(),
		AccountID:     dto.AccountID,
		AgentID:       dto.AgentID,
		Name:          strings.TrimSpace(dto.Name),
		Status:        domain.ExperimentStatusDraft,
		VariantABody:  dto.VariantABody,
		VariantBBody:  dto.VariantBBody,
		VariantALabel: orDefault(dto.VariantALabel, "A"),
		VariantBLabel: orDefault(dto.VariantBLabel, "B"),
		SplitPctB:     split,
	}
	if err := uc.expRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	uc.audit(ctx, dto.AccountID, "experiment.create", e.ID)
	return e, nil
}

func (uc *ExperimentUseCase) Start(ctx context.Context, accountID, id string) (*domain.PromptExperiment, error) {
	if err := uc.assertPlanAllows(ctx, accountID); err != nil {
		return nil, err
	}
	e, err := uc.fetchOwned(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if e.Status == domain.ExperimentStatusRunning {
		return e, nil
	}
	if e.Status != domain.ExperimentStatusDraft && e.Status != domain.ExperimentStatusStopped {
		return nil, domain.ErrExperimentStatusInvalid
	}
	now := time.Now().UTC()
	if err := uc.expRepo.UpdateStatus(ctx, id, domain.ExperimentStatusRunning, &now, nil); err != nil {
		return nil, err
	}
	if uc.cacheInv != nil {
		uc.cacheInv.InvalidateExperimentCache(e.AgentID)
	}
	uc.audit(ctx, accountID, "experiment.start", id)
	e.Status = domain.ExperimentStatusRunning
	e.StartedAt = &now
	return e, nil
}

func (uc *ExperimentUseCase) Stop(ctx context.Context, accountID, id string) (*domain.PromptExperiment, error) {
	e, err := uc.fetchOwned(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if e.Status != domain.ExperimentStatusRunning {
		return nil, domain.ErrExperimentNotRunning
	}
	now := time.Now().UTC()
	if err := uc.expRepo.UpdateStatus(ctx, id, domain.ExperimentStatusStopped, nil, &now); err != nil {
		return nil, err
	}
	if uc.cacheInv != nil {
		uc.cacheInv.InvalidateExperimentCache(e.AgentID)
	}
	uc.audit(ctx, accountID, "experiment.stop", id)
	e.Status = domain.ExperimentStatusStopped
	e.StoppedAt = &now
	return e, nil
}

func (uc *ExperimentUseCase) Get(ctx context.Context, accountID, id string) (*domain.PromptExperiment, error) {
	return uc.fetchOwned(ctx, accountID, id)
}

func (uc *ExperimentUseCase) ListByAgent(ctx context.Context, accountID, agentID string, limit, offset int) ([]*domain.PromptExperiment, error) {
	if err := uc.assertAgentOwnership(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return uc.expRepo.ListByAgentID(ctx, agentID, limit, offset)
}

// Results bundles per-variant metrics + a "is_significant" flag using
// a threshold heuristic (≥30 conversations per arm AND a meaningful
// delta in CSAT or AI-resolved %). The flag is labelled "indicative"
// in the UI — it is NOT a real Bayesian posterior.
func (uc *ExperimentUseCase) Results(ctx context.Context, accountID, id string) (*domain.ExperimentResults, error) {
	e, err := uc.fetchOwned(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	a, err := uc.expRepo.MetricsForVariant(ctx, e.ID, domain.VariantA)
	if err != nil {
		return nil, err
	}
	b, err := uc.expRepo.MetricsForVariant(ctx, e.ID, domain.VariantB)
	if err != nil {
		return nil, err
	}
	a.Label = e.VariantALabel
	b.Label = e.VariantBLabel
	res := &domain.ExperimentResults{Experiment: e, VariantA: *a, VariantB: *b}
	res.IsSignificant, res.LikelyWinner = computeLikelyWinner(*a, *b)
	return res, nil
}

func (uc *ExperimentUseCase) fetchOwned(ctx context.Context, accountID, id string) (*domain.PromptExperiment, error) {
	e, err := uc.expRepo.GetByID(ctx, id)
	if err != nil || e == nil {
		return nil, domain.ErrExperimentNotFound
	}
	if e.AccountID != accountID {
		return nil, domain.ErrExperimentNotFound
	}
	return e, nil
}

func (uc *ExperimentUseCase) assertAgentOwnership(ctx context.Context, accountID, agentID string) error {
	a, err := uc.agentRepo.GetByID(ctx, agentID)
	if err != nil || a == nil {
		return domain.ErrAgentNotFound
	}
	if a.AccountID != accountID {
		return domain.ErrAgentNotFound
	}
	return nil
}

func (uc *ExperimentUseCase) assertPlanAllows(ctx context.Context, accountID string) error {
	plan, err := uc.resolvePlan(ctx, accountID)
	if err != nil {
		return err
	}
	if !plan.AllowsExperiments() {
		return domain.ErrPlanFeatureLocked
	}
	return nil
}

func (uc *ExperimentUseCase) resolvePlan(ctx context.Context, accountID string) (*domain.Plan, error) {
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	if acc.PlanID == nil {
		return domain.TrialPlan(), nil
	}
	p, err := uc.planRepo.GetByID(ctx, *acc.PlanID)
	if err != nil || p == nil {
		return domain.TrialPlan(), nil
	}
	return p, nil
}

func (uc *ExperimentUseCase) audit(ctx context.Context, accountID, action, id string) {
	_ = uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		Action:     action,
		EntityType: "experiment",
		EntityID:   id,
		CreatedAt:  time.Now().UTC(),
	})
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// computeLikelyWinner applies an "indicative" threshold heuristic. It
// is intentionally conservative — both variants must have ≥30
// conversations AND one of two thresholds must clear (CSAT delta ≥0.4
// or AI-resolved-rate delta ≥10pp). This is documented as indicative
// in the UI and is NOT a substitute for a Bayesian test.
func computeLikelyWinner(a, b domain.VariantMetrics) (bool, *string) {
	const minSample = 30
	if a.ConversationCount < minSample || b.ConversationCount < minSample {
		return false, nil
	}
	csatDelta := 0.0
	if a.CSATAverage != nil && b.CSATAverage != nil {
		csatDelta = *a.CSATAverage - *b.CSATAverage
	}
	resA := rate(a.AIResolvedCount, a.ConversationCount)
	resB := rate(b.AIResolvedCount, b.ConversationCount)
	resDelta := resA - resB

	const csatThreshold = 0.4
	const resolutionThreshold = 0.10
	if !(absFloat(csatDelta) >= csatThreshold || absFloat(resDelta) >= resolutionThreshold) {
		return false, nil
	}

	winner := domain.VariantA
	switch {
	case csatDelta < -csatThreshold || resDelta < -resolutionThreshold:
		winner = domain.VariantB
	}
	return true, &winner
}

func rate(n, d int64) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// Compile-time guard that errors.Is doesn't drift if domain helpers change.
var _ = errors.Is
