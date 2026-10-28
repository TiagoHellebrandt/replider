package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// ── Error response helpers ──────────────────────────────────────────────────
// These eliminate the ~140 occurrences of boilerplate error handling
// spread across all handler files:
//
//   if appErr, ok := err.(*domain.AppError); ok {
//       c.JSON(http.StatusBadRequest, appErr)
//       return
//   }
//   c.JSON(http.StatusInternalServerError, domain.AppError{...})
//
// Instead, handlers call:  respondError(c, err)

// httpStatusForCode maps well-known domain error codes to HTTP status codes.
// Unmapped codes default to 400 Bad Request.
var httpStatusForCode = map[string]int{
	// Product import (pillar 2). Registered because respondError silently
	// defaults an unmapped code to 400 — a file over the cap deserves 413, and
	// "não consegui ler" is a 400 the client should not dress as a server fault.
	"PRODUCT_IMPORT_TOO_LARGE": http.StatusRequestEntityTooLarge,
	// PDF brochure import (Bloco C): oversize is 413 like its CSV sibling;
	// "not wired / no antivirus / provider can't read" are environment gaps
	// (503), not client errors.
	"PRODUCT_PDF_TOO_LARGE":          http.StatusRequestEntityTooLarge,
	"PRODUCT_PDF_IMPORT_UNAVAILABLE": http.StatusServiceUnavailable,
	"PRODUCT_PDF_SCAN_UNAVAILABLE":   http.StatusServiceUnavailable,
	// 429 so a batch import can tell "this file" from "no more files today"
	// and stop, instead of grinding through sixteen doomed uploads.
	"PRODUCT_PDF_QUOTA_EXCEEDED": http.StatusTooManyRequests,
	// 504 is honest (the model did not answer in time) and deliberately NOT
	// treated as service-level by the client: it is a property of THIS
	// document's size, so the next file still deserves its turn.
	"PRODUCT_PDF_TIMEOUT":       http.StatusGatewayTimeout,
	"PRODUCT_IMPORT_NO_HEADER":  http.StatusBadRequest,
	"PRODUCT_IMPORT_EMPTY":      http.StatusBadRequest,
	"PRODUCT_IMPORT_UNREADABLE": http.StatusBadRequest,
	// Proposals (pillar 3). Registered here on purpose: respondError silently
	// defaults an unmapped code to 400, so a missing entry turns "not found" into
	// "bad request" and the client shows a validation error for a missing row.
	"PROPOSAL_NOT_FOUND":            http.StatusNotFound,
	"PROPOSAL_NOT_EDITABLE":         http.StatusConflict,
	"PROPOSAL_APPROVAL_UNAVAILABLE": http.StatusServiceUnavailable,
	"AUTH_UNAUTHORIZED":             http.StatusUnauthorized,
	"MCP_TOKEN_NOT_FOUND":           http.StatusNotFound,
	"MCP_TOKEN_LIMIT_REACHED":       http.StatusConflict,
	"AUTH_EMAIL_NOT_VERIFIED":       http.StatusForbidden,
	"AUTH_LOCKED":                   http.StatusTooManyRequests,
	"AUTH_SESSION_REVOKED":          http.StatusUnauthorized,
	"INVALID_MEDIA_URL":             http.StatusBadRequest,
	"EMAIL_CONNECTION_FAILED":       http.StatusBadGateway,
	"REGISTRATION_PAUSED":           http.StatusServiceUnavailable,
	// Sales-led signup invites. INVITE_INVALID is deliberately ONE code for
	// every refusal (missing/expired/used/revoked/wrong e-mail) — 400, and
	// the message never says which. Owner-side codes are richer: the person
	// holding /owner is not the person guessing tokens.
	"INVITE_INVALID":        http.StatusBadRequest,
	"INVITE_PLAN_NOT_FOUND": http.StatusNotFound,
	"INVITE_NOT_PENDING":    http.StatusConflict,
	"INVITES_UNAVAILABLE":   http.StatusServiceUnavailable,
	"AGENT_NOT_FOUND":       http.StatusNotFound,
	"AGENT_LIMIT_REACHED":   http.StatusConflict, // legacy alias — see PLAN_LIMIT_EXCEEDED
	"PLAN_LIMIT_EXCEEDED":   http.StatusConflict,
	// SDR tier caps (US-M013) — same 409 as every other plan ceiling, so
	// the client's existing upgrade prompt fires without a new branch.
	"PLAN_LIMIT_EXCEEDED_FUNNELS":     http.StatusConflict,
	"PLAN_LIMIT_EXCEEDED_MAILBOXES":   http.StatusConflict,
	"PLAN_LIMIT_EXCEEDED_LEADS":       http.StatusConflict,
	"PLAN_LIMIT_EXCEEDED_ENRICHMENTS": http.StatusConflict,
	"AI_PROVIDER_UNAVAILABLE":         http.StatusBadGateway,
	"CHANNEL_NOT_FOUND":               http.StatusNotFound,
	"ATTACHMENT_NOT_FOUND":            http.StatusNotFound,
	"MESSAGE_NOT_FOUND":               http.StatusNotFound,
	"EMAIL_TEMPLATE_NOT_FOUND":        http.StatusNotFound,
	"APPROACH_HAS_PLACEHOLDER":        http.StatusUnprocessableEntity,
	"CONVERSATION_BRIEF_UNAVAILABLE":  http.StatusServiceUnavailable,
	"EMAIL_ORIGINAL_UNAVAILABLE":      http.StatusServiceUnavailable,
	// 403 rather than 404: the file exists and the caller may see it — what
	// is being refused is the download, and the client renders a different
	// message for each of the two.
	"ATTACHMENT_NOT_DOWNLOADABLE":       http.StatusForbidden,
	"ATTACHMENT_INFECTED":               http.StatusForbidden,
	"CONV_NOT_FOUND":                    http.StatusNotFound,
	"CONV_OUTSIDE_24H":                  http.StatusForbidden,
	"CONV_NOT_ASSUMED":                  http.StatusForbidden,
	"ACCOUNT_NOT_FOUND":                 http.StatusNotFound,
	"PLAN_NOT_FOUND":                    http.StatusNotFound,
	"USAGE_LIMIT_EXCEEDED":              http.StatusTooManyRequests,
	"ACCOUNT_SUSPENDED":                 http.StatusPaymentRequired,
	"BILLING_PLAN_MISSING":              http.StatusInternalServerError,
	"KNOWLEDGE_NOT_FOUND":               http.StatusNotFound,
	"PRODUCT_NOT_FOUND":                 http.StatusNotFound,
	"SALES_FUNNEL_NOT_FOUND":            http.StatusNotFound,
	"SALES_FUNNEL_IS_DEFAULT":           http.StatusConflict,
	"FUNNEL_WEBHOOK_TEST_FAILED":        http.StatusBadGateway,
	"FUNNEL_WEBHOOK_TESTER_UNAVAILABLE": http.StatusServiceUnavailable,
	// US-M006: the verifier is unwired, which is a deployment gap — never
	// report it as "your domain is not verified".
	"EMAIL_DNS_UNAVAILABLE": http.StatusServiceUnavailable,
	// Hybrid sending: UNAVAILABLE is a platform gap (no provider key);
	// NOT_READY is the customer's domain missing registration/verification —
	// a state conflict lifted on the channel screen; PROVIDER_ERROR is the
	// provider refusing a call on our behalf.
	"BULK_SENDING_UNAVAILABLE":  http.StatusServiceUnavailable,
	"BULK_SENDING_NOT_READY":    http.StatusConflict,
	"BULK_PROVIDER_ERROR":       http.StatusBadGateway,
	"BULK_PROVIDER_CONFLICT":    http.StatusConflict,
	"BULK_DOMAIN_NOT_FOUND":     http.StatusBadGateway,
	"LEAD_NOT_FOUND":            http.StatusNotFound,
	"LEAD_ALREADY_CONTACTED":    http.StatusConflict,
	"LEAD_NOT_PENDING_APPROVAL": http.StatusConflict,
	"LEAD_FUNNEL_NO_CHANNEL":    http.StatusConflict,
	"FUNNEL_STAGE_BACKWARD":     http.StatusConflict,
	"LEAD_SEND_FAILED":          http.StatusBadGateway,
	"CONTACT_OPTED_OUT":         http.StatusConflict,
	// State refusals the composer/approval flow hits, registered so the app
	// can tell "fix your setup" (409) from "bad input" (400). Each one is
	// something the human changes elsewhere — a contact with no e-mail, an
	// unconfigured AI key, an empty catalog — not a field to retype.
	"LEAD_CONTACT_NOT_MESSAGEABLE":  http.StatusConflict,
	"LEAD_NO_CONVERSATION":          http.StatusConflict,
	"LEAD_REPLY_UNAVAILABLE":        http.StatusServiceUnavailable,
	"APPROACH_NEEDS_PRODUCTS":       http.StatusConflict,
	"APPROACH_NEEDS_CHANNEL":        http.StatusConflict,
	"APPROACH_NEEDS_ADDRESS":        http.StatusConflict,
	"CONTRACT_CALENDAR_UNAVAILABLE": http.StatusServiceUnavailable,
	"PROPOSAL_HAS_NO_LINES":         http.StatusConflict,
	"CONTRACT_WINDOW_NOT_FOUND":     http.StatusNotFound,
	"SIGNAL_NOT_FOUND":              http.StatusNotFound,
	"BYOK_NOT_CONFIGURED":           http.StatusConflict,
	// Same class as its sibling: the account has to add a key, which is not a
	// malformed request. Unmapped it silently defaulted to 400.
	"BYOK_KEY_MISSING": http.StatusConflict,
	// The mailbox's daily cold-send cap: a rate ceiling, so 429 like the
	// other "wait" refusals — sending resumes tomorrow, nothing to fix.
	"EMAIL_DAILY_CAP_REACHED": http.StatusTooManyRequests,
	// The customer's domain is not verified yet: lifted on the channel
	// screen, not by changing the request.
	"EMAIL_DOMAIN_NOT_VERIFIED": http.StatusConflict,
	// The OAuth callback's state did not match — a bad/stale request.
	// Explicit even though 400 is the default, so nobody "fixes" it later.
	"EMAIL_OAUTH_INVALID_STATE": http.StatusBadRequest,
	// Same shape as an opt-out: the request is well-formed, the state of the
	// world refuses it.
	"EMAIL_ADDRESS_SUPPRESSED": http.StatusConflict,
	// Warmup fail-closed (NULL nunca libera): a state conflict the human
	// lifts by verifying the mailbox, not an input error.
	"EMAIL_WARMUP_NOT_STARTED":   http.StatusConflict,
	"PROSPECTING_NOT_CONFIGURED": http.StatusServiceUnavailable,
	"LEAD_OWNER_NOT_IN_ACCOUNT":  http.StatusBadRequest,
	"LEAD_ASSIGN_UNAVAILABLE":    http.StatusServiceUnavailable,
	"LEAD_INVALID_VISIBILITY":    http.StatusBadRequest,
	"LEAD_BULK_ASSIGN_EMPTY":     http.StatusBadRequest,
	"LEAD_BULK_ASSIGN_TOO_MANY":  http.StatusBadRequest,
	"PROSPECTING_RATE_LIMITED":   http.StatusTooManyRequests,
	"ENRICHMENT_NOT_CONFIGURED":  http.StatusServiceUnavailable,
	"ENRICHMENT_REQUIRES_GEMINI": http.StatusBadRequest,
	"CALENDAR_NOT_FOUND":         http.StatusNotFound,
	"TEAM_MEMBER_NOT_FOUND":      http.StatusNotFound,
	"SKILL_NOT_FOUND":            http.StatusNotFound,
	"WA_PHONE_OWNED_ELSEWHERE":   http.StatusConflict,
	"COUPON_EXHAUSTED":           http.StatusConflict,
	// USD billing (US-017 review fixes): checkout of a plan in the other
	// currency and BRL-only addon packs on a USD account are state
	// conflicts (the account's immutable currency vs the product), not
	// input-validation 400s.
	"PLAN_CURRENCY_MISMATCH":     http.StatusConflict,
	"ADDON_CURRENCY_UNSUPPORTED": http.StatusConflict,
	// Owner-panel contract flows. Without these, every OWNER_* validation
	// error fell through to the default 400 — so creating a contract with
	// an email that already belongs to an account read as a generic "Bad
	// Request" instead of a 409 telling the operator to use the
	// "existing account" mode. The pure input-validation codes
	// (OWNER_NAME_REQUIRED / OWNER_EMAIL_INVALID / OWNER_PLAN_REQUIRED /
	// OWNER_INVALID_STATUS / OWNER_*_INVALID / OWNER_SUSPEND_REASON_REQUIRED)
	// are legitimately 400 and keep the default.
	"OWNER_EMAIL_EXISTS":   http.StatusConflict,
	"OWNER_PLAN_NOT_FOUND": http.StatusNotFound,
	"OWNER_NO_ADMIN":       http.StatusNotFound,
	// Seller-link flow (vendedores v3). OWNER_INVALID_LEAD_SOURCE and
	// OWNER_SELLER_INACTIVE stay on the default 400.
	"OWNER_SELLER_NOT_FOUND":        http.StatusNotFound,
	"OWNER_PLATFORM_NOT_CONFIGURED": http.StatusServiceUnavailable,
	// Seller convert-lead flow (US-004). SELLER_CONV_UNASSIGNED and
	// SELLER_CANNOT_CONVERT_PLATFORM stay on the default 400.
	"SELLER_NOT_CONV_ASSIGNEE":      http.StatusForbidden,
	"SELLER_ACCOUNT_ALREADY_LINKED": http.StatusConflict,
	// US-014 (review fixes): explicit seller flag + convert-lead
	// tie-down.
	"SELLER_NOT_FLAGGED":           http.StatusForbidden,
	"SELLER_CONV_TARGET_UNRELATED": http.StatusConflict,
	// Provider-cost table (US-005). The PROVIDER_COST_INVALID_* codes
	// stay on the default 400.
	"PROVIDER_COST_NOT_FOUND": http.StatusNotFound,
	"PROVIDER_COST_DUPLICATE": http.StatusConflict,
	// Commission ledger (US-007). PROFIT_INVALID_CYCLE (shared with the
	// monthly-profit endpoints) stays on the default 400.
	"COMMISSION_SELLER_NOT_FOUND": http.StatusNotFound,
	// US-005: settling the running cycle needs the explicit
	// allow_open_cycle override — a 409 the app turns into a confirm
	// dialog, not a validation 400.
	"COMMISSION_CYCLE_OPEN": http.StatusConflict,
	// Client-facing commission rules (§6.6 G1). COMMISSION_RULE_INVALID,
	// COMMISSION_RULE_SPLIT_NOT_WHOLE and COMMISSION_SIMULATION_INVALID are
	// validation text and keep the default 400.
	"COMMISSION_RULE_NOT_FOUND":     http.StatusNotFound,
	"COMMISSION_RULE_GLOBAL_EXISTS": http.StatusConflict,
	// The monthly close (G2). COMMISSION_CYCLE_INVALID, COMMISSION_ADJUST_*
	// and COMMISSION_PAY_INVALID are validation text and keep the default 400.
	// A cycle that has not ended is a 409 the app turns into a second confirm
	// dialog — advancing commission is legitimate, but not by accident.
	"COMMISSION_CYCLE_STILL_OPEN": http.StatusConflict,
	"COMMISSION_NOTHING_PENDING":  http.StatusConflict,
	// Contracts (§6.11 G4). CONTRACT_INVALID and CONTRACT_FILE_TOO_LARGE are
	// validation text and keep the default 400.
	"CONTRACT_NOT_FOUND": http.StatusNotFound,
	// The contract exists but has no stored PDF — a state, not bad input.
	"CONTRACT_NO_DOCUMENT": http.StatusNotFound,
	// The reading is unavailable because no AI provider is wired: a deployment
	// gap (503), never "your contract could not be read".
	"CONTRACT_EXTRACT_UNAVAILABLE": http.StatusServiceUnavailable,
	// Clause alerts (G5/G6). CONTRACT_ALERT_INVALID and CONTRACT_RECOVERY_INVALID
	// are validation text and keep the default 400.
	"CONTRACT_ALERT_NOT_FOUND": http.StatusNotFound,
	// Scheduling a clause the contract does not contain is a state conflict, not
	// bad input: the request is well-formed and the paper disagrees.
	"CONTRACT_ALERT_NOT_APPLICABLE": http.StatusConflict,
	// The renewal lead could not be opened — no contact, no phone, no active
	// funnel. Each is a 409: the deadline stands, the pipeline row cannot.
	"RENEWAL_LEAD_NO_CONTACT": http.StatusConflict,
	"RENEWAL_LEAD_NO_PHONE":   http.StatusConflict,
	"RENEWAL_LEAD_NO_FUNNEL":  http.StatusConflict,
	// Electronic signature (§6.11 G7). SIGNATURE_INVALID keeps the default 400.
	"SIGNATURE_NOT_FOUND": http.StatusNotFound,
	// The integration is missing, not the request wrong.
	"SIGNATURE_NOT_CONFIGURED":      http.StatusServiceUnavailable,
	"SIGNATURE_STORAGE_UNAVAILABLE": http.StatusServiceUnavailable,
	"SIGNATURE_CIPHER_UNCONFIGURED": http.StatusServiceUnavailable,
	// The customer's own provider rejected their token: theirs to fix, and a 403
	// so the app can point at Settings instead of showing a generic failure.
	"SIGNATURE_UNAUTHORIZED": http.StatusForbidden,
	// Nothing to send: a state, like a contract with no stored PDF.
	"SIGNATURE_NO_DOCUMENT": http.StatusConflict,
	// US-009 (review fixes): attendant reply persisted but the channel
	// adapter refused/errored the send — an upstream delivery fault, so
	// 502 like LEAD_SEND_FAILED (not a client 400, not a silent 200).
	"MESSAGE_SEND_FAILED": http.StatusBadGateway,
	// US-012: channel credentials are encrypted at rest; a deployment
	// without the encryption key cannot accept credential writes — a
	// server misconfiguration (503), not a client error (same posture
	// as BYOK_CIPHER_UNCONFIGURED in me_handler).
	"CHANNEL_CIPHER_UNCONFIGURED": http.StatusServiceUnavailable,
	// Google Calendar OAuth integration: NOT_CONFIGURED is a server
	// misconfiguration (no OAuth client credentials); NOT_CONNECTED means
	// the account hasn't linked Google yet (the picker 404s until then).
	// GCAL_INVALID_STATE stays on the default 400.
	"GCAL_NOT_CONFIGURED": http.StatusServiceUnavailable,
	// The platform has no Google client — an operator problem, not the
	// customer's, and a 400 would tell them their request was wrong.
	"EMAIL_OAUTH_UNAVAILABLE": http.StatusServiceUnavailable,
	// The grant is gone (revoked, or expired while the app is unverified).
	// 409: the request is fine, the connection behind it is not.
	"EMAIL_OAUTH_NOT_CONNECTED": http.StatusConflict,
	"GCAL_CIPHER_UNCONFIGURED":  http.StatusServiceUnavailable,
	"GCAL_NOT_CONNECTED":        http.StatusNotFound,
	// US-015 (review fixes): the owner currency correction refuses an
	// account with an active Stripe subscription — a state conflict
	// (cancel the subscription first), not a validation 400.
	"OWNER_CURRENCY_HAS_SUBSCRIPTION": http.StatusConflict,
	// Internal CS channel (canal-interno US-002). EMPTY_MESSAGE and
	// MESSAGE_TOO_LONG stay on the default 400.
	"SUPPORT_NOT_AVAILABLE": http.StatusServiceUnavailable,
	// LEGACY_WORKERS off: the cadence runner and the autopilot hook are not
	// running, so the surfaces that promise them say so instead of storing
	// an intent nothing will honour.
	"SEQUENCES_UNAVAILABLE":       http.StatusServiceUnavailable,
	"AUTOPILOT_UNAVAILABLE":       http.StatusServiceUnavailable,
	"APPROACH_REVIEW_UNAVAILABLE": http.StatusServiceUnavailable,
	// Canal interno US-007: the shared account note is gated to platform
	// sellers/admins — a plain attendant gets 403, not a validation 400.
	// SUPPORT_NOTE_TOO_LONG stays on the default 400.
	"SUPPORT_NOTE_FORBIDDEN": http.StatusForbidden,
	// US-F009: per-resource authorization. 403 (not 404) because the caller
	// IS inside the right account — hiding existence would only confuse a
	// legitimate teammate looking at a colleague's private lead.
	"FORBIDDEN":      http.StatusForbidden,
	"INTERNAL_ERROR": http.StatusInternalServerError,
}

