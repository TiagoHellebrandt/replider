package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// WorkContractWindowUseCase turns a line on the calendar into a card in the
// pipeline.
//
// Without this the calendar is a dead end. Every other surface in this product
// ends in something you can do — a draft to approve, a lead to move, a reply to
// answer — and a list whose only action is "open the public record" gets read
// once and never again. The whole point of knowing a contract ends in January
// is to be working that buyer in October.
//
// Nothing is drafted and nothing is sent. The buyer arrives as a card with the
// contract's facts attached; the ordinary pipeline takes it from there, gates
// and all. In particular a public body published on PNCP has no e-mail and no
// phone, so drafting would refuse with APPROACH_NEEDS_ADDRESS — which is the
// correct answer, and the work the card is asking for: find the buyer.
type WorkContractWindowUseCase struct {
	contracts  domain.PublicContractRepository
	contacts   domain.ContactRepository
	identities domain.ContactIdentityRepository
	funnels    domain.SalesFunnelRepository
	leads      domain.LeadRepository
}

func NewWorkContractWindowUseCase(
	contracts domain.PublicContractRepository,
	contacts domain.ContactRepository,
	funnels domain.SalesFunnelRepository,
	leads domain.LeadRepository,
) *WorkContractWindowUseCase {
	return &WorkContractWindowUseCase{
		contracts: contracts, contacts: contacts, funnels: funnels, leads: leads,
	}
}

// SetIdentityRepo wires the contact-identity index, so a buyer the account
// already knows is reused rather than minted twice.
func (uc *WorkContractWindowUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identities = r
}

// ContractLeadSource marks where a card came from, so the queue can group it
// and the dossier can explain it.
const ContractLeadSource = "contract_window"

// Execute puts one expiring commitment into the funnel and answers with the
// lead, whether it was created now or already existed.
func (uc *WorkContractWindowUseCase) Execute(ctx context.Context, accountID, pncpID string) (*domain.Lead, error) {
	pncpID = strings.TrimSpace(pncpID)
	if pncpID == "" {
		return nil, domain.ErrContractWindowNotFound
	}

	contract, err := uc.find(ctx, accountID, pncpID)
	if err != nil {
		return nil, err
	}

	funnel, err := uc.defaultFunnel(ctx, accountID)
	if err != nil || funnel == nil {
		return nil, fmt.Errorf("work contract window: no default funnel: %w", err)
	}

	contact, err := uc.upsertBuyer(ctx, accountID, contract)
	if err != nil {
		return nil, fmt.Errorf("work contract window: contact: %w", err)
	}

	// Already in the pipeline? Then this is a navigation, not a creation.
	// Clicking twice on a calendar row must not mint a second card for the
	// same hospital — a duplicate lead is a duplicate conversation.
	if existing, err := uc.leads.GetByFunnelAndContact(ctx, funnel.ID, contact.ID); err == nil && existing != nil {
		return existing, nil
	}

	lead := &domain.Lead{
		ID:                ulid.Make().String(),
		AccountID:         accountID,
		FunnelID:          funnel.ID,
		ContactID:         contact.ID,
		Status:            domain.LeadStatusProspected,
		Source:            ContractLeadSource,
		CollectedMetadata: json.RawMessage("{}"),
		Enrichment:        contractEnrichment(contract),
	}
	if err := uc.leads.Upsert(ctx, lead); err != nil {
		return nil, fmt.Errorf("work contract window: lead: %w", err)
	}
	return lead, nil
}

// find locates the window, scoped to what this account is allowed to see.
//
// The contracts table is shared public record, so the scope is the account's
// own buyer: a lead may only be created for a contract the calendar would have
// shown. Reached through ListByBuyer rather than a by-id read because the id
// carries the buyer's CNPJ as its first segment — PNCP's own control number is
// "<cnpj>-<n>-<seq>/<year>".
func (uc *WorkContractWindowUseCase) find(ctx context.Context, accountID, pncpID string) (domain.PublicContract, error) {
	cnpj := buyerCNPJFromPNCPID(pncpID)
	if cnpj == "" {
		return domain.PublicContract{}, domain.ErrContractWindowNotFound
	}
	rows, err := uc.contracts.ListByBuyer(ctx, cnpj, 200)
	if err != nil {
		return domain.PublicContract{}, err
	}
	for _, c := range rows {
		if c.PNCPID == pncpID {
			return c, nil
		}
	}
	return domain.PublicContract{}, domain.ErrContractWindowNotFound
}

// buyerCNPJFromPNCPID reads the buyer out of the control number.
//
// "03507415002864-1-000124/2026" — the first segment is the CNPJ. Parsed
// rather than looked up because it saves a table scan on a shared table, and
// validated to fourteen digits so a malformed id becomes "not found" instead
// of an unfiltered query.
func buyerCNPJFromPNCPID(pncpID string) string {
	head, _, found := strings.Cut(pncpID, "-")
	if !found {
		return ""
	}
	digits := onlyDigitsOf(head)
	if len(digits) != 14 {
		return ""
	}
	return digits
}

func (uc *WorkContractWindowUseCase) defaultFunnel(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	all, err := uc.funnels.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, f := range all {
		if f.IsDefault {
			return f, nil
		}
	}
	return nil, fmt.Errorf("no default funnel")
}

// upsertBuyer registers the public body as a contact, keyed on its CNPJ.
//
// The same key the tender flow uses, so a hospital that arrives once through a
// published tender and once through an expiring contract is ONE customer with
// two reasons to be called — not two cards competing for the same person.
func (uc *WorkContractWindowUseCase) upsertBuyer(
	ctx context.Context, accountID string, c domain.PublicContract,
) (*domain.Contact, error) {
	contact := &domain.Contact{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ExternalID: c.BuyerCNPJ,
	}
	if name := strings.TrimSpace(c.BuyerName); name != "" {
		contact.Name = &name
		contact.CompanyName = &name
	}
	saved, _, err := newContactResolver(uc.contacts, uc.identities).
		resolve(ctx, contact, contactResolveOpts{})
	return saved, err
}

// contractEnrichment carries the window's facts onto the lead.
//
// The dossier already reads this buyer's contracts by CNPJ, so this is not the
// only path those facts travel — but a card the seller opens next month must
// say WHY it was created without a second round trip to PNCP, and "vence em
// janeiro" is the entire reason it exists.
func contractEnrichment(c domain.PublicContract) json.RawMessage {
	m := map[string]any{
		"source":       ContractLeadSource,
		"cnpj":         c.BuyerCNPJ,
		"company_name": c.BuyerName,
		"contrato":     c.Object,
		"contrato_id":  c.PNCPID,
		"contrato_url": c.URL(),
	}
	if c.ExpiresAt != nil {
		m["contrato_vence_em"] = c.ExpiresAt.Format("2006-01-02")
	}
	if c.ValueCents > 0 {
		m["contrato_valor"] = domain.FormatMoneyCents(c.ValueCents, domain.CurrencyBRL)
	}
	if s := strings.TrimSpace(c.SupplierName); s != "" {
		m["fornecedor_atual"] = s
	}
	if c.IsPriceRegistry() {
		m["ata_registro_precos"] = c.AtaPNCPID
	}
	if c.FromAdhesion {
		// The most useful fact on the card: this body buys by riding other
		// bodies' registries, so it can buy from you with no tender at all.
		m["compra_por_adesao"] = "sim"
	}
	if c.UF != "" {
		m["uf"] = c.UF
	}
	if c.City != "" {
		m["cidade"] = c.City
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
