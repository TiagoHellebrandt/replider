package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/channels/telegram"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
)

// ──────────────────────────────── CreateChannelUseCase ────────────────────────────────

type CreateChannelDTO struct {
	AccountID  string
	AgentID    *string // Optional — nil means human-only channel
	Type       string  // "whatsapp" | "telegram" | "instagram" | "sms" ("internal" is rejected — startup-only)
	Name       string
	Config     json.RawMessage // phone_number, bot_token, etc.
	AppBaseURL string          // API origin for the Telegram webhook URL (callers pass cfg.APIPublicURL, NOT the dashboard AppBaseURL)
}

type CreateChannelUseCase struct {
	channelRepo domain.ChannelRepository
	agentRepo   domain.AgentRepository
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	// inboxRegistrar makes a new email channel's mailbox readable. Optional:
	// unwired means replies are never fetched.
	inboxRegistrar EmailInboxRegistrar
	// funnelLinker binds the default funnel to a new email channel. Optional;
	// nil means the send path falls back to ActiveEmailChannel at send time.
	funnelLinker *FunnelChannelLinker
}

// SetEmailInboxRegistrar wires mailbox registration.
func (uc *CreateChannelUseCase) SetEmailInboxRegistrar(r EmailInboxRegistrar) {
	uc.inboxRegistrar = r
}

// SetFunnelLinker wires the default-funnel ↔ email-channel link.
func (uc *CreateChannelUseCase) SetFunnelLinker(l *FunnelChannelLinker) {
	uc.funnelLinker = l
}

func NewCreateChannelUseCase(
	channelRepo domain.ChannelRepository,
	agentRepo domain.AgentRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
) *CreateChannelUseCase {
	return &CreateChannelUseCase{
		channelRepo: channelRepo,
		agentRepo:   agentRepo,
		accountRepo: accountRepo,
		planRepo:    planRepo,
	}
}

