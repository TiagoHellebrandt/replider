package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// Email authentication setup (US-M006, US-M022).
//
// The endpoint exists because the customer cannot fix what they cannot see:
// an unauthenticated sending domain fails silently — SMTP accepts the mail
// and Gmail drops it — so the setup screen has to show, per record, what to
// publish and whether it is live yet.

// SetEmailDNSVerifier wires the verifier. Optional setter so the handler
// constructor and every existing call site stay untouched; unwired, the
// route answers 503 instead of pretending the domain is unverified.
func (h *ChannelHandler) SetEmailDNSVerifier(v *services.EmailDNSVerifier) {
	h.emailDNS = v
}

// SetDNSProvisioners wires the one-click providers, keyed by slug.
func (h *ChannelHandler) SetDNSProvisioners(ps ...domain.DNSProvisioner) {
	if h.dnsProvisioners == nil {
		h.dnsProvisioners = map[string]domain.DNSProvisioner{}
	}
	for _, p := range ps {
		if p != nil {
			h.dnsProvisioners[p.Name()] = p
		}
	}
}

type applyDNSReq struct {
	// Provider is the slug the setup's nameserver_hint reported.
	Provider string `json:"provider" binding:"required"`
	// APIToken is used for the seconds it takes to write the records and
	// then discarded. See HandleApplyEmailDNS.
	APIToken string `json:"api_token" binding:"required"`
}

