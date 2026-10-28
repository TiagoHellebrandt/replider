package handlers

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// ConversationHandler handles HTTP requests for conversations
type ConversationHandler struct {
	convRepo domain.ConversationRepository
	// waitingCounter backs GET /conversations/waiting-count (the shell
	// badge). Narrow port on the concrete repo, wired via SetWaitingCounter.
	waitingCounter interface {
		CountWaiting(ctx context.Context, accountID string) (int64, error)
	}
	msgRepo     domain.MessageRepository
	channelRepo domain.ChannelRepository
	noteRepo    domain.NoteRepository
	// debounceRepo backs "responder agora" and the pending-run read (K1).
	// Optional — see SetInboundDebounceRepo.
	debounceRepo domain.InboundDebounceRepository
	providers    map[string]domain.MessageProvider // keyed by channel type
	hub          *Hub
	gcsAdapter   *storage.GCSAdapter
	// mediaSigner mints media-proxy URLs for the dashboard. Optional: unset
	// falls back to a GCS signed URL, which is the pre-proxy behaviour — it
	// works everywhere except in a browser that has to FETCH the bytes, which
	// is exactly the case the proxy exists for. See MediaFileHandler.
	mediaSigner *storage.URLSigner
	// attachmentRepo is optional (setter-wired): nil means the transcript
	// ships without attachment rows, which is what every pre-email channel
	// expects anyway.
	attachmentRepo domain.MessageAttachmentRepository
	// riskRepo is optional: nil means the transcript ships without the
	// impersonation verdict.
	riskRepo domain.EmailRiskRepository
	// briefUC is optional: nil makes the summary endpoint answer 503 instead
	// of panicking.
	briefUC         *usecases.ConversationBriefUseCase
	personalityHook func(agentID, accountID string) // optional, invoked after each human reply
	csatRepo        domain.CSATRepository           // optional, enables survey tracking on close
	auditLogRepo    domain.AuditLogRepository       // optional, logs the first bot→human takeover per conversation
	// takeoverUC owns Assume / Release / SendHumanMessage. The legacy
	// HandleHumanReply / HandleResumeBot / HandleAssignConversation
	// stay in this handler for backward compat — new takeover routes
	// delegate through the use case so tests don't need Gin.
	takeoverUC *usecases.ConversationTakeoverUseCase
	// events fans state-change events out to tenant outbound webhooks
	// (Zapier/n8n/CRM). Optional — nil disables the integration layer
	// without touching dashboard/WS broadcasts.
	events EventEmitter
	// smartReplier is the LLM-backed reply-suggester. Optional — when
	// nil the /suggest-replies endpoint 503s but the rest of the
	// conversation surface keeps working.
	smartReplier SmartReplier
	// summarizer condenses a long thread to one sentence so the
	// attendant can pick up an in-flight conversation without
	// scrolling. Nil disables the /summary endpoint.
	summarizer ConversationSummarizer
	// agentRepo is consulted by smart-reply to pull the agent's
	// system prompt + RAG persona. Optional; without it suggestions
	// fall back to a generic support-assistant tone.
	agentRepo domain.AgentRepository
}

// SetMediaSigner wires the media-proxy signer so every media URL the dashboard
// receives points at the API instead of at Google.
func (h *ConversationHandler) SetMediaSigner(s *storage.URLSigner) {
	h.mediaSigner = s
}

// mediaURLFor turns a stored GCS object path into a URL the dashboard can
// actually load, and leaves anything else (an external CDN URL, a geo: URI)
// untouched.
//
// One helper rather than the four inline copies this replaced: they had already
// drifted to three different TTLs, and the next reader would have had to check
// all four to know what a media URL means.
func (h *ConversationHandler) mediaURLFor(ctx context.Context, accountID, stored string) (string, bool) {
	if !storage.IsGCSPath(stored) {
		return "", false
	}
	if h.mediaSigner != nil && storage.ObjectBelongsToAccount(accountID, stored) {
		return h.mediaSigner.SignObjectURL(accountID, stored, mediaURLTTL), true
	}
	if h.gcsAdapter == nil {
		return "", false
	}
	signed, err := h.gcsAdapter.GenerateReadSignedURL(ctx, stored, mediaURLTTL)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate media read URL",
			"object", stored, "error", err)
		return "", false
	}
	return signed, true
}

// mediaURLTTL bounds how long a minted media URL stays valid. Long enough that
// a thread left open mid-read keeps working, short enough that a URL pasted into
// a chat stops working the same day.
const mediaURLTTL = 30 * time.Minute

// SetAgentRepo wires the agent repo so smart-reply can pull the
// agent's system prompt when the conversation is bound to one.
func (h *ConversationHandler) SetAgentRepo(r domain.AgentRepository) {
	h.agentRepo = r
}

