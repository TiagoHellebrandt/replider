package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Signal thresholds (US-014). Fixed by the PRD's commission model, not
// owner-tunable settings: the co-pilot is advisory, so a threshold
// change is a code change, not a payroll variable.
const (
	suggestionTakeoverPct      = 30 // takeover rate strictly above this → add knowledge
	suggestionUsagePct         = 80 // >= this % of the plan allowance → suggest upgrade
	suggestionOnboardingDays   = 7  // checklist still < 100% after this many days → stuck
	suggestionChurnMinElapsed  = 7  // days into the month before the pace signal is trusted
	suggestionCacheTTL         = 60 * time.Second
	suggestionNPSPromoterScore = 9 // >= promoter, <= 6 detractor (domain convention)
	suggestionNPSDetractorMax  = 6
)

// Narrow local interfaces (mock only what the engine touches).
type suggestionCarteiraRepo interface {
	ListCarteiraAccounts(ctx context.Context, sellerUserID, cycle string) ([]domain.SellerCarteiraAccount, error)
}

// suggestionOnboardingReader is the one remaining per-account read: the
// onboarding EXISTS lookups. Account fields, plan id and usage counters
// all ride on the carteira row itself (US-022 N+1 fix).
type suggestionOnboardingReader interface {
	GetOnboardingProgress(ctx context.Context, accountID string) (*domain.OnboardingProgress, error)
}

type suggestionPlanReader interface {
	GetByID(ctx context.Context, id string) (*domain.Plan, error)
}

// SellerSuggestionsUseCase is the co-pilot signal engine (US-014): for
// each carteira account it derives proactive suggestions from data the
// platform already collects — takeover rate, conversation pace, plan
// allowance usage, onboarding checklist, WhatsApp ban risk and NPS.
// Everything is computed on demand behind a short in-memory cache (no
// new worker); only dismissals are persisted.
type SellerSuggestionsUseCase struct {
	carteira suggestionCarteiraRepo
	repo     domain.SellerSuggestionRepository
	accounts suggestionOnboardingReader
	plans    suggestionPlanReader
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]suggestionCacheEntry
	// gen invalidation counter per seller: a store whose compute began
	// before the latest invalidate is stale (it read pre-dismiss state)
	// and must not overwrite the invalidation (US-022 cache race).
	gen map[string]uint64
}

type suggestionCacheEntry struct {
	feed *SellerSuggestionsFeed
	at   time.Time
}

func NewSellerSuggestionsUseCase(carteira suggestionCarteiraRepo, repo domain.SellerSuggestionRepository, accounts suggestionOnboardingReader, plans suggestionPlanReader) *SellerSuggestionsUseCase {
	return &SellerSuggestionsUseCase{
		carteira: carteira,
		repo:     repo,
		accounts: accounts,
		plans:    plans,
		now:      time.Now,
		cache:    map[string]suggestionCacheEntry{},
		gen:      map[string]uint64{},
	}
}

// SellerSuggestionsFeed is the GET /seller/suggestions payload.
type SellerSuggestionsFeed struct {
	Cycle       string                    `json:"cycle"`
	Suggestions []domain.SellerSuggestion `json:"suggestions"`
}

var suggestionSeverityRank = map[string]int{
	domain.SuggestionSeverityCritical: 0,
	domain.SuggestionSeverityWarning:  1,
	domain.SuggestionSeverityInfo:     2,
}

// actionTextFor returns the ready-made action text per suggestion type.
// Portuguese on purpose — the seller team is the Replider platform
// account; the app layer (US-015) owns localized templates.
func actionTextFor(sType string) string {
	switch sType {
	case domain.SuggestionAddKnowledge:
		return "Taxa de takeover alta: adicione conhecimento à base do agente para reduzir intervenções humanas."
	case domain.SuggestionChurnRiskCheckin:
		return "Volume de conversas caindo forte: faça um check-in com o cliente — risco de churn."
	case domain.SuggestionSuggestUpgrade:
		return "Uso perto do limite da franquia: sugira um upgrade de plano."
	case domain.SuggestionOnboardingStuck:
		return "Onboarding parado há mais de uma semana: ajude o cliente no passo travado."
	case domain.SuggestionWhatsAppBanRisk:
		return "Número WhatsApp com qualidade RED (risco de ban): oriente o cliente sobre boas práticas imediatamente."
	case domain.SuggestionNPSPromoter:
		return "Cliente promotor (NPS 9-10): peça uma indicação com o seu link ?ref=."
	case domain.SuggestionNPSDetractor:
		return "Cliente detrator (NPS 0-6): converse com ele antes que vire churn."
	}
	return ""
}

