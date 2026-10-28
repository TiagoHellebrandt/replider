package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Clause alerts and the recovered-revenue ledger (§6.11, G5/G6).
//
// The red-flag list is derived from confirmed fields — see domain.DeriveRedFlags,
// which is deterministic on purpose. This layer's job is the other half: turning
// a flag into a dated task in one click, and recording what the account actually
// went back and collected.
//
// That second number is the one that renews the subscription. It is a LEDGER,
// not a computed figure: the accumulated index is what was owed, and what was
// agreed differs every time somebody negotiates. Only a person can say which.

type ContractAlertUseCase struct {
	contracts   domain.ContractRepository
	alerts      domain.ContractAlertRepository
	recoveries  domain.ContractRecoveryRepository
	adjustments *ContractAdjustmentUseCase

	now func() time.Time
}

func NewContractAlertUseCase(
	contracts domain.ContractRepository,
	alerts domain.ContractAlertRepository,
	recoveries domain.ContractRecoveryRepository,
) *ContractAlertUseCase {
	return &ContractAlertUseCase{
		contracts: contracts, alerts: alerts, recoveries: recoveries,
		now: time.Now,
	}
}

// SetAdjustments wires the index accumulation. Without it the flag list still
// works — it simply carries no adjustment line, which is honest: no series means
// no percentage to claim.
func (uc *ContractAlertUseCase) SetAdjustments(a *ContractAdjustmentUseCase) {
	uc.adjustments = a
}

func (uc *ContractAlertUseCase) SetClock(now func() time.Time) { uc.now = now }

var errContractAlertNotFound = &domain.AppError{
	Code: "CONTRACT_ALERT_NOT_FOUND", Message: "alerta não encontrado",
}

// ContractAttention is a contract's plain-language summary plus its red flags.
type ContractAttention struct {
	ContractID string   `json:"contract_id"`
	Summary    []string `json:"summary"`
	// Confirmed repeats the gate for the client: an unconfirmed reading produces
	// NO flags, and the screen has to say why rather than showing an empty list
	// that looks like a clean contract.
	Confirmed bool                     `json:"confirmed"`
	Flags     []domain.ContractRedFlag `json:"flags"`
	// TotalAtStakeCents is what the whole list is worth. The number that decides
	// whether somebody opens it.
	TotalAtStakeCents int64 `json:"total_at_stake_cents"`
}

// Attention derives the summary and the flag list for one contract.
func (uc *ContractAlertUseCase) Attention(
	ctx context.Context, accountID, contractID string,
) (*ContractAttention, error) {
	contract, err := uc.contracts.Get(ctx, accountID, contractID)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, errContractNotFound
	}

	var adj *domain.ContractAdjustment
	if uc.adjustments != nil && contract.Confirmed() {
		adj = uc.adjustments.For(ctx, contract)
	}
	flags := domain.DeriveRedFlags(contract, adj, uc.now())

	// Mark the flags that already have a task, so the screen offers "agendado"
	// instead of a second "agendar" that would be a no-op.
	existing, err := uc.alerts.ListByContract(ctx, accountID, contractID)
	if err != nil {
		return nil, err
	}
	scheduled := map[string]bool{}
	for _, a := range existing {
		if a.Status != domain.AlertPending {
			continue
		}
		scheduled[alertKey(a.Kind, a.DueOn)] = true
	}

	out := &ContractAttention{
		ContractID: contractID,
		Summary:    domain.PlainSummary(contract),
		Confirmed:  contract.Confirmed(),
		Flags:      make([]domain.ContractRedFlag, 0, len(flags)),
	}
	for _, flag := range flags {
		if flag.DueOn != nil {
			flag.Scheduled = scheduled[alertKey(flag.Kind, *flag.DueOn)]
		}
		out.TotalAtStakeCents += flag.MoneyAtStakeCents
		out.Flags = append(out.Flags, flag)
	}
	return out, nil
}

func alertKey(kind domain.ContractAlertKind, due time.Time) string {
	return string(kind) + "|" + due.Format("2006-01-02")
}