// SetEventEmitter wires the outbound webhook dispatcher so
// conversation lifecycle changes (assigned, closed) reach tenant
// integrations. Kept as a setter so the existing constructor stays
// stable for in-flight tests.
func (h *ConversationHandler) SetEventEmitter(e EventEmitter) {
	h.events = e
}

// SetTakeoverUseCase wires the takeover use case. Kept as a setter
// instead of a constructor arg so existing call sites in main.go keep
// compiling until the DI chain is updated.
func (h *ConversationHandler) SetTakeoverUseCase(uc *usecases.ConversationTakeoverUseCase) {
	h.takeoverUC = uc
}

// SetCSATRepo wires the CSAT repository. When set, resolving a
// conversation creates a pending survey row so the stats aggregator
// includes "sent but unanswered" in the response rate denominator. The
// actual survey message is shipped separately by the caller (template
// message or webchat overlay).
func (h *ConversationHandler) SetCSATRepo(r domain.CSATRepository) {
	h.csatRepo = r
}

// SetPersonalityHook wires an async trigger that gets called on every
// human-attendant reply. Implementations typically bump a per-agent
// counter and, past a threshold, kick off a learning job so the bot's
// tone keeps tracking how the team actually replies.
func (h *ConversationHandler) SetPersonalityHook(hook func(agentID, accountID string)) {
	h.personalityHook = hook
}

// SetAuditLogRepo wires the audit log repository so the conversation
// handler can record a "takeover" event the first time a human replies
// on a conversation that was running on the bot. Zero-impact when nil;
// used by the team-metrics drill-down to reconstruct who assumed
// conversations without parsing the message stream.
func (h *ConversationHandler) SetAuditLogRepo(r domain.AuditLogRepository) {
	h.auditLogRepo = r
}

// NewConversationHandler creates a new ConversationHandler.
// providers maps channel type ("whatsapp", "telegram", etc.) to its adapter.
func NewConversationHandler(
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	channelRepo domain.ChannelRepository,
	noteRepo domain.NoteRepository,
	providers map[string]domain.MessageProvider,
	hub *Hub,
	gcsAdapter *storage.GCSAdapter,
) *ConversationHandler {
	return &ConversationHandler{
		convRepo:    convRepo,
		msgRepo:     msgRepo,
		channelRepo: channelRepo,
		noteRepo:    noteRepo,
		providers:   providers,
		hub:         hub,
		gcsAdapter:  gcsAdapter,
	}
}