func (uc *CreateChannelUseCase) Execute(ctx context.Context, dto CreateChannelDTO) (*domain.Channel, error) {
	if dto.Name == "" || dto.Type == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "name and type are required"}
	}
	if !domain.ValidChannelType(dto.Type) {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "unsupported channel type: " + dto.Type}
	}
	if err := validateChannelConfig(dto.Type, dto.Config); err != nil {
		return nil, err
	}

	// The internal CS channel is platform infrastructure, not a customer
	// feature — it's created idempotently at startup (EnsureInternalChannel),
	// never through the API.
	if dto.Type == domain.ChannelTypeInternal {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "internal channels cannot be created via API"}
	}

	// Enforce the channel cap. Resolved via account.EffectiveMaxChannels
	// so per-account overrides apply and contract/enterprise plans
	// (MaxChannels == 0, "negotiate per deal") resolve to UnlimitedLimit
	// instead of the trial floor — otherwise a contract account is
	// capped at 2 channels. Falls back to 2 (trial default) when plan
	// isn't resolvable so a misconfigured plan_id can't unlock unlimited
	// channels.
	existing, err := uc.channelRepo.ListByAccountID(ctx, dto.AccountID)
	if err != nil {
		return nil, err
	}
	maxChannels := 2
	maxMailboxes := 0
	if uc.accountRepo != nil && uc.planRepo != nil {
		if acc, err := uc.accountRepo.GetByID(ctx, dto.AccountID); err == nil && acc != nil && acc.PlanID != nil {
			if plan, err := uc.planRepo.GetByID(ctx, *acc.PlanID); err == nil && plan != nil {
				maxChannels = acc.EffectiveMaxChannels(plan)
				maxMailboxes = acc.EffectiveMaxEmailMailboxes(plan)
			}
		}
	}
	if len(existing) >= maxChannels {
		return nil, domain.NewPlanLimitError("channels", len(existing), maxChannels)
	}

	// The SDR tiers sell warmed MAILBOXES, not channels (US-M013), and a
	// mailbox is created on first send from an email channel — so the cap
	// is enforced where the channel is created, which is the last moment
	// the customer can be told before the address exists. 0 means the tier
	// predates the cap, not "no email allowed".
	if dto.Type == "email" && maxMailboxes > 0 {
		emailChannels := 0
		for _, ch := range existing {
			if ch.Type == "email" {
				emailChannels++
			}
		}
		if emailChannels >= maxMailboxes {
			return nil, &domain.AppError{
				Code:    "PLAN_LIMIT_EXCEEDED_MAILBOXES",
				Message: "seu plano não permite mais caixas de e-mail — faça upgrade para adicionar outra",
			}
		}
	}

	// If an agent is specified, verify it belongs to the requesting account
	if dto.AgentID != nil && *dto.AgentID != "" {
		agent, err := uc.agentRepo.GetByID(ctx, *dto.AgentID)
		if err != nil {
			return nil, err
		}
		if agent == nil || agent.AccountID != dto.AccountID {
			return nil, domain.ErrAgentNotFound
		}
	}

	webhookToken := ulid.Make().String()

	config := dto.Config
	if dto.Type == "instagram" {
		config = stampInstagramTokenExpiry(nil, config, time.Now())
	}

	ch := &domain.Channel{
		ID:           ulid.Make().String(),
		AccountID:    dto.AccountID,
		AgentID:      dto.AgentID,
		Type:         dto.Type,
		Name:         dto.Name,
		WebhookToken: webhookToken,
		Config:       config,
		IsActive:     true,
	}

	if err := uc.channelRepo.Create(ctx, ch); err != nil {
		return nil, err
	}

	// An email channel created WITH its credentials is readable right away;
	// one created as a stub registers on the save that fills them in.
	registerEmailInbox(ctx, uc.inboxRegistrar, ch)

	// The first email channel becomes the default funnel's door. Best-effort:
	// the channel exists either way and the sender falls back at send time.
	if dto.Type == "email" && uc.funnelLinker != nil {
		if err := uc.funnelLinker.LinkDefaultToChannel(ctx, ch.AccountID, ch.ID); err != nil {
			slog.WarnContext(ctx, "create channel: default funnel not linked to email channel",
				"account_id", ch.AccountID, "channel_id", ch.ID, "error", err)
		}
	}

	// Auto-register webhook for Telegram
	if dto.Type == "telegram" && dto.AppBaseURL != "" {
		var cfg struct {
			BotToken string `json:"bot_token"`
		}
		if err := json.Unmarshal(dto.Config, &cfg); err == nil && cfg.BotToken != "" {
			webhookURL := fmt.Sprintf("%s/v1/webhooks/%s", dto.AppBaseURL, webhookToken)
			if err := telegram.RegisterWebhook(ctx, cfg.BotToken, webhookURL, webhookToken); err != nil {
				slog.Warn("Telegram webhook registration failed", "channel_id", ch.ID, "error", err)
				// Non-fatal: channel is created, user can retry
			}
		}
	}

	return ch, nil
}

// ──────────────────────────────── ListChannelsUseCase ────────────────────────────────

type ListChannelsUseCase struct {
	channelRepo domain.ChannelRepository
}

func NewListChannelsUseCase(channelRepo domain.ChannelRepository) *ListChannelsUseCase {
	return &ListChannelsUseCase{channelRepo: channelRepo}
}

func (uc *ListChannelsUseCase) Execute(ctx context.Context, accountID string) ([]*domain.Channel, error) {
	return uc.channelRepo.ListByAccountID(ctx, accountID)
}

// ──────────────────────────────── UpdateChannelUseCase ────────────────────────────────

type UpdateChannelDTO struct {
	AccountID string
	ChannelID string
	AgentID   *string // Can be set to nil to unassign the agent
	Name      *string
	Config    json.RawMessage
}

// EmailInboxRegistrar registers a channel's mailbox with the poller.
//
// Narrow port rather than the full cursor repository: creating a channel must
// never be able to move a cursor, and the only thing this side needs is "make
// sure a row exists".
type EmailInboxRegistrar interface {
	EnsureForChannel(ctx context.Context, accountID, channelID, mailbox string) (*domain.EmailInboxCursor, error)
}

