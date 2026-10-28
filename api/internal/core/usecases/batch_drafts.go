package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Overnight drafts through the provider's Batch API (custo-BYOK): the same
// leads the synchronous night pass would draft, submitted as ONE job at 50%
// of the price, collected by a worker before the morning. The selection
// filters (draftable, opt-out, plan allowance) and the post-processing
// (metering, forbidden-claim repair, promise repair) are the SAME code the
// synchronous path runs — prepareDraft and completeDraft — so batch can
// never drift into a second, cheaper-but-sloppier drafting pipeline.

// SetBatchJobs wires the job store (optional; nil disables batch submit).
func (uc *ApproachUseCase) SetBatchJobs(r domain.AIBatchJobRepository) {
	uc.batchJobs = r
}

// SubmitOvernightBatch mirrors DraftBatchForAccount's selection but, instead
// of generating inline, submits every prepared prompt as one provider batch
// job. Returns ErrBatchUnsupported when the account's provider cannot batch
// — the caller falls back to the synchronous pass.
func (uc *ApproachUseCase) SubmitOvernightBatch(
	ctx context.Context, accountID, funnelID string, limit int,
) (submitted, optedOut, budget int, err error) {
	if uc.batchJobs == nil {
		return 0, 0, 0, domain.ErrBatchUnsupported
	}
	// One job in flight per account: re-submitting the same leads the next
	// night, before the first job was collected, would double-draft them.
	if open, err := uc.batchJobs.HasOpenJob(ctx, accountID, "overnight_drafts"); err != nil || open {
		if err != nil {
			return 0, 0, 0, err
		}
		return 0, 0, 0, &domain.AppError{Code: "AI_BATCH_IN_FLIGHT", Message: "a batch job is already awaiting collection"}
	}

	// Same funnel resolution as DraftBatchForAccount: a pinned funnel is the
	// customer's explicit choice (used as-is, ownership checked below), empty
	// means every active funnel.
	var funnels []*domain.SalesFunnel
	if f := strings.TrimSpace(funnelID); f != "" {
		funnel, getErr := uc.funnelRepo.GetByID(ctx, f)
		if getErr != nil {
			return 0, 0, 0, getErr
		}
		if funnel == nil || funnel.AccountID != accountID {
			return 0, 0, 0, domain.ErrSalesFunnelNotFound
		}
		funnels = []*domain.SalesFunnel{funnel}
	} else {
		all, listErr := uc.funnelRepo.ListByAccountID(ctx, accountID)
		if listErr != nil {
			return 0, 0, 0, listErr
		}
		for _, f := range all {
			if f.Active {
				funnels = append(funnels, f)
			}
		}
	}

	if limit <= 0 {
		limit = defaultDraftBatch
	}
	if limit > maxDraftBatch {
		limit = maxDraftBatch
	}
	allowance, err := uc.spend.remainingFor(ctx, accountID, domain.SpendAIDraft, limit)
	if err != nil {
		return 0, 0, 0, err
	}

	var reqs []domain.AIBatchRequest
	var items []domain.AIBatchItem
	var provider domain.AIProvider
	var plan *domain.Plan

	for _, funnel := range funnels {
		if len(reqs) >= limit {
			break
		}
		leads, listErr := uc.leadRepo.ListByFunnel(ctx, funnel.ID, "", int32(maxDraftBatch), 0) //nolint:gosec // G115: small constant
		if listErr != nil {
			continue
		}
		for _, lead := range leads {
			if len(reqs) >= limit {
				break
			}
			if !draftable(lead.Status) {
				continue
			}
			if uc.leadOptedOut(ctx, accountID, lead) {
				optedOut++
				continue
			}
			if len(reqs) >= allowance {
				budget++
				continue
			}
			prep, prepErr := uc.prepareDraft(ctx, accountID, funnel, lead)
			if prepErr != nil {
				continue
			}
			provider, plan = prep.provider, prep.plan
			reqs = append(reqs, domain.AIBatchRequest{
				Key:          lead.ID,
				SystemPrompt: prep.systemPrompt,
				UserContent:  prep.payload,
			})
			items = append(items, domain.AIBatchItem{
				LeadID: lead.ID, FunnelID: funnel.ID,
				SystemPrompt: prep.systemPrompt, Payload: prep.payload,
			})
			// The matcher's pick and the anticipated objections were stamped
			// on the lead struct by prepareDraft; persist them NOW, because
			// the collector no longer re-runs the matcher when it lands.
			if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
				slog.WarnContext(ctx, "batch submit: could not persist the matcher pick",
					"lead_id", lead.ID, "error", err)
			}
		}
	}
	if len(reqs) == 0 {
		return 0, optedOut, budget, nil
	}

	batcher, ok := provider.(domain.AIBatchProvider)
	if !ok {
		return 0, optedOut, budget, domain.ErrBatchUnsupported
	}
	jobRef, err := batcher.SubmitBatch(ctx, "overnight-"+accountID, reqs)
	if err != nil {
		return 0, optedOut, budget, fmt.Errorf("batch submit: %w", err)
	}
	job := &domain.AIBatchJob{
		ID: ulid.Make().String(), AccountID: accountID, Kind: "overnight_drafts",
		Provider: plan.AIProvider, Model: plan.AIModel, JobRef: jobRef, Items: items,
	}
	if err := uc.batchJobs.Create(ctx, job); err != nil {
		// The provider has the job but we lost the handle — the money is
		// spent and the results are unreachable. Loud, because this is the
		// one failure that costs without delivering.
		return 0, optedOut, budget, fmt.Errorf("batch job persist (job %s is orphaned at the provider): %w", jobRef, err)
	}
	return len(reqs), optedOut, budget, nil
}

