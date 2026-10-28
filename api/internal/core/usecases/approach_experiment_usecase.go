package usecases

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Managing the approach A/B (US-F216).
type ApproachExperimentUseCase struct {
	repo       domain.ApproachExperimentRepository
	funnelRepo domain.SalesFunnelRepository
}

func NewApproachExperimentUseCase(repo domain.ApproachExperimentRepository, funnelRepo domain.SalesFunnelRepository) *ApproachExperimentUseCase {
	return &ApproachExperimentUseCase{repo: repo, funnelRepo: funnelRepo}
}

type CreateApproachExperimentDTO struct {
	AccountID      string
	FunnelID       string
	Name           string
	VariantASuffix string
	VariantBSuffix string
	VariantALabel  string
	VariantBLabel  string
	VariantAModel  string
	VariantBModel  string
	SplitPctB      int
}

// Create starts an experiment on a funnel.
//
// Refuses a second running experiment on the same funnel. Two at once make
// the result unattributable: a lift could belong to either, and the honest
// answer afterwards would be "we do not know", which is worse than not
// having run them.
func (uc *ApproachExperimentUseCase) Create(ctx context.Context, dto CreateApproachExperimentDTO) (*domain.ApproachExperiment, error) {
	funnel, err := uc.funnelRepo.GetByID(ctx, dto.FunnelID)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != dto.AccountID {
		return nil, domain.ErrUnauthorized
	}
	if strings.TrimSpace(dto.Name) == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "experiment name is required"}
	}
	// Identical arms would burn the pilot's whole lead budget to prove that
	// a thing equals itself.
	if strings.TrimSpace(dto.VariantASuffix) == strings.TrimSpace(dto.VariantBSuffix) &&
		strings.TrimSpace(dto.VariantAModel) == strings.TrimSpace(dto.VariantBModel) {
		return nil, &domain.AppError{
			Code:    "EXPERIMENT_VARIANTS_IDENTICAL",
			Message: "as duas variantes são iguais — não há o que comparar",
		}
	}
	if running, err := uc.repo.GetRunningByFunnel(ctx, dto.FunnelID); err == nil && running != nil {
		return nil, &domain.AppError{
			Code:    "EXPERIMENT_ALREADY_RUNNING",
			Message: "já existe um experimento rodando neste funil",
		}
	}
	split := dto.SplitPctB
	if split <= 0 || split >= 100 {
		split = 50
	}
	e := &domain.ApproachExperiment{
		ID: ulid.Make().String(), AccountID: dto.AccountID, FunnelID: dto.FunnelID,
		Name: strings.TrimSpace(dto.Name), Status: domain.ApproachExperimentRunning,
		VariantASuffix: dto.VariantASuffix, VariantBSuffix: dto.VariantBSuffix,
		VariantALabel: orDefault(dto.VariantALabel, "A"),
		VariantBLabel: orDefault(dto.VariantBLabel, "B"),
		VariantAModel: dto.VariantAModel, VariantBModel: dto.VariantBModel,
		SplitPctB: split,
	}
	if err := uc.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ApproachExperimentReport is the experiment plus both arms.
type ApproachExperimentReport struct {
	Experiment *domain.ApproachExperiment `json:"experiment"`
	Arms       []domain.ApproachArmStats  `json:"arms"`
	// Verdict is deliberately conservative prose, not a winner flag. The
	// number of drafts a pilot produces in a month is small enough that a
	// visible gap is usually noise, and a UI that crowns a winner invites
	// someone to act on it.
	Verdict string `json:"verdict"`
}

// minDecidedForVerdict is the point below which any gap is noise. At a 60%
// baseline, detecting a 20% lift needs ~245 decided per arm; this is a floor
// for saying anything at all, not the sample that would settle it.
const minDecidedForVerdict = 30

func (uc *ApproachExperimentUseCase) Report(ctx context.Context, accountID, experimentID string) (*ApproachExperimentReport, error) {
	e, err := uc.repo.GetByID(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, &domain.AppError{Code: "EXPERIMENT_NOT_FOUND", Message: "experiment not found"}
	}
	if e.AccountID != accountID {
		return nil, domain.ErrUnauthorized
	}
	arms, err := uc.repo.Stats(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	for i := range arms {
		arms[i].Label = e.LabelFor(arms[i].Variant)
	}
	return &ApproachExperimentReport{Experiment: e, Arms: arms, Verdict: verdict(arms)}, nil
}

func verdict(arms []domain.ApproachArmStats) string {
	if len(arms) < 2 {
		return "Ainda sem os dois braços — nada a comparar."
	}
	menor := arms[0].Decided
	for _, a := range arms {
		if a.Decided < menor {
			menor = a.Decided
		}
	}
	if menor < minDecidedForVerdict {
		return "Amostra pequena demais para dizer qualquer coisa. Continue rodando."
	}
	melhor, pior := arms[0], arms[1]
	if pior.UneditedRate > melhor.UneditedRate {
		melhor, pior = pior, melhor
	}
	diff := melhor.UneditedRate - pior.UneditedRate
	if diff < 0.10 {
		return "Empate técnico: a diferença é menor que o ruído desta amostra."
	}
	return melhor.Label + " está à frente em aprovação sem edição, mas confirme com mais volume antes de adotar."
}

// Stop closes an experiment, optionally naming a winner.
func (uc *ApproachExperimentUseCase) Stop(ctx context.Context, accountID, experimentID, winner string) error {
	e, err := uc.repo.GetByID(ctx, experimentID)
	if err != nil {
		return err
	}
	if e == nil {
		return &domain.AppError{Code: "EXPERIMENT_NOT_FOUND", Message: "experiment not found"}
	}
	if e.AccountID != accountID {
		return domain.ErrUnauthorized
	}
	return uc.repo.Stop(ctx, experimentID, winner)
}

func (uc *ApproachExperimentUseCase) List(ctx context.Context, accountID string, limit int32) ([]*domain.ApproachExperiment, error) {
	return uc.repo.ListByAccount(ctx, accountID, limit)
}
