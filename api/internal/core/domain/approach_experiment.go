package domain

import (
	"context"
	"strings"
	"time"
)

// A/B of approach writing, judged by whether a human approved the draft
// WITHOUT editing it (US-F216, the feasible slice).
//
// Reply rate is the metric everyone wants and nobody can afford yet: at a 10%
// baseline, detecting a 30% lift needs ~3,500 sends — seventeen months at the
// entry tier's volume. An optimiser fed twenty replies does not optimise; it
// chases noise and picks a loser with confidence.
//
// Approval-without-editing is dense and immediate: it exists the moment the
// operator acts, and at a 60% baseline detects a 20% lift in ~490 drafts.
// Reply rate keeps accumulating alongside and becomes the criterion once
// there is volume to support it.

type ApproachExperimentStatus string

const (
	ApproachExperimentRunning ApproachExperimentStatus = "running"
	ApproachExperimentStopped ApproachExperimentStatus = "stopped"
)

// ApproachOutcome is what the human did with the draft.
type ApproachOutcome string

const (
	// ApproachOutcomePending is a draft nobody has acted on yet. It counts
	// toward neither arm: an undecided draft is not evidence.
	ApproachOutcomePending   ApproachOutcome = "pending"
	ApproachOutcomeUnedited  ApproachOutcome = "approved_unedited"
	ApproachOutcomeEdited    ApproachOutcome = "approved_edited"
	ApproachOutcomeDiscarded ApproachOutcome = "discarded"
)

type ApproachExperiment struct {
	ID        string                   `json:"id"`
	AccountID string                   `json:"account_id"`
	FunnelID  string                   `json:"funnel_id"`
	Name      string                   `json:"name"`
	Status    ApproachExperimentStatus `json:"status"`

	// Each variant is a suffix APPENDED to the funnel's persona, never a
	// replacement: the experiment compares one change, not two products.
	VariantASuffix string `json:"variant_a_suffix"`
	VariantBSuffix string `json:"variant_b_suffix"`
	VariantALabel  string `json:"variant_a_label"`
	VariantBLabel  string `json:"variant_b_label"`
	// Optional per-variant model, so MODEL can be the thing under test
	// without a second experiment machine.
	VariantAModel string `json:"variant_a_model"`
	VariantBModel string `json:"variant_b_model"`

	SplitPctB int       `json:"split_pct_b"`
	Winner    *string   `json:"winner,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SuffixFor returns the persona addition for a variant.
func (e *ApproachExperiment) SuffixFor(variant string) string {
	if variant == "B" {
		return e.VariantBSuffix
	}
	return e.VariantASuffix
}

// ModelFor returns the model override for a variant, empty meaning "whatever
// the plan says".
func (e *ApproachExperiment) ModelFor(variant string) string {
	if variant == "B" {
		return strings.TrimSpace(e.VariantBModel)
	}
	return strings.TrimSpace(e.VariantAModel)
}

// LabelFor is the human name of a variant, for reporting.
func (e *ApproachExperiment) LabelFor(variant string) string {
	if variant == "B" {
		return e.VariantBLabel
	}
	return e.VariantALabel
}

type ApproachVariantResult struct {
	ID           string          `json:"id"`
	ExperimentID string          `json:"experiment_id"`
	LeadID       string          `json:"lead_id"`
	Variant      string          `json:"variant"`
	DraftedText  string          `json:"drafted_text"`
	Outcome      ApproachOutcome `json:"outcome"`
	EditRatio    float32         `json:"edit_ratio"`
	RiskCount    int             `json:"risk_count"`
	DecidedAt    *time.Time      `json:"decided_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// ApproachArmStats is one arm's tally.
type ApproachArmStats struct {
	Variant   string `json:"variant"`
	Label     string `json:"label"`
	Drafted   int    `json:"drafted"`
	Decided   int    `json:"decided"`
	Unedited  int    `json:"approved_unedited"`
	Edited    int    `json:"approved_edited"`
	Discarded int    `json:"discarded"`
	// UneditedRate is over DECIDED drafts, not over drafted: a queue nobody
	// has worked through yet would otherwise read as a losing arm.
	UneditedRate float64 `json:"unedited_rate"`
	AvgEditRatio float64 `json:"avg_edit_ratio"`
	AvgRisks     float64 `json:"avg_risks"`
}

type ApproachExperimentRepository interface {
	GetRunningByFunnel(ctx context.Context, funnelID string) (*ApproachExperiment, error)
	GetByID(ctx context.Context, id string) (*ApproachExperiment, error)
	Create(ctx context.Context, e *ApproachExperiment) error
	Stop(ctx context.Context, id, winner string) error
	ListByAccount(ctx context.Context, accountID string, limit int32) ([]*ApproachExperiment, error)

	// UpsertResult records (or replaces) a lead's draft under an experiment.
	UpsertResult(ctx context.Context, r *ApproachVariantResult) error
	// GetResult reads a lead's row, needed to compare the sent text against
	// the draft AS GENERATED.
	GetResult(ctx context.Context, experimentID, leadID string) (*ApproachVariantResult, error)
	// SetOutcome closes a result. Idempotent by (experiment, lead).
	SetOutcome(ctx context.Context, experimentID, leadID string, outcome ApproachOutcome, editRatio float32) error
	// Stats tallies both arms.
	Stats(ctx context.Context, experimentID string) ([]ApproachArmStats, error)
}

// AssignVariant picks the arm for a lead, deterministically.
//
// Deterministic on purpose: a lead re-drafted must land in the SAME arm.
// Random assignment would let someone regenerate until they got the variant
// they preferred, which quietly turns the experiment into a preference poll.
func AssignVariant(leadID string, splitPctB int) string {
	if splitPctB <= 0 {
		return "A"
	}
	if splitPctB >= 100 {
		return "B"
	}
	// FNV-1a over the lead id: stable across processes and deploys, unlike
	// Go's map hash, which is seeded per process.
	var h uint32 = 2166136261
	for i := 0; i < len(leadID); i++ {
		h ^= uint32(leadID[i])
		h *= 16777619
	}
	if int(h%100) < splitPctB {
		return "B"
	}
	return "A"
}

// EditRatio measures how much of the draft survived, as a fraction changed.
// 0 means sent verbatim; 1 means nothing of the draft remains.
//
// Character-level Levenshtein over runes: a boolean "edited" would score a
// fixed typo the same as a full rewrite, and those are opposite verdicts on
// the draft's quality.
func EditRatio(draft, sent string) float32 {
	d, s := []rune(strings.TrimSpace(draft)), []rune(strings.TrimSpace(sent))
	if len(d) == 0 && len(s) == 0 {
		return 0
	}
	if len(d) == 0 || len(s) == 0 {
		return 1
	}
	prev := make([]int, len(s)+1)
	cur := make([]int, len(s)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(d); i++ {
		cur[0] = i
		for j := 1; j <= len(s); j++ {
			cost := 1
			if d[i-1] == s[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	longest := len(d)
	if len(s) > longest {
		longest = len(s)
	}
	r := float32(prev[len(s)]) / float32(longest)
	if r > 1 {
		return 1
	}
	return r
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
