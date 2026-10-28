package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// Per-plan spend caps against a real database (§6.14, K4).
//
// The cap counts the AI COST LEDGER — the same rows the invoice line reads — so
// what a test has to prove is that the counting query agrees with the billing
// query about the same month, and that the ledger is filtered by account,
// operation and window rather than by any two of the three.

func seedAICost(t *testing.T, accountID, operation string, hoursAgo int) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO ai_costs (id, account_id, operation, provider, model,
		                       input_tokens, output_tokens, cost_usd, occurred_at)
		 VALUES ($1, $2, $3, 'google', 'gemini-3.1-flash-lite',
		         1000, 200, 0.0004, NOW() - ($4 || ' hours')::interval)`,
		ulid.Make().String(), accountID, operation, fmt.Sprint(hoursAgo))
	require.NoError(t, err)
}

func TestSpendCaps_CountsOnlyThisAccountsOperationsInTheWindow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)

	// Three drafts of mine this hour, one enrichment (different operation), one
	// draft of theirs, and one of mine from 60 days ago.
	seedAICost(t, mine, "approach_draft", 1)
	seedAICost(t, mine, "approach_draft", 2)
	seedAICost(t, mine, "approach_draft", 3)
	seedAICost(t, mine, "enrichment", 1)
	seedAICost(t, theirs, "approach_draft", 1)
	seedAICost(t, mine, "approach_draft", 24*60)

	since := time.Now().AddDate(0, 0, -30)
	count, err := repo.CountOperations(ctx, mine,
		domain.SpendOperations(domain.SpendAIDraft), since)
	require.NoError(t, err)

	// Three. Not four (the enrichment is a different allowance), not five
	// (theirs), not six (outside the window).
	assert.Equal(t, int64(3), count)
}

func TestSpendCaps_BothExtractionRoutesCountAsOneAllowance(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)

	// A contract with a text layer and a scanned one: two documents, two units.
	seedAICost(t, accountID, "contract_extract", 1)
	seedAICost(t, accountID, "contract_extract_vision", 2)
	// A draft must not count against the document allowance.
	seedAICost(t, accountID, "approach_draft", 1)

	count, err := repo.CountOperations(ctx, accountID,
		domain.SpendOperations(domain.SpendDocumentExtraction),
		time.Now().AddDate(0, 0, -30))
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestSpendCaps_OurRetriesDoNotConsumeTheCustomersAllowance(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)

	seedAICost(t, accountID, "approach_draft", 1)
	// A malformed model response we asked again for. Real money, our bug.
	seedAICost(t, accountID, "approach_draft_retry", 1)
	seedAICost(t, accountID, "approach_draft_retry", 1)

	count, err := repo.CountOperations(ctx, accountID,
		domain.SpendOperations(domain.SpendAIDraft), time.Now().AddDate(0, 0, -30))
	require.NoError(t, err)

	// One draft was asked for; one unit consumed.
	assert.Equal(t, int64(1), count)

	// But the retries ARE billed — they show in the spend summary, so the
	// margin sees them even though the customer's cap does not.
	summary, err := repo.SumByAccount(ctx, accountID,
		time.Now().AddDate(0, 0, -30), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Greater(t, summary.TotalUSD, 0.0)
	// Broken down BY OPERATION, so "approach_draft_retry" is a line somebody can
	// see rather than cost hidden inside the drafts it repaired.
	assert.Contains(t, summary.ByOperation, "approach_draft")
	assert.Contains(t, summary.ByOperation, "approach_draft_retry")
}

func TestSpendCaps_AnEmptyOperationListCountsNothing(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)
	seedAICost(t, accountID, "approach_draft", 1)

	// An unknown SpendKind resolves to no operations. It must count zero rather
	// than degenerate into "every row", which would refuse the account on its
	// first call.
	count, err := repo.CountOperations(context.Background(), accountID, nil,
		time.Now().AddDate(0, 0, -30))
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestSpendCaps_ThePlanLadderCarriesRealNumbers(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewPlanRepository(testPool)

	// `IsContractPlan` is "has no Stripe price", which Enterprise satisfies — so a
	// column left at the 0 default would resolve to UNLIMITED, and the draft cap
	// would silently mean no cap. Migration 000228 collapsed the catalogue to the
	// single Enterprise contract tier and lifted its draft/doc caps to real
	// numbers for exactly this reason; this is the assertion that keeps them
	// seeded. (leads/enrichments/mailboxes/funnels stay 0 = unlimited-for-contract
	// on purpose, so they are NOT asserted here.)
	plan, err := repo.GetByID(ctx, "01PLAN_ENTERPRISE_00000")
	require.NoError(t, err)
	require.NotNil(t, plan, "Enterprise plan missing")

	assert.Greater(t, plan.MaxAIDraftsPerMonth, 0, "Enterprise draft cap")
	assert.Greater(t, plan.MaxDocumentExtractionsPerMonth, 0, "Enterprise doc cap")
	// Drafts are sized above the lead allowance: a salesperson rewrites, and a cap
	// equal to the lead count would punish careful use.
	assert.GreaterOrEqual(t, plan.MaxAIDraftsPerMonth, plan.MaxLeadsPerMonth,
		"Enterprise should allow at least one draft per lead")
}

func TestSpendCaps_NoPlanIsLeftWithoutAnAllowance(t *testing.T) {
	ctx := context.Background()
	var orphans int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM plans WHERE max_ai_drafts_per_month = 0`).Scan(&orphans))

	// A cap introduced as a regression is a cap that gets reverted: the
	// migration backfills the retired ladder and the trial rather than letting
	// them inherit 0, which on a self-serve plan means "not allowed".
	assert.Equal(t, 0, orphans,
		"every plan needs a draft allowance, or drafting breaks for whoever is on it")
}
