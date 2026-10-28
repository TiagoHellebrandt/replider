package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ComputeCommissionsUseCase turns each cycle's account_monthly_profit
// rows (US-006) into commission_ledger entries (US-007, vendedores v3).
//
// Track: lead_source 'seller_ref' pays commission_rate_own (default
// 50%), anything else pays commission_rate_inbound (default 20%).
// Activity clause: an inactive AM inside residual_until earns 15%
// (reason 'residual'); past residual_until, nothing. Negative net
// profit records the base but pays 0 — commission is never negative.
// Recomputes upsert on UNIQUE(seller, account, cycle, reason) and never
// touch paid rows, so reprocessing a cycle is idempotent.
type ComputeCommissionsUseCase struct {
	repo     domain.CommissionRepository
	users    commissionUserGetter
	auditLog domain.AuditLogRepository
	// settings backs the owner-editable bonus amounts (US-008). nil
	// falls back to the defaults.
	settings domain.PlatformSettingRepository
	// platformAccountID gates SetSellerActivity to platform-account
	// users when configured (empty skips the check).
	platformAccountID string
	// now is swappable in tests.
	now func() time.Time
}

// commissionUserGetter is the slim slice of UserRepository the activity
// endpoint needs (narrow local interface — keeps mocks tiny).
type commissionUserGetter interface {
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

func NewComputeCommissionsUseCase(
	repo domain.CommissionRepository,
	users commissionUserGetter,
	auditLog domain.AuditLogRepository,
) *ComputeCommissionsUseCase {
	return &ComputeCommissionsUseCase{
		repo:     repo,
		users:    users,
		auditLog: auditLog,
		now:      time.Now,
	}
}

// SetPlatformAccountID wires the platform-account gate (same setter
// pattern as GetMeUseCase/OwnerAdminUseCase).
func (uc *ComputeCommissionsUseCase) SetPlatformAccountID(id string) {
	uc.platformAccountID = id
}

// SetPlatformSettings wires the KV backing the owner-editable bonus
// amounts (US-008). Optional — nil keeps the defaults.
func (uc *ComputeCommissionsUseCase) SetPlatformSettings(s domain.PlatformSettingRepository) {
	uc.settings = s
}

// Commission defaults when the seller has no seller_settings row, plus
// the activity-clause residual (PRD: 15% for 12 months) and the
// objective-event bonus defaults (US-008, match the 000128 seeds).
const (
	DefaultCommissionRateOwn     = 50.0
	DefaultCommissionRateInbound = 20.0
	ResidualCommissionRate       = 15.0
	residualMonths               = 12

	DefaultBonusNPSCents       = 10000
	DefaultBonusRetentionCents = 10000
	maxBonusCents              = 1000000 // R$10k/bonus sanity rail
	retentionBonusMonths       = 12
)

func errCommissionSellerNotFound() error {
	return &domain.AppError{Code: "COMMISSION_SELLER_NOT_FOUND", Message: "seller not found"}
}

// ComputeCycle recomputes the cycle's ledger from the profit table and
// returns the persisted entries. Idempotent: same cycle, same inputs,
// same rows; paid entries are never rewritten; a profit-share entry
// whose (seller, reason) changed since the last run replaces its stale
// pending predecessor. A cycle whose profit share was already PAID
// (any seller, any reason) never gains a second full pending row — the
// recompute's delta vs the paid total lands as an 'adjustment' entry
// instead (US-004). Objective-event bonuses (US-008) ride the same
// pass: a plan upgrade flips the cycle's profit share to the own-lead
// rate (reason 'bonus_upgrade'); promoter NPS and 12-month retention
// add fixed-amount entries on top.
func (uc *ComputeCommissionsUseCase) ComputeCycle(ctx context.Context, actorEmail, cycle string) ([]*domain.CommissionEntry, error) {
	from, to, err := parseCycle(cycle)
	if err != nil {
		return nil, err
	}

	// US-006: the whole read-compute-write pass runs in one transaction
	// under the cycle's advisory lock. Without it, a recompute racing
	// MarkPayoutPaid could read "nothing paid yet" (SumPaidProfitShare),
	// lose the row it upserted to a concurrent settle and mint a fresh
	// full pending row next to the newly-paid one — double-pay. The
	// bonus check-then-inserts (HasBonusEntry -> UpsertEntry) serialize
	// under the same lock.
	var entries []*domain.CommissionEntry
	err = uc.repo.InCycleTx(ctx, []string{cycle}, func(txRepo domain.CommissionRepository) error {
		txUC := *uc
		txUC.repo = txRepo
		entries, err = txUC.computeCycleLocked(ctx, actorEmail, cycle, from, to)
		return err
	})
	return entries, err
}

// computeCycleLocked is ComputeCycle's body, running on a
// transaction-bound repo with the cycle's advisory lock held.
func (uc *ComputeCommissionsUseCase) computeCycleLocked(ctx context.Context, actorEmail, cycle string, from, to time.Time) ([]*domain.CommissionEntry, error) {
	bases, err := uc.repo.ListCommissionBases(ctx, cycle, from, to)
	if err != nil {
		return nil, err
	}

	// US-009: measure the cycle's first-response SLA and walk the
	// ladder before pricing anything — step 2 (2nd consecutive breach)
	// drops the cycle's profit share to the reduced rate.
	slaStates, err := uc.computeSLAStates(ctx, cycle, from, to)
	if err != nil {
		return nil, err
	}

	bonusNPSCents, err := uc.getBonusSetting(ctx, domain.SettingBonusNPSCents, DefaultBonusNPSCents)
	if err != nil {
		return nil, err
	}
	bonusRetentionCents, err := uc.getBonusSetting(ctx, domain.SettingBonusRetentionCents, DefaultBonusRetentionCents)
	if err != nil {
		return nil, err
	}

	settingsCache := map[string]*domain.SellerSettings{}
	written := 0
	for _, b := range bases {
		settings, ok := settingsCache[b.SellerUserID]
		if !ok {
			settings, err = uc.repo.GetSellerSettings(ctx, b.SellerUserID)
			if err != nil {
				return nil, err
			}
			settingsCache[b.SellerUserID] = settings
		}

		rate, reason := commissionRateFor(settings, &b, from, slaStates[b.SellerUserID])
		if reason == "" {
			// Past the residual window: no entry — and any stale
			// pending profit-share row from earlier runs goes away.
			if err := uc.repo.DeleteStaleProfitShare(ctx, b.AccountID, cycle, "", ""); err != nil {
				return nil, err
			}
			continue
		}

		amount := commissionAmountCents(b.NetProfitCents, rate)

		// US-004: money already settled for this (account, cycle) is
		// immutable. Without this guard a reason flip (standard ->
		// sla_reduced/bonus_upgrade) or a seller reallocation would
		// insert a fresh full pending row NEXT TO the paid one (different
		// UNIQUE key) and pay the same profit twice. Instead, the
		// difference vs everything already paid becomes a single
		// 'adjustment' entry (negative deltas record a saldo).
		paidCents, paidSeller, hasPaid, err := uc.repo.SumPaidProfitShare(ctx, b.AccountID, cycle)
		if err != nil {
			return nil, err
		}
		if hasPaid {
			keepSeller, keepReason := "", ""
			if delta := amount - paidCents; delta != 0 {
				// US-023: a clawback (negative delta) debits the seller
				// who was actually PAID — after an A→B reallocation the
				// downward recompute must not shrink B's payout with
				// money A received. Positive deltas (new work under the
				// current link) stay with the current seller.
				adjustmentSeller := b.SellerUserID
				if delta < 0 && paidSeller != "" {
					adjustmentSeller = paidSeller
				}
				if err := uc.repo.UpsertEntry(ctx, &domain.CommissionEntry{
					ID:                 ulid.Make().String(),
					SellerUserID:       adjustmentSeller,
					AccountID:          b.AccountID,
					Cycle:              cycle,
					BaseNetProfitCents: b.NetProfitCents,
					RateApplied:        rate,
					AmountCents:        delta,
					Reason:             domain.CommissionReasonAdjustment,
					Status:             domain.CommissionStatusPending,
				}); err != nil {
					return nil, err
				}
				keepSeller, keepReason = adjustmentSeller, domain.CommissionReasonAdjustment
				written++
				slog.Warn("commission recompute hit an already-paid cycle; recorded adjustment",
					"account_id", b.AccountID, "cycle", cycle, "seller_user_id", adjustmentSeller,
					"paid_seller_user_id", paidSeller, "current_seller_user_id", b.SellerUserID,
					"paid_cents", paidCents, "computed_cents", amount, "delta_cents", delta,
					"computed_reason", reason)
				// US-005: the audit trail must record the adjusted delta
				// itself, not just that a recompute happened.
				uc.auditPayload(ctx, actorEmail, "commission.adjustment", b.AccountID, map[string]any{
					"cycle":                  cycle,
					"seller_user_id":         adjustmentSeller,
					"paid_seller_user_id":    paidSeller,
					"current_seller_user_id": b.SellerUserID,
					"paid_cents":             paidCents,
					"computed_cents":         amount,
					"delta_cents":            delta,
					"computed_reason":        reason,
				})
			}
			if err := uc.repo.DeleteStaleProfitShare(ctx, b.AccountID, cycle, keepSeller, keepReason); err != nil {
				return nil, err
			}
		} else {
			entry := &domain.CommissionEntry{
				ID:                 ulid.Make().String(),
				SellerUserID:       b.SellerUserID,
				AccountID:          b.AccountID,
				Cycle:              cycle,
				BaseNetProfitCents: b.NetProfitCents,
				RateApplied:        rate,
				AmountCents:        amount,
				Reason:             reason,
				Status:             domain.CommissionStatusPending,
			}
			if err := uc.repo.UpsertEntry(ctx, entry); err != nil {
				return nil, err
			}
			if err := uc.repo.DeleteStaleProfitShare(ctx, b.AccountID, cycle, b.SellerUserID, reason); err != nil {
				return nil, err
			}
			written++
		}

		// Additive bonuses only reward ACTIVE management — a residual
		// (deactivated) seller earns the profit share alone.
		if reason == domain.CommissionReasonResidual {
			continue
		}
		if err := uc.grantNPSBonus(ctx, &b, cycle, bonusNPSCents); err != nil {
			return nil, err
		}
		if err := uc.grantRetentionBonus(ctx, &b, cycle, to, bonusRetentionCents); err != nil {
			return nil, err
		}
	}

	uc.audit(ctx, actorEmail, "commission.recompute", cycle)
	slog.Info("owner recomputed commissions",
		"actor", actorEmail, "cycle", cycle, "bases", len(bases), "entries", written)
	return uc.repo.ListEntriesByCycle(ctx, cycle)
}

// grantNPSBonus upserts the fixed promoter-NPS bonus (score 9-10 in the
// cycle), capped at one per account per calendar quarter — a grant in
// ANY quarter cycle, current included, by ANY seller and any status
// blocks this one (US-004: a seller reallocation must not re-mint the
// bonus under the new seller). 0 config disables. Idempotent: the
// already-granted row itself blocks the re-grant.
func (uc *ComputeCommissionsUseCase) grantNPSBonus(ctx context.Context, b *domain.CommissionBase, cycle string, bonusCents int64) error {
	if !b.HasPromoterNPS || bonusCents <= 0 {
		return nil
	}
	already, err := uc.repo.HasBonusEntry(ctx, b.AccountID, domain.CommissionReasonBonusNPS, quarterCycles(cycle))
	if err != nil || already {
		return err
	}
	return uc.repo.UpsertEntry(ctx, &domain.CommissionEntry{
		ID:           ulid.Make().String(),
		SellerUserID: b.SellerUserID,
		AccountID:    b.AccountID,
		Cycle:        cycle,
		RateApplied:  0,
		AmountCents:  bonusCents,
		Reason:       domain.CommissionReasonBonusNPS,
		Status:       domain.CommissionStatusPending,
	})
}

// grantRetentionBonus upserts the one-time 12-active-months bonus: the
// account's 12-month anniversary falls inside (or before) the cycle,
// the subscription is active, and no retention entry exists in ANY
// cycle under ANY seller (once-ever — the check surviving a seller
// reallocation is what keeps the bonus from doubling, US-004). 0 config
// disables.
func (uc *ComputeCommissionsUseCase) grantRetentionBonus(ctx context.Context, b *domain.CommissionBase, cycle string, cycleEnd time.Time, bonusCents int64) error {
	if bonusCents <= 0 || b.SubscriptionStatus != domain.SubscriptionStatusActive || b.AccountCreatedAt.IsZero() {
		return nil
	}
	if b.AccountCreatedAt.AddDate(0, retentionBonusMonths, 0).After(cycleEnd) {
		return nil // hasn't completed 12 months yet
	}
	already, err := uc.repo.HasBonusEntry(ctx, b.AccountID, domain.CommissionReasonBonusRetention, nil)
	if err != nil || already {
		return err
	}
	return uc.repo.UpsertEntry(ctx, &domain.CommissionEntry{
		ID:           ulid.Make().String(),
		SellerUserID: b.SellerUserID,
		AccountID:    b.AccountID,
		Cycle:        cycle,
		RateApplied:  0,
		AmountCents:  bonusCents,
		Reason:       domain.CommissionReasonBonusRetention,
		Status:       domain.CommissionStatusPending,
	})
}

// quarterCycles returns the three YYYY-MM cycles of the cycle's
// calendar quarter (the NPS bonus cap window).
func quarterCycles(cycle string) []string {
	t, _ := time.Parse("2006-01", cycle)
	qStart := time.Date(t.Year(), time.Month((int(t.Month())-1)/3*3+1), 1, 0, 0, 0, 0, time.UTC)
	return []string{
		qStart.Format("2006-01"),
		qStart.AddDate(0, 1, 0).Format("2006-01"),
		qStart.AddDate(0, 2, 0).Format("2006-01"),
	}
}

// getBonusSetting parses an integer-cents platform setting, falling
// back to the default when unwired, absent or corrupted.
func (uc *ComputeCommissionsUseCase) getBonusSetting(ctx context.Context, key string, def int64) (int64, error) {
	return getIntSetting(ctx, uc.settings, key, def)
}

// getIntSetting parses an integer platform setting, falling back to
// the default when the KV is unwired, the key absent or corrupted.
func getIntSetting(ctx context.Context, settings domain.PlatformSettingRepository, key string, def int64) (int64, error) {
	if settings == nil {
		return def, nil
	}
	raw, err := settings.GetSetting(ctx, key)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		slog.Warn("corrupted bonus setting, using default", "key", key, "raw", raw)
		return def, nil
	}
	return v, nil
}