// registerEmailInbox makes an email channel readable.
//
// Called on create AND on update, because the credentials that make an inbox
// readable often arrive on a later save than the channel itself — and because a
// separate "enable reading" step is a step somebody forgets, whose symptom is
// silence rather than an error.
//
// Best-effort: a channel that saved but did not register still sends, and the
// next edit registers it. Failing the save over poll bookkeeping would be the
// wrong trade.
func registerEmailInbox(ctx context.Context, registrar EmailInboxRegistrar, ch *domain.Channel) {
	if registrar == nil || ch == nil || ch.Type != "email" {
		return
	}
	if domain.ChannelConfigString(ch, "smtp_password") == "" {
		// Nothing to authenticate with yet. A cursor here would be polled every
		// interval, fail every time, and fill the failure counter before the
		// customer has finished filling in the form.
		return
	}
	if _, err := registrar.EnsureForChannel(ctx, ch.AccountID, ch.ID, ""); err != nil {
		slog.WarnContext(ctx, "channel: could not register email inbox for polling",
			"channel_id", ch.ID, "error", err)
	}
}

type UpdateChannelUseCase struct {
	channelRepo domain.ChannelRepository
	agentRepo   domain.AgentRepository
	// mailboxRepo revokes an email channel's send proof when its transport
	// changes. Optional: unwired means the proof outlives the credentials it
	// was earned with, which is a stale permission rather than a broken send.
	mailboxRepo domain.EmailMailboxRepository
	// inboxRegistrar makes the channel's mailbox readable once it has
	// credentials. Optional: unwired means replies are never fetched.
	inboxRegistrar EmailInboxRegistrar
}

// SetEmailInboxRegistrar wires mailbox registration.
func (uc *UpdateChannelUseCase) SetEmailInboxRegistrar(r EmailInboxRegistrar) {
	uc.inboxRegistrar = r
}

func NewUpdateChannelUseCase(channelRepo domain.ChannelRepository, agentRepo domain.AgentRepository) *UpdateChannelUseCase {
	return &UpdateChannelUseCase{channelRepo: channelRepo, agentRepo: agentRepo}
}

// SetEmailMailboxRepo wires the send-proof revocation.
func (uc *UpdateChannelUseCase) SetEmailMailboxRepo(r domain.EmailMailboxRepository) {
	uc.mailboxRepo = r
}

func (uc *UpdateChannelUseCase) Execute(ctx context.Context, dto UpdateChannelDTO) (*domain.Channel, error) {
	ch, err := uc.channelRepo.GetByID(ctx, dto.ChannelID)
	if err != nil {
		return nil, err
	}
	if ch == nil || ch.AccountID != dto.AccountID {
		return nil, domain.ErrChannelNotFound
	}

	// If assigning a new agent, verify it belongs to the same account
	if dto.AgentID != nil && *dto.AgentID != "" {
		agent, err := uc.agentRepo.GetByID(ctx, *dto.AgentID)
		if err != nil {
			return nil, err
		}
		if agent == nil || agent.AccountID != dto.AccountID {
			return nil, domain.ErrAgentNotFound
		}
		ch.AgentID = dto.AgentID
	} else if dto.AgentID != nil {
		// Explicitly unassigning agent (empty string passed)
		ch.AgentID = nil
	}

	if dto.Name != nil {
		ch.Name = *dto.Name
	}
	// Captured before the merge overwrites it: the revocation below compares
	// what the transport WAS against what it becomes.
	previousConfig := channelConfigBytes(ch.Config)
	if dto.Config != nil {
		merged := preserveSensitiveConfig(ch.Config, dto.Config)
		if ch.Type == "instagram" {
			merged = stampInstagramTokenExpiry(channelConfigBytes(ch.Config), merged, time.Now())
		}
		// Validate the MERGED config (stored credentials already folded
		// back in) so a masked round-trip still passes the non-empty
		// checks while a genuinely broken update is rejected.
		if err := validateChannelConfig(ch.Type, merged); err != nil {
			return nil, err
		}
		ch.Config = merged
	}

	if err := uc.channelRepo.Update(ctx, ch); err != nil {
		return nil, err
	}
	uc.revokeSendProofIfTransportChanged(ctx, ch, previousConfig)
	registerEmailInbox(ctx, uc.inboxRegistrar, ch)
	return ch, nil
}

