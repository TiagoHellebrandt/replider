package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// What we already know about the buyer, and never showed.
//
// Two questions decide how a seller opens: do these people know my name, and
// who is already selling to them. Both were answerable from data sitting in
// this database with no screen reading it.
//
// Both were also got WRONG on the first attempt, in production, and the fixes
// are the reason this file reads the way it does.

// BuyerRelationship is what this account IS to the buyer.
//
// Every value here must be backed by a record of something that HAPPENED. The
// two versions before this one both got that wrong, in production, on the same
// screen:
//
//   - the first read a CONTACT row as a sale, and told a seller he had sold to
//     Instituto Patris and to Nova Maringá on an account whose sales table was
//     empty;
//   - the second read the same contact row as an approach ("você já abordou"),
//     on an account with ZERO channels connected and ZERO outbound messages
//     ever sent. Those buyers have no e-mail address stored and no draft
//     written. Nobody had approached anybody.
//
// Both failures were one mistake: inferring an event from the existence of a
// row this product creates by itself. The radar mints a contact and a lead the
// moment it spots an opportunity, so neither proves contact was made. A state
// is therefore allowed here only when some record says the thing occurred —
// a sale row, or a lead status that only a completed send can stamp.
type BuyerRelationship string

const (
	// BuyerCold: this buyer is not in the funnel at all.
	BuyerCold BuyerRelationship = "cold"
	// BuyerListed: a lead card exists and nothing has been written yet. Says
	// "don't create a second card", not "you talked to them".
	BuyerListed BuyerRelationship = "listed"
	// BuyerQueued: an offer is written and waiting on a human. Still nothing
	// sent — the approval queue is the gate, and it has never been passed on
	// this account.
	BuyerQueued BuyerRelationship = "queued"
	// BuyerContacted: an offer actually went out. The ONLY evidence accepted
	// is a lead status that `ApproveSend` stamps after the channel adapter
	// confirms delivery.
	BuyerContacted BuyerRelationship = "contacted"
	// BuyerSold: a sale is recorded against this contact.
	BuyerSold BuyerRelationship = "sold"
)

// leadStatusProvesASend lists the statuses a lead can only reach by having had
// an approach delivered.
//
// `lost` is deliberately absent: an opt-out marks a lead lost without anything
// ever being sent, so treating it as proof would re-introduce exactly the kind
// of claim this type exists to prevent.
func leadStatusProvesASend(s domain.LeadStatus) bool {
	switch s {
	case domain.LeadStatusContacted, domain.LeadStatusResponded,
		domain.LeadStatusQualified, domain.LeadStatusWon:
		return true
	default:
		return false
	}
}

// BuyerContext travels on a radar row whose source published a CNPJ.
type BuyerContext struct {
	Relationship BuyerRelationship `json:"relationship"`
	// ContactID lets the panel link straight to the customer.
	ContactID string `json:"contact_id,omitempty"`
	// Name as this account stored it, usually friendlier than the registry's
	// all-caps legal name.
	Name string `json:"name,omitempty"`

	// Contracts counts this buyer's published contracts THAT ARE IN OUR
	// TRADE. Unfiltered it was worse than useless: Itiquira has 20 stored
	// contracts and the most recent two are marquee rentals, so the panel
	// named "PARQUE DE DIVERSÕES XAVIER LTDA" as the incumbent to beat. The
	// supplier worth naming sat two rows down — M.S DIAGNÓSTICA, who supplied
	// parts to the same municipal laboratory this tender is about.
	//
	// Wrong competitive intelligence is worse than none: a seller who repeats
	// it in a meeting is embarrassed by us.
	Contracts int `json:"contracts,omitempty"`
	// LastSupplier is who won the most recent one IN OUR TRADE. The only
	// place a competitor's name may come from is this published record.
	LastSupplier   string     `json:"last_supplier,omitempty"`
	LastObject     string     `json:"last_object,omitempty"`
	LastContractAt *time.Time `json:"last_contract_at,omitempty"`

	// Scanned says the buyer's contracts were actually collected. Without it
	// "0 contracts in our trade" is indistinguishable from "we never looked",
	// and the panel would turn a gap in collection into a claim about a
	// hospital.
	Scanned bool `json:"scanned"`

	// HistoryVia names the organization the contract history below actually
	// belongs to, when it is NOT this hospital.
	//
	// A public hospital rarely signs anything: its secretaria does. Reading
	// the secretaria's contracts is the only way to know who already supplies
	// the place — and saying "este comprador já contratou" about them would
	// be false, and repeated identically on the seven hospitals that share
	// 57252971000146. Empty means the history is the buyer's own.
	HistoryVia string `json:"history_via,omitempty"`

	// LeadID is the card this buyer already has, so the panel can open it
	// instead of describing it. A state the seller cannot act on is a dead
	// end: "já está no seu quadro" used to leave them hunting for it by name
	// across a board of seventy-three cards.
	LeadID string `json:"lead_id,omitempty"`
	// Stage is the Kanban column that card sits in. "In the funnel" is not a
	// position — a card in "Ganhos" and one nobody has touched are both in
	// the funnel and call for opposite next moves. Empty when the funnel has
	// no column mapping the lead's status; rendered as nothing, never guessed.
	Stage string `json:"stage,omitempty"`
}

