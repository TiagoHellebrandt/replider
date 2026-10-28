package domain

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A Proposal is a priced offer made to one customer at one moment.
//
// The third pillar of the product — "gere propostas de valor" — had no object at
// all: a lead carried a single free-text `ApproachDraft`, overwritten each time,
// so there was nowhere to record WHAT was offered, at what price, or that it had
// been offered before. "Mais ofertas boas na hora certa" needs a history, and a
// history is not one column.
//
// What a Proposal deliberately is NOT: a second way to send. The body it produces
// is written into the lead's approach draft, so the approval queue reviews it and
// ApproveSend delivers it — with the opt-out check, the spend budget, the brand
// layers and the case record already in place. A parallel send path would be a
// second set of those rules, diverging from the first the day one changes.
type Proposal struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	// LeadID ties the offer to the pipeline card it belongs to. The contact is
	// reachable through the lead; storing it twice invites the two to disagree.
	LeadID string         `json:"lead_id"`
	Status ProposalStatus `json:"status"`
	// Currency governs every amount on the proposal. Never assumed: the same
	// deployment sells in BRL and USD, and a total whose currency is inferred
	// from the reader's locale is off by a factor of five.
	Currency string         `json:"currency"`
	Items    []ProposalItem `json:"items"`
	// Body is the text the customer will read. Held here as well as on the
	// lead's draft because the draft is a scratchpad the salesperson edits and
	// the next proposal overwrites — this is the record of what was proposed.
	Body      string     `json:"body"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ProposalItem is one line: a product, how many, and what it cost THEN.
type ProposalItem struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	// Name is a snapshot too. A product renamed or deleted after the fact must
	// not make an old proposal unreadable, and a line that reads "produto
	// removido" cannot be reconciled against what the customer actually got.
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	// UnitPriceCents is copied from the catalogue at creation and never joined
	// live. A price change tomorrow must not silently restate an offer already
	// made — that is the difference between a record and a report.
	UnitPriceCents int64 `json:"unit_price_cents"`
}

// TotalCents is quantity times unit price. Integers throughout: cents are the
// unit of money in this codebase precisely so no rounding decision is made in
// the middle of arithmetic.
func (i ProposalItem) TotalCents() int64 {
	return int64(i.Quantity) * i.UnitPriceCents
}

type ProposalStatus string

const (
	// ProposalDraft is the only status in which items and body may change.
	ProposalDraft ProposalStatus = "draft"
	// ProposalSent is stamped when the approach carrying this proposal goes
	// out. Terminal here: what happens next (a reply, a sale) is recorded by
	// the lead and the sale, not by rewriting the offer.
	ProposalSent ProposalStatus = "sent"
)

// Editable answers whether the offer may still change. A sent proposal is a
// statement somebody received; editing it would make the record disagree with
// what the customer is holding.
func (p *Proposal) Editable() bool { return p.Status == ProposalDraft }

// TotalCents sums the lines.
func (p *Proposal) TotalCents() int64 {
	var total int64
	for _, item := range p.Items {
		total += item.TotalCents()
	}
	return total
}

var (
	ErrProposalNotFound = &AppError{
		Code:    "PROPOSAL_NOT_FOUND",
		Message: "proposta não encontrada",
	}
	ErrProposalNoItems = &AppError{
		Code:    "PROPOSAL_NO_ITEMS",
		Message: "uma proposta precisa de pelo menos um item",
	}
	ErrProposalNotEditable = &AppError{
		Code:    "PROPOSAL_NOT_EDITABLE",
		Message: "esta proposta já foi enviada e não pode mais ser alterada",
	}
	// A proposal whose lines are priced in different currencies has no total.
	// Refusing at the boundary is the only honest answer: summing them would
	// invent a number, and picking one currency would misstate the rest.
	ErrProposalMixedCurrency = &AppError{
		Code:    "PROPOSAL_MIXED_CURRENCY",
		Message: "todos os produtos de uma proposta precisam estar na mesma moeda",
	}
	ErrProposalQuantity = &AppError{
		Code:    "PROPOSAL_INVALID_QUANTITY",
		Message: "a quantidade de cada item precisa ser maior que zero",
	}
)

// ProposalLine is what a caller asks for: a product and how many.
type ProposalLine struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// BuildItems turns requested lines into priced items, snapshotting name and
// price, and refuses anything that would produce a total nobody can defend.
//
// The currency is taken from the products themselves rather than from a
// parameter: the catalogue is the authority on what a product costs, and a
// caller-supplied currency could silently relabel BRL prices as USD.
func BuildItems(lines []ProposalLine, catalogue map[string]*Product) ([]ProposalItem, string, error) {
	if len(lines) == 0 {
		return nil, "", ErrProposalNoItems
	}

	items := make([]ProposalItem, 0, len(lines))
	currency := ""

	for _, line := range lines {
		product, ok := catalogue[line.ProductID]
		if !ok || product == nil {
			return nil, "", ErrProductNotFound
		}
		if line.Quantity <= 0 {
			return nil, "", ErrProposalQuantity
		}

		lineCurrency := strings.ToUpper(strings.TrimSpace(product.Currency))
		if lineCurrency == "" {
			// A product with no currency cannot be priced into an offer. Better
			// to refuse than to guess the account's default and be wrong once.
			return nil, "", ErrProposalMixedCurrency
		}
		if currency == "" {
			currency = lineCurrency
		} else if currency != lineCurrency {
			return nil, "", ErrProposalMixedCurrency
		}

		items = append(items, ProposalItem{
			ProductID:      product.ID,
			Name:           product.Name,
			Quantity:       line.Quantity,
			UnitPriceCents: product.PriceCents,
		})
	}

	return items, currency, nil
}

// ProposalRepository is the port. Items travel with their proposal: a line has
// no meaning apart from the offer it belongs to, and loading them separately
// would let a caller render a total from half the rows.
type ProposalRepository interface {
	Create(ctx context.Context, p *Proposal) error
	// GetByID is account-scoped on purpose. Every read of a priced offer is a
	// read of somebody's commercial terms.
	GetByID(ctx context.Context, accountID, id string) (*Proposal, error)
	ListByLead(ctx context.Context, accountID, leadID string) ([]*Proposal, error)
	// ReplaceItems swaps the whole set. Diffing lines would need stable line
	// identity the UI does not have; the offer is edited as a whole.
	ReplaceItems(ctx context.Context, accountID, id string, items []ProposalItem, currency string) error
	UpdateBody(ctx context.Context, accountID, id, body string) error
	MarkSent(ctx context.Context, accountID, id string, at time.Time) error
}

// FormatMoneyCents renders an amount for a human to read, with the ISO code
// rather than a symbol.
//
// The separator follows the CURRENCY, not the server's locale: "1.234,56" and
// "1,234.56" are the same amount to a machine and a factor of a thousand apart
// to a reader who assumes the other convention. The code is kept instead of a
// symbol because "R$" and "$" are the same glyph family across four currencies,
// and because a symbol next to an interpolated number is where this codebase has
// been bitten before.
func FormatMoneyCents(cents int64, currency string) string {
	code := strings.ToUpper(strings.TrimSpace(currency))

	negative := cents < 0
	if negative {
		cents = -cents
	}
	units := cents / 100
	fraction := cents % 100

	// Group the units in threes. Done by hand because the standard library has
	// no locale-aware grouping and pulling one in for two currencies is a
	// dependency for a for-loop.
	digits := []byte(strconv.FormatInt(units, 10))
	var grouped []byte
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped = append(grouped, '#')
		}
		grouped = append(grouped, d)
	}

	thousands, decimal := ",", "."
	if code == "BRL" {
		thousands, decimal = ".", ","
	}

	out := strings.ReplaceAll(string(grouped), "#", thousands)
	sign := ""
	if negative {
		sign = "-"
	}
	if code == "" {
		return fmt.Sprintf("%s%s%s%02d", sign, out, decimal, fraction)
	}
	return fmt.Sprintf("%s%s%s%02d %s", sign, out, decimal, fraction, code)
}

// ErrProposalDraftUnavailable is what a proposal reports when the approval queue
// is not wired. Pushing an offer nowhere and reporting success would leave the
// salesperson waiting for an approval that never appears.
var ErrProposalDraftUnavailable = &AppError{
	Code:    "PROPOSAL_APPROVAL_UNAVAILABLE",
	Message: "a fila de aprovação não está disponível",
}