// HandleApplyEmailDNS serves POST /v1/channels/:id/email-dns/apply.
//
// Publishes the records into the customer's own zone, because doing it by
// hand is where email onboarding dies: four records, a registrar panel that
// disagrees with our instructions about FQDN vs subdomain, nothing verifies,
// and the send caps stay shut.
//
// The token is NEVER STORED. It is read from the body, passed down, and
// dropped when the request ends — no column, no cache, no log. A token that
// can edit a company's DNS is close to the keys to their identity: anyone
// holding it can redirect their mail. Keeping one at rest to save a future
// paste is a trade nobody asked for.
//
// @Summary  Publish email DNS records automatically
// @Tags     channels
// @Param    id path string true "Channel ID"
// @Success  200 {object} domain.DNSProvisionReport
// @Failure  400 {object} domain.AppError
// @Failure  404 {object} domain.AppError
// @Router   /channels/{id}/email-dns/apply [post]
func (h *ChannelHandler) HandleApplyEmailDNS(c *gin.Context) {
	if h.emailDNS == nil {
		respondError(c, &domain.AppError{
			Code:    "EMAIL_DNS_UNAVAILABLE",
			Message: "verificação de DNS de e-mail não configurada",
		})
		return
	}
	var req applyDNSReq
	if !bindJSON(c, &req) {
		return
	}

	ch, sendingDomain, appErr := h.emailChannelDomain(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	provisioner := h.dnsProvisioners[strings.ToLower(strings.TrimSpace(req.Provider))]
	if provisioner == nil {
		respondError(c, domain.ErrDNSProviderUnsupported)
		return
	}

	setup := h.emailDNS.SetupForChannel(sendingDomain, domain.IsOAuthEmail(ch))
	// MatchAny rows carry a SUGGESTION, not a value — the exact record only
	// the customer's provider knows. Publishing the suggestion literally
	// would put a placeholder in their zone.
	kept := setup.Records[:0]
	for _, r := range setup.Records {
		if !r.MatchAny {
			kept = append(kept, r)
		}
	}
	setup.Records = kept

	report, err := provisioner.Apply(c.Request.Context(), req.APIToken, setup)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

// HandleEmailDNS serves GET /v1/channels/:id/email-dns.
//
// `?verify=false` returns the instructions without touching DNS — the screen
// renders instantly, then polls with verification on.
func (h *ChannelHandler) HandleEmailDNS(c *gin.Context) {
	if h.emailDNS == nil {
		respondError(c, &domain.AppError{
			Code:    "EMAIL_DNS_UNAVAILABLE",
			Message: "verificação de DNS de e-mail não configurada",
		})
		return
	}

	ch, sendingDomain, appErr := h.emailChannelDomain(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	// Verification is a live DNS round trip, so it is opt-out: a screen that
	// only wants the instructions should not wait on the network.
	if c.Query("verify") == "false" {
		c.JSON(http.StatusOK, h.emailDNS.SetupForChannel(sendingDomain, domain.IsOAuthEmail(ch)))
		return
	}
	setup := h.emailDNS.VerifyForChannel(c.Request.Context(), sendingDomain, domain.IsOAuthEmail(ch))
	h.recordDNSVerified(c, setup)
	c.JSON(http.StatusOK, setup)
}

// recordDNSVerified writes a passing verification onto the mailbox.
//
// This is the wire that was missing. `dns_verified_at` had no writer anywhere
// in production, so `EmailMailbox.DomainAuthenticated()` was permanently false
// and every cold email approval refused with `dns_unverified` — a block the
// customer could not lift by publishing the records, because the screen that
// checked them told nobody. The verification result has to LAND somewhere for
// the gate downstream to mean anything.
//
// Only the passing case is written, and only forward: a poll that fails while
// DNS propagates must never revoke sending the customer already earned, and a
// transient resolver hiccup is indistinguishable from a deleted record on a
// single lookup. Revocation belongs to a re-check that has seen it fail more
// than once, not to a screen refresh.
func (h *ChannelHandler) recordDNSVerified(c *gin.Context, setup domain.EmailDNSSetup) {
	// setup.Verified is the per-channel verdict (AllRequiredOK, or
	// ProviderManaged for a personal @gmail.com — Google authenticates that
	// domain, so the stamp is earned by connecting the mailbox).
	if h.mailboxRepo == nil || !setup.Verified {
		return
	}
	ch, err := h.channelRepo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil || ch == nil {
		return
	}
	address := emailFromAddressOf(ch)
	if address == "" {
		return
	}
	// Best-effort: the customer asked whether their DNS is right, and the
	// answer above is true either way. A failed stamp keeps them on the low
	// tier, which the next poll fixes.
	if _, err := h.mailboxRepo.EnsureForChannel(c.Request.Context(), ch.AccountID, ch.ID, address); err != nil {
		slog.WarnContext(c.Request.Context(), "email dns: could not read mailbox",
			"channel_id", ch.ID, "error", err)
		return
	}
	if err := h.mailboxRepo.MarkDNSVerified(c.Request.Context(), ch.AccountID, address, time.Now()); err != nil {
		slog.WarnContext(c.Request.Context(), "email dns: could not stamp verification",
			"channel_id", ch.ID, "error", err)
	}
}

// emailChannelDomain resolves the caller's email channel to its sending
// domain, enforcing ownership. Shared by the read and the apply routes so
// the tenant check can never drift between them. The channel rides along
// because the record set is derived from HOW it sends (Gmail vs own SMTP).
func (h *ChannelHandler) emailChannelDomain(c *gin.Context) (*domain.Channel, string, error) {
	ch, err := h.channelRepo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		return nil, "", err
	}
	if ch == nil || ch.AccountID != getAccountID(c) {
		return nil, "", domain.ErrChannelNotFound
	}
	if ch.Type != "email" {
		return nil, "", &domain.AppError{
			Code:    "CHANNEL_NOT_EMAIL",
			Message: "este canal não é de e-mail",
		}
	}
	sendingDomain := emailDomainOf(ch)
	if sendingDomain == "" {
		return nil, "", &domain.AppError{
			Code:    "CHANNEL_MISSING_CONFIG",
			Message: "o canal não tem from_address configurado",
		}
	}
	return ch, sendingDomain, nil
}

// emailDomainOf reads the sending domain out of the channel's from_address.
//
// Channel.Config is an `interface{}` that arrives as raw JSON from the repo
// and as a map from in-process callers, so it is normalised the same way the
// WhatsApp credential reader does.
func emailDomainOf(ch *domain.Channel) string {
	from := emailFromAddressOf(ch)
	at := strings.LastIndex(from, "@")
	if at < 0 || at == len(from)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(from[at+1:]))
}

// emailFromAddressOf reads the channel's sending address, whichever shape the
// config arrived in. The mailbox is keyed by the full ADDRESS, not the domain:
// two mailboxes on one domain have separate reputations and separate caps, and
// collapsing them would let one burn the other's budget.
func emailFromAddressOf(ch *domain.Channel) string {
	if ch == nil || ch.Config == nil {
		return ""
	}
	cfg := map[string]string{}
	switch v := ch.Config.(type) {
	case []byte:
		_ = json.Unmarshal(v, &cfg)
	case json.RawMessage:
		_ = json.Unmarshal([]byte(v), &cfg)
	case map[string]interface{}:
		for k, val := range v {
			if s, ok := val.(string); ok {
				cfg[k] = s
			}
		}
	default:
		raw, _ := json.Marshal(v)
		_ = json.Unmarshal(raw, &cfg)
	}
	return strings.ToLower(strings.TrimSpace(cfg["from_address"]))
}
