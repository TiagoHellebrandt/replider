package domain

import (
	"context"
	"strings"
	"time"
)

// Sales (§5, F1).
//
// The one record three separate features read: the portfolio (what this
// customer buys and when they will need it again), commissions (what the
// salesperson earned), and contracts (what was actually agreed). Getting the
// fields wrong here costs all three.
//
// So it is born with what the commission calculation needs — gross, cost and
// tax as SEPARATE amounts — rather than with a single "valor". A commission
// paid on gross revenue is a commission paid on money the company never had:
// in distribution the cost of goods is most of the invoice, and 5% of gross
// can exceed 100% of margin.
type Sale struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	// ContactID is who bought. Required: a sale with no customer cannot
	// feed a portfolio, which is half the reason this table exists.
	ContactID string `json:"contact_id"`
	// LeadID links the deal it came from, when there was one. Optional —
	// plenty of sales come from a phone call nobody logged.
	LeadID string `json:"lead_id,omitempty"`
	// SellerUserID is who gets the commission. Optional: a house account
	// sale has no salesperson, and inventing one would pay somebody for
	// work they did not do.
	SellerUserID string `json:"seller_user_id,omitempty"`

	Items []SaleItem `json:"items,omitempty"`

	// The three amounts, in cents, in Currency. Integers throughout: money
	// in floats accumulates error, and a cent lost per sale is an argument
	// with a salesperson every month.
	GrossCents int64 `json:"gross_cents"`
	// CostCents is what the goods or delivery cost. Zero is a legitimate
	// value for a service with no marginal cost — and a dangerous default
	// for a distributor, which is why the UI asks rather than assumes.
	CostCents int64 `json:"cost_cents"`
	// TaxCents is what leaves for the government. Separate from cost
	// because a Brazilian company's tax burden is not a cost of goods and
	// is not negotiable the same way.
	TaxCents int64  `json:"tax_cents"`
	Currency string `json:"currency"`

	SoldAt time.Time `json:"sold_at"`

	// CycleDays is how long until this customer is likely to need it again.
	// The single field the repurchase engine (F3) runs on — "200 caixas de
	// luvas, ciclo ~30d" is a prediction somebody can act on, and a sale
	// with no cycle is one the engine will never resurface.
	CycleDays int `json:"cycle_days,omitempty"`

	Source SaleSource `json:"source"`
	// ConfirmedByUserID is who approved an AI-extracted sale. Empty on a
	// manual one, where the person entering it IS the confirmation.
	ConfirmedByUserID string `json:"confirmed_by_user_id,omitempty"`

	Notes string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaleItem is one line. Kept as a list rather than a free-text description
// because the repurchase engine needs to say WHAT to offer again.
type SaleItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	// UnitCents is optional: plenty of deals are agreed as a total, and
	// demanding a unit price would make somebody invent one.
	UnitCents int64 `json:"unit_cents,omitempty"`
	// ProductID links the catalogue when the item matches something in it.
	ProductID string `json:"product_id,omitempty"`
}

// SaleSource says where the record came from, because the two have different
// trust levels and different failure modes.
type SaleSource string

const (
	// SaleManual was typed by a person.
	SaleManual SaleSource = "manual"
	// SaleAIExtracted was read out of a conversation by a model and then
	// CONFIRMED by a human. It never reaches the table unconfirmed — the
	// house pattern is that the AI proposes and a person validates.
	SaleAIExtracted SaleSource = "ai_extracted"
	// SaleImported came from a spreadsheet or an ERP.
	SaleImported SaleSource = "imported"
)

// NetCents is what the company actually keeps: gross minus cost minus tax.
//
// This — not gross — is what a commission should be calculated on. In
// distribution the cost of goods is most of the invoice, and a percentage of
// gross can quietly exceed the whole margin.
func (s *Sale) NetCents() int64 {
	return s.GrossCents - s.CostCents - s.TaxCents
}

