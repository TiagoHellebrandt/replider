package domain

import (
	"context"
	"strings"
	"time"
)

// Why yours rather than the one they already own.
//
// The account had a list of claims it MAY make and a library of objections it
// will hear. Neither answers the question a buyer actually asks — and without
// an answer the draft argues from the product's existence, which is what
// "genérico" means.
//
// There is deliberately NO built-in seed, unlike DefaultHealthObjections. A
// default objection is a fact about the market ("só compramos por licitação");
// a default differentiator would be a claim about somebody's equipment, and
// inventing one for a message a hospital can check is worse than saying
// nothing.
type Differentiator struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`
	// ProductID scopes it to one product. Empty is the common case: most
	// differentiators belong to the COMPANY (assistance in the state, parts in
	// stock) and apply to everything it sells.
	ProductID string `json:"product_id,omitempty"`
	// Label is the claim in a few words ("Assistência em Cuiabá, 24h").
	Label string `json:"label"`
	// Detail is the evidence behind it, for the seller and the model — never
	// invented, because this is the account's own text.
	Detail    string    `json:"detail,omitempty"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// DifferentiatorRepository stores the library. Replace swaps the whole list in
// one transaction, exactly like the objection library: the edit surface is a
// short ordered list and per-row CRUD over ten rows is ceremony.
type DifferentiatorRepository interface {
	List(accountID string) ([]Differentiator, error)
	Replace(accountID string, items []Differentiator) error
}

// PublicContract is one procurement contract from PNCP.
//
// The only machine-readable source that can name a competitor. CNES publishes
// equipment counts and never a brand, so "they use X" derived from the park
// would be a guess — and a guess about a supplier, in a message to the
// hospital that signed with them, loses the meeting.
type PublicContract struct {
	// PNCPID is the control number, and the link back to the public record.
	PNCPID       string     `json:"pncp_id"`
	BuyerCNPJ    string     `json:"buyer_cnpj"`
	BuyerName    string     `json:"buyer_name,omitempty"`
	SupplierCNPJ string     `json:"supplier_cnpj,omitempty"`
	SupplierName string     `json:"supplier_name,omitempty"`
	Object       string     `json:"object,omitempty"`
	ValueCents   int64      `json:"value_cents"`
	SignedAt     *time.Time `json:"signed_at,omitempty"`
	// ExpiresAt is when the incumbent's contract runs out — a buying window,
	// and the only date here anybody acts on.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UF        string     `json:"uf,omitempty"`
	City      string     `json:"city,omitempty"`

	// AtaPNCPID names the price registry this contract was signed under, when
	// there is one. PNCP hands it over in the same payload and nothing read it.
	//
	// It matters to a salesperson more than the contract does: an ata is a
	// twelve-month licence to sell at a registered price, not a single closed
	// purchase, and a body that has one is buying against it all year.
	AtaPNCPID string `json:"ata_pncp_id,omitempty"`

	// FromAdhesion is PNCP's `frutoAdesao`: this body rode somebody ELSE's
	// price registry instead of running its own process.
	//
	// The most useful single bit in the record. A buyer that takes rides is a
	// buyer reachable without winning a tender — you get on an ata somewhere
	// else and they can buy from you tomorrow.
	FromAdhesion bool `json:"from_adhesion,omitempty"`
}

// IsPriceRegistry reports whether this commitment is an ata rather than a
// plain contract. They expire the same way and are worked the same way; the
// screen only has to name the door correctly.
func (c PublicContract) IsPriceRegistry() bool { return strings.TrimSpace(c.AtaPNCPID) != "" }

// DaysUntilExpiry is how long the window is, counted in whole days from the
// given day. Negative once it has passed; nil when no end date was published.
//
// Takes `now` rather than reading the clock so the number on the screen and
// the number in the ordering are the same number.
func (c PublicContract) DaysUntilExpiry(now time.Time) *int {
	if c.ExpiresAt == nil {
		return nil
	}
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := time.Date(c.ExpiresAt.Year(), c.ExpiresAt.Month(), c.ExpiresAt.Day(), 0, 0, 0, 0, time.UTC)
	d := int(to.Sub(from).Hours() / 24)
	return &d
}

// URL is the public record this came from, so every claim built on it can be
// checked by the person who receives it.
func (c PublicContract) URL() string {
	if c.PNCPID == "" {
		return ""
	}
	return "https://pncp.gov.br/app/contratos/" + c.PNCPID
}

// ContractCalendarFilter narrows the expiry calendar to the work one account
// can actually do: their states, their window, and their words.
type ContractCalendarFilter struct {
	// States comes from the account's territory.
	//
	// Deliberately no city narrow: PNCP writes the municipality as free text
	// on the contract, so matching it would need a fold the database does not
	// have and would disagree with the IBGE-code narrow the rest of the
	// product uses. A state is the right grain for a buying calendar anyway —
	// a contract two towns over is still a contract you can bid on.
	States []string
	// Keywords are matched against the contract's object, whole-word and
	// accent-folded, exactly as a tender is matched. Empty matches nothing —
	// "every contract in Mato Grosso" is thousands of rows about school meals
	// and street lighting.
	Keywords []string
	// From/To bound the window. A calendar that shows what expired last year
	// is a history book.
	From  time.Time
	To    time.Time
	Limit int
}

// ContractScanTarget is a buyer the sweep has not asked PNCP about recently.
type ContractScanTarget struct {
	CNPJ string
	Name string
	UF   string
	City string
	// LastScannedAt is zero when this buyer has never been asked about.
	LastScannedAt time.Time
}

// PublicContractRepository stores what PNCP answered about a buyer.
type PublicContractRepository interface {
	// ListByBuyer returns a buyer's contracts, newest signature first.
	ListByBuyer(ctx context.Context, buyerCNPJ string, limit int) ([]PublicContract, error)
	// Save upserts by PNCP id — the same contract fetched twice is one row.
	Save(ctx context.Context, items []PublicContract) error
	// LastScan reports when this buyer was last asked about; zero means never.
	LastScan(ctx context.Context, buyerCNPJ string) (time.Time, error)
	// MarkScanned records the ask, so a lead drafted twice in a week does not
	// re-fetch.
	MarkScanned(ctx context.Context, buyerCNPJ string, found int) error

	// ListExpiring is the calendar: commitments in this territory that run out
	// inside the window, soonest first.
	//
	// The one read a salesperson cannot build for themselves. A contract
	// ending is a purchase that MUST happen, on a date already published,
	// three to six months before any tender exists — the only way to arrive
	// before the specification is written instead of after.
	ListExpiring(ctx context.Context, f ContractCalendarFilter) ([]PublicContract, error)

	// BuyersNeedingScan lists establishments in the territory whose contracts
	// have not been read recently, least-recently-asked first.
	//
	// cityCodes are IBGE municipality codes (the same ones
	// resolveTerritoryScope produces), not names. Empty means the whole state.
	BuyersNeedingScan(ctx context.Context, states, cityCodes []string, staleBefore time.Time, limit int) ([]ContractScanTarget, error)
}
