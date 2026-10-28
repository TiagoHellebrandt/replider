package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/adapters/documents"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// ProposalHandler exposes priced offers.
//
// There is deliberately no "send" route. The only way out of a proposal is the
// approval queue, which is what makes pillar 4 — "envie com a confirmação prévia
// dela" — structural rather than a habit.
type ProposalHandler struct {
	uc *usecases.ProposalUseCase
	// brand/customers are optional (SetPDFContext): they supply the names on
	// the printed page, never its numbers.
	brand     pdfBrandReader
	customers pdfCustomerNamer
}

func NewProposalHandler(uc *usecases.ProposalUseCase) *ProposalHandler {
	return &ProposalHandler{uc: uc}
}

type proposalItemResponse struct {
	ProductID      string `json:"product_id"`
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	TotalCents     int64  `json:"total_cents"`
}

// proposalResponse carries the total and the currency TOGETHER, always. A client
// that receives cents without a currency has to guess, and the guess is right
// until the first account that sells in the other one.
type proposalResponse struct {
	ID         string                 `json:"id"`
	LeadID     string                 `json:"lead_id"`
	Status     string                 `json:"status"`
	Currency   string                 `json:"currency"`
	Body       string                 `json:"body"`
	Items      []proposalItemResponse `json:"items"`
	TotalCents int64                  `json:"total_cents"`
	// Rendered is exactly what the approval queue will show. Sent so the review
	// screen never has to reproduce the server's formatting and disagree with it.
	Rendered  string `json:"rendered"`
	SentAt    string `json:"sent_at,omitempty"`
	CreatedAt string `json:"created_at"`
}

func toProposalResponse(p *domain.Proposal) proposalResponse {
	items := make([]proposalItemResponse, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, proposalItemResponse{
			ProductID:      it.ProductID,
			Name:           it.Name,
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
			TotalCents:     it.TotalCents(),
		})
	}

	out := proposalResponse{
		ID:         p.ID,
		LeadID:     p.LeadID,
		Status:     string(p.Status),
		Currency:   p.Currency,
		Body:       p.Body,
		Items:      items,
		TotalCents: p.TotalCents(),
		Rendered:   usecases.RenderProposal(p),
		CreatedAt:  p.CreatedAt.Format(time.RFC3339),
	}
	if p.SentAt != nil {
		out.SentAt = p.SentAt.Format(time.RFC3339)
	}
	return out
}

// HandleCreate prices an offer against the account's catalogue.
//
//	@Summary		Create a proposal
//	@Description	Prices the requested products against the account's own catalogue and stores the offer as a draft. Sends nothing.
//	@Tags			proposals
//	@Accept			json
//	@Produce		json
//	@Param			body	body		usecases.CreateProposalInput	true	"Lead and lines"
//	@Success		201		{object}	proposalResponse
//	@Failure		400		{object}	domain.AppError
//	@Failure		404		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/proposals [post]
func (h *ProposalHandler) HandleCreate(c *gin.Context) {
	var req usecases.CreateProposalInput
	if !bindJSON(c, &req) {
		return
	}

	p, err := h.uc.Create(c.Request.Context(), getAccountID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProposalResponse(p))
}

// HandleGet returns one offer with its lines.
//
//	@Summary		Get a proposal
//	@Tags			proposals
//	@Produce		json
//	@Param			id	path		string	true	"Proposal ID"
//	@Success		200	{object}	proposalResponse
//	@Failure		404	{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/proposals/{id} [get]
func (h *ProposalHandler) HandleGet(c *gin.Context) {
	p, err := h.uc.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProposalResponse(p))
}

// HandleListForLead is the history of what this customer has been offered.
//
//	@Summary		List a lead's proposals
//	@Description	Newest first — the last offer is the one that shapes the next.
//	@Tags			proposals
//	@Produce		json
//	@Param			id	path		string	true	"Lead ID"
//	@Success		200	{array}		proposalResponse
//	@Failure		404	{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/leads/{id}/proposals [get]
func (h *ProposalHandler) HandleListForLead(c *gin.Context) {
	list, err := h.uc.ListForLead(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]proposalResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toProposalResponse(p))
	}
	c.JSON(http.StatusOK, out)
}

type replaceLinesRequest struct {
	Lines []domain.ProposalLine `json:"lines"`
}

// HandleReplaceLines re-prices the offer from today's catalogue.
//
//	@Summary		Replace a proposal's lines
//	@Description	Re-prices from the catalogue as it stands now. Refused once the offer has been sent.
//	@Tags			proposals
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Proposal ID"
//	@Param			body	body		replaceLinesRequest	true	"Lines"
//	@Success		200		{object}	proposalResponse
//	@Failure		400		{object}	domain.AppError
//	@Failure		404		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/proposals/{id}/lines [put]
func (h *ProposalHandler) HandleReplaceLines(c *gin.Context) {
	var req replaceLinesRequest
	if !bindJSON(c, &req) {
		return
	}

	p, err := h.uc.ReplaceLines(c.Request.Context(), getAccountID(c), c.Param("id"), req.Lines)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProposalResponse(p))
}