// radarContactReader is the one contact lookup the annotation makes.
type radarContactReader interface {
	GetByExternalID(ctx context.Context, accountID, externalID string) (*domain.Contact, error)
}

// radarBuyerLeadReader says what this account has already done about the buyer —
// the record that separates "we wrote to them" from "we noticed them" — and
// where the card sits.
type radarBuyerLeadReader interface {
	LeadStateForContact(ctx context.Context, accountID, contactID string) (domain.RadarLeadState, error)
}

// radarSaleChecker answers the question the first version did not ask.
type radarSaleChecker interface {
	HasSale(ctx context.Context, accountID, contactID string) (bool, error)
}

// radarBuyerHistory is the published record, narrowed to one question.
type radarBuyerHistory interface {
	ListByBuyer(ctx context.Context, buyerCNPJ string, limit int) ([]domain.PublicContract, error)
}

// SetBuyerContext wires the reads. Optional: unwired, rows carry no buyer
// block and the panel renders without it.
func (uc *RadarUseCase) SetBuyerContext(c radarContactReader, l radarBuyerLeadReader, s radarSaleChecker, h radarBuyerHistory) {
	uc.buyerContacts, uc.buyerLeads, uc.buyerSales, uc.buyerHistory = c, l, s, h
}

// The two halves of the block cost very different things, so they get
// different budgets. Sharing one cap made the cheap half as scarce as the
// expensive half, and scarcity here is not neutral: a row with no block reads
// as "not on your board", which is a claim. A page of 25 hospitals under a cap
// of 12 would have told the seller thirteen lies by omission.
const (
	// radarBuyerCardLookups bounds the CHEAP half — three indexed single-row
	// reads to find the contact, plus the lead and the sale. Sized past the
	// park page (radarParkLimit) so every hospital on screen gets an answer,
	// rather than the first twelve getting one and the rest getting silence
	// that looks like an answer.
	radarBuyerCardLookups = 40
	// radarBuyerLookups bounds the EXPENSIVE half: the published-contract
	// scan below, which reads 60 rows per buyer and runs the trade gate over
	// each. A buyer past this cap keeps its card and stage and simply carries
	// no history — Scanned stays false, which is exactly the flag that stops
	// "no contracts in your trade" being claimed on a buyer nobody read.
	radarBuyerLookups = 12
)

// radarBuyerContractScan is how deep the trade filter looks. A buyer's recent
// contracts are mostly not our trade, so reading only a handful would report
// "nothing in your trade" for a hospital that buys from us every year.
const radarBuyerContractScan = 60

// annotateBuyers fills the buyer block on every row that names a CNPJ.
//
// Runs after the fan-in, like markReady: it answers a question ABOUT the
// collected rows rather than producing any. Every lookup is best-effort — a
// failed read leaves the block empty, because a panel missing one section is a
// smaller problem than a feed that will not load.
func (uc *RadarUseCase) annotateBuyers(ctx context.Context, accountID string, items []RadarItem) {
	if uc.buyerContacts == nil && uc.buyerHistory == nil {
		return
	}
	seen := make(map[string]*BuyerContext)
	scanned := 0
	for i := range items {
		cnpj := strings.TrimSpace(items[i].CNPJ)
		keys := items[i].contactKeys
		// A row that names neither a buyer nor a way to recognise one has
		// nothing to annotate.
		if cnpj == "" && len(keys) == 0 {
			continue
		}
		// Who to read the published record for. The hospital when it has a
		// CNPJ; the entity that maintains it when it has none — which is
		// eleven of the twenty biggest parks in Mato Grosso, and was a blank
		// section on every one of them.
		src := historySource{CNPJ: cnpj}
		if src.CNPJ == "" {
			src.CNPJ = strings.TrimSpace(items[i].BuyerEntityCNPJ)
			src.Via = strings.TrimSpace(items[i].BuyerEntityName)
		}
		// Identity is what makes two ROWS the same buyer. Never the entity:
		// seven hospitals share one secretaria, and keying on it would give
		// all seven the first one's card.
		identity := cnpj
		if identity == "" {
			identity = keys[0]
		}
		block, ok := seen[identity]
		if !ok {
			if len(seen) >= radarBuyerCardLookups {
				continue
			}
			block = uc.buyerContextFor(ctx, accountID, src, keys, scanned < radarBuyerLookups)
			if src.CNPJ != "" {
				scanned++
			}
			seen[identity] = block
		}
		if block != nil {
			items[i].Buyer = block
		}
	}
}

