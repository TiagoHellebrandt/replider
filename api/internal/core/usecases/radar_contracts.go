package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// The forward-looking half of the radar.
//
// Every other collector reacts to something that already happened: a tender was
// published, a customer's stock is running down, a date is on the calendar. A
// public contract carries a published end date, so the purchase that replaces
// it is a known event three to six months before any edital exists — and by the
// time an edital IS published the specification has usually been written around
// whoever supplied the reference quote.
//
// These rows ride the radar instead of a screen of their own because a seller
// who has to remember WHICH list holds the good rows ends up working one list.
// That is the same reason the approval queue stopped being a tab: see Ready.

// radarContractReader is the calendar use case narrowed to its one call.
//
// The radar reuses it whole rather than re-querying public_contracts, because
// the calendar owns the territory narrow, the keyword resolution and the grace
// window — and a second copy of those rules is exactly how two screens start
// disagreeing about which contracts are "mine".
type radarContractReader interface {
	Execute(ctx context.Context, accountID string, windowDays int) (*ContractCalendar, error)
}

// radarLeadReader answers "is the offer already written?" for the whole feed.
type radarLeadReader interface {
	ListForApprovalQueue(ctx context.Context, f domain.ApprovalQueueFilter, limit, offset int32) ([]*domain.Lead, error)
}

// SetContractCalendar wires the forward-looking collector.
func (uc *RadarUseCase) SetContractCalendar(r radarContractReader) { uc.contracts = r }

// SetLeads wires the "already drafted" cross-check.
func (uc *RadarUseCase) SetLeads(r radarLeadReader) { uc.leads = r }

// radarPendingLimit bounds the cross-check read. A queue deeper than this is
// its own problem; the rows past it simply render as "prepare an offer", which
// is the safe direction (see markReady).
const radarPendingLimit = 200

func (uc *RadarUseCase) collectContracts(ctx context.Context, accountID string) ([]RadarItem, error) {
	if uc.contracts == nil {
		return []RadarItem{}, nil
	}
	cal, err := uc.contracts.Execute(ctx, accountID, CalendarWindowDays)
	if err != nil {
		return nil, err
	}
	out := make([]RadarItem, 0, len(cal.Entries))
	for _, e := range cal.Entries {
		kind, edge := RadarContrato, RadarEdgeBeforeTender
		// An ata is not "early" — it is a standing licence to sell at a
		// registered price, and a buyer who rides other bodies' atas can buy
		// today. Different claim, different badge.
		if e.PriceRegistry || e.RidesOthers {
			kind, edge = RadarAta, RadarEdgeNoTender
		}
		out = append(out, RadarItem{
			Type: kind, ID: "contract-" + e.PNCPID,
			Title:      e.BuyerName,
			Context:    contractContext(e),
			ValueCents: e.ValueCents,
			Deadline:   e.ExpiresAt,
			URL:        e.URL,
			City:       e.City,
			UF:         e.UF,
			Incumbent:  e.SupplierName,
			Edge:       edge,
			// The published object, whole. The card's sentence is composed
			// and truncates; this is the text the seller has to read before
			// deciding whether the contract is theirs to chase.
			Detail:    strings.TrimSpace(e.Object),
			CNPJ:      e.BuyerCNPJ,
			Reference: e.PNCPID,
		})
	}
	return out, nil
}

// contractContext writes the row's sentence in the radar's own voice: place
// first, then what is happening, then what is being bought, then who holds it.
func contractContext(e ContractCalendarEntry) string {
	parts := make([]string, 0, 5)
	if place := strings.Trim(strings.TrimSpace(e.City)+"/"+strings.TrimSpace(e.UF), "/"); place != "" {
		parts = append(parts, place)
	}
	if e.PriceRegistry {
		parts = append(parts, "ata de registro de preços aberta")
	} else {
		parts = append(parts, "contrato vencendo")
	}
	if obj := strings.TrimSpace(e.Object); obj != "" {
		parts = append(parts, obj)
	}
	// Absence is stated, never skipped: a row that just omits the supplier
	// reads as "nobody supplies them today", which is a different and much
	// more encouraging claim than the record actually makes.
	if s := strings.TrimSpace(e.SupplierName); s != "" {
		parts = append(parts, "fornecedor atual: "+s)
	} else {
		parts = append(parts, "o registro não nomeia fornecedor")
	}
	if e.RidesOthers {
		parts = append(parts, "este órgão compra por adesão")
	}
	return strings.Join(parts, " · ")
}

// markReady flags the rows whose offer is already written and waiting on a
// human, so the card's own verb can be "read it and send" instead of "prepare
// an offer".
//
// This is what lets the feed absorb the approval queue rather than keep it as a
// separate tab. A failed read leaves every row unmarked on purpose: a row that
// wrongly says "prepare" costs one click, while a row that wrongly says "ready
// to send" opens onto nothing and reads as broken.
//
// It scopes exactly like LeadUseCase.ApprovalQueue, and for the same reason:
// the mark is a claim about MY work. A vendedor seeing "já está escrita" over a
// colleague's draft would open a queue that does not contain it, and an
// attendant — who the queue refuses outright — must not learn the shape of a
// pipeline they cannot reach.
func (uc *RadarUseCase) markReady(ctx context.Context, actor domain.Actor, items []RadarItem) {
	if uc.leads == nil || len(items) == 0 || strings.TrimSpace(actor.AccountID) == "" {
		return
	}
	filter := domain.ApprovalQueueFilter{
		AccountID: actor.AccountID,
		Status:    domain.LeadStatusPendingApproval,
	}
	switch {
	case domain.RoleSatisfies(actor.Role, domain.RoleGestor):
		// The whole account's queue, same as the screen shows them.
	case actor.Role == domain.RoleVendedor:
		filter.OwnerUserID = actor.UserID
	default:
		// Everyone else (an attendant, or a caller with no role resolved)
		// gets no marks rather than someone else's.
		return
	}
	pending, err := uc.leads.ListForApprovalQueue(ctx, filter, radarPendingLimit, 0)
	if err != nil {
		return
	}
	// Two indexes because the feed carries two different handles: a tender the
	// auto-offer already drafted knows its LeadID, while a park hit or a
	// reorder only knows the CONTACT it belongs to.
	byLead := make(map[string]bool, len(pending))
	byContact := make(map[string]bool, len(pending))
	for _, l := range pending {
		if l == nil {
			continue
		}
		byLead[l.ID] = true
		if c := strings.TrimSpace(l.ContactID); c != "" {
			byContact[c] = true
		}
	}
	for i := range items {
		if id := items[i].LeadID; id != "" && byLead[id] {
			items[i].Ready = true
			continue
		}
		if c := items[i].ContactID; c != "" && byContact[c] {
			items[i].Ready = true
		}
	}
}
