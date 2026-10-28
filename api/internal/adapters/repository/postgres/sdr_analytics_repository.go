package postgres

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// SDR analytics (§6.12, I1).
//
// Raw SQL: every query here is an aggregation across four or five tables with
// FILTER clauses and EXISTS sub-selects, which sqlc turns into more code than
// it saves. None of these statements write.
//
// Two conventions hold throughout:
//
//   - Every query is account-scoped in its OWN WHERE clause, never only via a
//     join. A report that leaks one row across tenants is worse than a report
//     that fails.
//   - The window is half-open [from, to), matching every other aggregation in
//     the codebase, so a row on the boundary is counted exactly once.
type SDRAnalyticsRepository struct {
	db *pgxpool.Pool
}

func NewSDRAnalyticsRepository(db *pgxpool.Pool) *SDRAnalyticsRepository {
	return &SDRAnalyticsRepository{db: db}
}

var _ domain.SDRAnalyticsRepository = (*SDRAnalyticsRepository)(nil)

// Funnel counts the COHORT — leads that entered in the window — at each rung,
// following each lead to wherever it got to.
//
// Cohort rather than "events in the window" on purpose: a reply earned this
// week to an approach sent last month produces a conversion rate that can pass
// 100% and explains nothing. The trade-off is that a fresh cohort has not had
// time to convert yet, which is honest and visible (the numbers taper) rather
// than flattering.
//
// Each rung is a DISTINCT-lead existence test, so a lead approached four times
// counts once — otherwise "abordados" would exceed "entraram" for any account
// that follows up.
func (r *SDRAnalyticsRepository) Funnel(ctx context.Context, accountID string, from, to time.Time) (domain.SDRFunnelCounts, error) {
	var entered, researched, approached, replied, qualified, won int64
	err := r.db.QueryRow(ctx,
		`WITH cohort AS (
		     SELECT l.id, l.status, l.qualified_at
		     FROM leads l
		     WHERE l.account_id = $1
		       AND l.created_at >= $2 AND l.created_at < $3
		 )
		 SELECT
		     COUNT(*)::BIGINT,
		     COUNT(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM sales_events e
		         WHERE e.lead_id = c.id AND e.kind = 'enrichment_run'))::BIGINT,
		     COUNT(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM sales_events e
		         WHERE e.lead_id = c.id AND e.kind = 'approach_sent'))::BIGINT,
		     COUNT(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM sales_events e
		         WHERE e.lead_id = c.id AND e.kind = 'reply_received'))::BIGINT,
		     COUNT(*) FILTER (WHERE c.qualified_at IS NOT NULL)::BIGINT,
		     COUNT(*) FILTER (WHERE c.status = 'won')::BIGINT
		 FROM cohort c`,
		accountID, from, to,
	).Scan(&entered, &researched, &approached, &replied, &qualified, &won)
	if err != nil {
		return nil, err
	}
	return domain.SDRFunnelCounts{
		domain.SDRMetricEntered:    entered,
		domain.SDRMetricResearched: researched,
		domain.SDRMetricApproached: approached,
		domain.SDRMetricReplied:    replied,
		domain.SDRMetricQualified:  qualified,
		domain.SDRMetricWon:        won,
	}, nil
}

