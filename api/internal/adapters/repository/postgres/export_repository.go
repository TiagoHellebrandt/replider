package postgres

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Universal export (§6.18, I3).
//
// One repository, one query per entity, one shared scanner. Raw SQL because the
// filters are the SCREEN's filters — an arbitrary subset of columns per entity —
// and expressing "apply whichever of these five the caller sent" is exactly what
// a query builder is for and what sqlc is not.
type ExportRepository struct {
	db *pgxpool.Pool
}

func NewExportRepository(db *pgxpool.Pool) *ExportRepository {
	return &ExportRepository{db: db}
}

var _ domain.ExportRepository = (*ExportRepository)(nil)

// exportQuery is one entity's shape: the columns as the user reads them, the
// SELECT that produces them, and which filters it understands.
type exportQuery struct {
	columns []string
	// from is everything after SELECT: the expression list, the tables and the
	// account scope. `$1` is always the account id.
	selectSQL string
	// dateColumn is what From/To bound. Empty means the entity has no period.
	dateColumn string
	// filters maps a filter name the client may send to the SQL predicate it
	// becomes. A name absent from this map is IGNORED rather than refused: a
	// screen gains a filter before the export learns about it, and failing the
	// download is a worse answer than exporting the wider set — which is why the
	// audit records the filters that were actually applied.
	filters map[string]string
	orderBy string
}

// exportQueries is the registry. Adding an entity is one entry here plus one
// line in domain.ValidExportEntity — deliberately not one more handler.
var exportQueries = map[domain.ExportEntity]exportQuery{
	domain.ExportLeads: {
		columns: []string{"id", "contato", "telefone", "status", "origem", "score",
			"dono", "estrategia", "criado_em"},
		selectSQL: `l.id, COALESCE(ct.name, ''), COALESCE(ct.external_id, ''),
		            l.status, COALESCE(l.source, ''), l.score,
		            COALESCE(u.name, u.email, ''), COALESCE(st.name, ''),
		            l.created_at
		 FROM leads l
		 LEFT JOIN contacts ct ON ct.id = l.contact_id
		 LEFT JOIN users u ON u.id = l.owner_user_id
		 LEFT JOIN strategies st ON st.id = l.strategy_id AND st.account_id = l.account_id
		 WHERE l.account_id = $1`,
		dateColumn: "l.created_at",
		filters: map[string]string{
			"status":    "l.status = ",
			"funnel_id": "l.funnel_id = ",
			"source":    "l.source = ",
			"owner_id":  "l.owner_user_id = ",
		},
		orderBy: "l.created_at DESC",
	},
	domain.ExportContacts: {
		columns: []string{"id", "nome", "identificador", "email", "criado_em"},
		selectSQL: `c.id, COALESCE(c.name, ''), COALESCE(c.external_id, ''), COALESCE(c.email, ''), c.created_at
		 FROM contacts c
		 WHERE c.account_id = $1`,
		dateColumn: "c.created_at",
		filters:    map[string]string{},
		orderBy:    "c.created_at DESC",
	},
	domain.ExportSales: {
		columns: []string{"id", "cliente", "vendedor", "bruto_centavos", "custo_centavos",
			"imposto_centavos", "moeda", "origem", "vendido_em"},
		selectSQL: `s.id, COALESCE(ct.name, ''), COALESCE(u.name, u.email, ''),
		            s.gross_cents, s.cost_cents, s.tax_cents, s.currency,
		            s.source, s.sold_at
		 FROM sales s
		 LEFT JOIN contacts ct ON ct.id = s.contact_id
		 LEFT JOIN users u ON u.id = s.seller_user_id
		 WHERE s.account_id = $1`,
		dateColumn: "s.sold_at",
		filters: map[string]string{
			"seller_id": "s.seller_user_id = ",
			"source":    "s.source = ",
		},
		orderBy: "s.sold_at DESC",
	},
	domain.ExportConversations: {
		columns: []string{"id", "contato", "identificador", "canal", "status",
			"prioridade", "atribuido_a", "criado_em"},
		selectSQL: `c.id, COALESCE(c.contact_name, ''), c.contact_id,
		            COALESCE(ch.provider, ''), c.status, COALESCE(c.priority, ''),
		            COALESCE(u.name, u.email, ''), c.created_at
		 FROM conversations c
		 LEFT JOIN channels ch ON ch.id = c.channel_id
		 LEFT JOIN users u ON u.id = c.assigned_to
		 WHERE c.account_id = $1`,
		dateColumn: "c.created_at",
		filters: map[string]string{
			"status":     "c.status = ",
			"channel_id": "c.channel_id = ",
			"priority":   "c.priority = ",
		},
		orderBy: "c.created_at DESC",
	},
	domain.ExportSignals: {
		columns: []string{"id", "fonte", "organizacao", "cnpj", "titulo",
			"valor_centavos", "cidade", "uf", "publicado_em", "fecha_em"},
		selectSQL: `s.id, s.source, s.organization, s.cnpj, s.title,
		            s.value_cents, s.city, s.state, s.published_at,
		            COALESCE(to_char(s.closes_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), '')
		 FROM signals s
		 WHERE s.account_id = $1`,
		dateColumn: "s.published_at",
		filters: map[string]string{
			"source": "s.source = ",
			"state":  "s.state = ",
		},
		orderBy: "s.published_at DESC",
	},
}