// commissionRateFor resolves the profit-share (rate, reason) for one
// base. nil settings mean the defaults (active, 50/20). Empty reason =
// no entry (inactive past the residual window). "Inside residual_until"
// = the cycle's first day is on/before it. A plan upgrade in the cycle
// (US-008) pays the own-lead rate under reason 'bonus_upgrade' even on
// the inbound track. Precedence: activity > SLA > upgrade > track — an
// inactive seller stays on the residual regardless, and an SLA
// reduction (US-009, 2+ consecutive breaches) caps an active seller's
// cycle at the reduced rate, beating the upgrade bonus. The SLA cap
// never raises pay: a track rate already below it is kept.
func commissionRateFor(s *domain.SellerSettings, b *domain.CommissionBase, cycleStart time.Time, sla *domain.SellerSLAState) (float64, string) {
	rateOwn, rateInbound := DefaultCommissionRateOwn, DefaultCommissionRateInbound
	active := true
	var residualUntil *time.Time
	if s != nil {
		rateOwn, rateInbound = s.CommissionRateOwn, s.CommissionRateInbound
		active = s.IsActiveAM
		residualUntil = s.ResidualUntil
	}
	if active {
		trackRate := rateInbound
		if b.LeadSource != nil && *b.LeadSource == domain.LeadSourceSellerRef {
			trackRate = rateOwn
		}
		if slaReduced(sla) {
			rate := SLAReducedCommissionRate
			if trackRate < rate {
				rate = trackRate
			}
			return rate, domain.CommissionReasonSLAReduced
		}
		if b.HasPlanUpgrade {
			return rateOwn, domain.CommissionReasonBonusUpgrade
		}
		return trackRate, domain.CommissionReasonStandard
	}
	if residualUntil != nil && !cycleStart.After(*residualUntil) {
		return ResidualCommissionRate, domain.CommissionReasonResidual
	}
	return 0, ""
}