// Strategies tallies per strategy VERSION, over the leads worked in the window.
//
// Grouped by (strategy_id, strategy_version) because an approach sent under
// version 2 must not be credited to version 3's rewritten arguments — that
// versioning is the entire reason the column exists. The strategy's CURRENT
// name and status come from the join, so a renamed strategy keeps its history.
func (r *SDRAnalyticsRepository) Strategies(ctx context.Context, accountID string, from, to time.Time) ([]domain.SDRStrategyRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT l.strategy_id,
		        COALESCE(st.name, ''),
		        COALESCE(l.strategy_version, 0),
		        COALESCE(st.status, ''),
		        COUNT(*)::BIGINT,
		        COUNT(*) FILTER (WHERE EXISTS (
		            SELECT 1 FROM sales_events e
		            WHERE e.lead_id = l.id AND e.kind = 'reply_received'))::BIGINT,
		        COUNT(*) FILTER (WHERE l.status = 'won')::BIGINT,
		        COALESCE(SUM(sa.gross_cents), 0)::BIGINT,
		        COALESCE(MAX(sa.currency), '')
		 FROM leads l
		 LEFT JOIN strategies st ON st.id = l.strategy_id AND st.account_id = l.account_id
		 LEFT JOIN sales sa ON sa.lead_id = l.id AND sa.account_id = l.account_id
		 WHERE l.account_id = $1
		   AND l.strategy_id IS NOT NULL
		   AND l.created_at >= $2 AND l.created_at < $3
		 GROUP BY l.strategy_id, l.strategy_version, st.name, st.status
		 ORDER BY COUNT(*) DESC, st.name`,
		accountID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SDRStrategyRow{}
	for rows.Next() {
		var row domain.SDRStrategyRow
		if err := rows.Scan(&row.StrategyID, &row.Name, &row.Version, &row.Status,
			&row.Approaches, &row.Replied, &row.Won, &row.WonGrossCents, &row.Currency); err != nil {
			return nil, err
		}
		row.ReplyRate = domain.SDRRate{Numerator: row.Replied, Denominator: row.Approaches}.Value()
		row.WinRate = domain.SDRRate{Numerator: row.Won, Denominator: row.Approaches}.Value()
		out = append(out, row)
	}
	return out, rows.Err()
}

// Sellers is one row per salesperson over the window.
//
// Leads and sales are counted in SEPARATE sub-queries then joined, because a
// single query joining leads to sales multiplies one against the other: a
// seller with 3 leads and 2 sales would report 6 of each.
//
// Commission is summed through the SALE (sale_id → sales.sold_at), not through
// commission_lines.cycle. The cycle is a month string and the report's window
// is arbitrary, so matching on the cycle would credit a whole month to a
// seven-day view.
//
// Leads nobody owns come back as a row with an empty seller id. Reported
// rather than dropped: unowned work piling up IS the finding.
func (r *SDRAnalyticsRepository) Sellers(ctx context.Context, accountID string, from, to time.Time) ([]domain.SDRSellerRow, error) {
	rows, err := r.db.Query(ctx,
		`WITH lead_side AS (
		     SELECT COALESCE(l.owner_user_id, '') AS seller,
		            COUNT(*)::BIGINT AS leads_owned,
		            COUNT(*) FILTER (WHERE EXISTS (
		                SELECT 1 FROM sales_events e
		                WHERE e.lead_id = l.id AND e.kind = 'approach_sent'))::BIGINT AS approaches,
		            COUNT(*) FILTER (WHERE EXISTS (
		                SELECT 1 FROM sales_events e
		                WHERE e.lead_id = l.id AND e.kind = 'reply_received'))::BIGINT AS replied,
		            COUNT(*) FILTER (WHERE l.status = 'won')::BIGINT AS won
		     FROM leads l
		     WHERE l.account_id = $1
		       AND l.created_at >= $2 AND l.created_at < $3
		     GROUP BY COALESCE(l.owner_user_id, '')
		 ),
		 sale_side AS (
		     SELECT COALESCE(s.seller_user_id, '') AS seller,
		            COUNT(*)::BIGINT AS sales,
		            COALESCE(SUM(s.gross_cents), 0)::BIGINT AS gross,
		            COALESCE(SUM(s.gross_cents - s.cost_cents - s.tax_cents), 0)::BIGINT AS net,
		            COALESCE(MAX(s.currency), '') AS currency
		     FROM sales s
		     WHERE s.account_id = $1
		       AND s.sold_at >= $2 AND s.sold_at < $3
		     GROUP BY COALESCE(s.seller_user_id, '')
		 ),
		 commission_side AS (
		     SELECT cl.seller_user_id AS seller,
		            COALESCE(SUM(cl.amount_cents), 0)::BIGINT AS commission
		     FROM commission_lines cl
		     JOIN sales s ON s.id = cl.sale_id AND s.account_id = cl.account_id
		     WHERE cl.account_id = $1
		       AND s.sold_at >= $2 AND s.sold_at < $3
		     GROUP BY cl.seller_user_id
		 )
		 SELECT k.seller,
		        COALESCE(u.name, ''),
		        COALESCE(u.email, ''),
		        COALESCE(l.leads_owned, 0), COALESCE(l.approaches, 0),
		        COALESCE(l.replied, 0), COALESCE(l.won, 0),
		        COALESCE(sa.sales, 0), COALESCE(sa.gross, 0), COALESCE(sa.net, 0),
		        COALESCE(sa.currency, ''), COALESCE(cm.commission, 0)
		 FROM (
		     SELECT seller FROM lead_side
		     UNION SELECT seller FROM sale_side
		     UNION SELECT seller FROM commission_side
		 ) k
		 LEFT JOIN lead_side l ON l.seller = k.seller
		 LEFT JOIN sale_side sa ON sa.seller = k.seller
		 LEFT JOIN commission_side cm ON cm.seller = k.seller
		 LEFT JOIN users u ON u.id = k.seller
		 ORDER BY COALESCE(sa.gross, 0) DESC, COALESCE(l.leads_owned, 0) DESC, k.seller`,
		accountID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SDRSellerRow{}
	for rows.Next() {
		var row domain.SDRSellerRow
		var name, email string
		if err := rows.Scan(&row.SellerUserID, &name, &email,
			&row.LeadsOwned, &row.Approaches, &row.Replied, &row.Won,
			&row.Sales, &row.GrossCents, &row.NetCents, &row.Currency,
			&row.CommissionCents); err != nil {
			return nil, err
		}
		// The email is the fallback label, not a blank: a row reading
		// "(sem nome)" next to R$40k of sales is unusable.
		row.Name = strings.TrimSpace(name)
		if row.Name == "" {
			row.Name = email
		}
		row.ReplyRate = domain.SDRRate{Numerator: row.Replied, Denominator: row.Approaches}.Value()
		out = append(out, row)
	}
	return out, rows.Err()
}

// Channels is approaches and replies per channel TYPE.
//
// The type, not the individual channel row: a customer with three WhatsApp
// numbers thinks in "WhatsApp". Events with no recorded channel land under
// 'unknown' rather than being dropped — a send nobody can attribute is still a
// send that happened.
func (r *SDRAnalyticsRepository) Channels(ctx context.Context, accountID string, from, to time.Time) ([]domain.SDRChannelRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT CASE WHEN COALESCE(e.channel_type, '') = '' THEN 'unknown' ELSE e.channel_type END AS ch,
		        COUNT(*) FILTER (WHERE e.kind = 'approach_sent')::BIGINT,
		        COUNT(*) FILTER (WHERE e.kind = 'reply_received')::BIGINT
		 FROM sales_events e
		 WHERE e.account_id = $1
		   AND e.kind IN ('approach_sent', 'reply_received')
		   AND e.occurred_at >= $2 AND e.occurred_at < $3
		 GROUP BY ch
		 ORDER BY COUNT(*) FILTER (WHERE e.kind = 'approach_sent') DESC, ch`,
		accountID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SDRChannelRow{}
	for rows.Next() {
		var row domain.SDRChannelRow
		if err := rows.Scan(&row.Channel, &row.Approaches, &row.Replied); err != nil {
			return nil, err
		}
		row.ReplyRate = domain.SDRRate{Numerator: row.Replied, Denominator: row.Approaches}.Value()
		out = append(out, row)
	}
	return out, rows.Err()
}