// ScheduleInput is the 1-click create. Kind + date identify the deadline; the
// title and the money come from the derived flag so the task reads the same as
// the list it came from.
type ScheduleInput struct {
	ContractID string
	Kind       domain.ContractAlertKind
	DueOn      *time.Time
	Title      string
	Detail     string
	// MoneyAtStakeCents is optional for a custom alert. For a derived flag the
	// caller passes the flag's own figure rather than recomputing it.
	MoneyAtStakeCents int64
}

// Schedule turns a flag into a dated task.
//
// For a DERIVED kind the date and the wording are taken from the derivation, not
// from the request: a client that could set its own date on a renewal-notice
// alert could schedule the reminder for the day after the window closes.
func (uc *ContractAlertUseCase) Schedule(
	ctx context.Context, accountID, userID string, in ScheduleInput,
) (*domain.ContractAlert, error) {
	contract, err := uc.contracts.Get(ctx, accountID, in.ContractID)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, errContractNotFound
	}

	alert := &domain.ContractAlert{
		ID: ulid.Make().String(), AccountID: accountID,
		ContractID: in.ContractID, Kind: in.Kind,
		Title: strings.TrimSpace(in.Title), Detail: strings.TrimSpace(in.Detail),
		MoneyAtStakeCents: in.MoneyAtStakeCents,
		CreatedByUserID:   userID,
	}

	if in.Kind == domain.AlertCustom {
		if in.DueOn == nil {
			return nil, &domain.AppError{
				Code: "CONTRACT_ALERT_INVALID", Message: "informe a data do alerta"}
		}
		if alert.Title == "" {
			return nil, &domain.AppError{
				Code: "CONTRACT_ALERT_INVALID", Message: "descreva o que precisa ser feito"}
		}
		alert.DueOn = *in.DueOn
	} else {
		// A derived kind must exist in the derivation. Scheduling a
		// renewal-notice alert on a contract that does not renew itself would be
		// a task about a clause the paper does not contain.
		attention, err := uc.Attention(ctx, accountID, in.ContractID)
		if err != nil {
			return nil, err
		}
		var found *domain.ContractRedFlag
		for i := range attention.Flags {
			if attention.Flags[i].Kind == in.Kind && attention.Flags[i].Schedulable() {
				found = &attention.Flags[i]
				break
			}
		}
		if found == nil {
			return nil, &domain.AppError{
				Code: "CONTRACT_ALERT_NOT_APPLICABLE",
				Message: "este contrato não tem essa cláusula com data — confirme a " +
					"leitura ou use um alerta personalizado",
			}
		}
		alert.DueOn = *found.DueOn
		alert.Title = found.Title
		alert.Detail = found.Detail
		alert.MoneyAtStakeCents = found.MoneyAtStakeCents
	}

	return uc.alerts.Create(ctx, alert)
}

// Due is the account's pending deadlines through a horizon.
func (uc *ContractAlertUseCase) Due(
	ctx context.Context, accountID string, horizonDays, limit int,
) ([]*domain.ContractAlert, error) {
	if horizonDays <= 0 {
		horizonDays = 90
	}
	through := uc.now().AddDate(0, 0, horizonDays)
	return uc.alerts.ListDue(ctx, accountID, through, limit)
}

// CompleteInput closes an alert and, when money came of it, records that too.
type CompleteInput struct {
	// Dismiss closes it without claiming anything happened: plenty of clauses
	// are read, judged irrelevant and closed.
	Dismiss bool
	Note    string
	// RecoveredCents is what was actually collected. Optional — and when it is
	// present it is a human's figure, never the computed one.
	RecoveredCents int64
	RecognisedOn   *time.Time
}

