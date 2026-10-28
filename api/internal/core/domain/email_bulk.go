package domain

import "context"

// Bulk sending (hybrid model, 2026-08-25). One-by-one approvals always leave
// through the customer's own mailbox; BATCH approvals go through the
// platform's bulk provider (Resend), gated on the customer's domain being
// registered and verified there — so the per-message cost only exists for
// accounts that asked for volume.

// BulkDNSRecord is one row the bulk provider asks the customer to publish.
// Rendered verbatim by the channel screen with copy buttons, the same way the
// BYO DNS card renders our own records.
type BulkDNSRecord struct {
	Record   string `json:"record"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      string `json:"ttl,omitempty"`
	Priority *int   `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
}

// BulkDomain is the provider's view of a sending domain. Status values are
// the provider's own (Resend: not_started, pending, verified, failed,
// temporary_failure) — passed through, never re-invented.
type BulkDomain struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Records []BulkDNSRecord `json:"records"`
}

func (d *BulkDomain) Verified() bool { return d != nil && d.Status == "verified" }

// BulkDomainProvider is the port the channel screen's volume-sending setup
// drives. Satisfied by the Resend REST client.
type BulkDomainProvider interface {
	// Configured answers whether the platform has a provider key at all —
	// false fails closed with ErrBulkSendingUnavailable.
	Configured() bool
	// CreateDomain registers (or reuses) the domain and returns it with the
	// DNS records the customer must publish.
	CreateDomain(ctx context.Context, name string) (*BulkDomain, error)
	GetDomain(ctx context.Context, id string) (*BulkDomain, error)
	// VerifyDomain asks the provider to re-check the records; the outcome
	// lands asynchronously on the domain's status.
	VerifyDomain(ctx context.Context, id string) error
}
