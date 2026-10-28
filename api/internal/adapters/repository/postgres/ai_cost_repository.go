package postgres

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// AI cost ledger (US-M015). Raw SQL: two statements, one of which unions
// the ledger with the per-message stamp — a shape sqlc would not improve.
type AICostRepository struct {
	db *pgxpool.Pool
	// rateCents is the USD→BRL rate (BRL cents per 1 USD). Injected rather
	// than read per query so a rate lookup failure cannot make an invoice
	// line silently free.
	rateCents int
}

func NewAICostRepository(db *pgxpool.Pool, rateCents int) *AICostRepository {
	if rateCents <= 0 {
		rateCents = defaultUSDBRLRateCents
	}
	return &AICostRepository{db: db, rateCents: rateCents}
}

// defaultUSDBRLRateCents mirrors usecases.DefaultUSDBRLRateCents. Repeated
// rather than imported because an adapter must not depend on a use case;
// the value is a platform default the owner edits in settings.
const defaultUSDBRLRateCents = 550

var _ domain.AICostRepository = (*AICostRepository)(nil)

func (r *AICostRepository) Record(ctx context.Context, e *domain.AICostEntry) error {
	if e.ID == "" {
		e.ID = ulid.Make().String()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO ai_costs (id, account_id, operation, provider, model,
		                       input_tokens, output_tokens, cached_tokens, cost_usd, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		e.ID, e.AccountID, e.Operation, e.Provider, e.Model,
		e.InputTokens, e.OutputTokens, e.CachedTokens, e.CostUSD, e.OccurredAt)
	return err
}

// SumByAccount totals the LEDGER for the period — and only the ledger.
//
// Until 2026-09-01 it also added `messages.provider_cost_usd`. That stamp
// predates the ledger, but the conversational reply has landed in ai_costs
// as "agent_reply" since US-M015 (ProcessMessageUseCase meters it), so the
// same call was counted twice: once as agent_reply, once as "conversation".
// Every account's cycle spend, budget %, invoice line and the budget gate
// read this number, so the double count refused customers at half their
// ceiling. The ledger is the single source of truth; the message stamp
// stays for the per-conversation cost analytics (top-expensive threads),
// which read it directly.
func (r *AICostRepository) SumByAccount(ctx context.Context, accountID string, from, to time.Time) (*domain.AICostSummary, error) {
	out := &domain.AICostSummary{
		From:        from,
		To:          to,
		RateCents:   r.rateCents,
		ByOperation: map[string]float64{},
	}

	rows, err := r.db.Query(ctx,
		`SELECT operation, COALESCE(SUM(cost_usd), 0)::FLOAT8
		 FROM ai_costs
		 WHERE account_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		 GROUP BY operation`,
		accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var op string
		var usd float64
		if err := rows.Scan(&op, &usd); err != nil {
			return nil, err
		}
		out.ByOperation[op] = usd
		out.TotalUSD += usd
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// The caching ratio (K3). Read alongside the spend rather than derived from
	// it, so "structured for caching" is a number somebody can check instead of
	// a claim in a commit message.
	err = r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(input_tokens), 0)::BIGINT,
		        COALESCE(SUM(cached_tokens), 0)::BIGINT
		 FROM ai_costs
		 WHERE account_id = $1 AND occurred_at >= $2 AND occurred_at < $3`,
		accountID, from, to).Scan(&out.InputTokens, &out.CachedTokens)
	if err != nil {
		return nil, err
	}

	// Rounded ONCE, at the end: rounding each entry would drift by cents
	// on a busy account, and cents are the whole credibility of a
	// "no markup" line.
	out.TotalBRLCents = int64(math.Round(out.TotalUSD * float64(r.rateCents)))
	return out, nil
}

// CountOperations counts this account's ledger rows for any of the given
// operations since a moment (K4).
//
// Counts what was actually PAID FOR: the meter skips an entry when the provider
// reported no token usage, so a call we could not price does not consume the
// customer's allowance. That errs in the customer's favour, which is the right
// direction for a cap — refusing a paying customer over an accounting gap is a
// worse outcome than one uncharged draft, and the missing usage is logged where
// it happens.
func (r *AICostRepository) CountOperations(ctx context.Context, accountID string, operations []string, since time.Time) (int64, error) {
	if len(operations) == 0 {
		return 0, nil
	}
	var count int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)::BIGINT
		 FROM ai_costs
		 WHERE account_id = $1
		   AND operation = ANY($2)
		   AND occurred_at >= $3`,
		accountID, operations, since,
	).Scan(&count)
	return count, err
}