// Complete closes an alert and optionally writes the recovery it produced.
func (uc *ContractAlertUseCase) Complete(
	ctx context.Context, accountID, userID, alertID string, in CompleteInput,
) (*domain.ContractAlert, error) {
	existing, err := uc.alerts.Get(ctx, accountID, alertID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errContractAlertNotFound
	}
	if in.Dismiss && in.RecoveredCents > 0 {
		// Dismissing says nothing happened; a recovery says money arrived. Both
		// at once is a contradiction, and guessing which one the caller meant
		// would put a number in the headline that nobody intended.
		return nil, &domain.AppError{
			Code:    "CONTRACT_ALERT_INVALID",
			Message: "um alerta descartado não registra valor recuperado",
		}
	}

	status := domain.AlertDone
	if in.Dismiss {
		status = domain.AlertDismissed
	}
	alert, err := uc.alerts.Complete(ctx, accountID, alertID, status,
		strings.TrimSpace(in.Note), uc.now())
	if err != nil {
		return nil, err
	}

	if in.RecoveredCents > 0 {
		recognised := uc.now()
		if in.RecognisedOn != nil {
			recognised = *in.RecognisedOn
		}
		if err := uc.recoveries.Create(ctx, &domain.ContractRecovery{
			ID: ulid.Make().String(), AccountID: accountID,
			ContractID: existing.ContractID, AlertID: alertID,
			Kind: recoveryKindFor(existing.Kind), AmountCents: in.RecoveredCents,
			Note: strings.TrimSpace(in.Note), RecordedByUserID: userID,
			RecognisedOn: recognised,
		}); err != nil {
			// The alert is already closed. Failing the whole call here would
			// leave the user pressing a button that appears not to work on an
			// alert that is in fact done — so the write error surfaces instead.
			return alert, err
		}
	}
	return alert, nil
}

// recoveryKindFor maps a deadline to the leak it plugged.
//
// Four buckets rather than the seven alert kinds: the headline splits by where
// the money came from, and "penalty" or "exclusivity" never produce recovered
// revenue — those are constraints, not leaks.
func recoveryKindFor(kind domain.ContractAlertKind) domain.ContractAlertKind {
	switch kind {
	case domain.AlertAdjustment:
		return "adjustment"
	case domain.AlertRenewalNotice, domain.AlertExpiry:
		return "renewal"
	case domain.AlertRegistryBalance:
		return "registry"
	default:
		return "other"
	}
}

// RecordRecovery writes a recovery with no alert behind it.
//
// Somebody who renegotiated a price without the product telling them to still
// recovered contracted revenue, and a ledger that only counts what the product
// prompted would flatter the product and understate the customer.
func (uc *ContractAlertUseCase) RecordRecovery(
	ctx context.Context, accountID, userID, contractID string,
	kind domain.ContractAlertKind, cents int64, note string, recognisedOn *time.Time,
) (*domain.ContractRecovery, error) {
	if cents <= 0 {
		return nil, &domain.AppError{
			Code: "CONTRACT_RECOVERY_INVALID", Message: "informe o valor recuperado"}
	}
	contract, err := uc.contracts.Get(ctx, accountID, contractID)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, errContractNotFound
	}

	recognised := uc.now()
	if recognisedOn != nil {
		recognised = *recognisedOn
	}
	rec := &domain.ContractRecovery{
		ID: ulid.Make().String(), AccountID: accountID, ContractID: contractID,
		Kind: recoveryKindFor(kind), AmountCents: cents,
		Note: strings.TrimSpace(note), RecordedByUserID: userID,
		RecognisedOn: recognised,
	}
	if err := uc.recoveries.Create(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Recovered is the headline: contracted revenue this account went back and
// collected in a period.
func (uc *ContractAlertUseCase) Recovered(
	ctx context.Context, accountID string, from, to time.Time,
) (*domain.ContractRecoverySummary, error) {
	return uc.recoveries.Summary(ctx, accountID, from, to)
}

// RecoveryLedger is the list behind the headline. A total nobody can drill into
// is a total nobody believes.
func (uc *ContractAlertUseCase) RecoveryLedger(
	ctx context.Context, accountID string, from, to time.Time, limit int,
) ([]*domain.ContractRecovery, error) {
	list, err := uc.recoveries.List(ctx, accountID, from, to, limit)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.ContractRecovery{}
	}
	return list, nil
}