// HandleListConversations godoc
// @Summary      List conversations by status (enriched)
// @Description  Returns a list of conversations with contact_name, channel_type, and last_message, ordered by waiting_since ASC (priority queue).
// @Tags         conversations
// @Produce      json
// @Param        status query string false "Filter by status (open, pending_human, closed)" default(open)
// @Param        assigned query string false "Filter by assigned user id, or 'none' for unassigned conversations (applied in SQL, before pagination)"
// @Param        limit  query int    false "Maximum number of results (1-200)" default(50)
// @Param        offset query int    false "Number of results to skip" default(0)
// @Success      200  {array}   domain.ConversationSummary  "Enriched list of conversations"
// @Failure      500  {object}  domain.AppError              "Internal server error"
// @Security     BearerAuth
// @Router       /conversations [get]
func (h *ConversationHandler) HandleListConversations(c *gin.Context) {
	acctID := getAccountID(c)
	if acctID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	status := c.DefaultQuery("status", "open")
	search := c.Query("q")            // search by contact name/username
	assignedTo := c.Query("assigned") // filter by assigned user
	priority := c.Query("priority")   // filter by priority
	sort := c.Query("sort")           // ORDER BY — "", "first_in_conv", "oldest"
	channelType := c.Query("channel") // filter by channel: whatsapp|email|...

	// Whitelist enum query params before any downstream use.
	// Defense-in-depth: sqlc-generated queries are already parameterized,
	// but accepting arbitrary values would let callers probe internal
	// states and bypass UI filters. Reject unknown values with 400.
	if !isValidConversationStatus(status) {
		respondError(c, &domain.AppError{Code: "INVALID_FILTER", Message: "invalid status filter"})
		return
	}
	if priority != "" && !isValidConversationPriority(priority) {
		respondError(c, &domain.AppError{Code: "INVALID_FILTER", Message: "invalid priority filter"})
		return
	}
	if !isValidConversationSort(sort) {
		respondError(c, &domain.AppError{Code: "INVALID_FILTER", Message: "invalid sort filter"})
		return
	}
	if assignedTo != "" && assignedTo != domain.AssignedFilterUnassigned && !isValidUserIDFilter(assignedTo) {
		respondError(c, &domain.AppError{Code: "INVALID_FILTER", Message: "invalid assigned filter"})
		return
	}
	// Same allow-list the channel table is gated by, so an unknown value is
	// a 400 rather than a silently empty inbox.
	if channelType != "" && !domain.ValidChannelType(channelType) {
		respondError(c, &domain.AppError{Code: "INVALID_FILTER", Message: "invalid channel filter"})
		return
	}

	// Inbox category. Validated against the closed set because the value
	// reaches SQL — "whatever the client sent" never does.
	category := strings.TrimSpace(c.Query("category"))
	if category != "" && !domain.ValidInboxCategory(category) {
		respondError(c, &domain.AppError{Code: "INVALID_FILTER", Message: "invalid category filter"})
		return
	}

	limit, offset := pagination(c)
	if offset > math.MaxInt32 {
		offset = math.MaxInt32
	}

	// The assigned and channel filters run in SQL, BEFORE LIMIT/OFFSET —
	// filtering after pagination hid an attendant's conversations whenever
	// they ranked past the page limit ("Minhas" showed an empty list), and
	// would do the same to every email thread behind a page of WhatsApp.
	summaries, filteredInSQL, err := h.listConversations(c, domain.ConversationListFilter{
		AccountID: acctID, Status: status,
		Limit: int32(limit), Offset: int32(offset), //nolint:gosec // G109: pagination() clamps limit to [1,200]; offset floored at 0 and capped to MaxInt32 above
		Sort: sort, AssignedTo: assignedTo, ChannelType: channelType, Category: category,
		Search: strings.TrimSpace(search), Priority: priority,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list conversations"})
		return
	}

	// Search and priority now run in SQL too (ConversationLister). The Go
	// filter below survives ONLY for a repository without that port — a
	// page filtered here shows the wrong 50 rows, which is the bug the port
	// exists to fix.
	if !filteredInSQL && (search != "" || priority != "") {
		filtered := make([]*domain.ConversationSummary, 0, len(summaries))
		searchLower := strings.ToLower(search)
		for _, s := range summaries {
			if search != "" {
				nameMatch := strings.Contains(strings.ToLower(s.ContactName), searchLower)
				usernameMatch := s.ContactUsername != nil && strings.Contains(strings.ToLower(*s.ContactUsername), searchLower)
				msgMatch := s.LastMessage != nil && strings.Contains(strings.ToLower(*s.LastMessage), searchLower)
				if !nameMatch && !usernameMatch && !msgMatch {
					continue
				}
			}
			if priority != "" && s.Priority != priority {
				continue
			}
			filtered = append(filtered, s)
		}
		summaries = filtered
	}

	c.JSON(http.StatusOK, summaries)
}

type ReplyRequest struct {
	Message     string `json:"message"`
	MessageType string `json:"message_type"` // "text" (default), "image", "audio", "voice", "video", "document", "location"
	MediaURL    string `json:"media_url"`    // GCS object_path from /media/upload, or external URL
}

// validConversationStatuses is the allowed set of `status` query values
// for /v1/conversations. Mirrors the domain.ConversationStatus enum plus
// the empty default that callers may send to mean "no filter".
var validConversationStatuses = map[string]struct{}{
	"":              {},
	"open":          {},
	"pending_human": {},
	"closed":        {},
}

// validConversationPriorities mirrors the CHECK constraint on
// conversations.priority (see migration 000008).
var validConversationPriorities = map[string]struct{}{
	"low":    {},
	"normal": {},
	"high":   {},
	"urgent": {},
}

// listConversations routes to the filtered repository method when one is
// available.
//
// The category filter lives on a narrow optional port rather than on
// domain.ConversationRepository, which dozens of test mocks implement —
// widening that interface for one screen's filter would rewrite all of them.
// An implementation without it simply ignores the category, which degrades to
// the pre-triage inbox rather than to an error.
//
// The second return says whether search/priority were applied in SQL
// (domain.ConversationLister); when false the caller filters the page.
func (h *ConversationHandler) listConversations(
	c *gin.Context, f domain.ConversationListFilter,
) ([]*domain.ConversationSummary, bool, error) {
	if lister, ok := h.convRepo.(domain.ConversationLister); ok {
		rows, err := lister.ListEnrichedQuery(c.Request.Context(), f)
		return rows, true, err
	}
	if filtered, ok := h.convRepo.(interface {
		ListEnrichedFiltered(ctx context.Context, accountID, status string, limit, offset int32, sort, assignedTo, channelType, category string) ([]*domain.ConversationSummary, error)
	}); ok {
		rows, err := filtered.ListEnrichedFiltered(c.Request.Context(), f.AccountID, f.Status,
			f.Limit, f.Offset, f.Sort, f.AssignedTo, f.ChannelType, f.Category)
		return rows, false, err
	}
	rows, err := h.convRepo.ListEnriched(c.Request.Context(), f.AccountID, f.Status,
		f.Limit, f.Offset, f.Sort, f.AssignedTo, f.ChannelType)
	return rows, false, err
}

// validConversationSorts mirrors the sort branches in
// ConversationRepository.ListEnriched.
var validConversationSorts = map[string]struct{}{
	"":              {},
	"first_in_conv": {},
	"oldest":        {},
	// Triage order (§6.10 camada 3): a lead who answered outranks a
	// newsletter that arrived a minute ago.
	"importance": {},
}

func isValidConversationStatus(s string) bool {
	_, ok := validConversationStatuses[s]
	return ok
}

func isValidConversationPriority(s string) bool {
	_, ok := validConversationPriorities[s]
	return ok
}

func isValidConversationSort(s string) bool {
	_, ok := validConversationSorts[s]
	return ok
}

// isValidUserIDFilter sanity-checks the `assigned` query param. IDs are
// ULIDs (26 chars, Crockford base32). We do not parse — we only confirm
// the input is a fixed-length identifier-shaped string so attackers
// cannot smuggle metadata characters through enumerable filters.
func isValidUserIDFilter(s string) bool {
	if len(s) != 26 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isDigit := c >= '0' && c <= '9'
		isUpper := c >= 'A' && c <= 'Z'
		isLower := c >= 'a' && c <= 'z'
		if !isDigit && !isUpper && !isLower {
			return false
		}
	}
	return true
}