// CollectBatchDrafts polls every in-flight job once and lands the finished
// answers in the approval queue. Returns how many drafts landed.
func (uc *ApproachUseCase) CollectBatchDrafts(ctx context.Context) int {
	if uc.batchJobs == nil {
		return 0
	}
	jobs, err := uc.batchJobs.ListSubmitted(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "batch collect: job listing failed", "error", err)
		return 0
	}
	landed := 0
	for _, job := range jobs {
		landed += uc.collectOne(ctx, job)
	}
	return landed
}

func (uc *ApproachUseCase) collectOne(ctx context.Context, job *domain.AIBatchJob) int {
	// Any lead of the job resolves the same account provider; the first
	// item's funnel is only used to re-prepare per lead below.
	byLead := map[string]domain.AIBatchItem{}
	for _, it := range job.Items {
		byLead[it.LeadID] = it
	}
	if len(byLead) == 0 {
		_ = uc.batchJobs.SetStatus(ctx, job.ID, domain.AIBatchFailed, "job had no items")
		return 0
	}

	// Resolve a provider to poll with (the account's own key).
	//
	// Walks job.Items, a SLICE, not the byLead map: Go randomises map order,
	// so probing the map meant the lead this touched — and therefore which
	// lead's funnel and account state had to be readable for the whole job
	// to be collectable — changed between two runs on identical data. A job
	// that collects four times out of five is worse than one that always
	// fails, because the failure looks like a fluke.
	var probe *draftPrep
	for _, item := range job.Items {
		leadID := item.LeadID
		lead, err := uc.leadRepo.GetByID(ctx, leadID)
		if err != nil || lead == nil {
			continue
		}
		funnel, err := uc.funnelRepo.GetByID(ctx, item.FunnelID)
		if err != nil || funnel == nil {
			continue
		}
		// Context only: the probe needs a provider, not a payload.
		if p, err := uc.prepareDraftContext(ctx, job.AccountID, funnel, lead); err == nil {
			probe = p
			break
		}
	}
	if probe == nil {
		_ = uc.batchJobs.SetStatus(ctx, job.ID, domain.AIBatchFailed, "no lead of the job is preparable any more")
		return 0
	}
	batcher, ok := probe.provider.(domain.AIBatchProvider)
	if !ok {
		_ = uc.batchJobs.SetStatus(ctx, job.ID, domain.AIBatchFailed, "provider lost batch capability")
		return 0
	}

	done, results, err := batcher.FetchBatch(ctx, job.JobRef)
	if err != nil {
		if done {
			// Job-level failure: leads stay draftable, the next night runs
			// synchronously (the in-flight guard clears with the status).
			_ = uc.batchJobs.SetStatus(ctx, job.ID, domain.AIBatchFailed, err.Error())
		} else {
			slog.WarnContext(ctx, "batch collect: poll failed, retrying next tick",
				"job_id", job.ID, "error", err)
		}
		return 0
	}
	if !done {
		return 0
	}

	landed := 0
	for _, res := range results {
		item, known := byLead[res.Key]
		if !known || strings.TrimSpace(res.Err) != "" || res.Text == "" {
			continue
		}
		if uc.landBatchDraft(ctx, job.AccountID, item, res) {
			landed++
		}
	}
	_ = uc.batchJobs.SetStatus(ctx, job.ID, domain.AIBatchCollected, "")
	slog.InfoContext(ctx, "batch collect: job landed",
		"job_id", job.ID, "account_id", job.AccountID, "drafts", landed, "of", len(job.Items))
	return landed
}

// landBatchDraft runs one collected answer through the SAME post-processing
// and persistence the synchronous draft uses.
func (uc *ApproachUseCase) landBatchDraft(ctx context.Context, accountID string, item domain.AIBatchItem, res domain.AIBatchResult) bool {
	leadID, funnelID := item.LeadID, item.FunnelID
	lead, err := uc.leadRepo.GetByID(ctx, leadID)
	if err != nil || lead == nil || lead.AccountID != accountID {
		return false
	}
	// The night is long: a lead somebody already worked (drafted by hand,
	// contacted, lost) must not be overwritten by a stale batch answer.
	if !draftable(lead.Status) || strings.TrimSpace(lead.ApproachDraft) != "" {
		return false
	}
	funnel, err := uc.funnelRepo.GetByID(ctx, funnelID)
	if err != nil || funnel == nil {
		return false
	}
	// The prompt that was sent travels on the item, so landing costs no
	// model call. A job from before that field existed re-prepares.
	var prep *draftPrep
	if item.Payload != "" {
		prep, err = uc.prepareDraftContext(ctx, accountID, funnel, lead)
		if err == nil {
			prep.payload = item.Payload
			if item.SystemPrompt != "" {
				prep.systemPrompt = item.SystemPrompt
			}
		}
	} else {
		prep, err = uc.prepareDraft(ctx, accountID, funnel, lead)
	}
	if err != nil {
		return false
	}
	text, err := uc.completeDraft(ctx, prep, res.Text, res.Usage)
	if err != nil {
		return false
	}
	lead.ApproachDraft = text
	lead.Status = domain.LeadStatusPendingApproval
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		slog.WarnContext(ctx, "batch collect: lead persist failed", "lead_id", leadID, "error", err)
		return false
	}
	return true
}