// commissionAmountCents applies a percent rate to the net-profit base
// with integer math (rate → basis points, one half-up rounding).
// Negative or zero base pays 0 — commission is never negative.
func commissionAmountCents(baseCents int64, rate float64) int64 {
	if baseCents <= 0 {
		return 0
	}
	rateBps := int64(math.Round(rate * 100))
	return roundedDiv(baseCents*rateBps, 10000)
}

// SetSellerActivity flips the activity clause for one seller.
// Deactivating stamps deactivated_at = now and residual_until = now +
// 12 months (the PRD's residual window); reactivating clears both.
func (uc *ComputeCommissionsUseCase) SetSellerActivity(ctx context.Context, actorEmail, sellerUserID string, isActive bool) (*domain.SellerSettings, error) {
	user, err := uc.users.GetByID(ctx, sellerUserID)
	if err != nil {
		return nil, err
	}
	if user == nil || (uc.platformAccountID != "" && user.AccountID != uc.platformAccountID) {
		return nil, errCommissionSellerNotFound()
	}

	var deactivatedAt, residualUntil *time.Time
	if !isActive {
		now := uc.now()
		until := now.AddDate(0, residualMonths, 0)
		deactivatedAt, residualUntil = &now, &until
	}
	if err := uc.repo.UpsertSellerActivity(ctx, sellerUserID, isActive, deactivatedAt, residualUntil); err != nil {
		return nil, err
	}

	uc.audit(ctx, actorEmail, "seller.set_activity", sellerUserID)
	slog.Info("owner set seller activity",
		"actor", actorEmail, "seller_user_id", sellerUserID, "is_active", isActive)
	return uc.repo.GetSellerSettings(ctx, sellerUserID)
}