// HandleHumanReply godoc
// @Summary      Send a human reply
// @Description  Sends a message from a human agent and automatically pauses the bot (IsBotPaused = true).
//               Clears waiting_since, persists the message, broadcasts it via WebSocket, and dispatches the
//               reply through the appropriate channel adapter (WhatsApp, Telegram, etc.) based on the
//               conversation's channel type. Enforces the 24-hour messaging window.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        id   path      string       true  "Conversation ID"
// @Param        request body ReplyRequest true "Reply payload"
// @Success      200  {object}  map[string]string  "Reply sent"
// @Failure      403  {object}  domain.AppError     "code: CONV_OUTSIDE_24H — outside 24h messaging window"
// @Failure      404  {object}  domain.AppError     "code: CONV_NOT_FOUND — conversation not found"
// @Failure      500  {object}  domain.AppError     "Internal server error"
// @Security     BearerAuth
// @Router       /conversations/{id}/reply [post]

// SetAttachmentRepo wires inbound attachments onto the transcript. Optional,
// so the constructor and its test call sites stay untouched.
func (h *ConversationHandler) SetAttachmentRepo(r domain.MessageAttachmentRepository) {
	h.attachmentRepo = r
}

// SetEmailRiskRepo wires the impersonation verdict onto the transcript.
func (h *ConversationHandler) SetEmailRiskRepo(r domain.EmailRiskRepository) {
	h.riskRepo = r
}

// SetBriefUseCase wires human-readable thread summaries.
func (h *ConversationHandler) SetBriefUseCase(uc *usecases.ConversationBriefUseCase) {
	h.briefUC = uc
}

// SetWaitingCounter wires the badge count (optional; nil → 503).
func (h *ConversationHandler) SetWaitingCounter(c interface {
	CountWaiting(ctx context.Context, accountID string) (int64, error)
}) {
	h.waitingCounter = c
}

// HandleWaitingCount godoc
// @Summary      Count conversations waiting on a human reply
// @Description  Open conversations whose last customer message has no human answer — the navigation badge.
// @Tags         conversations
// @Produce      json
// @Success      200 {object} map[string]int64
// @Security     BearerAuth
// @Router       /conversations/waiting-count [get]
func (h *ConversationHandler) HandleWaitingCount(c *gin.Context) {
	if h.waitingCounter == nil {
		respondError(c, &domain.AppError{Code: "WAITING_COUNT_UNAVAILABLE", Message: "waiting count not configured"})
		return
	}
	count, err := h.waitingCounter.CountWaiting(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// HandleUnreadCount godoc
// @Summary      Count conversations with unread customer messages
// @Description  Open conversations holding an inbound message with no read_at (POST /conversations/:id/read clears it). `count` is conversations, `messages` the total unread messages.
// @Tags         conversations
// @Produce      json
// @Success      200 {object} map[string]int64
// @Failure      503 {object} domain.AppError "code: UNREAD_COUNT_UNAVAILABLE"
// @Security     BearerAuth
// @Router       /conversations/unread-count [get]
func (h *ConversationHandler) HandleUnreadCount(c *gin.Context) {
	counter, ok := h.convRepo.(domain.UnreadCounter)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "UNREAD_COUNT_UNAVAILABLE", Message: "unread count not configured"})
		return
	}
	conversations, messages, err := counter.CountUnread(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": conversations, "messages": messages})
}