// respondError writes a JSON error response. If err is a *domain.AppError,
// its Code is used to determine the HTTP status (see httpStatusForCode).
// Otherwise, a generic 500 Internal Server Error is returned and the real
// error is logged.
//
// Security (US-010): any 5xx is sanitized before it reaches the client.
// Raw error strings — including ones built with err.Error() — can carry
// DB schema hints ("constraint violation on users.email"), file paths,
// or PII. For server-fault statuses we therefore discard the original
// Message and return a fixed generic body, while logging the real error
// server-side keyed by the X-Request-ID so support can still trace it.
// Client-fault statuses (4xx) keep their Message: those are deliberate,
// user-facing validation text and never carry internal detail.
func respondError(c *gin.Context, err error) {
	if appErr, ok := err.(*domain.AppError); ok {
		status, exists := httpStatusForCode[appErr.Code]
		if !exists {
			status = http.StatusBadRequest
		}
		if status >= http.StatusInternalServerError {
			respondSanitized5xx(c, status, appErr.Code, err)
			return
		}
		c.JSON(status, appErr)
		return
	}
	// Unknown error type (typically a raw DB/driver error bubbling up) —
	// always a 500 and always sanitized.
	respondSanitized5xx(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
}

// respondSanitized5xx logs the real error (with the request ID for
// correlation) and writes a generic body that leaks no schema or PII.
// The request ID is echoed in metadata so a user can quote it to support.
func respondSanitized5xx(c *gin.Context, status int, code string, err error) {
	requestID := c.GetString("requestID")
	slog.Error("server error",
		"path", c.Request.URL.Path,
		"request_id", requestID,
		"code", code,
		"error", err,
	)
	body := domain.AppError{
		Code:    code,
		Message: "internal server error",
	}
	if requestID != "" {
		body.Metadata = map[string]any{"request_id": requestID}
	}
	c.JSON(status, body)
}

// ── Request binding helpers ─────────────────────────────────────────────────

// bindJSON binds the JSON request body into req. On validation error it
// writes a 400 response and returns false; on success it returns true.
// Usage:
//
//	var req MyRequest
//	if !bindJSON(c, &req) { return }
func bindJSON(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "INVALID_REQUEST",
			Message: "invalid request body",
		})
		return false
	}
	return true
}