// revokeSendProofIfTransportChanged drops an email channel's SMTP proof when
// the credentials that earned it were replaced.
//
// The proof means "this provider accepted an authenticated send from us". Point
// the channel at a different host, user or address and that sentence is about a
// transport we are no longer using — so the customer runs the test again, which
// costs them one click and catches a typo before a prospect does.
//
// Compares VALUES, not which keys the payload carried. The edit dialog submits
// the whole form on every save, so a presence check would revoke the proof each
// time somebody renamed the channel and quietly send them back to the blocked
// tier for changing nothing.
//
// The password is deliberately not compared: an unchanged one arrives masked
// (preserveSensitiveConfig folds the stored value back in), so it never looks
// equal to what is stored. A genuinely wrong new password fails on the next
// send and is reported there, which is the only place it can be known.
func (uc *UpdateChannelUseCase) revokeSendProofIfTransportChanged(ctx context.Context, ch *domain.Channel, previousConfig []byte) {
	if uc.mailboxRepo == nil || ch.Type != "email" {
		return
	}
	before := configStrings(previousConfig)
	after := configStrings(channelConfigBytes(ch.Config))
	changed := false
	for _, key := range []string{"from_address", "smtp_host", "smtp_port", "smtp_username"} {
		if before[key] != after[key] {
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	if err := uc.mailboxRepo.ClearSMTPProof(ctx, ch.AccountID, ch.ID); err != nil {
		slog.WarnContext(ctx, "channel: could not revoke email send proof",
			"channel_id", ch.ID, "error", err)
	}
}

// configStrings reads a channel config's string fields, tolerating an absent or
// unparseable blob as "no fields" — an unreadable previous config compares equal
// to nothing and errs toward asking for a fresh test.
func configStrings(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = strings.TrimSpace(s)
		}
	}
	return out
}

// validateChannelConfig enforces per-type shape rules on a channel's
// config before it is persisted (US-012). Rules run on create and on
// the MERGED config of an update — never on the raw client payload of
// an update, where masked credentials would false-fail the non-empty
// checks. Types with a stub-first flow (whatsapp via Embedded Signup,
// telegram, webchat) keep their historical leniency; sms and instagram
// take real credentials in the same POST that creates them, so garbage
// there is a hard reject instead of a channel that fails on first send.
func validateChannelConfig(chType string, config json.RawMessage) error {
	invalid := func(msg string) error {
		return &domain.AppError{Code: "VALIDATION_ERROR", Message: msg}
	}
	var m map[string]interface{}
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m); err != nil {
			return invalid("config must be a JSON object")
		}
	}
	field := func(key string) string {
		s, _ := m[key].(string)
		return strings.TrimSpace(s)
	}
	switch chType {
	case "sms":
		if !domain.IsTwilioAccountSID(field("account_sid")) {
			return invalid("account_sid must be a Twilio Account SID (AC followed by 32 hex characters)")
		}
		if field("auth_token") == "" {
			return invalid("auth_token is required for SMS channels")
		}
		if !domain.IsE164(field("from_number")) {
			return invalid("from_number must be in E.164 format (e.g. +15551234567)")
		}
	case "email":
		// US-M003. The from address is the identity every deliverability
		// signal (SPF/DKIM/DMARC, reputation, complaint rate) attaches to,
		// so it is validated hard: a typo here poisons a domain the
		// customer owns, and there is no undo for a burned sender.
		from := field("from_address")
		if !domain.IsEmailAddress(from) {
			return invalid("from_address must be a valid email address (e.g. contato@suaclinica.com.br)")
		}
		// A mailbox connected by consent is a different arrangement, and the
		// rules below do not apply to it: there is no host, username or
		// password to require, because none of them were typed. The free-
		// provider refusal is lifted too — the mail is sent THROUGH the
		// account rather than on its behalf, so Google signs its own outbound
		// and alignment is Google's to satisfy, not the customer's. That
		// refusal is what made the product unusable for every small business
		// whose company email is a Gmail account.
		if strings.EqualFold(field(domain.EmailAuthMethodKey), string(domain.EmailAuthGoogleOAuth)) {
			// The grant is written by the OAuth callback, never by a client:
			// a channel claiming this method with no token would sit in the
			// poller failing every interval.
			if field(domain.EmailOAuthRefreshTokenKey) == "" {
				return invalid("conecte a conta do Google antes de salvar este canal")
			}
			return nil
		}
		// Free-mailbox senders cannot pass DMARC alignment for a domain the
		// customer controls, so cold outbound from them lands in spam and
		// takes the reputation of the whole shared domain with it.
		if domain.IsFreeMailProvider(from) {
			return invalid("from_address must use your own domain — free providers (gmail, hotmail, yahoo…) cannot be authenticated for outbound, ou conecte sua conta Google em vez de digitar SMTP")
		}
		if field("smtp_host") == "" {
			return invalid("smtp_host is required for email channels")
		}
		// The host is dialed by the API with its own network identity, so
		// it must be a public mail server: loopback, RFC1918, link-local
		// and the metadata range are refused after DNS resolution (same
		// resolver rules as customer webhooks). The dialer re-checks at
		// connect time; this is the fail-fast half.
		if err := httpsafe.ValidateHost(field("smtp_host")); err != nil {
			return invalid("smtp_host must be the public hostname of your mail provider")
		}
		if h := field("imap_host"); h != "" {
			if err := httpsafe.ValidateHost(h); err != nil {
				return invalid("imap_host must be the public hostname of your mail provider")
			}
		}
		if field("smtp_username") == "" {
			return invalid("smtp_username is required for email channels")
		}
		if field("smtp_password") == "" {
			return invalid("smtp_password is required for email channels")
		}
		// imap_host/imap_port are optional: the poller falls back to the SMTP
		// host and 993, which is right for every provider that hosts the
		// mailbox it sends through (on Locaweb they are the same name). Only
		// the shape is checked, so a typo does not become a poller that fails
		// every two minutes.
		if p := field("imap_port"); p != "" {
			if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
				return invalid("imap_port must be a port number between 1 and 65535")
			}
		}
	case "instagram":
		if field("page_id") == "" {
			return invalid("page_id is required for Instagram channels")
		}
		if field("access_token") == "" {
			return invalid("access_token is required for Instagram channels")
		}
	case "whatsapp":
		// Embedded Signup creates the channel server-side with a full
		// config; the manual path may send only phone_number_id. A nil
		// config stays allowed (stub created before signup completes).
		if m != nil && field("phone_number_id") == "" && field("phone_number") == "" {
			return invalid("phone_number_id or phone_number is required for WhatsApp channels")
		}
	}
	return nil
}