// SetSellerFlag grants/revokes the explicit seller flag (US-014). Only
// flagged users reach the /seller panel, mint ref codes, resolve at
// signup or get stamped by convert-lead — being a platform-account user
// alone never makes someone payable.
func (uc *ComputeCommissionsUseCase) SetSellerFlag(ctx context.Context, actorEmail, sellerUserID string, isSeller bool) (*domain.SellerSettings, error) {
	user, err := uc.users.GetByID(ctx, sellerUserID)
	if err != nil {
		return nil, err
	}
	if user == nil || (uc.platformAccountID != "" && user.AccountID != uc.platformAccountID) {
		return nil, errCommissionSellerNotFound()
	}

	if err := uc.repo.SetSellerFlag(ctx, sellerUserID, isSeller); err != nil {
		return nil, err
	}

	uc.audit(ctx, actorEmail, "seller.set_flag", sellerUserID)
	slog.Info("owner set seller flag",
		"actor", actorEmail, "seller_user_id", sellerUserID, "is_seller", isSeller)
	return uc.repo.GetSellerSettings(ctx, sellerUserID)
}

// CommissionCycleListing bundles the cycle's ledger with its totals,
// the owner-editable bonus + SLA settings, the cycle's SLA ladder
// states and the informational detractor-NPS flags (US-009 — flags
// NEVER touch a rate) so the owner screen renders in one GET.
type CommissionCycleListing struct {
	Cycle               string                       `json:"cycle"`
	Items               []*domain.CommissionEntry    `json:"items"`
	TotalPendingCents   int64                        `json:"total_pending_cents"`
	TotalPaidCents      int64                        `json:"total_paid_cents"`
	BonusNPSCents       int64                        `json:"bonus_nps_cents"`
	BonusRetentionCents int64                        `json:"bonus_retention_cents"`
	SLAStates           []*domain.SellerSLAState     `json:"sla_states"`
	DetractorFlags      []domain.SellerDetractorFlag `json:"detractor_flags"`
	SLASettings         *SLASettings                 `json:"sla_settings"`
}

