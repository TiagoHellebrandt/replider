package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// LeadHandler exposes the dashboard's outbound-sales lead/CRM endpoints
// (US-015): list a funnel's leads, fetch one lead, edit collected metadata
// manually (US-014), and move a lead through the pipeline status. Account
// scoping is enforced by the use case via the accountID claim.
type LeadHandler struct {
	leadUC     *usecases.LeadUseCase
	approachUC *usecases.ApproachUseCase
	// enrichUC is optional (US-023): set via SetEnrichUseCase. When nil the
	// enrich endpoint returns 503 instead of panicking.
	enrichUC *usecases.EnrichLeadUseCase
	// timelineUC is optional (setter-wired): the lead's conversation history,
	// authorised by lead ownership rather than by the inbox's role. nil → 503.
	timelineUC *usecases.LeadTimelineUseCase
	// convFinder is optional (setter-wired): nil means the lead panel gets no
	// link to the conversation.
	convFinder LeadConversationFinder
	// reviewUC is optional (second-opinion review). nil → 503 rather than
	// a nil dereference mid-approval.
	reviewUC *usecases.ApproachReviewUseCase
	// scheduleReader is optional (phase 1b): labels queue cards whose send
	// is already scheduled.
	scheduleReader scheduleReader
	// sendChannels is optional (setter-wired): whether the account has
	// anywhere to send from. Nil leaves the queue unannotated, which is the
	// permissive answer.
	sendChannels queueSendChannelLister
	// queueContacts is optional (setter-wired): resolves a queue page's
	// contacts in one round trip so each card can say whether it has an
	// address. Nil leaves the cards unannotated — the permissive answer.
	queueContacts queueContactReader
	// leadParkUC is optional (Clientes redesign): the panel's CNES card.
	leadParkUC *usecases.LeadParkUseCase
	// replyUC is optional (setter-wired): answering the customer from the
	// lead, gated by the lead's ownership. nil → 503. See lead_reply_handler.go.
	replyUC *usecases.LeadReplyUseCase
}

// SetReviewUseCase wires the optional approach reviewer.
func (h *LeadHandler) SetReviewUseCase(uc *usecases.ApproachReviewUseCase) { h.reviewUC = uc }

func NewLeadHandler(leadUC *usecases.LeadUseCase, approachUC *usecases.ApproachUseCase) *LeadHandler {
	return &LeadHandler{leadUC: leadUC, approachUC: approachUC}
}

// SetEnrichUseCase wires the optional lead-enrichment use case (US-023),
// keeping the constructor signature stable for existing call sites/tests.
func (h *LeadHandler) SetEnrichUseCase(uc *usecases.EnrichLeadUseCase) {
	h.enrichUC = uc
}

// HandleListByFunnel serves GET /v1/funnels/:id/leads with an optional
// ?status= filter.
// @Summary     List a funnel's leads
// @Description Leads of one funnel, newest first. A `vendedor` sees only what they own plus the team-visible ones.
// @Tags        Leads
// @Produce     json
// @Param       id     path  string true  "Funnel ID"
// @Param       status query string false "new | prospected | enriching | enriched | pending_approval | contacted | responded | qualified | won | lost"
// @Param       limit  query int    false "Page size (default 50, max 200)"
// @Param       offset query int    false "Page offset"
// @Success     200 {array}  domain.Lead
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /funnels/{id}/leads [get]
func (h *LeadHandler) HandleListByFunnel(c *gin.Context) {
	status := domain.LeadStatus(c.Query("status"))
	limit, offset := pagination(c)
	leads, err := h.leadUC.ListByFunnelForActor(c.Request.Context(), actorFrom(c), c.Param("id"), status, int32(limit), int32(offset)) //nolint:gosec // G115: pagination() clamps limit/offset to small non-negative ints
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, leads)
}

// queueSendChannelLister is the one channel fact the queue needs: is there
// anywhere to send from. A narrow port rather than domain.ChannelRepository,
// which dozens of test mocks implement.
type queueSendChannelLister interface {
	ListByAccountID(ctx context.Context, accountID string) ([]*domain.Channel, error)
}