// Signals is one row per public-signal SOURCE.
//
// Dismissed signals stay counted: "arrived 40, worked 3" is the finding, and
// hiding the 37 somebody rejected would make the source look better the more
// junk it produced.
func (r *SDRAnalyticsRepository) Signals(ctx context.Context, accountID string, from, to time.Time) ([]domain.SDRSignalRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT s.source,
		        COUNT(*)::BIGINT,
		        COUNT(*) FILTER (WHERE s.lead_id IS NOT NULL)::BIGINT,
		        COUNT(*) FILTER (WHERE l.status = 'won')::BIGINT,
		        COALESCE(SUM(s.value_cents), 0)::BIGINT,
		        COUNT(*) FILTER (WHERE s.value_cents > 0)::BIGINT
		 FROM signals s
		 LEFT JOIN leads l ON l.id = s.lead_id AND l.account_id = s.account_id
		 WHERE s.account_id = $1
		   AND s.published_at >= $2 AND s.published_at < $3
		   -- Same rule as the radar: a signal counts while some rule of this
		   -- account still claims it. An orphaned collection (its watch
		   -- deleted) kept inflating "valor em jogo" — R$ 2,45 mi of it on a
		   -- hospital-equipment account, made of PVC reducers and a garden
		   -- sprinkler. A number nobody can act on is worse than no number.
		   -- Only for sources we POLL: a signal an integration pushed in was
		   -- claimed by the act of pushing it and will never have a watch.
		   AND (
		       s.source <> ANY($4::text[])
		       OR EXISTS (
		           SELECT 1 FROM signal_watches w
		            WHERE w.account_id = s.account_id
		              AND w.source = s.source
		              AND w.is_active
		       )
		   )
		 GROUP BY s.source
		 ORDER BY COUNT(*) DESC, s.source`,
		accountID, from, to, polledSources(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SDRSignalRow{}
	for rows.Next() {
		var row domain.SDRSignalRow
		if err := rows.Scan(&row.Source, &row.Signals, &row.Worked, &row.Won,
			&row.ValueCents, &row.DisclosedCount); err != nil {
			return nil, err
		}
		row.WorkRate = domain.SDRRate{Numerator: row.Worked, Denominator: row.Signals}.Value()
		// Win rate is against WORKED, not against arrived: nobody loses a
		// tender they never bid on.
		row.WinRate = domain.SDRRate{Numerator: row.Won, Denominator: row.Worked}.Value()
		out = append(out, row)
	}
	return out, rows.Err()
}

// SalesTotals sums the window's closed sales.
//
// Net is gross − cost − tax, the same subtraction the commission engine makes:
// two places computing "net" differently is how a salesperson and a report end
// up disagreeing about the same month.
//
// Currency is MAX() — one account bills in one currency, and mixing cents
// across currencies is the one arithmetic this codebase refuses to do. A mixed
// account would report the alphabetically-last one, which is visibly wrong
// rather than silently summed.
func (r *SDRAnalyticsRepository) SalesTotals(ctx context.Context, accountID string, from, to time.Time) (int64, int64, int64, string, error) {
	var sales, gross, net int64
	var currency string
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)::BIGINT,
		        COALESCE(SUM(gross_cents), 0)::BIGINT,
		        COALESCE(SUM(gross_cents - cost_cents - tax_cents), 0)::BIGINT,
		        COALESCE(MAX(currency), '')
		 FROM sales
		 WHERE account_id = $1 AND sold_at >= $2 AND sold_at < $3`,
		accountID, from, to,
	).Scan(&sales, &gross, &net, &currency)
	return sales, gross, net, currency, err
}