// Execute computes the seller's suggestion feed for the current cycle,
// serving a <=60s-old cached copy when available.
func (uc *SellerSuggestionsUseCase) Execute(ctx context.Context, sellerUserID string) (*SellerSuggestionsFeed, error) {
	nowT := uc.now().UTC()
	feed, genSnapshot := uc.cached(sellerUserID, nowT)
	if feed != nil {
		return feed, nil
	}

	cycle := nowT.Format("2006-01")
	monthStart := time.Date(nowT.Year(), nowT.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevStart := monthStart.AddDate(0, -1, 0)

	// Carteira is the account universe; the cycle param only feeds the
	// profit columns we don't use here — last closed month keeps the
	// query on its warm path.
	accounts, err := uc.carteira.ListCarteiraAccounts(ctx, sellerUserID, prevStart.Format("2006-01"))
	if err != nil {
		return nil, fmt.Errorf("list carteira: %w", err)
	}
	feed = &SellerSuggestionsFeed{Cycle: cycle, Suggestions: []domain.SellerSuggestion{}}
	if len(accounts) == 0 {
		uc.store(sellerUserID, nowT, feed, genSnapshot)
		return feed, nil
	}

	convStats, err := uc.repo.ListConversationStats(ctx, sellerUserID, monthStart, nowT, prevStart, monthStart)
	if err != nil {
		return nil, fmt.Errorf("conversation stats: %w", err)
	}
	statsByAccount := map[string]domain.SellerConvStats{}
	for _, s := range convStats {
		statsByAccount[s.AccountID] = s
	}

	risks, err := uc.repo.ListWhatsAppRisk(ctx, sellerUserID)
	if err != nil {
		return nil, fmt.Errorf("whatsapp risk: %w", err)
	}
	riskByAccount := map[string][]domain.SellerChannelRisk{}
	for _, r := range risks {
		riskByAccount[r.AccountID] = append(riskByAccount[r.AccountID], r)
	}

	npsSignals, err := uc.repo.ListLatestNPS(ctx, sellerUserID, monthStart, nowT)
	if err != nil {
		return nil, fmt.Errorf("nps signals: %w", err)
	}
	npsByAccount := map[string]domain.SellerNPSSignal{}
	for _, n := range npsSignals {
		npsByAccount[n.AccountID] = n
	}

	states, err := uc.repo.ListSuggestionStates(ctx, sellerUserID, cycle)
	if err != nil {
		return nil, fmt.Errorf("suggestion states: %w", err)
	}
	// account|type|channel dismissed or done this cycle (channel "" for
	// account-level types — ban-risk dismissals are per channel).
	handled := map[string]bool{}
	for _, s := range states {
		handled[s.AccountID+"|"+s.SuggestionType+"|"+s.ChannelID] = true
	}

	add := func(acc domain.SellerCarteiraAccount, sType, channelID, severity string, signal map[string]interface{}) {
		if handled[acc.AccountID+"|"+sType+"|"+channelID] {
			return
		}
		var chPtr *string
		if channelID != "" {
			ch := channelID
			chPtr = &ch
		}
		feed.Suggestions = append(feed.Suggestions, domain.SellerSuggestion{
			Type:           sType,
			AccountID:      acc.AccountID,
			AccountName:    acc.Name,
			Severity:       severity,
			Signal:         signal,
			ActionText:     actionTextFor(sType),
			ConversationID: acc.ConvertedConversationID,
			ChannelID:      chPtr,
		})
	}

	// Plans repeat across a carteira — memoize per compute pass instead
	// of one GetByID per account (nil entries cache "not found" too).
	planCache := map[string]*domain.Plan{}

	for _, acc := range accounts {
		cs := statsByAccount[acc.AccountID]
		account := acc.EngineAccount()

		// (1) Takeover rate > 30% this month → add knowledge.
		if cs.CurrentTotal > 0 && cs.CurrentHuman*100 > cs.CurrentTotal*suggestionTakeoverPct {
			add(acc, domain.SuggestionAddKnowledge, "", domain.SuggestionSeverityWarning, map[string]interface{}{
				"takeover_pct":        cs.CurrentHuman * 100 / cs.CurrentTotal,
				"human_conversations": cs.CurrentHuman,
				"total_conversations": cs.CurrentTotal,
			})
		}

		// (2) Conversation pace < 50% of last month → churn check-in.
		// The current month is partial, so last month is pro-rated to
		// the elapsed days; the first week is too noisy to trust.
		elapsed := nowT.Day()
		daysInMonth := time.Date(nowT.Year(), nowT.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if elapsed >= suggestionChurnMinElapsed && cs.PrevTotal > 0 &&
			cs.CurrentTotal*daysInMonth*2 < cs.PrevTotal*elapsed {
			add(acc, domain.SuggestionChurnRiskCheckin, "", domain.SuggestionSeverityCritical, map[string]interface{}{
				"current_month_conversations":  cs.CurrentTotal,
				"previous_month_conversations": cs.PrevTotal,
				"elapsed_days":                 elapsed,
			})
		}

		// (3) >= 80% of the plan allowance (conversations or tokens) →
		// suggest upgrade. Limits via account.EffectiveMax* — never the
		// raw plan caps (per-account overrides + contract plans).
		if err := uc.addUsageSignal(ctx, acc, account, planCache, add); err != nil {
			return nil, err
		}

		// (4) Onboarding checklist < 100% after 7 days → help unblock.
		if err := uc.addOnboardingSignal(ctx, acc, account, nowT, add); err != nil {
			return nil, err
		}

		// (5) WhatsApp numbers flagged RED by the quality worker — one
		// suggestion PER channel so dismissing one number never hides
		// another that goes RED later (US-022).
		for _, ch := range riskByAccount[acc.AccountID] {
			add(acc, domain.SuggestionWhatsAppBanRisk, ch.ChannelID, domain.SuggestionSeverityCritical, map[string]interface{}{
				"quality_rating": "RED",
				"channels":       []string{ch.ChannelName},
			})
		}

		// (6) NPS answered this month: promoter → ask for a referral,
		// detractor → act before churn. Passives (7-8) stay silent.
		if n, ok := npsByAccount[acc.AccountID]; ok {
			if n.Score >= suggestionNPSPromoterScore {
				add(acc, domain.SuggestionNPSPromoter, "", domain.SuggestionSeverityInfo, map[string]interface{}{
					"score": n.Score,
				})
			} else if n.Score <= suggestionNPSDetractorMax {
				add(acc, domain.SuggestionNPSDetractor, "", domain.SuggestionSeverityCritical, map[string]interface{}{
					"score": n.Score,
				})
			}
		}
	}

	sort.SliceStable(feed.Suggestions, func(i, j int) bool {
		a, b := feed.Suggestions[i], feed.Suggestions[j]
		if suggestionSeverityRank[a.Severity] != suggestionSeverityRank[b.Severity] {
			return suggestionSeverityRank[a.Severity] < suggestionSeverityRank[b.Severity]
		}
		if a.AccountName != b.AccountName {
			return a.AccountName < b.AccountName
		}
		return a.Type < b.Type
	})

	uc.store(sellerUserID, nowT, feed, genSnapshot)
	return feed, nil
}

func (uc *SellerSuggestionsUseCase) planFor(ctx context.Context, cache map[string]*domain.Plan, planID string) (*domain.Plan, error) {
	if p, ok := cache[planID]; ok {
		return p, nil
	}
	p, err := uc.plans.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan %s: %w", planID, err)
	}
	cache[planID] = p
	return p, nil
}

func (uc *SellerSuggestionsUseCase) addUsageSignal(ctx context.Context, acc domain.SellerCarteiraAccount, account *domain.Account, planCache map[string]*domain.Plan, add func(domain.SellerCarteiraAccount, string, string, string, map[string]interface{})) error {
	plan := domain.TrialPlan()
	if account.PlanID != nil {
		p, err := uc.planFor(ctx, planCache, *account.PlanID)
		if err != nil {
			return err
		}
		if p != nil {
			plan = p
		}
	}

	convUsed := int64(0)
	if acc.UsageConversations != nil {
		convUsed = int64(*acc.UsageConversations)
	}
	tokensUsed := int64(0)
	if acc.UsageAITokens != nil {
		tokensUsed = *acc.UsageAITokens
	}
	addon := int64(0)
	if acc.UsageAddonTokens != nil {
		addon = *acc.UsageAddonTokens
	}

	// A contract account gets no "suggest an upgrade": there is no shelf to move
	// up to, its number is negotiated. Since the unnegotiated case started
	// answering with the platform ceiling (rather than infinity), without this
	// guard the seller would get the wrong advice for the wrong customer.
	if account.IsContractPlan(plan) {
		return nil
	}

	convMax := int64(account.EffectiveMaxConversationsPerMonth(plan))
	tokenMax := account.EffectiveMaxAITokensPerMonth(plan) + addon

	convHit := convMax > 0 && convUsed*100 >= convMax*suggestionUsagePct
	tokenHit := tokenMax > 0 && tokensUsed*100 >= tokenMax*suggestionUsagePct
	if !convHit && !tokenHit {
		return nil
	}
	signal := map[string]interface{}{}
	if convHit {
		signal["conversations_used"] = convUsed
		signal["conversations_limit"] = convMax
	}
	if tokenHit {
		signal["tokens_used"] = tokensUsed
		signal["tokens_limit"] = tokenMax
	}
	add(acc, domain.SuggestionSuggestUpgrade, "", domain.SuggestionSeverityInfo, signal)
	return nil
}

func (uc *SellerSuggestionsUseCase) addOnboardingSignal(ctx context.Context, acc domain.SellerCarteiraAccount, account *domain.Account, nowT time.Time, add func(domain.SellerCarteiraAccount, string, string, string, map[string]interface{})) error {
	if account.CreatedAt.After(nowT.AddDate(0, 0, -suggestionOnboardingDays)) {
		return nil
	}
	progress, err := uc.accounts.GetOnboardingProgress(ctx, acc.AccountID)
	if err != nil {
		return fmt.Errorf("onboarding progress %s: %w", acc.AccountID, err)
	}
	if progress == nil {
		// Account deleted between the carteira read and here — skip it,
		// never 500 the whole feed (US-022).
		slog.Warn("seller suggestions: account vanished mid-compute, skipped", "account_id", acc.AccountID)
		return nil
	}
	// The repo returns raw milestone booleans; CompletionPercentage is a
	// derived field only GetOnboardingProgressUseCase fills. Derive it
	// here the same way — trusting the zero value flagged EVERY account
	// >= 7 days old as stuck, including fully onboarded ones (US-022).
	pct := computeOnboardingPercentage(progress)
	if pct >= 100 {
		return nil
	}
	// The SAME six the percentage counts. Naming a different set would put a
	// seller on the phone about a knowledge base while the account has no
	// products — and the percentage next to it would not add up.
	missing := []string{}
	if !progress.ChannelConnected {
		missing = append(missing, "channel")
	}
	if !progress.ProductsAdded {
		missing = append(missing, "products")
	}
	if !progress.CustomersAdded {
		missing = append(missing, "customers")
	}
	if !progress.ProposalMade {
		missing = append(missing, "proposal")
	}
	if !progress.FirstInboundReceived {
		missing = append(missing, "first_inbound")
	}
	if !progress.TeamInvited {
		missing = append(missing, "team")
	}
	add(acc, domain.SuggestionOnboardingStuck, "", domain.SuggestionSeverityWarning, map[string]interface{}{
		"completion_percentage": pct,
		"missing_steps":         missing,
		"account_age_days":      int(nowT.Sub(account.CreatedAt).Hours() / 24),
	})
	return nil
}

// SetState persists dismissed/done for one suggestion in the current
// cycle so it stops appearing in the feed until next month. Only
// carteira accounts of the calling seller are accepted. channelID
// scopes channel-level suggestions (whatsapp_ban_risk); "" otherwise.
func (uc *SellerSuggestionsUseCase) SetState(ctx context.Context, sellerUserID, accountID, suggestionType, channelID, status string) error {
	if !domain.IsValidSuggestionType(suggestionType) || !domain.IsValidSuggestionStatus(status) {
		return &domain.AppError{Code: "SELLER_INVALID_SUGGESTION", Message: "unknown suggestion type or status"}
	}
	if suggestionType != domain.SuggestionWhatsAppBanRisk {
		channelID = "" // channel scoping only applies to channel-level types
	}
	cycle := uc.now().UTC().Format("2006-01")
	accounts, err := uc.carteira.ListCarteiraAccounts(ctx, sellerUserID, cycle)
	if err != nil {
		return fmt.Errorf("list carteira: %w", err)
	}
	mine := false
	for _, a := range accounts {
		if a.AccountID == accountID {
			mine = true
			break
		}
	}
	if !mine {
		return domain.ErrAccountNotFound
	}
	if err := uc.repo.UpsertSuggestionState(ctx, sellerUserID, accountID, suggestionType, channelID, cycle, status); err != nil {
		return fmt.Errorf("upsert suggestion state: %w", err)
	}
	uc.invalidate(sellerUserID)
	return nil
}

// cached returns the fresh cache entry (nil on miss) plus the seller's
// current invalidation generation — Execute passes it back to store so
// a compute that raced a dismissal is dropped instead of cached.
func (uc *SellerSuggestionsUseCase) cached(sellerUserID string, nowT time.Time) (*SellerSuggestionsFeed, uint64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if e, ok := uc.cache[sellerUserID]; ok && nowT.Sub(e.at) < suggestionCacheTTL {
		return e.feed, uc.gen[sellerUserID]
	}
	return nil, uc.gen[sellerUserID]
}

// store caches the computed feed unless an invalidate happened since
// the compute began (genSnapshot mismatch): that feed was built from
// pre-dismissal state and caching it would resurrect the dismissed
// suggestion for the whole TTL (US-022 cache race).
func (uc *SellerSuggestionsUseCase) store(sellerUserID string, nowT time.Time, feed *SellerSuggestionsFeed, genSnapshot uint64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.gen[sellerUserID] != genSnapshot {
		return
	}
	uc.cache[sellerUserID] = suggestionCacheEntry{feed: feed, at: nowT}
}

func (uc *SellerSuggestionsUseCase) invalidate(sellerUserID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.gen[sellerUserID]++
	delete(uc.cache, sellerUserID)
}