// MarginRate is net over gross, or nil when nothing was sold. Zero over zero
// is unknown, not zero.
func (s *Sale) MarginRate() *float64 {
	if s == nil || s.GrossCents == 0 {
		return nil
	}
	r := float64(s.NetCents()) / float64(s.GrossCents)
	return &r
}

// Underwater reports a sale that cost more than it brought in.
//
// Surfaced rather than refused: discounted deals, loss leaders and
// mis-entered costs all look like this, and only a person can tell which. But
// paying commission on one without anybody noticing is how a good month
// becomes a bad quarter.
func (s *Sale) Underwater() bool {
	return s != nil && s.GrossCents > 0 && s.NetCents() < 0
}

// NextPurchaseDue estimates when this customer will need it again. Nil when
// the sale carries no cycle — a guess would put a real customer on a
// follow-up list on a date nobody chose.
func (s *Sale) NextPurchaseDue() *time.Time {
	if s == nil || s.CycleDays <= 0 {
		return nil
	}
	due := s.SoldAt.AddDate(0, 0, s.CycleDays)
	return &due
}

// Validate reports what stops a sale from being recorded.
//
// A list, like the cadence validator, so a form reports everything at once.
func (s *Sale) Validate() []string {
	var out []string
	if strings.TrimSpace(s.ContactID) == "" {
		out = append(out, "informe o cliente")
	}
	if s.GrossCents <= 0 {
		// Zero would silently sit in every report as a sale that happened
		// and earned nothing.
		out = append(out, "informe o valor da venda")
	}
	if s.CostCents < 0 || s.TaxCents < 0 {
		out = append(out, "custo e imposto não podem ser negativos")
	}
	if s.SoldAt.IsZero() {
		out = append(out, "informe a data da venda")
	}
	return out
}

// SaleExtraction is what a model proposes after reading a conversation.
//
// Deliberately NOT a Sale: it is a suggestion awaiting confirmation, and
// giving it the same type as a recorded sale is how one gets written by
// accident.
type SaleExtraction struct {
	ConversationID string     `json:"conversation_id"`
	ContactID      string     `json:"contact_id,omitempty"`
	Items          []SaleItem `json:"items,omitempty"`
	GrossCents     int64      `json:"gross_cents,omitempty"`
	CycleDays      int        `json:"cycle_days,omitempty"`
	// Question is the confirmation to put in front of the human, in their
	// words: "fechou 200 cx de luvas, ciclo ~30d?". The whole interaction is
	// this one sentence, so the model writes it rather than the UI
	// assembling one from fields.
	Question string `json:"question"`
	// Found is false when the conversation holds no sale. Explicit rather
	// than an empty struct, so "não encontrei uma venda aqui" is an answer
	// the UI can show instead of a blank form.
	Found bool `json:"found"`
}

// SaleRepository persists sales.
type SaleRepository interface {
	Create(ctx context.Context, s *Sale) error
	Get(ctx context.Context, id, accountID string) (*Sale, error)
	Update(ctx context.Context, s *Sale) error
	Delete(ctx context.Context, id, accountID string) error

	List(ctx context.Context, accountID string, limit, offset int) ([]*Sale, error)
	// ListByContact is the portfolio view: everything one customer bought.
	ListByContact(ctx context.Context, accountID, contactID string) ([]*Sale, error)
	// Totals is the money summary a period report needs.
	Totals(ctx context.Context, accountID string, from, to time.Time) (SaleTotals, error)
	// HasSale answers the inbox-triage question: has this contact ever
	// bought? One boolean rather than a list, because that is all the
	// classifier needs and a list would be a wasted read on every message.
	HasSale(ctx context.Context, accountID, contactID string) (bool, error)
}

// SaleTotals is a period's money, kept as the three amounts rather than one.
type SaleTotals struct {
	Count      int   `json:"count"`
	GrossCents int64 `json:"gross_cents"`
	CostCents  int64 `json:"cost_cents"`
	TaxCents   int64 `json:"tax_cents"`
}

// NetCents is what the company kept over the period.
func (t SaleTotals) NetCents() int64 {
	return t.GrossCents - t.CostCents - t.TaxCents
}