// ── JWT context helpers ─────────────────────────────────────────────────────
// Extracted from the ~26 handlers that repeat c.GetString("accountID").

// respondFeatureUnavailable answers 503 with a deliberate, user-facing
// message for a feature that is switched off by configuration (LEGACY_WORKERS
// and friends). Unlike respondError's 5xx path there is nothing to sanitize:
// the text is ours, not a driver's.
func respondFeatureUnavailable(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, domain.AppError{Code: code, Message: message})
}

func getAccountID(c *gin.Context) string { return c.GetString("accountID") }
func getUserID(c *gin.Context) string    { return c.GetString("userID") }

// getUserRole is the normalized role, for the handlers that shape a RESPONSE by
// it (contract redaction) rather than gating a route on it.
func getUserRole(c *gin.Context) string {
	return string(domain.NormalizeRole(c.GetString("role")))
}

// actorFrom builds the authorization subject from the JWT claims the auth
// middleware stamped (US-F009). Every per-resource decision takes this —
// account alone answers "is it my company's?", never "is it mine?".
func actorFrom(c *gin.Context) domain.Actor {
	return domain.Actor{
		UserID:    getUserID(c),
		AccountID: getAccountID(c),
		Role:      domain.NormalizeRole(c.GetString("role")),
	}
}

// ── Request origin helpers ──────────────────────────────────────────────────

// clientCountry extracts the requester's country from the CDN/edge geo
// headers when the deployment sits behind one (Cloudflare's CF-IPCountry
// is the common case). Empty when no edge stamps it — callers persist
// NULL. Used for the signup currency-arbitrage audit (US-015).
func clientCountry(c *gin.Context) string {
	for _, h := range []string{"CF-IPCountry", "X-Country-Code"} {
		if v := strings.TrimSpace(c.GetHeader(h)); v != "" && !strings.EqualFold(v, "XX") {
			return strings.ToUpper(v)
		}
	}
	return ""
}

// ── Pagination helpers ──────────────────────────────────────────────────────
// Extracted from 5+ endpoints that repeat limit/offset parsing with caps.

// pagination parses limit and offset query params with defaults (50/0)
// and hard caps (max 200, min 1 for limit; min 0 for offset).
func pagination(c *gin.Context) (limit, offset int) {
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 200 {
		limit = 200
	}
	if limit < 1 {
		limit = 1
	}
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	return
}

// NOTE: marshalRaw() lives in calendar_handler.go (historically). It
// converts interface{} → []byte for JSON columns. Used by channel and
// agent handlers too.
