package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// AppLogRepository guarda as linhas de log consultáveis.
//
// Hand-written SQL, no sqlc: the query is assembled from optional filters and the
// insert is a variable-sized batch — the two cases where code generation creates
// more friction than it saves.
type AppLogRepository struct {
	db *pgxpool.Pool
}

func NewAppLogRepository(db *pgxpool.Pool) *AppLogRepository {
	return &AppLogRepository{db: db}
}

// maxQueryLimit caps rows per query. It exists because a model is what reads
// this: with no cap, an absent-minded request would pull the whole table into the
// context window.
const maxQueryLimit = 200

func (r *AppLogRepository) InsertBatch(ctx context.Context, logs []*domain.AppLog) error {
	if len(logs) == 0 {
		return nil
	}
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`INSERT INTO app_logs (at, level, msg, request_id, account_id, path, code, attrs) VALUES `)
	for i, l := range logs {
		if i > 0 {
			sb.WriteString(",")
		}
		base := i * 8
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8)
		attrs := l.Attrs
		if len(attrs) == 0 {
			attrs = []byte(`{}`)
		}
		args = append(args, l.At, l.Level, l.Msg,
			nullIfEmpty(l.RequestID), nullIfEmpty(l.AccountID),
			nullIfEmpty(l.Path), nullIfEmpty(l.Code), attrs)
	}
	_, err := r.db.Exec(ctx, sb.String(), args...)
	return err
}

func (r *AppLogRepository) Query(ctx context.Context, f domain.AppLogFilter) ([]*domain.AppLog, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if !f.Since.IsZero() {
		add("at >= $%d", f.Since)
	}
	if !f.Until.IsZero() {
		add("at <= $%d", f.Until)
	}
	if lvl := strings.ToUpper(strings.TrimSpace(f.Level)); lvl != "" {
		add("level = $%d", lvl)
	}
	if s := strings.TrimSpace(f.RequestID); s != "" {
		add("request_id = $%d", s)
	}
	if s := strings.TrimSpace(f.AccountID); s != "" {
		add("account_id = $%d", s)
	}
	if s := strings.TrimSpace(f.Contains); s != "" {
		// Searches the message text AND the attributes: half of what anyone looks
		// for ("which route", "which code") is in the attributes, not the
		// sentence.
		args = append(args, "%"+s+"%")
		where = append(where,
			fmt.Sprintf("(msg ILIKE $%d OR attrs::text ILIKE $%d)", len(args), len(args)))
	}

	limit := f.Limit
	if limit <= 0 || limit > maxQueryLimit {
		limit = maxQueryLimit
	}
	args = append(args, limit)

	q := `SELECT id, at, level, msg,
	             COALESCE(request_id,''), COALESCE(account_id,''),
	             COALESCE(path,''), COALESCE(code,''), attrs
	        FROM app_logs`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY at DESC, id DESC LIMIT $%d", len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AppLog
	for rows.Next() {
		l := &domain.AppLog{}
		if err := rows.Scan(&l.ID, &l.At, &l.Level, &l.Msg,
			&l.RequestID, &l.AccountID, &l.Path, &l.Code, &l.Attrs); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *AppLogRepository) Summarize(ctx context.Context, since, until time.Time,
	limit int) ([]*domain.AppLogGroup, error) {
	if limit <= 0 || limit > maxQueryLimit {
		limit = 50
	}
	if until.IsZero() {
		until = time.Now()
	}
	// Groups by message rather than the record's whole text: the question is
	// "which failure, how many times", and the attributes (id, account) differ on
	// every occurrence of the SAME failure.
	const q = `
		SELECT level, msg, COALESCE(code,''), COALESCE(path,''),
		       COUNT(*), MIN(at), MAX(at)
		  FROM app_logs
		 WHERE at >= $1 AND at <= $2
		 GROUP BY level, msg, COALESCE(code,''), COALESCE(path,'')
		 ORDER BY COUNT(*) DESC, MAX(at) DESC
		 LIMIT $3`
	rows, err := r.db.Query(ctx, q, since, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AppLogGroup
	for rows.Next() {
		g := &domain.AppLogGroup{}
		if err := rows.Scan(&g.Level, &g.Msg, &g.Code, &g.Path,
			&g.Count, &g.First, &g.Last); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *AppLogRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM app_logs WHERE at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
