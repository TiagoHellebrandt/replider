package domain

import (
	"context"
	"encoding/json"
	"time"
)

// AuditLog tracks actions performed in the system for metrics and accountability
type AuditLog struct {
	ID         string          `json:"id"`
	AccountID  string          `json:"account_id"`
	ActorID    *string         `json:"actor_id,omitempty"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	// IP + UserAgent record the request origin (US-073). Nil for
	// background/worker writes that have no inbound HTTP request.
	IP        *string   `json:"ip,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// FillMetaFromContext stamps the request origin (client IP + User-Agent)
// onto the entry from the context — see WithAuditMeta — UNLESS the caller
// already set those fields explicitly. This lets the repository capture
// "from where" for every admin mutation without each write site having
// to thread the gin request through (US-073).
func (l *AuditLog) FillMetaFromContext(ctx context.Context) {
	if l.IP != nil && l.UserAgent != nil {
		return
	}
	meta := GetAuditMeta(ctx)
	if l.IP == nil && meta.IP != "" {
		ip := meta.IP
		l.IP = &ip
	}
	if l.UserAgent == nil && meta.UserAgent != "" {
		ua := meta.UserAgent
		l.UserAgent = &ua
	}
}
