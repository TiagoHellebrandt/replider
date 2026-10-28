package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ctxKey is a private type for context keys to avoid collisions
type ctxKey struct{}

var txKey = ctxKey{}

// getQueries returns the transactional queries if present in the context, otherwise the pool-based ones
func getQueries(ctx context.Context, q *Queries) Querier {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return q.WithTx(tx)
	}
	return q
}

// toBytes converts an interface{} domain value to []byte for postgres writes.
// If v is already []byte it is returned as-is; otherwise it is JSON-marshalled.
func toBytes(v interface{}) []byte {
	if v == nil {
		return nil
	}
	if b, ok := v.([]byte); ok {
		return b
	}
	b, _ := json.Marshal(v)
	return b
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// nullableString is the inverse: an empty domain string becomes SQL NULL.
// Writing "" into a nullable FK column is not the same as writing nothing —
// it fails the reference instead of recording "no source".
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// timestamptzPtr converts a pgtype.Timestamptz to *time.Time. Invalid (NULL)
// timestamps become nil; valid ones yield a pointer to the underlying Time.
func timestamptzPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// timeToTimestamptz is the inverse of timestamptzPtr: nil in becomes a
// NULL timestamptz (Valid=false); a populated *time.Time becomes Valid.
func timeToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