type proposalBodyRequest struct {
	Body string `json:"body"`
}

// HandleSetBody stores the salesperson's own words.
//
//	@Summary		Set a proposal's text
//	@Tags			proposals
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Proposal ID"
//	@Param			body	body		proposalBodyRequest	true	"Text"
//	@Success		200		{object}	proposalResponse
//	@Failure		404		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/proposals/{id}/body [put]
func (h *ProposalHandler) HandleSetBody(c *gin.Context) {
	var req proposalBodyRequest
	if !bindJSON(c, &req) {
		return
	}

	p, err := h.uc.SetBody(c.Request.Context(), getAccountID(c), c.Param("id"), req.Body)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProposalResponse(p))
}

// HandleSendForApproval puts the offer in the approval queue.
//
// The route is named for what it does. It does NOT reach the customer: a person
// still reads the draft and decides, which is the same gate every cold approach
// passes through.
//
//	@Summary		Send a proposal for approval
//	@Description	Renders the offer into the lead's approach draft so it appears in the approval queue. Delivers nothing to the customer.
//	@Tags			proposals
//	@Produce		json
//	@Param			id	path		string	true	"Proposal ID"
//	@Success		200	{object}	proposalResponse
//	@Failure		404	{object}	domain.AppError
//	@Failure		503	{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/proposals/{id}/send-for-approval [post]
func (h *ProposalHandler) HandleSendForApproval(c *gin.Context) {
	p, err := h.uc.SendForApproval(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProposalResponse(p))
}

// pdfContextReader supplies the letterhead: who is sending, and to whom.
//
// Narrow ports rather than the broad repositories, because the only thing this
// needs from each is one name. Both optional — a proposal with no company name
// on it is worse-looking, not unusable, and refusing to produce the file over
// a missing brand kit would put the salesperson back in Word.
type pdfBrandReader interface {
	Get(ctx context.Context, accountID string) (*domain.BrandKit, error)
}

type pdfCustomerNamer interface {
	NameForProposal(ctx context.Context, accountID, leadID string) string
}

// SetPDFContext wires the names printed on the document.
func (h *ProposalHandler) SetPDFContext(brand pdfBrandReader, customers pdfCustomerNamer) {
	h.brand = brand
	h.customers = customers
}

// HandleDownloadPDF serves GET /v1/proposals/:id/pdf.
//
// The file the salesperson actually sends. Being second with the price is the
// commonest way to lose a deal in this market, and until now the proposal
// screen could compose the lines and the totals and then hand over nothing —
// the work finished and the seller went back to Word.
//
//	@Summary		Download the proposal as a PDF
//	@Description	One page: who is sending, who it is for, the salesperson's own words, the priced lines, the total, and the validity. Nothing is sent — this is a file to hand over.
//	@Tags			proposals
//	@Produce		application/pdf
//	@Param			id	path		string	true	"Proposal ID"
//	@Success		200	{file}		binary
//	@Failure		404	{object}	domain.AppError
//	@Failure		409	{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/proposals/{id}/pdf [get]
func (h *ProposalHandler) HandleDownloadPDF(c *gin.Context) {
	ctx := c.Request.Context()
	accountID := getAccountID(c)

	p, err := h.uc.Get(ctx, accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}

	doc := documents.ProposalDocument{
		Reference: p.ID,
		Currency:  p.Currency,
		Items:     p.Items,
		Body:      p.Body,
		CreatedAt: p.CreatedAt,
	}
	// Best-effort on both: a missing brand kit costs a letterhead, never the
	// document.
	if h.brand != nil {
		if kit, err := h.brand.Get(ctx, accountID); err == nil && kit != nil {
			doc.Seller = kit.CompanyName
			doc.Signature = kit.Signature
		}
	}
	if h.customers != nil {
		doc.Customer = h.customers.NameForProposal(ctx, accountID, p.LeadID)
	}
	// Thirty days from the day it was written. A date the salesperson did not
	// choose, but one every purchasing department expects to see — and the
	// alternative is a price that stands forever.
	if !p.CreatedAt.IsZero() {
		until := p.CreatedAt.AddDate(0, 0, proposalValidityDays)
		doc.ValidUntil = &until
	}

	out, err := documents.RenderProposal(doc)
	if err != nil {
		// The only way here is an offer with no lines, which is a state the
		// screen can fix rather than an outage.
		respondError(c, domain.ErrProposalHasNoLines)
		return
	}

	// attachment, not inline: the point is a file the seller can forward.
	c.Header("Content-Disposition",
		`attachment; filename="proposta-`+shortProposalRef(p.ID)+`.pdf"`)
	c.Data(http.StatusOK, "application/pdf", out)
}

// proposalValidityDays is how long the prices stand.
//
// Thirty days because that is what this market reads as normal; longer and a
// purchasing department will hold you to a price the exchange rate moved under.
const proposalValidityDays = 30

func shortProposalRef(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}