// SetSendChannels wires the "can this account send at all" lookup.
func (h *LeadHandler) SetSendChannels(l queueSendChannelLister) { h.sendChannels = l }

// queueContactReader is the one contact fact the queue needs: the addresses
// on a page of leads, in one round trip.
type queueContactReader interface {
	ListByIDs(ctx context.Context, accountID string, ids []string) ([]*domain.Contact, error)
}

// SetQueueContacts wires the per-card "is there anywhere to post this"
// lookup.
func (h *LeadHandler) SetQueueContacts(r queueContactReader) { h.queueContacts = r }

// annotateReach marks the cards that have no address any of this account's
// channels could deliver to.
//
// The account-level banner says "you have no door"; this says "this one has no
// address", and the difference is the whole backlog. An account with a mailbox
// and 70 phone-only contacts passes the banner and still cannot post a single
// letter — production had exactly that, 50 finished offers deep.
//
// One batched read per page, best-effort: a failed lookup leaves every card
// unannotated rather than marking good ones undeliverable.
func (h *LeadHandler) annotateReach(c *gin.Context, items []approvalQueueItem) {
	if h.queueContacts == nil || h.sendChannels == nil || len(items) == 0 {
		return
	}
	ctx := c.Request.Context()
	accountID := getAccountID(c)

	channels, err := h.sendChannels.ListByAccountID(ctx, accountID)
	if err != nil {
		return
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		if it.ContactID != "" {
			ids = append(ids, it.ContactID)
		}
	}
	if len(ids) == 0 {
		return
	}
	contacts, err := h.queueContacts.ListByIDs(ctx, accountID, ids)
	if err != nil {
		slog.WarnContext(ctx, "approval queue: contact lookup failed", "error", err)
		return
	}
	byID := make(map[string]*domain.Contact, len(contacts))
	for _, ct := range contacts {
		if ct != nil {
			byID[ct.ID] = ct
		}
	}
	for i := range items {
		// A contact that did not come back is unresolved, not proven
		// unreachable. Silence beats a false accusation on a card someone is
		// about to act on.
		ct, ok := byID[items[i].ContactID]
		if !ok {
			continue
		}
		if !domain.CanReachContact(channels, ct) {
			items[i].SendBlockedReason = domain.QueueBlockedNoAddress
			// The one door left open. A blocked card is usually blocked
			// because the account has a mailbox and this buyer only left a
			// phone number — which the seller can dial themselves.
			if ct.Phone != nil {
				items[i].WhatsAppPhone = strings.TrimSpace(*ct.Phone)
			}
		}
	}
}

// sendBlockedReason names what stops this queue being sent, or "".
//
// The queue used to render "Aprovar e enviar" as an enabled primary button on
// an account with no channel at all: 41 written offers, one big blue button,
// and the failure only after the click. Production shipped exactly that.
//
// Best-effort — a failed lookup reports nothing rather than blocking a queue
// that would have sent fine.
func (h *LeadHandler) sendBlockedReason(c *gin.Context) string {
	if h.sendChannels == nil {
		return ""
	}
	channels, err := h.sendChannels.ListByAccountID(c.Request.Context(), getAccountID(c))
	if err != nil {
		slog.WarnContext(c.Request.Context(), "approval queue: channel lookup failed", "error", err)
		return ""
	}
	if domain.AccountCanSend(channels) {
		return ""
	}
	return domain.QueueBlockedNoChannel
}

