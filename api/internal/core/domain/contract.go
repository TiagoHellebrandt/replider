package domain

import (
	"context"
	"strings"
	"time"
)

// Contracts (§6.11, G4).
//
// B2B health runs on contracts that live in an email folder today: nobody
// remembers the renewal window, the penalty, or the month the price is indexed.
// This is where they live instead.
//
// The model READS the document and PROPOSES the fields; a person confirms them.
// Both readings are kept — AIProposal is what the model said, the fields are
// what a human accepted — because "a IA propõe, o humano valida" is only true
// if you can still see what was proposed after somebody edited a date.

// ContractExtractionStatus is the lifecycle of the reading, not of the
// contract: a failed extraction says the model could not read the PDF, never
// that the contract is invalid.
type ContractExtractionStatus string

const (
	ContractExtractionPending ContractExtractionStatus = "pending"
	ContractExtractionRunning ContractExtractionStatus = "extracting"
	ContractExtractionDone    ContractExtractionStatus = "extracted"
	ContractExtractionFailed  ContractExtractionStatus = "failed"
	// ContractExtractionSkipped is a contract registered by hand, with no
	// document to read.
	ContractExtractionSkipped ContractExtractionStatus = "skipped"
)

// ContractParty is one side of the agreement.
type ContractParty struct {
	Name string `json:"name"`
	// Role is free text the document uses ("contratante", "contratada",
	// "fiadora"). Not an enum: contracts name these however they like, and an
	// enum would force the extractor to pick the wrong word.
	Role string `json:"role,omitempty"`
	// Document is the CNPJ/CPF as printed. Kept as text, unnormalised: what
	// matters is matching the paper, and "reformatting" a document number is
	// how a typo becomes invisible.
	Document string `json:"document,omitempty"`
}

// ContractPenalty is one multa.
//
// The most expensive clause in the document and the one nobody remembers, so
// it is structured rather than buried in a paragraph of prose.
type ContractPenalty struct {
	Description string `json:"description"`
	// Percent is the rate when the clause states one; zero when the penalty is
	// a fixed amount or a non-monetary consequence.
	Percent float64 `json:"percent,omitempty"`
	// AmountCents is a fixed penalty, when the clause states one.
	AmountCents int64 `json:"amount_cents,omitempty"`
	// Trigger is what sets it off ("atraso no pagamento", "rescisão
	// antecipada").
	Trigger string `json:"trigger,omitempty"`
}

