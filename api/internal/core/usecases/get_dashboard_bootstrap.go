package usecases

import (
	"context"
	"log/slog"
	"sync"

	"github.com/replider/api/internal/core/domain"
)

// GetDashboardBootstrapUseCase is the BFF (Backend-for-Frontend) aggregator
// behind GET /v1/me/dashboard. It composes the half-dozen independent reads the
// Flutter dashboard fires on cold start (identity, plan limits, usage counters,
// agents, channels, recent conversations) into a SINGLE response so the client
// pays one HTTP round-trip + one auth-middleware pass instead of six.
//
// Design notes (see docs/bff-decision.md):
//   - This is a thin orchestration layer over existing use cases / repos. It
//     owns NO business logic of its own — each section is produced by the same
//     code path the dedicated REST endpoint uses, so the two can never drift.
//   - Identity (`me`) is essential: a failure there aborts the whole bootstrap
//     (the dashboard cannot render without it). Every other section is
//     best-effort — a failing subsystem degrades to an empty section plus a
//     `warnings` entry rather than blanking the entire dashboard. This mirrors
//     how the REST client behaves when one of its parallel calls 500s.
//   - The secondary reads run concurrently (each pulls its own pgx connection
//     from the pool) so server-side latency stays close to the slowest single
//     query, matching what the client got from issuing the REST calls in
//     parallel.
type GetDashboardBootstrapUseCase struct {
	getMeUC        *GetMeUseCase
	getUsageUC     *GetUsageUseCase
	listAgentsUC   *ListAgentsUseCase
	listChannelsUC *ListChannelsUseCase
	convRepo       domain.ConversationRepository
}

func NewGetDashboardBootstrapUseCase(
	getMeUC *GetMeUseCase,
	getUsageUC *GetUsageUseCase,
	listAgentsUC *ListAgentsUseCase,
	listChannelsUC *ListChannelsUseCase,
	convRepo domain.ConversationRepository,
) *GetDashboardBootstrapUseCase {
	return &GetDashboardBootstrapUseCase{
		getMeUC:        getMeUC,
		getUsageUC:     getUsageUC,
		listAgentsUC:   listAgentsUC,
		listChannelsUC: listChannelsUC,
		convRepo:       convRepo,
	}
}

// recentConversationsLimit caps the conversation preview baked into the
// bootstrap payload. The dashboard lazily pages the full list afterwards; the
// bootstrap only needs enough rows to paint the first screen.
const recentConversationsLimit = 20

// DashboardBootstrapResponse is the aggregated payload. Each field is the exact
// shape the equivalent standalone endpoint returns, so a client can route a
// section straight into the same model it already uses.
type DashboardBootstrapResponse struct {
	Me                  *GetMeResponse                `json:"me"`
	PlanLimits          *EffectivePlanLimits          `json:"plan_limits,omitempty"`
	Usage               *GetUsageResponse             `json:"usage,omitempty"`
	Agents              []*domain.Agent               `json:"agents"`
	Channels            []*domain.Channel             `json:"channels"`
	RecentConversations []*domain.ConversationSummary `json:"recent_conversations"`
	// Warnings names the sections that failed to load (best-effort reads). Empty
	// on a fully successful bootstrap. The client can decide whether to retry
	// the affected section via its dedicated endpoint.
	Warnings []string `json:"warnings,omitempty"`
}

// Execute aggregates the dashboard bootstrap for the calling user/account.
// Returns an error only when identity resolution fails; all other sections are
// best-effort and surface via Warnings.
func (uc *GetDashboardBootstrapUseCase) Execute(ctx context.Context, userID, accountID string) (*DashboardBootstrapResponse, error) {
	// Identity is the one hard dependency — abort if it fails.
	me, err := uc.getMeUC.Execute(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}

	resp := &DashboardBootstrapResponse{
		Me:                  me,
		Agents:              []*domain.Agent{},
		Channels:            []*domain.Channel{},
		RecentConversations: []*domain.ConversationSummary{},
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	warn := func(section string, err error) {
		slog.WarnContext(ctx, "dashboard bootstrap section failed",
			"section", section, "account_id", accountID, "error", err.Error())
		mu.Lock()
		resp.Warnings = append(resp.Warnings, section)
		mu.Unlock()
	}

	// Each secondary read runs concurrently and pulls its own pgx connection.
	wg.Add(5)

	go func() {
		defer wg.Done()
		limits, err := uc.getMeUC.ResolvePlanLimits(ctx, accountID)
		if err != nil {
			warn("plan_limits", err)
			return
		}
		resp.PlanLimits = limits
	}()

	go func() {
		defer wg.Done()
		usage, err := uc.getUsageUC.Execute(ctx, accountID)
		if err != nil {
			warn("usage", err)
			return
		}
		resp.Usage = usage
	}()

	go func() {
		defer wg.Done()
		agents, err := uc.listAgentsUC.Execute(ctx, accountID)
		if err != nil {
			warn("agents", err)
			return
		}
		if agents != nil {
			resp.Agents = agents
		}
	}()

	go func() {
		defer wg.Done()
		channels, err := uc.listChannelsUC.Execute(ctx, accountID)
		if err != nil {
			warn("channels", err)
			return
		}
		if channels != nil {
			resp.Channels = channels
		}
	}()

	go func() {
		defer wg.Done()
		// "open" mirrors HandleListConversations' default landing filter.
		convs, err := uc.convRepo.ListEnriched(ctx, accountID, "open", recentConversationsLimit, 0, "", "", "")
		if err != nil {
			warn("recent_conversations", err)
			return
		}
		if convs != nil {
			resp.RecentConversations = convs
		}
	}()

	wg.Wait()
	return resp, nil
}