// preserveSensitiveConfig merges the stored config into an incoming
// update so a masked credential round-tripped by a client never
// overwrites the real value. The API masks *_token/*_secret/*_key
// keys on every read (handlers.maskChannelConfig), so a client that
// re-submits what it fetched would replace e.g. the Twilio auth_token
// with "abcd****". Rule per sensitive key: masked (contains '*'),
// empty, or missing keeps the stored value; any other non-empty
// string rotates the credential.
func preserveSensitiveConfig(stored interface{}, incoming json.RawMessage) json.RawMessage {
	var in map[string]interface{}
	if err := json.Unmarshal(incoming, &in); err != nil || in == nil {
		return incoming
	}
	var st map[string]interface{}
	if raw := channelConfigBytes(stored); raw != nil {
		_ = json.Unmarshal(raw, &st)
	}
	for key, val := range in {
		if !domain.SensitiveConfigKey(key) {
			continue
		}
		s, isStr := val.(string)
		if !isStr || (s != "" && !strings.Contains(s, "*")) {
			continue // real replacement value — rotate
		}
		if old, ok := st[key]; ok {
			in[key] = old
		} else {
			// Masked/empty value with nothing stored: drop the
			// garbage instead of persisting a mask as credential.
			delete(in, key)
		}
	}
	for key, val := range st {
		if domain.SensitiveConfigKey(key) {
			if _, ok := in[key]; !ok {
				in[key] = val
			}
		}
	}
	// auth_method is not a credential, but it decides how every other
	// email key is validated: a mailbox connected by Google consent has no
	// SMTP host and a @gmail.com from_address, both of which the SMTP rules
	// refuse. The edit form never renders the key, so a client that
	// re-submits what it fetched drops it — and the stored grant would be
	// re-validated as a hand-typed SMTP setup and rejected. Carried
	// forward whenever the client omitted it; an explicit value still wins
	// (that is how a channel is downgraded to SMTP on purpose).
	if _, ok := in[domain.EmailAuthMethodKey]; !ok {
		if v, ok := st[domain.EmailAuthMethodKey]; ok {
			in[domain.EmailAuthMethodKey] = v
		}
	}
	out, err := json.Marshal(in)
	if err != nil {
		return incoming
	}
	return out
}