// Rows renders one entity into a table.
//
// Every predicate is a PARAMETER, never interpolated text: the filter VALUES come
// from a query string, and the only thing taken from the client without a
// placeholder is the filter NAME — which is looked up in a fixed map and dropped
// when it is not there.
func (r *ExportRepository) Rows(ctx context.Context, req domain.ExportRequest) (domain.ExportTable, error) {
	q, ok := exportQueries[req.Entity]
	if !ok {
		return domain.ExportTable{}, &domain.AppError{
			Code: "EXPORT_INVALID_ENTITY", Message: "entidade inválida para exportação"}
	}

	args := []any{req.AccountID}
	var where strings.Builder

	// Deterministic order over the map, so two identical requests build the
	// same statement and the query plan cache is not defeated by Go's map
	// iteration.
	names := make([]string, 0, len(req.Filters))
	for name := range req.Filters {
		names = append(names, name)
	}
	sortStrings(names)

	for _, name := range names {
		predicate, known := q.filters[name]
		value := strings.TrimSpace(req.Filters[name])
		if !known || value == "" {
			continue
		}
		args = append(args, value)
		where.WriteString(" AND " + predicate + "$" + strconv.Itoa(len(args)))
	}

	if q.dateColumn != "" {
		if req.From != nil {
			args = append(args, *req.From)
			where.WriteString(" AND " + q.dateColumn + " >= $" + strconv.Itoa(len(args)))
		}
		if req.To != nil {
			args = append(args, *req.To)
			where.WriteString(" AND " + q.dateColumn + " < $" + strconv.Itoa(len(args)))
		}
	}

	args = append(args, req.Limit)
	sql := "SELECT " + q.selectSQL + where.String() +
		" ORDER BY " + q.orderBy + " LIMIT $" + strconv.Itoa(len(args))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return domain.ExportTable{}, err
	}
	defer rows.Close()

	table := domain.ExportTable{Columns: q.columns, Rows: [][]string{}}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return domain.ExportTable{}, err
		}
		record := make([]string, len(values))
		for i, v := range values {
			record[i] = exportCell(v)
		}
		table.Rows = append(table.Rows, record)
	}
	return table, rows.Err()
}

// exportCell renders one value as text.
//
// Times go out as RFC3339 rather than a locale format: every spreadsheet parses
// it, and a Brazilian date written as 03/04 is a month somebody will read wrong
// in the other hemisphere. A NULL is an empty cell, never the string "<nil>".
func exportCell(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case time.Time:
		return value.Format(time.RFC3339)
	case bool:
		if value {
			return "sim"
		}
		return "não"
	case int64:
		return strconv.FormatInt(value, 10)
	case int32:
		return strconv.FormatInt(int64(value), 10)
	case int:
		return strconv.Itoa(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return strings.TrimSpace(strings.ReplaceAll(toString(value), "\x00", ""))
	}
}

func toString(v any) string {
	if s, ok := v.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// RecordAudit appends the "who exported what" row.
func (r *ExportRepository) RecordAudit(ctx context.Context, audit *domain.ExportAudit) error {
	if audit.ID == "" {
		audit.ID = ulid.Make().String()
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO export_audits
		     (id, account_id, user_id, user_email, entity, format, filters,
		      rows, truncated, ip, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		audit.ID, audit.AccountID, audit.UserID, audit.UserEmail,
		string(audit.Entity), string(audit.Format),
		domain.FilterSummary(audit.Filters),
		audit.Rows, audit.Truncated, audit.IP, audit.UserAgent)
	return err
}

// ListAudits is the admin read.
func (r *ExportRepository) ListAudits(ctx context.Context, accountID string, limit, offset int) ([]*domain.ExportAudit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, user_id, user_email, entity, format, filters,
		        rows, truncated, ip, user_agent, created_at
		 FROM export_audits
		 WHERE account_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.ExportAudit{}
	for rows.Next() {
		var a domain.ExportAudit
		var entity, format, filters string
		if err := rows.Scan(&a.ID, &a.AccountID, &a.UserID, &a.UserEmail,
			&entity, &format, &filters, &a.Rows, &a.Truncated,
			&a.IP, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Entity = domain.ExportEntity(entity)
		a.Format = domain.ExportFormat(format)
		// The stored summary is already the rendered text; it goes back on the
		// struct under a single key so the reader has something to display
		// without re-parsing a format nothing else produces.
		if filters != "" {
			a.Filters = map[string]string{"resumo": filters}
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}