// Contract is one agreement, or one amendment to one.
type Contract struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	ContactID string `json:"contact_id,omitempty"`
	Title     string `json:"title"`

	// ParentContractID chains an amendment (aditivo) to the contract it
	// changes; Version is its place in that chain.
	ParentContractID string `json:"parent_contract_id,omitempty"`
	Version          int    `json:"version"`

	// ObjectPath is the PDF in the private bucket. Empty when the contract was
	// registered from pasted text.
	ObjectPath string `json:"object_path,omitempty"`
	FileName   string `json:"file_name,omitempty"`

	ExtractionStatus ContractExtractionStatus `json:"extraction_status"`
	ExtractionError  string                   `json:"extraction_error,omitempty"`

	// AIProposal is the model's reading, verbatim. Never written by a human and
	// never used to drive an alert on its own.
	AIProposal map[string]any `json:"ai_proposal,omitempty"`

	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty"`
	ConfirmedByUserID string     `json:"confirmed_by_user_id,omitempty"`

	Parties []ContractParty `json:"parties,omitempty"`
	Subject string          `json:"subject,omitempty"`

	ValueCents   int64  `json:"value_cents"`
	Currency     string `json:"currency"`
	PaymentTerms string `json:"payment_terms,omitempty"`

	// StartsOn/EndsOn are calendar dates: a contract runs on days, and putting
	// a timezone on "vence em 31/12" only creates off-by-one arguments.
	StartsOn *time.Time `json:"starts_on,omitempty"`
	EndsOn   *time.Time `json:"ends_on,omitempty"`

	// AutoRenewal + NoticeDays is the most expensive pair of facts in the
	// document: miss the window and the contract renews itself.
	AutoRenewal bool `json:"auto_renewal"`
	NoticeDays  int  `json:"notice_days"`

	AdjustmentIndex string `json:"adjustment_index,omitempty"`
	AdjustmentMonth *int   `json:"adjustment_month,omitempty"`

	// The price registry (ata de registro de preços). A public body committed to
	// buy, the supplier delivered part of it, and the remaining balance expires
	// with the ata — pre-approved revenue nobody goes back for.
	PriceRegistry        bool   `json:"price_registry"`
	RegistryNumber       string `json:"registry_number,omitempty"`
	RegistryBalanceCents int64  `json:"registry_balance_cents"`
	// RegistryBalanceOn is when the balance was last known. A balance with no
	// date is a number nobody can trust: "R$80k left as of last week" and
	// "R$80k left as of a year ago" are different facts.
	RegistryBalanceOn *time.Time `json:"registry_balance_on,omitempty"`

	// LastAdjustedOn is when the price was actually raised. Without it an
	// accumulation would run from the contract's start forever and double-count
	// every adjustment already applied.
	LastAdjustedOn *time.Time `json:"last_adjusted_on,omitempty"`

	SLAText         string            `json:"sla_text,omitempty"`
	Penalties       []ContractPenalty `json:"penalties,omitempty"`
	Exclusivity     bool              `json:"exclusivity"`
	ExclusivityText string            `json:"exclusivity_text,omitempty"`
	Guarantees      string            `json:"guarantees,omitempty"`
	Termination     string            `json:"termination,omitempty"`
	Notes           string            `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Amendments is filled on a detail read: the chain in force, oldest first.
	Amendments []*Contract `json:"amendments,omitempty"`

	// Redacted says fields were REMOVED for the caller's role. Without this
	// flag a redacted value_cents of 0 reads as "this contract is free", which
	// is a worse lie than showing nothing — the client renders "oculto" only
	// because the server admitted it hid something.
	Redacted bool `json:"redacted,omitempty"`
}

// Confirmed reports a reading a person has signed off.
func (c *Contract) Confirmed() bool { return c != nil && c.ConfirmedAt != nil }

// IsAmendment reports an aditivo.
func (c *Contract) IsAmendment() bool {
	return c != nil && strings.TrimSpace(c.ParentContractID) != ""
}

// DaysUntilEnd is how long the contract still runs, negative once it has
// expired. Nil when there is no end date — an unknown end is not "today".
func (c *Contract) DaysUntilEnd(now time.Time) *int {
	if c == nil || c.EndsOn == nil {
		return nil
	}
	days := int(c.EndsOn.Sub(startOfDay(now)).Hours() / 24)
	return &days
}

// NoticeDeadline is the last day to give notice before an automatic renewal.
//
// Nil when the contract does not renew itself or states no window: inventing a
// deadline would put a task on somebody's list on a date the paper never
// mentions.
func (c *Contract) NoticeDeadline() *time.Time {
	if c == nil || !c.AutoRenewal || c.EndsOn == nil || c.NoticeDays <= 0 {
		return nil
	}
	deadline := c.EndsOn.AddDate(0, 0, -c.NoticeDays)
	return &deadline
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// MaxContractNoticeDays bounds the notice window.
//
// Two years. Above that the model misread a number — a notice window longer
// than most contracts' whole term is never what the paper said.
const MaxContractNoticeDays = 730

// Validate refuses what nobody can have meant.
func (c *Contract) Validate() error {
	if c == nil {
		return &AppError{Code: "CONTRACT_INVALID", Message: "contrato vazio"}
	}
	if strings.TrimSpace(c.Title) == "" {
		return &AppError{Code: "CONTRACT_INVALID", Message: "dê um título ao contrato"}
	}
	if c.ValueCents < 0 {
		return &AppError{Code: "CONTRACT_INVALID", Message: "valor não pode ser negativo"}
	}
	if c.NoticeDays < 0 || c.NoticeDays > MaxContractNoticeDays {
		return &AppError{Code: "CONTRACT_INVALID",
			Message: "prazo de aviso fora do razoável (0 a 730 dias)"}
	}
	if c.AdjustmentMonth != nil && (*c.AdjustmentMonth < 1 || *c.AdjustmentMonth > 12) {
		return &AppError{Code: "CONTRACT_INVALID", Message: "mês de reajuste inválido"}
	}
	// A term that ends before it starts is a transcription error, and letting
	// it through would make every "expira em" calculation lie.
	if c.StartsOn != nil && c.EndsOn != nil && c.EndsOn.Before(*c.StartsOn) {
		return &AppError{Code: "CONTRACT_INVALID",
			Message: "a data de término é anterior à de início"}
	}
	for _, p := range c.Penalties {
		if p.Percent < 0 || p.Percent > 100 {
			return &AppError{Code: "CONTRACT_INVALID",
				Message: "percentual de multa precisa ficar entre 0 e 100"}
		}
		if p.AmountCents < 0 {
			return &AppError{Code: "CONTRACT_INVALID", Message: "multa negativa"}
		}
	}
	return nil
}

// ContractFullAccessRoles are the roles that read a contract whole.
//
// Owner, admin and gestor: the first two run the company, and a gestor approves
// for the whole team and needs the price to do it. A vendedor and an attendant
// see that the contract exists, its term and its SLA — which is what serving
// the customer requires — and not what it cost.
var ContractFullAccessRoles = map[Role]bool{
	RoleOwner: true, RoleAdminRole: true, RoleGestor: true,
}

// RedactForRole strips the commercially sensitive clauses from a copy of the
// contract for somebody who may see that it exists but not what it costs.
//
// Deleting the fields server-side rather than hiding them in the UI is the
// difference between authorization and decoration: a client that decides what
// to hide is one devtools tab away from showing everything.
func (c *Contract) RedactForRole(role string) *Contract {
	if c == nil || ContractFullAccessRoles[NormalizeRole(role)] {
		return c
	}
	clone := *c
	clone.ValueCents = 0
	clone.PaymentTerms = ""
	clone.Penalties = nil
	clone.ExclusivityText = ""
	clone.Guarantees = ""
	clone.Termination = ""
	clone.Notes = ""
	// The proposal carries every field the model read, including the ones just
	// removed — handing it over would undo the redaction in one line.
	clone.AIProposal = nil
	clone.Redacted = true

	if len(c.Amendments) > 0 {
		clone.Amendments = make([]*Contract, 0, len(c.Amendments))
		for _, a := range c.Amendments {
			clone.Amendments = append(clone.Amendments, a.RedactForRole(role))
		}
	}
	return &clone
}

// ContractRepository persists contracts.
type ContractRepository interface {
	Create(ctx context.Context, c *Contract) error
	Get(ctx context.Context, accountID, id string) (*Contract, error)
	Update(ctx context.Context, c *Contract) error
	Delete(ctx context.Context, accountID, id string) error

	// List returns the top-level contracts (never amendments — those belong to
	// their parent's detail), newest first.
	List(ctx context.Context, accountID string, limit, offset int) ([]*Contract, error)
	// ListAmendments is one contract's chain, oldest first.
	ListAmendments(ctx context.Context, accountID, parentID string) ([]*Contract, error)
	// NextVersion is the next place in a chain, so two people adding an
	// amendment at once do not both get "aditivo 2".
	NextVersion(ctx context.Context, accountID, parentID string) (int, error)

	// DueForAttention is the cross-account sweep the alert worker runs: every
	// CONFIRMED contract that could owe somebody a deadline — a term ending
	// inside the horizon, a stated adjustment month, or an ata with balance.
	//
	// The SQL narrows, the domain decides: which of those actually produce a
	// flag is DeriveRedFlags's call, and duplicating that logic in a WHERE
	// clause is how the two drift apart.
	DueForAttention(ctx context.Context, through time.Time, limit int) ([]*Contract, error)

	// SetExtraction stores the model's reading and moves the status. Separate
	// from Update because the worker must not overwrite the fields a human has
	// been editing while it ran.
	SetExtraction(ctx context.Context, accountID, id string, status ContractExtractionStatus,
		proposal map[string]any, extractionError string, fill *Contract) error
}