// AICostUSDByChannel is the AI spend per channel type, from the immutable
// per-message stamp.
//
// Same source as the profit computation (`messages.provider_cost_usd`) and for
// the same reason: it is stamped at send time and never recomputed, so a
// deleted agent or an edited price table cannot rewrite what a past period
// cost. No join to `agents` for exactly that reason.
//
// This is OUR cost, not the channel's total cost — the carrier's and Meta's
// per-message fees are billed on the customer's own provider account and we
// never see that invoice. The disclosure travels in the payload; see
// domain.SDRCostDisclosure.
func (r *SDRAnalyticsRepository) AICostUSDByChannel(ctx context.Context, accountID string, from, to time.Time) (map[string]float64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT CASE WHEN COALESCE(ch.provider, '') = '' THEN 'unknown' ELSE ch.provider END AS channel,
		        COALESCE(SUM(m.provider_cost_usd), 0)::FLOAT8
		 FROM messages m
		 JOIN conversations c ON c.id = m.conversation_id
		 LEFT JOIN channels ch ON ch.id = c.channel_id
		 WHERE c.account_id = $1
		   AND m.created_at >= $2 AND m.created_at < $3
		   AND COALESCE(m.provider_cost_usd, 0) > 0
		 GROUP BY channel`,
		accountID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]float64{}
	for rows.Next() {
		var channel string
		var cost float64
		if err := rows.Scan(&channel, &cost); err != nil {
			return nil, err
		}
		out[channel] = cost
	}
	return out, rows.Err()
}

// DrillLeads lists the leads behind one number.
//
// One query for every drill-down in the report, so opening a funnel rung, a
// strategy row and a channel row all land on the same list with the same
// semantics. The rung is translated into the SAME existence test the funnel
// counted it with — if the two ever drifted, a number would open onto a list
// with a different length, which is the fastest way to lose an operator's
// trust in the whole screen.
//
// `f.Metric` is validated by domain.ValidSDRMetric before it reaches here; it
// selects a fixed clause from the map below and is never interpolated.
func (r *SDRAnalyticsRepository) DrillLeads(ctx context.Context, f domain.SDRDrillFilter) ([]domain.SDRLeadRow, error) {
	rungClause, ok := drillRungClauses[f.Metric]
	if !ok {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "invalid metric"}
	}

	// $1..$3 are fixed; the optional cuts append in a stable order so the
	// placeholder numbers match the args slice.
	args := []any{f.AccountID, f.Period.From, f.Period.To}
	var filters strings.Builder
	filters.WriteString(rungClause)

	if f.StrategyID != "" {
		args = append(args, f.StrategyID)
		filters.WriteString(" AND l.strategy_id = $" + strconv.Itoa(len(args)))
	}
	switch {
	case f.SellerUserID == domain.SDRUnownedSeller:
		// The bucket an operator most wants to open. "" already means
		// "any seller", hence the sentinel.
		filters.WriteString(" AND l.owner_user_id IS NULL")
	case f.SellerUserID != "":
		args = append(args, f.SellerUserID)
		filters.WriteString(" AND l.owner_user_id = $" + strconv.Itoa(len(args)))
	}
	if f.Channel != "" {
		args = append(args, f.Channel)
		filters.WriteString(` AND EXISTS (SELECT 1 FROM sales_events e
		     WHERE e.lead_id = l.id AND e.kind = 'approach_sent'
		       AND e.channel_type = $` + strconv.Itoa(len(args)) + ")")
	}
	if f.SignalSource != "" {
		args = append(args, f.SignalSource)
		filters.WriteString(` AND EXISTS (SELECT 1 FROM signals sg
		     WHERE sg.lead_id = l.id AND sg.account_id = l.account_id
		       AND sg.source = $` + strconv.Itoa(len(args)) + ")")
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args = append(args, limit)
	limitPlaceholder := strconv.Itoa(len(args))
	args = append(args, f.Offset)
	offsetPlaceholder := strconv.Itoa(len(args))

	rows, err := r.db.Query(ctx,
		`SELECT l.id, l.contact_id, COALESCE(ct.name, ''), l.status, l.source,
		        COALESCE(u.name, u.email, ''), COALESCE(l.strategy_id, ''),
		        l.created_at,
		        COALESCE(SUM(sa.gross_cents), 0)::BIGINT,
		        COALESCE(MAX(sa.currency), '')
		 FROM leads l
		 LEFT JOIN contacts ct ON ct.id = l.contact_id
		 LEFT JOIN users u ON u.id = l.owner_user_id
		 LEFT JOIN sales sa ON sa.lead_id = l.id AND sa.account_id = l.account_id
		 WHERE l.account_id = $1
		   AND l.created_at >= $2 AND l.created_at < $3
		   AND `+filters.String()+`
		 GROUP BY l.id, l.contact_id, ct.name, l.status, l.source, u.name, u.email,
		          l.strategy_id, l.created_at
		 ORDER BY l.created_at DESC
		 LIMIT $`+limitPlaceholder+` OFFSET $`+offsetPlaceholder,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.SDRLeadRow{}
	for rows.Next() {
		var row domain.SDRLeadRow
		if err := rows.Scan(&row.LeadID, &row.ContactID, &row.ContactName, &row.Status,
			&row.Source, &row.OwnerName, &row.StrategyID, &row.CreatedAt,
			&row.GrossCents, &row.Currency); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// drillRungClauses is the one place a funnel rung becomes SQL. The funnel
// counts and the drill-down list read from this same table of clauses, which is
// what keeps "40 abordados" opening onto exactly 40 rows.
var drillRungClauses = map[string]string{
	// Every lead in the cohort — the window bounds already did the work.
	domain.SDRMetricEntered: "TRUE",
	domain.SDRMetricResearched: `EXISTS (SELECT 1 FROM sales_events e
	     WHERE e.lead_id = l.id AND e.kind = 'enrichment_run')`,
	domain.SDRMetricApproached: `EXISTS (SELECT 1 FROM sales_events e
	     WHERE e.lead_id = l.id AND e.kind = 'approach_sent')`,
	domain.SDRMetricReplied: `EXISTS (SELECT 1 FROM sales_events e
	     WHERE e.lead_id = l.id AND e.kind = 'reply_received')`,
	domain.SDRMetricQualified: "l.qualified_at IS NOT NULL",
	domain.SDRMetricWon:       "l.status = 'won'",
}
