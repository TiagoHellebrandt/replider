package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupLeadRoutes mounts the outbound-sales lead/CRM endpoints (US-015).
// Unlike funnel CRUD (admin-only), these are dashboard reads/edits an
// attendant performs while handling a conversation, so they allow both
// admin and attendant roles.
// aiRL is the per-account cap for the endpoints that spend AI budget
// (US-F012). It was declared in router.go but never wired — a runaway
// integration could burn a customer's provider quota at the default
// 100 req/min. Only the AI-spending routes carry it; CRM reads stay free.
func setupLeadRoutes(v1 *gin.RouterGroup, h *handlers.LeadHandler, caseMetrics *handlers.CaseMetricsHandler, copilot *handlers.CopilotHandler, approachExp *handlers.ApproachExperimentHandler, authPaidMw gin.HandlerFunc, aiRL gin.HandlerFunc) {
	// The copilot spends AI budget on every turn, so it carries the same
	// per-account cap as the other AI-spending routes (US-F012).
	if copilot != nil {
		v1.POST("/copilot/ask", authPaidMw, middleware.RequireRole("admin", "attendant"), aiRL, copilot.HandleAsk)
	}
	dashboardRole := middleware.RequireRole("admin", "attendant")

	// A vendedor satisfies NEITHER admin nor attendant — deliberately, because
	// opening every conversation in the account is not their job. But the lead
	// READS below are actor-scoped: each one resolves through `*ForActor`, which
	// applies ownership and visibility per resource, so a vendedor sees their own
	// leads and the team-visible ones and gets the same "not found" as anyone
	// else for a colleague's private lead.
	//
	// Without this the role was unusable: every lead route answered 403 while the
	// use cases behind them had been written to serve it. The file already fixed
	// the same mismatch for `gestor` on two routes — same reasoning, one role
	// further.
	//
	// The write routes now use the SAME role set, because each one resolves
	// through `MutableForActor` before doing anything (see `ensureLeadMutable`).
	// Until that guard existed they authorised by ACCOUNT only, which made the
	// write surface WIDER than the read surface: an attendant could not list or
	// open a lead and could still change its status, overwrite its draft and
	// approve the send. Now the rule is one rule — gestor and above may change
	// anything in the account, a vendedor their own and unowned leads, nobody
	// else anything.
	leadActorRole := middleware.RequireRole("admin", "attendant", "vendedor")

	// These live in their OWN group rather than as per-route middleware, and the
	// distinction is not cosmetic: gin runs a GROUP's middleware first, so a
	// route-level `RequireRole` can only ever NARROW what the group already
	// admits. Mounted inside `leads`, every one of these would still answer 403
	// to a vendedor. This file already learned that lesson for `gestor` two
	// routes down — same shape, one role further.
	//
	// Registration order matters as much: "/leads/overview" must be declared
	// before "/leads/:id" or gin reads "overview" as a lead id.
	leadReads := v1.Group("/leads", authPaidMw, leadActorRole)
	{
		leadReads.GET("/approval-queue", h.HandleApprovalQueue)
		leadReads.GET("/overview", h.HandlePipelineOverview)
		leadReads.GET("/:id", h.HandleGet)
		// What was actually said to this customer. The whole reason it exists is
		// that a vendedor could send a proposal and never see the reply: the
		// inbox is attendant-gated, and this is not the inbox — it is ONE lead's
		// history, behind that lead's own ownership rules.
		leadReads.GET("/:id/timeline", h.HandleTimeline)
		// The public registry's answer for THIS lead (Clientes redesign).
		leadReads.GET("/:id/cnes-park", h.HandleLeadPark)
	}
	// The writes, on the same role set as the reads — because each one resolves
	// through `MutableForActor` first (`ensureLeadMutable`, or the use case's own
	// call). Their own group for the same gin reason as above: a route-level
	// middleware can only narrow the group's, never widen it.
	leadWrites := v1.Group("/leads", authPaidMw, leadActorRole)
	{
		// Static before /:id so gin does not read "batch-discard" as an id.
		// Queue batch actions (Ofertas → Hoje): ownership is checked per
		// lead inside, so the group's role set is the right gate here.
		leadWrites.POST("/batch-discard", h.HandleBatchDiscard)
		leadWrites.POST("/batch-redraft", aiRL, h.HandleBatchRedraft)
		// Column-scoped, not lead-scoped: clears the manual Kanban order of
		// one stage ("ordenar por parados"). Ownership is the stage's own
		// account check inside the use case; it moves nothing through the
		// pipeline, so the group's role set is the right gate.
		leadWrites.POST("/board-order/reset", h.HandleResetBoardOrder)
		leadWrites.PATCH("/:id/metadata", h.HandleUpdateMetadata)
		leadWrites.PATCH("/:id/status", h.HandleUpdateStatus)
		// US-M008: dragging a card writes the stage AND the canonical status
		// together.
		leadWrites.POST("/:id/move", h.HandleMoveLead)
		leadWrites.POST("/:id/meeting-booked", h.HandleMeetingBooked)
		// Stores what the person wrote, without sending and without queuing.
		leadWrites.PATCH("/:id/approach", h.HandleSaveDraft)
		// Human-in-the-loop outbound (US-020): draft suggests, approve-send sends
		// the human-edited text. No auto mass-send exists.
		leadWrites.POST("/:id/draft", aiRL, h.HandleDraftApproach)
		leadWrites.POST("/:id/review-approach", aiRL, h.HandleReviewApproach)
		leadWrites.POST("/:id/approve-send", h.HandleApproveSend)
		leadWrites.POST("/:id/enrich", aiRL, h.HandleEnrich)
		// Answer the customer from the lead. The timeline let a vendedor READ
		// the reply; `/conversations/*` (where answering lives) is
		// admin/attendant, so she could see "yes" and had nowhere to say
		// "great". Lead-scoped and ownership-gated like every write above.
		leadWrites.POST("/:id/reply", h.HandleReply)
	}

	funnelReads := v1.Group("/funnels", authPaidMw, leadActorRole)
	{
		funnelReads.GET("/:id/leads", h.HandleListByFunnel)
		funnelReads.GET("/:id/board", h.HandleBoard)
	}

	// The case report (US-M011). Admin-only: it is the account's whole
	// commercial performance in one payload.
	if caseMetrics != nil {
		v1.GET("/sales/case-metrics", authPaidMw, middleware.RequireRole("admin"), caseMetrics.HandleCaseMetrics)
		// Same numbers plus the previous period; ?format=html renders the
		// printable one-pager (US-M012).
		v1.GET("/sales/case-report", authPaidMw, middleware.RequireRole("admin"), caseMetrics.HandleCaseReport)
	}

	// Funnel-scoped lead listing lives under /funnels but with the dashboard
	// role set (the /funnels CRUD group is admin-only).
	funnels := v1.Group("/funnels", authPaidMw, dashboardRole)
	{
		// Manual entry (a referral, a card from a conference). The only
		// other ways into a funnel are the Places import and an inbound
		// message, neither of which covers a lead someone just met.
		funnels.POST("/:id/leads", h.HandleCreateLead)
		// The base she already has. Every other door assumes a stranger — the
		// manual form demands a typed phone, the discovery import only accepts
		// what its own Sent-folder scan found, the directory import only Places
		// results — so customers brought in by CSV had no way into a funnel
		// except retyping each one by hand.
		funnels.POST("/:id/leads/from-contacts", h.HandleContactsToFunnel)
		if approachExp != nil {
			funnels.POST("/:id/approach-experiments", approachExp.HandleCreate)
		}
		// Bulk-queue drafts for a funnel's leads (human-in-the-loop, US-020).
		// One call fans out to N AI generations — the most expensive route
		// in the product, so it takes the per-account cap.
		funnels.POST("/:id/leads/draft-batch", aiRL, h.HandleDraftBatch)
	}

	if approachExp != nil {
		exp := v1.Group("/approach-experiments", authPaidMw, middleware.RequireRole("admin"))
		exp.GET("", approachExp.HandleList)
		exp.GET("/:id", approachExp.HandleReport)
		exp.POST("/:id/stop", approachExp.HandleStop)
	}

	leads := v1.Group("/leads", authPaidMw, dashboardRole)
	{
		// Registered BEFORE /:id so gin does not read "approval-queue" as a
		// lead id.
		// Antes de /:id, senão gin lê "bulk-assign" como id de lead.
		leads.POST("/bulk-assign", middleware.RequireRole("admin", "owner", "gestor"), h.HandleBulkAssignOwner)
		// Owner and visibility are manager-and-above only — the same rule the
		// use case applies. Outside the default group because that one admits
		// `attendant` and not `gestor`: mounted there, the middleware would
		// refuse exactly who the use case authorises.
		leads.PATCH("/:id/owner", middleware.RequireRole("admin", "owner", "gestor"), h.HandleAssignOwner)
		leads.PATCH("/:id/visibility", middleware.RequireRole("admin", "owner", "gestor"), h.HandleSetVisibility)
		// Human-in-the-loop outbound approach (US-020): draft suggests,
		// approve-send sends the human-edited text. No auto mass-send exists.
		// Second opinion on the text about to be sent. AI-bound, so it
		// takes the same per-account cap as the other generation routes.
		// Stores what the person wrote, without sending and without queuing.
		// With no route, hand-typed text lived only in the screen's field:
		// closing the panel lost all of it. It skips the AI limiter — it
		// generates nothing.
		// On-demand decision-maker enrichment via Gemini Grounding (US-023):
		// returns 202 and runs the grounded search in the background.
	}
}
