package domain

import (
	"context"
	"encoding/json"
	"time"
)

// AppLog is a log line stored for remote diagnosis.
//
// Ordinary logging goes to the container's stdout and dies in Docker's file.
// Investigating a production failure meant SSH onto the VPS, and by then the line
// that mattered had already rolled out of the file. These stay.
type AppLog struct {
	ID        int64           `json:"id"`
	At        time.Time       `json:"at"`
	Level     string          `json:"level"`
	Msg       string          `json:"msg"`
	RequestID string          `json:"request_id,omitempty"`
	AccountID string          `json:"account_id,omitempty"`
	Path      string          `json:"path,omitempty"`
	Code      string          `json:"code,omitempty"`
	Attrs     json.RawMessage `json:"attrs,omitempty"`
}

// AppLogFilter is one query's slice. Every field is optional; whatever is not
// given does not filter.
type AppLogFilter struct {
	Since     time.Time
	Until     time.Time
	Level     string
	Contains  string
	RequestID string
	AccountID string
	Limit     int
}

// AppLogGroup is one summary row: the same error, how many times, and when the
// last one was. It answers "what is breaking right now" without reading a
// thousand identical lines.
type AppLogGroup struct {
	Level string    `json:"level"`
	Msg   string    `json:"msg"`
	Code  string    `json:"code,omitempty"`
	Path  string    `json:"path,omitempty"`
	Count int64     `json:"count"`
	First time.Time `json:"first_at"`
	Last  time.Time `json:"last_at"`
}

// AppLogRepository guarda e devolve as linhas.
//
// Insert recebe um lote porque o caminho de escrita NUNCA pode estar no meio
// of a request: a log that waits on the database turns log slowness into product
// slowness.
type AppLogRepository interface {
	InsertBatch(ctx context.Context, logs []*AppLog) error
	Query(ctx context.Context, f AppLogFilter) ([]*AppLog, error)
	Summarize(ctx context.Context, since, until time.Time, limit int) ([]*AppLogGroup, error)
	// DeleteOlderThan removes what is past retention and returns how many lines
	// went, so the worker can say what it did.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// AppLogRetention is how long a line stays.
//
// Seven days: ample time to investigate what somebody reported "yesterday" or
// "on Friday", and short enough that the table does not become a dead archive
// nobody reads and everybody pays for.
const AppLogRetention = 7 * 24 * time.Hour