// HandleApprovalQueue serves GET /v1/leads/approval-queue — every draft
// waiting on a human, across all funnels (US-M020).
// @Summary     List every draft awaiting a human
// @Description Approach drafts that no person has approved yet — every funnel by default. Nothing in this list has been sent. `total` and `by_funnel` count the WHOLE queue, not the returned page.
// @Tags        Leads
// @Produce     json
// @Param       funnel_id query string false "Narrow to one funnel (default: every funnel)"
// @Param       sort      query string false "oldest (default) | newest"
// @Param       limit     query int    false "Page size (default 50, max 200)"
// @Param       offset    query int    false "Page offset"
// @Success     200 {object} approvalQueueResponse
// @Failure     401 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/approval-queue [get]
func (h *LeadHandler) HandleApprovalQueue(c *gin.Context) {
	limit, offset := pagination(c)
	opts := usecases.ApprovalQueueOptions{
		FunnelID: c.Query("funnel_id"),
		// Anything other than an explicit "newest" keeps the oldest-first
		// default — a typo in a query param must not silently reorder the
		// backlog so the stalest draft sinks to the bottom.
		NewestFirst: c.Query("sort") == "newest",
	}
	page, err := h.leadUC.ApprovalQueue(c.Request.Context(), actorFrom(c), opts, int32(limit), int32(offset)) //nolint:gosec // G115: pagination() clamps limit/offset to small non-negative ints
	if err != nil {
		respondError(c, err)
		return
	}
	items := make([]approvalQueueItem, 0, len(page.Items))
	for _, l := range page.Items {
		items = append(items, approvalQueueItem{
			Lead: l,
			// A queued draft never carries a file yet: attachments are added
			// when the human composes the send, so "segue em anexo" in a
			// draft is always a mismatch worth flagging.
			Findings: findingsOrEmpty(l.ApproachDraft, false),
		})
	}
	// Cards whose send is already scheduled get labeled, not hidden — one
	// batched lookup over the page. Best-effort: a failed read leaves the
	// labels off, never the queue.
	if h.scheduleReader != nil && len(items) > 0 {
		ids := make([]string, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		if pending, err := h.scheduleReader.PendingByLeadIDs(
			c.Request.Context(), getAccountID(c), ids); err == nil {
			for i := range items {
				if at, ok := pending[items[i].ID]; ok {
					when := at
					items[i].ScheduledFor = &when
				}
			}
		}
	}
	h.annotateReach(c, items)
	byFunnel := page.ByFunnel
	if byFunnel == nil {
		byFunnel = []domain.QueueFunnelCount{}
	}
	c.JSON(http.StatusOK, approvalQueueResponse{
		Items:             items,
		Total:             page.Total,
		ByFunnel:          byFunnel,
		SendBlockedReason: h.sendBlockedReason(c),
	})
}

// approvalQueueResponse wraps the page in the shape of the backlog behind it.
//
// This endpoint used to answer with a bare array. An array cannot say how much
// it left out, so the client had no way to tell a 50-draft queue from the first
// page of 340, and no way to build a funnel filter that was not a lie about the
// rest of the work. Total and ByFunnel are counted over the whole queue.
type approvalQueueResponse struct {
	Items []approvalQueueItem `json:"items"`
	// Total is every draft awaiting the caller, ignoring funnel_id and paging.
	Total int64 `json:"total"`
	// ByFunnel is empty when the count failed — the list still came back, so
	// the screen degrades to no chips rather than to no queue.
	ByFunnel []domain.QueueFunnelCount `json:"by_funnel"`
	// SendBlockedReason names what stops these drafts going out, or is
	// absent. The screen disables the send controls and says so, instead of
	// offering a button that fails after the click.
	SendBlockedReason string `json:"send_blocked_reason,omitempty"`
}

// approvalQueueItem is the lead PLUS what the promise detector found in the
// draft it is carrying.
//
// The detector is regexes over text already in memory — free, so it runs for
// every card instead of on demand. That is the whole point: the operator sees
// "this sentence commits you to a deadline" while scanning the queue, not
// after they open something. The AI reviewer stays on demand
// (POST /leads/:id/review-approach) because it costs a model call.
//
// Embedded pointer so the lead's own fields stay at the top level — the
// client reads contact_label/approach_draft/enrichment straight off the item.
type approvalQueueItem struct {
	*domain.Lead
	Findings []domain.ApproachFinding `json:"approach_findings"`
	// ScheduledFor labels an approved-but-capped card ("sai amanhã às 08h")
	// instead of hiding it — the send is planned, and planned work the
	// operator cannot see is work they will redo.
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	// SendBlockedReason names what stops THIS card going out, when the queue
	// as a whole is fine. Today the only value is NO_ADDRESS.
	SendBlockedReason string `json:"send_blocked_reason,omitempty"`
	// WhatsAppPhone is the way out of a NO_ADDRESS card: the platform cannot
	// deliver to this contact, but the SELLER can, from the phone already in
	// their pocket.
	//
	// Stamped only on a blocked card, and only when the contact actually has a
	// number — offering the escape hatch on a card that will send normally
	// would teach people to route around the product's own send, and offering
	// it with no number to dial is a button that lies.
	//
	// This is deliberately not a WhatsApp integration. The Business API bills
	// per conversation and needs approved templates; a wa.me link costs
	// nothing, needs no approval, and matches how this trade already works.
	WhatsAppPhone string `json:"whatsapp_phone,omitempty"`
}

// scheduleReader is the one slice of the schedule repo the queue needs.
type scheduleReader interface {
	PendingByLeadIDs(ctx context.Context, accountID string, leadIDs []string) (map[string]time.Time, error)
}

// SetScheduleReader wires the approach-schedule lookup (optional; nil leaves
// queue cards unlabeled).
func (h *LeadHandler) SetScheduleReader(r scheduleReader) { h.scheduleReader = r }

// findingsOrEmpty never returns nil: a JSON `null` and an empty list mean the
// same thing to a human and different things to a client, and "no problems
// found" is the answer this field exists to give.
//
// Three checks, all free, all deterministic: promises and invented facts,
// unfilled placeholders, and a message that says it carries a file when
// nothing is attached. The last one is the mistake that makes a salesperson
// look careless at exactly the moment they were trying not to — the customer
// answers "nothing came through" and the thread starts on an apology.
func findingsOrEmpty(text string, hasAttachment bool) []domain.ApproachFinding {
	out := domain.ApproachFindings(text)
	out = append(out, domain.PlaceholderFindings(text)...)

	if !hasAttachment && domain.PromisesAnAttachment(text) {
		out = append(out, domain.ApproachFinding{
			Kind:  "missing_attachment",
			Quote: text,
		})
	}
	if out == nil {
		return []domain.ApproachFinding{}
	}
	return out
}

// HandlePipelineOverview serves GET /v1/leads/overview — the command
// centre's counters (US-M023).
// @Summary     Pipeline counts by status
// @Tags        Leads
// @Produce     json
// @Success     200 {object} map[string]int
// @Failure     401 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/overview [get]
func (h *LeadHandler) HandlePipelineOverview(c *gin.Context) {
	overview, err := h.leadUC.Overview(c.Request.Context(), actorFrom(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}

// HandleMeetingBooked serves POST /v1/leads/:id/meeting-booked — a human
// confirming a meeting happened (US-M011).
//
// Explicit rather than derived: the platform checks availability but does
// not book, so inferring a meeting from a status change would invent the
// case report's most flattering number. Idempotent per lead.
// @Summary     Record that a meeting was booked
// @Description Stamps the meeting on the lead. Idempotent: a second call for the same lead changes nothing.
// @Tags        Leads
// @Produce     json
// @Param       id path string true "Lead ID"
// @Success     200 {object} domain.Lead
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/meeting-booked [post]
func (h *LeadHandler) HandleMeetingBooked(c *gin.Context) {
	if err := h.leadUC.MarkMeetingBooked(c.Request.Context(), actorFrom(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleGet serves GET /v1/leads/:id.
// @Summary     Get one lead
// @Tags        Leads
// @Produce     json
// @Param       id path string true "Lead ID"
// @Success     200 {object} domain.Lead
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id} [get]
func (h *LeadHandler) HandleGet(c *gin.Context) {
	actor := actorFrom(c)
	lead, err := h.leadUC.GetForActor(c.Request.Context(), actor, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}

	// The conversation with this contact, when there is one. Without it the
	// lead panel is a dead end: the salesperson can see a draft and a
	// timeline but cannot reach the thread the customer is actually in.
	out := leadWithConversation{
		Lead:     lead,
		Findings: findingsOrEmpty(lead.ApproachDraft, false),
	}
	if h.convFinder != nil && lead.ContactID != "" {
		if conv, err := h.convFinder.FindByAccountContact(c.Request.Context(), actor.AccountID, lead.ContactID); err == nil && conv != nil {
			out.ConversationID = conv.ID
		}
	}
	c.JSON(http.StatusOK, out)
}

// leadWithConversation is the lead plus the thread it belongs to. Embedded
// pointer so every existing field stays at the top level.
type leadWithConversation struct {
	*domain.Lead
	// ConversationID is empty when nobody has exchanged messages with this
	// contact yet, which is the normal state of a freshly prospected lead.
	ConversationID string `json:"conversation_id,omitempty"`
	// Findings is the same check the approval queue shows.
	//
	// It used to exist only in the queue's payload, and the lead panel — which
	// ALSO sends, with the same button — never saw it. Two paths to one action,
	// one with a net and one without, and the netless one is what somebody
	// reaches for first: open the lead, generate, send. The check is a regex over
	// text already in memory, so running it here too costs nothing.
	Findings []domain.ApproachFinding `json:"approach_findings"`
}

type assignOwnerReq struct {
	// A null/absent OwnerUserID UNASSIGNS. It is the only way to take a lead
	// from somebody without inventing a new verb.
	OwnerUserID *string `json:"owner_user_id"`
}

// HandleAssignOwner serves PATCH /v1/leads/:id/owner.
//
// The action existed in the use case from the start and never had a route: the
// board marked every lead "no owner" and nothing could change that. An alarm
// nobody can switch off.
// @Summary     Assign a lead's owner
// @Description Hands the lead to a user of the same account, or takes it from everybody with a null owner. Manager and above.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string           true "Lead ID"
// @Param       payload body assignOwnerReq    true "Owner to set, null to unassign"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/owner [patch]
func (h *LeadHandler) HandleAssignOwner(c *gin.Context) {
	var req assignOwnerReq
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.leadUC.AssignOwner(c.Request.Context(), actorFrom(c), c.Param("id"), req.OwnerUserID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

type bulkAssignReq struct {
	LeadIDs []string `json:"lead_ids" binding:"required"`
	// Null UNASSIGNS the whole batch — the way to undo a wrong redistribution
	// without clicking 200 times.
	OwnerUserID *string `json:"owner_user_id"`
}

// HandleBulkAssignOwner serves POST /v1/leads/bulk-assign.
//
// Takes the LIST of ids rather than a filter ("everything unowned in funnel X")
// on purpose: the client counts on screen and sends exactly what it counted, so
// the number confirmed in the dialog is the number that changes. A filter
// evaluated server-side would also catch whatever entered the funnel between the
// count and the click.
// @Summary     Assign many leads to one owner
// @Description Takes the ids the caller counted, not a filter, so the number confirmed is the number that changes. Capped at 200 per call; a lead that no longer exists is skipped and reported rather than aborting the batch.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       payload body bulkAssignReq true "Lead ids and the owner to set"
// @Success     200 {object} usecases.BulkAssignResult
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/bulk-assign [post]
func (h *LeadHandler) HandleBulkAssignOwner(c *gin.Context) {
	var req bulkAssignReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.leadUC.AssignOwnerBulk(c.Request.Context(), actorFrom(c), req.LeadIDs, req.OwnerUserID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type setVisibilityReq struct {
	Visibility string `json:"visibility" binding:"required"`
}

// HandleSetVisibility serves PATCH /v1/leads/:id/visibility.
//
// The `private` read filter has existed since the migration and ran over a
// column that was always 'team', because nothing in the product could write
// it.
// @Summary     Set a lead's visibility
// @Description `team` or `private`. Private hides the lead from everyone but its owner — so it only means anything once an owner is set. Manager and above.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string          true "Lead ID"
// @Param       payload body setVisibilityReq true "team | private"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/visibility [patch]
func (h *LeadHandler) HandleSetVisibility(c *gin.Context) {
	var req setVisibilityReq
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.leadUC.SetVisibility(c.Request.Context(), actorFrom(c),
		c.Param("id"), domain.ResourceVisibility(req.Visibility))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

type updateLeadMetadataReq struct {
	FieldKey string `json:"field_key" binding:"required"`
	// Value is intentionally NOT required: a human may clear a wrongly
	// captured field by saving an empty string.
	Value string `json:"value"`
}

// HandleUpdateMetadata serves PATCH /v1/leads/:id/metadata — a single-field
// manual edit recorded with source:'human'.
// @Summary     Answer one of the funnel's collection fields
// @Description One field per call. These fields are what makes a lead qualified, so writing the last required one can move its status.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string         true "Lead ID"
// @Param       payload body updateLeadMetadataReq true "Field key and value"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/metadata [patch]
func (h *LeadHandler) HandleUpdateMetadata(c *gin.Context) {
	accountID := getAccountID(c)
	// Authorisation BEFORE binding: a caller who may not write this lead should
	// be told that, not that their payload is malformed.
	if !h.ensureLeadMutable(c) {
		return
	}
	var req updateLeadMetadataReq
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.leadUC.UpdateMetadataField(c.Request.Context(), accountID, c.Param("id"), req.FieldKey, req.Value)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

type updateLeadStatusReq struct {
	Status string `json:"status" binding:"required"`
}

// HandleUpdateStatus serves PATCH /v1/leads/:id/status.
// @Summary     Set a lead's status
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string          true "Lead ID"
// @Param       payload body updateLeadStatusReq true "Status to set"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/status [patch]
func (h *LeadHandler) HandleUpdateStatus(c *gin.Context) {
	accountID := getAccountID(c)
	if !h.ensureLeadMutable(c) {
		return
	}
	var req updateLeadStatusReq
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.leadUC.UpdateStatus(c.Request.Context(), accountID, c.Param("id"), domain.LeadStatus(req.Status))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

// ── Kanban board (US-M008) ──────────────────────────────────────────────────

// HandleBoard serves GET /v1/funnels/:id/board — the columns plus the cards
// this caller may see. Scoping is inherited from the use case, so the board
// can never show more than the list endpoint does.
// @Summary     The funnel as a board
// @Description Stages and the cards in each. The board is a VIEW: a card's column is what sets the lead's canonical status.
// @Tags        Leads
// @Produce     json
// @Param       id path string true "Funnel ID"
// @Success     200 {object} usecases.Board
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /funnels/{id}/board [get]
func (h *LeadHandler) HandleBoard(c *gin.Context) {
	limit, offset := pagination(c)
	board, err := h.leadUC.Board(c.Request.Context(), actorFrom(c), c.Param("id"), int32(limit), int32(offset)) //nolint:gosec // G115: pagination() clamps to small non-negative ints
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, board)
}

type moveLeadRequest struct {
	StageID string `json:"stage_id" binding:"required"`
	// The drop slot's neighbors as the client rendered them: before = the
	// card that will sit ABOVE, after = the one BELOW. Either may be absent
	// at an edge; with neither, the card keeps no manual position (automatic
	// order). A never-placed neighbor counts as absent server-side.
	BeforeLeadID string `json:"before_lead_id"`
	AfterLeadID  string `json:"after_lead_id"`
}

type resetBoardOrderRequest struct {
	FunnelID string `json:"funnel_id" binding:"required"`
	StageID  string `json:"stage_id" binding:"required"`
}

type createLeadRequest struct {
	// Name is the person; Company the establishment. Phone OR Email is
	// required (a lead needs a way to be reached); the rest is optional.
	Name     string            `json:"name"`
	Company  string            `json:"company"`
	Role     string            `json:"role"`
	Email    string            `json:"email"`
	Phone    string            `json:"phone"`
	City     string            `json:"city"`
	Metadata map[string]string `json:"metadata"`
}

// HandleCreateLead serves POST /v1/funnels/:id/leads — a lead typed in by
// hand.
//
// Until now the only ways into a funnel were the Google Places import and
// an inbound message, so a referral or a card picked up at a conference had
// no route into the pipeline except the MCP tool. The use case behind it
// already existed and is idempotent, which is why the response says whether
// the lead was created or matched: re-submitting the same phone must read
// as "already in the pipeline", not as a silent duplicate.
// @Summary     Create a lead by hand
// @Description For a referral or a card picked up at a conference — the other ways into a funnel are the directory import and an inbound message. Idempotent on the phone: a repeat submission matches the existing lead instead of creating a second one.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string        true "Funnel ID"
// @Param       payload body createLeadRequest true "Company, person, phone and/or e-mail, city, and any funnel fields"
// @Success     201 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /funnels/{id}/leads [post]
func (h *LeadHandler) HandleCreateLead(c *gin.Context) {
	var req createLeadRequest
	if !bindJSON(c, &req) {
		return
	}
	lead, created, err := h.leadUC.Create(c.Request.Context(), usecases.CreateLeadDTO{
		AccountID: getAccountID(c),
		FunnelID:  c.Param("id"),
		Name:      req.Name,
		Company:   req.Company,
		Role:      req.Role,
		Email:     req.Email,
		Phone:     req.Phone,
		City:      req.City,
		Metadata:  req.Metadata,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"lead": lead, "created": created})
}

// HandleMoveLead serves POST /v1/leads/:id/move — a card drag.
// @Summary     Move a lead to another stage, or reorder it inside its own
// @Description Writes the stage and the canonical status together, so the board and the pipeline never disagree. `before_lead_id`/`after_lead_id` name the drop slot's neighbors and give the card a persisted manual position; a same-stage move only changes that position.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string      true "Lead ID"
// @Param       payload body moveLeadRequest true "Target stage and the drop slot's neighbors"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/move [post]
func (h *LeadHandler) HandleMoveLead(c *gin.Context) {
	// MoveLead re-checks inside the use case; this settles the permission before
	// the payload is read, so every write on a lead answers the same way in the
	// same order.
	if !h.ensureLeadMutable(c) {
		return
	}
	var req moveLeadRequest
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.leadUC.MoveLead(c.Request.Context(), actorFrom(c), c.Param("id"),
		req.StageID, req.BeforeLeadID, req.AfterLeadID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

// HandleResetBoardOrder serves POST /v1/leads/board-order/reset — the column
// header's "ordenar por parados" action.
// @Summary     Reset a column's manual order
// @Description Clears every manually placed position in one Kanban column, restoring the automatic most-stalled-first order. Moves nothing through the pipeline.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       payload body resetBoardOrderRequest true "The column (funnel + stage)"
// @Success     204
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/board-order/reset [post]
func (h *LeadHandler) HandleResetBoardOrder(c *gin.Context) {
	var req resetBoardOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.leadUC.ResetBoardOrder(c.Request.Context(), actorFrom(c), req.FunnelID, req.StageID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// LeadConversationFinder answers "which thread is this contact in".
//
// A narrow port rather than domain.ConversationRepository: that interface is
// implemented by a dozen test mocks, and this needs one method.
type LeadConversationFinder interface {
	FindByAccountContact(ctx context.Context, accountID, contactID string) (*domain.Conversation, error)
}

// SetConversationFinder lets the lead detail carry the thread it belongs to.
func (h *LeadHandler) SetConversationFinder(f LeadConversationFinder) {
	h.convFinder = f
}

// SetLeadParkUC wires the CNES-park card of the lead panel (optional; nil →
// the route answers empty).
func (h *LeadHandler) SetLeadParkUC(uc *usecases.LeadParkUseCase) { h.leadParkUC = uc }

// HandleLeadPark serves GET /v1/leads/:id/cnes-park.
//
// @Summary     The lead's CNES installed park
// @Description What the public registry says this establishment declares — counts, never equipment age. The bridge is phone-digits first; a name match only counts when it is unique. 204 when nothing matches: an absent card is the honest render.
// @Tags        Leads
// @Produce     json
// @Param       id path string true "Lead ID"
// @Success     200 {object} usecases.LeadParkResult
// @Success     204 {string} string "no match"
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/cnes-park [get]
func (h *LeadHandler) HandleLeadPark(c *gin.Context) {
	if h.leadParkUC == nil {
		c.Status(http.StatusNoContent)
		return
	}
	res, err := h.leadParkUC.Execute(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if res == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, res)
}
