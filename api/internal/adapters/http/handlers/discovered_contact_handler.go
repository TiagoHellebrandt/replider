package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// "Traz os clientes que você já tem" — read from her own Sent folder.
//
// Every route here is about a REVIEW. The scan produces candidates, the list
// shows them, and only an explicit selection creates anything. There is
// deliberately no "import everything" endpoint: the whole value of the review is
// that a human looked, and a bulk button would make looking optional.
type DiscoveredContactHandler struct {
	uc *usecases.DiscoverContactsUseCase
	// creator turns a confirmed candidate into a contact + lead. Held here
	// rather than inside the use case so the lead rules (funnel ownership,
	// dedupe, the identity index) stay in the one place that owns them.
	creator usecases.DiscoveredLeadCreator
}

func NewDiscoveredContactHandler(
	uc *usecases.DiscoverContactsUseCase,
	creator usecases.DiscoveredLeadCreator,
) *DiscoveredContactHandler {
	return &DiscoveredContactHandler{uc: uc, creator: creator}
}

// HandleScan serves POST /v1/channels/:id/discover-contacts.
//
// Synchronous on purpose: the customer clicks "buscar meus clientes" and waits,
// and a run that reports what it found (read 4000, found 60, 12 skipped) is
// worth more than one that finishes invisibly. The use case caps the walk.
// @Summary     Learn the customer list from a mailbox's Sent folder
// @Description Reads To/Cc/Date of sent messages — never bodies — and records the addresses as CANDIDATES. Creates nothing until a human confirms.
// @Tags        Contacts
// @Produce     json
// @Param       id path string true "Email channel ID"
// @Success     200 {object} usecases.ScanResult
// @Failure     400 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /channels/{id}/discover-contacts [post]
func (h *DiscoveredContactHandler) HandleScan(c *gin.Context) {
	out, err := h.uc.Scan(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleList serves GET /v1/discovered-contacts.
// @Summary     Customer-list candidates awaiting review
// @Description Undecided candidates, most-emailed first. sent_count is the ranking signal: 40 messages over two years is a customer, one is a stranger who asked a question.
// @Tags        Contacts
// @Produce     json
// @Param       limit  query int false "Page size (default 50, max 200)"
// @Param       offset query int false "Page offset"
// @Success     200 {object} discoveredListResponse
// @Security    BearerAuth
// @Router      /discovered-contacts [get]
func (h *DiscoveredContactHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	accountID := getAccountID(c)
	items, err := h.uc.ListPending(c.Request.Context(), accountID, int32(limit), int32(offset)) //nolint:gosec // G115: pagination() clamps to small non-negative ints
	if err != nil {
		respondError(c, err)
		return
	}
	// Counted over the WHOLE pending set, not the page: the screen's job is to
	// tell her how much is waiting, and a total derived from a page shrinks
	// silently as the list grows.
	total, err := h.uc.CountPending(c.Request.Context(), accountID)
	if err != nil {
		total = int64(len(items))
	}
	if items == nil {
		items = []*domain.DiscoveredContact{}
	}
	c.JSON(http.StatusOK, discoveredListResponse{Items: items, Total: total})
}

type discoveredListResponse struct {
	Items []*domain.DiscoveredContact `json:"items"`
	Total int64                       `json:"total"`
}

type discoveredDecisionRequest struct {
	// Addresses is the human's selection. Required — there is no "all".
	Addresses []string `json:"addresses" binding:"required,min=1"`
	// FunnelID is where imported customers land. Required on import: a lead
	// with no funnel is a row nobody sees.
	FunnelID    string `json:"funnel_id"`
	OwnerUserID string `json:"owner_user_id"`
}

// HandleImport serves POST /v1/discovered-contacts/import.
// @Summary     Import confirmed candidates as leads
// @Description Creates a contact + lead per confirmed address, keyed by the address so a later inbound from the same person lands on the same contact. Already-imported selections are skipped, never duplicated.
// @Tags        Contacts
// @Accept      json
// @Produce     json
// @Param       body body discoveredDecisionRequest true "Selection + target funnel"
// @Success     200 {object} usecases.ImportResult
// @Failure     400 {object} domain.AppError
// @Security    BearerAuth
// @Router      /discovered-contacts/import [post]
func (h *DiscoveredContactHandler) HandleImport(c *gin.Context) {
	var req discoveredDecisionRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.uc.Import(c.Request.Context(), usecases.ImportInput{
		AccountID:   getAccountID(c),
		FunnelID:    req.FunnelID,
		Addresses:   req.Addresses,
		OwnerUserID: req.OwnerUserID,
	}, h.creator)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleDismiss serves POST /v1/discovered-contacts/dismiss.
//
// Separate from import because "not a customer" is not "already done": one
// column would either re-offer what she refused or hide what she has not
// decided.
// @Summary     Mark candidates as not customers
// @Tags        Contacts
// @Accept      json
// @Produce     json
// @Param       body body discoveredDecisionRequest true "Selection"
// @Success     204
// @Failure     400 {object} domain.AppError
// @Security    BearerAuth
// @Router      /discovered-contacts/dismiss [post]
func (h *DiscoveredContactHandler) HandleDismiss(c *gin.Context) {
	var req discoveredDecisionRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.Dismiss(c.Request.Context(), getAccountID(c), req.Addresses); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