// buyerContextFor answers both halves: who this buyer is TO US (the contact,
// the card, the stage) and what the published record says about them (the
// contracts, the incumbent).
//
// The two halves take different keys and that is the point. History is keyed
// on the CNPJ, which only a tender publishes. Recognition is keyed on
// everything the CNES door might have filed the hospital under — the CNPJ, the
// registry id, the phone — because a park row publishes no CNPJ at all and
// used to arrive with no buyer block whatsoever. That is the largest hospital
// in the state showing up on the radar as a stranger while its card sat in the
// funnel.
// historySource is whose published contracts to read for a row, and under
// whose name to report them.
//
// A hospital with its own CNPJ answers for itself. One without — eleven of the
// twenty biggest parks in MT — is a unit of a secretaria, and the secretaria
// is both the only readable buyer and a different organization from the
// hospital. Returning the name alongside the key is what keeps the screen
// honest about which of the two it is talking about.
type historySource struct {
	CNPJ string
	// Via is empty when the CNPJ is the buyer's own.
	Via string
}

func (uc *RadarUseCase) buyerContextFor(
	ctx context.Context, accountID string, src historySource, keys []string, withHistory bool,
) *BuyerContext {
	cnpj := src.CNPJ
	out := &BuyerContext{Relationship: BuyerCold}

	if uc.buyerContacts != nil {
		probes := make([]string, 0, 1+len(keys))
		if cnpj != "" {
			probes = append(probes, cnpj)
		}
		probes = append(probes, keys...)
		for _, key := range probes {
			c, err := uc.buyerContacts.GetByExternalID(ctx, accountID, key)
			if err != nil || c == nil {
				continue
			}
			// A contact row proves only that this product noticed them. What
			// was actually DONE is on the lead and on the sale.
			out.ContactID = c.ID
			if c.Name != nil {
				out.Name = strings.TrimSpace(*c.Name)
			}
			uc.gradeBuyer(ctx, accountID, c.ID, out)
			break
		}
	}

	if uc.buyerHistory != nil && cnpj != "" && withHistory {
		if rows, err := uc.buyerHistory.ListByBuyer(ctx, cnpj, radarBuyerContractScan); err == nil {
			out.Scanned = true
			out.HistoryVia = src.Via
			// Newest first (the repository's own order), so the first row that
			// passes the trade gate is the incumbent worth naming.
			for _, c := range rows {
				// The gate's second argument is the BUYER, not the notice —
				// a body named "Hospital …" makes a badly-written object a
				// health purchase. Each row carries its own, so pass that
				// and never the radar item's title (the tender's object),
				// which would answer a different question entirely.
				if !domain.IsGoodsPurchaseInOurTrade(c.Object, c.BuyerName) {
					continue
				}
				out.Contracts++
				if out.LastSupplier == "" && out.LastObject == "" {
					out.LastSupplier = strings.TrimSpace(c.SupplierName)
					out.LastObject = strings.TrimSpace(c.Object)
					out.LastContractAt = c.SignedAt
				}
			}
		}
	}

	// Nothing learned is not a block worth sending. An empty section reads as
	// a failure; an absent one reads as nothing to say.
	if out.Relationship == BuyerCold && out.Contracts == 0 && !out.Scanned {
		return nil
	}
	return out
}

// gradeBuyer grades what this account has done about a buyer, from the
// strongest evidence down, and fills in the card it can be acted on through.
// Every rung needs a record; a missing reader or a failed read leaves the grade
// where it was rather than promoting it, because every wrong answer here is a
// claim the seller repeats to a stranger.
func (uc *RadarUseCase) gradeBuyer(ctx context.Context, accountID, contactID string, out *BuyerContext) {
	sold := false
	if uc.buyerSales != nil {
		if got, err := uc.buyerSales.HasSale(ctx, accountID, contactID); err == nil && got {
			sold = true
			out.Relationship = BuyerSold
		}
	}
	if uc.buyerLeads == nil {
		// Without the lead reader there is no evidence of anything having
		// been done — and "cold" is the only answer that cannot be wrong in
		// the expensive direction.
		return
	}
	state, err := uc.buyerLeads.LeadStateForContact(ctx, accountID, contactID)
	if err != nil || state.Status == "" {
		return
	}
	// The card travels even when the grade came from a sale: the seller still
	// wants to open it, and a sale does not remove it from the board.
	out.LeadID, out.Stage = state.LeadID, state.Stage
	if sold {
		return
	}
	switch {
	case leadStatusProvesASend(state.Status):
		out.Relationship = BuyerContacted
	case state.Drafted:
		out.Relationship = BuyerQueued
	default:
		out.Relationship = BuyerListed
	}
}