func (uc *ComputeCommissionsUseCase) List(ctx context.Context, cycle string) (*CommissionCycleListing, error) {
	from, to, err := parseCycle(cycle)
	if err != nil {
		return nil, err
	}
	items, err := uc.repo.ListEntriesByCycle(ctx, cycle)
	if err != nil {
		return nil, err
	}
	out := &CommissionCycleListing{Cycle: cycle, Items: items}
	for _, e := range items {
		if e.Status == domain.CommissionStatusPaid {
			out.TotalPaidCents += e.AmountCents
		} else {
			out.TotalPendingCents += e.AmountCents
		}
	}
	if out.BonusNPSCents, err = uc.getBonusSetting(ctx, domain.SettingBonusNPSCents, DefaultBonusNPSCents); err != nil {
		return nil, err
	}
	if out.BonusRetentionCents, err = uc.getBonusSetting(ctx, domain.SettingBonusRetentionCents, DefaultBonusRetentionCents); err != nil {
		return nil, err
	}
	if out.SLAStates, err = uc.repo.ListSLAStatesByCycle(ctx, cycle); err != nil {
		return nil, err
	}
	if out.DetractorFlags, err = uc.repo.ListDetractorFlags(ctx, from, to); err != nil {
		return nil, err
	}
	if out.SLASettings, err = uc.loadSLAConfig(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// SetBonusSettings persists the owner-edited bonus amounts (US-008).
// 0 disables a bonus; the rail keeps a typo from minting a fortune.
func (uc *ComputeCommissionsUseCase) SetBonusSettings(ctx context.Context, actorEmail string, npsCents, retentionCents int64) error {
	if uc.settings == nil {
		return &domain.AppError{Code: "COMMISSION_SETTINGS_UNAVAILABLE", Message: "platform settings are not configured"}
	}
	if npsCents < 0 || npsCents > maxBonusCents || retentionCents < 0 || retentionCents > maxBonusCents {
		return &domain.AppError{Code: "COMMISSION_INVALID_BONUS", Message: "bonus values must be between 0 and 1000000 cents"}
	}
	if err := uc.settings.SetSetting(ctx, domain.SettingBonusNPSCents, strconv.FormatInt(npsCents, 10), actorEmail); err != nil {
		return err
	}
	if err := uc.settings.SetSetting(ctx, domain.SettingBonusRetentionCents, strconv.FormatInt(retentionCents, 10), actorEmail); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, "commission.set_bonus_settings", domain.SettingBonusNPSCents)
	slog.Info("owner set commission bonus settings",
		"actor", actorEmail, "bonus_nps_cents", npsCents, "bonus_retention_cents", retentionCents)
	return nil
}

func (uc *ComputeCommissionsUseCase) audit(ctx context.Context, actorEmail, action, entityID string) {
	uc.auditPayload(ctx, actorEmail, action, entityID, nil)
}

// auditPayload records an audit entry with a structured JSON payload —
// used where the WHAT matters, not just that the action ran (US-005:
// the adjustment's delta, the payout's open-cycle override).
//
// Shape matters: audit_logs.account_id is NOT NULL + FK and actor_id
// is a users(id) FK, so commission audits land on the platform account
// with the actor EMAIL inside the payload (the owner-panel pattern).
// The pre-US-005 helper passed the email AS actor_id and left
// account_id empty — every commission audit write failed its FK checks
// and was silently dropped.
func (uc *ComputeCommissionsUseCase) auditPayload(ctx context.Context, actorEmail, action, entityID string, payload map[string]any) {
	if uc.auditLog == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["actor"] = actorEmail
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = uc.auditLog.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  uc.platformAccountID,
		Action:     action,
		EntityType: "commission",
		EntityID:   entityID,
		Payload:    raw,
		CreatedAt:  time.Now().UTC(),
	})
}