// stampInstagramTokenExpiry writes `token_expires_at` (RFC3339, now +
// domain.InstagramTokenTTL) into an Instagram channel config whenever
// its access_token is set for the first time or rotated to a new value.
// An unchanged token carries the stored stamp forward (clients that
// round-trip the config without the stamp must not erase it). Meta
// long-lived tokens expire ~60 days after issuance and the platform
// gives no webhook warning — the stamp is what lets the API flag a
// channel as "reconnect soon" before DMs silently stop arriving.
// A token-refresh worker is the documented follow-up
// (docs/BACKEND_TODOS.md); until then the flag drives a manual
// re-connect.
func stampInstagramTokenExpiry(stored []byte, incoming json.RawMessage, now time.Time) json.RawMessage {
	var in map[string]interface{}
	if err := json.Unmarshal(incoming, &in); err != nil || in == nil {
		return incoming
	}
	token, _ := in["access_token"].(string)
	if token == "" {
		return incoming // stub channel — nothing to stamp yet
	}

	var st map[string]interface{}
	if len(stored) > 0 {
		_ = json.Unmarshal(stored, &st)
	}
	storedToken, _ := st["access_token"].(string)

	if token == storedToken {
		// Same credential: preserve the original issuance stamp when the
		// incoming payload dropped it.
		if _, ok := in["token_expires_at"]; !ok {
			if prev, ok := st["token_expires_at"]; ok {
				in["token_expires_at"] = prev
			}
		}
	} else {
		in["token_expires_at"] = now.Add(domain.InstagramTokenTTL).UTC().Format(time.RFC3339)
	}

	out, err := json.Marshal(in)
	if err != nil {
		return incoming
	}
	return out
}

// channelConfigBytes normalizes ch.Config — interface{} backed by
// []byte (JSONB), json.RawMessage, or a decoded map — to raw JSON.
func channelConfigBytes(cfg interface{}) []byte {
	switch v := cfg.(type) {
	case nil:
		return nil
	case []byte:
		return v
	case json.RawMessage:
		return []byte(v)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return raw
	}
}

// ──────────────────────────────── DeleteChannelUseCase ────────────────────────────────

type DeleteChannelUseCase struct {
	channelRepo domain.ChannelRepository
}

func NewDeleteChannelUseCase(channelRepo domain.ChannelRepository) *DeleteChannelUseCase {
	return &DeleteChannelUseCase{channelRepo: channelRepo}
}

func (uc *DeleteChannelUseCase) Execute(ctx context.Context, accountID, channelID string) error {
	ch, err := uc.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if ch == nil || ch.AccountID != accountID {
		return domain.ErrChannelNotFound
	}

	// The platform's internal CS channel is not deletable — losing it would
	// orphan every support thread; startup would just recreate it anyway.
	if ch.Type == domain.ChannelTypeInternal {
		return &domain.AppError{Code: "VALIDATION_ERROR", Message: "internal channels cannot be deleted"}
	}

	// Remove Telegram webhook before deleting
	if ch.Type == "telegram" {
		var cfg struct {
			BotToken string `json:"bot_token"`
		}
		if err := json.Unmarshal(channelConfigBytes(ch.Config), &cfg); err == nil && cfg.BotToken != "" {
			if err := telegram.DeleteWebhook(ctx, cfg.BotToken); err != nil {
				slog.Warn("failed to delete Telegram webhook", "channel_id", ch.ID, "error", err)
			}
		}
	}

	return uc.channelRepo.Delete(ctx, channelID)
}

// ──────────────────────────────── ToggleChannelUseCase ────────────────────────────────

type ToggleChannelUseCase struct {
	channelRepo domain.ChannelRepository
}

func NewToggleChannelUseCase(channelRepo domain.ChannelRepository) *ToggleChannelUseCase {
	return &ToggleChannelUseCase{channelRepo: channelRepo}
}

func (uc *ToggleChannelUseCase) Execute(ctx context.Context, accountID, channelID string) (*domain.Channel, error) {
	ch, err := uc.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if ch == nil || ch.AccountID != accountID {
		return nil, domain.ErrChannelNotFound
	}
	return uc.channelRepo.Toggle(ctx, channelID, !ch.IsActive)
}
