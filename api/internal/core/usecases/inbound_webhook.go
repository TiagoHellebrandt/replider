package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// InboundMessageDTO carries the parsed inbound payload from the channel webhook
type InboundMessageDTO struct {
	ChannelToken   string // from URL path :channel_token
	SenderPhone    string // normalized phone number or chat_id (the customer)
	SenderName     string // display name of the sender (optional, used for contact_name)
	SenderUsername string // username handle e.g. @johndoe (optional, Telegram)
	// SenderAvatarURL is the customer's profile picture URL when the
	// channel exposes one (Telegram today). Stored on the Conversation
	// row only when a new conversation is created — existing rows are
	// left untouched so we don't hammer the provider on every message.
	SenderAvatarURL string
	MessageType     string // 'text', 'image', 'audio', 'voice', 'video', 'document', 'sticker', 'location'
	Content         string // text body or caption (empty for media-only)
	MediaURL        string // resolved CDN / signed URL for the media file; empty for text
	Metadata        []byte // raw JSON for media metadata
	// ExternalMessageID is the provider-side id (WhatsApp wamid). When set,
	// SaveInbound short-circuits on webhook retries that would otherwise
	// create duplicates and double-fire the AI.
	ExternalMessageID string
	// Subject and ListUnsubscribe are email-only and exist for ONE
	// decision: whether an unsolicited inbound is a person worth minting a
	// lead for (domain.IsProspectInbound). Empty/false for every other
	// channel, which is the permissive answer.
	Subject         string
	ListUnsubscribe bool
}

// ErrDuplicateInbound signals that an inbound message with the same
// (channel_id, external_message_id) was already processed. Callers must
// ack the webhook with 200 without running the AI or broadcasting.
var ErrDuplicateInbound = &domain.AppError{Code: "DUPLICATE_INBOUND", Message: "inbound message already processed"}

// InboundPartialResult is returned by SaveInbound before AI processing.
type InboundPartialResult struct {
	AccountID      string
	ConversationID string
	ChannelID      string
	AgentID        *string // nil when channel has no agent (human-only)
	IsBotPaused    bool
	ContactID      string
	Message        *domain.Message
	// IsNewConversation flips true ONLY on the first inbound for a
	// (channel, sender) pair — i.e. when SaveInbound created the
	// conversation row. Lets webhook fan-out emit a
	// `conversation.created` event without re-querying the repo.
	IsNewConversation bool
	// ContactAvatarFetchedAt mirrors conversations.contact_avatar_fetched_at
	// so the webhook handler can decide whether the stored profile photo
	// is stale enough to re-fetch from the provider. Nil = never fetched.
	ContactAvatarFetchedAt *time.Time
}

// InboundMessageResult is returned to the handler for WebSocket broadcast
type InboundMessageResult struct {
	AccountID      string          `json:"account_id"`
	ConversationID string          `json:"conversation_id"`
	Message        *domain.Message `json:"message"`
	AIResponse     *domain.Message `json:"ai_response,omitempty"` // nil if bot paused
}

// SentimentAnalyzer is the local mirror of services.SentimentAnalyzer
// — defined here as a tiny interface so the usecases layer doesn't
// import the services package and create an arch-direction inversion.
// Runs off the request path (goroutine) so a slow LLM call never
// pushes Meta past its 20s webhook budget.
type SentimentAnalyzer interface {
	Analyze(ctx context.Context, accountID, text string) (label string, score float64, err error)
}

// IntentClassifier mirrors services.IntentClassifier — same
// adapter→service interface trick to keep arch direction clean. Used
// to auto-tag conversations by intent (cobrança / compra /
// cancelamento / suporte / dúvida / outro).
type IntentClassifier interface {
	Classify(ctx context.Context, accountID, text string) (label string, score float64, err error)
}

// FailureReporter receives rollups of AI/provider failures. Optional;
// nil = no alerting. Decoupled from the observability package via this
// tiny interface so the usecases layer doesn't import adapters.
type FailureReporter interface {
	Record(component, message string)
}

// InboundWebhookUseCase orchestrates the full inbound message flow:
// Channel lookup → Find/Create Conversation → Save Message → AI trigger → Outbound reply
type InboundWebhookUseCase struct {
	// cases records pipeline facts for the case report (US-M011); nil is a
	// valid state — see CaseRecorder.
	cases       *CaseRecorder
	channelRepo domain.ChannelRepository
	convRepo    domain.ConversationRepository
	msgRepo     domain.MessageRepository
	processMsg  *ProcessMessageUseCase
	agentRepo   domain.AgentRepository
	autoAssign  *AutoAssignUseCase       // optional: round-robin assignment
	contactRepo domain.ContactRepository // optional: used for opt-out handling
	// identityRepo resolves an inbound sender to a contact by ANY
	// identifier (US-F007). Optional — unwired keeps the pre-pivot
	// behaviour (no contact row is created by inbound at all).
	identityRepo domain.ContactIdentityRepository
	// optOutSender sends the short confirmation reply when a contact opts out
	// via keyword (US-025). Optional — nil only skips the acknowledgment.
	optOutSender OptOutConfirmationSender
	accountRepo  domain.AccountRepository // optional: used to mark first inbound for onboarding
	// debounce queues a burst instead of answering each message (K1).
	// Optional: nil answers every inbound immediately, exactly as before.
	debounce           domain.InboundDebounceRepository
	debounceNotifier   DebounceNotifier
	debounceDispatcher DebouncedDispatcher
	csatRepo           domain.CSATRepository  // optional: intercepts numeric CSAT replies before the AI answers
	alerter            FailureReporter        // optional: rolls AI failures into alerts
	sentiment          SentimentAnalyzer      // optional: classifies inbound tone for the dashboard tile badge
	intent             IntentClassifier       // optional: tags conversations by support intent
	tagRepo            domain.TagRepository   // optional: required by intent classifier to store the tag
	usageRepo          domain.UsageRepository // optional: bumps conversations_count on new-conversation creation
	// salesFunnelRepo + leadRepo are optional (US-015): when both are set,
	// an inbound message on a channel bound to an active sales funnel
	// creates/advances the contact's lead to status=responded.
	salesFunnelRepo domain.SalesFunnelRepository
	leadRepo        domain.LeadRepository
	// templateStats credits a reply back to the email template that earned
	// it. Optional — nil just means the numbers are not kept.
	templateStats domain.EmailTemplateRepository
	// sequences stops a lead's cadence when they answer (§1.5 E1). Optional,
	// but unwired means follow-ups keep going out after a reply — see
	// stopSequence.
	sequences SequenceStopper
	// inflight tracks the cancel func of the in-flight AI generation per
	// conversation so a human takeover can cancel a mid-stream reply
	// before it's sent (US-044). Optional; nil = no cancellation.
	inflight *services.InflightAIRegistry
	// serializer forces ProcessAI calls for the SAME conversation to run
	// one at a time so rapid-fire inbounds don't produce out-of-order AI
	// replies (US-046). Optional; nil = no serialization.
	serializer *services.ConversationSerializer
}

// SetInflightRegistry wires the shared in-flight AI registry so a human
// assuming a conversation can cancel a reply that's already streaming
// (US-044). The same registry instance must be wired into the takeover
// use case for the signal to cross between them.
func (uc *InboundWebhookUseCase) SetInflightRegistry(r *services.InflightAIRegistry) {
	uc.inflight = r
}

// SetSerializer wires the per-conversation serializer so concurrent
// ProcessAI calls for the same conversation run sequentially, in arrival
// order (US-046). Without it, the webhook handlers' detached AI goroutines
// can interleave and deliver replies out of order.
func (uc *InboundWebhookUseCase) SetSerializer(s *services.ConversationSerializer) {
	uc.serializer = s
}

// SetAlerter wires a failure alerter that fires when AI errors exceed a
// threshold inside a window. Without it, ProcessAI failures only hit
// slog, which nobody watches.
// SetInboundDebounce wires the burst queue (K1). Unwired, every inbound is
// answered immediately — the pre-debounce behaviour.
func (uc *InboundWebhookUseCase) SetInboundDebounce(r domain.InboundDebounceRepository) {
	uc.debounce = r
}

func (uc *InboundWebhookUseCase) SetAlerter(a FailureReporter) {
	uc.alerter = a
}

func NewInboundWebhookUseCase(
	channelRepo domain.ChannelRepository,
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	processMsg *ProcessMessageUseCase,
	agentRepo domain.AgentRepository,
) *InboundWebhookUseCase {
	return &InboundWebhookUseCase{
		channelRepo: channelRepo,
		convRepo:    convRepo,
		msgRepo:     msgRepo,
		processMsg:  processMsg,
		agentRepo:   agentRepo,
	}
}

// SetContactRepo enables opt-out auto-detection. When set, SaveInbound
// inspects the message content for opt-out keywords and flags the contact
// so future broadcast campaigns skip them.
func (uc *InboundWebhookUseCase) SetContactRepo(r domain.ContactRepository) {
	uc.contactRepo = r
}

// SetCaseRecorder wires case instrumentation (US-M011). Optional: unwired,
// inbound works exactly as before and simply is not counted.
func (uc *InboundWebhookUseCase) SetCaseRecorder(r *CaseRecorder) {
	uc.cases = r
}

// SetIdentityRepo wires the inbound identity resolver (US-F007). With both
// this and SetContactRepo wired, every inbound sender is resolved to (or
// registered as) a contact keyed on the channel's identifier kind — which
// is what lets an email-first prospect and a WhatsApp reply be the same
// person instead of two strangers.
func (uc *InboundWebhookUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identityRepo = r
}

// identityKindForChannel maps a channel type to the kind of identifier its
// senders are addressed by. Unknown/new channel types default to phone,
// which is the pre-pivot assumption and stays correct for telephony.
func identityKindForChannel(channelType string) domain.IdentityKind {
	switch channelType {
	case "email":
		return domain.IdentityKindEmail
	case "telegram":
		return domain.IdentityKindTelegram
	case "instagram":
		return domain.IdentityKindInstagram
	default:
		return domain.IdentityKindPhone
	}
}

// resolveContact finds the contact behind an inbound sender, creating it on
// first contact (US-F007). Resolution is by IDENTITY, not by phone: the same
// person writing from a work email and later from WhatsApp lands on one
// contact, and the funnel/lead history follows them across channels.
//
// Entirely best-effort. Inbound message persistence is the critical path;
// a failure to index identity degrades the CRM view, it must never drop a
// customer's message.
func (uc *InboundWebhookUseCase) resolveContact(ctx context.Context, channel *domain.Channel, dto InboundMessageDTO) *domain.Contact {
	if uc.contactRepo == nil || uc.identityRepo == nil || channel == nil {
		return nil
	}
	sender := strings.TrimSpace(dto.SenderPhone)
	if sender == "" {
		return nil
	}
	accountID := channel.AccountID
	kind := identityKindForChannel(channel.Type)
	// The sender as the CHANNEL wrote it is not yet an identity value: Meta
	// sends a WhatsApp number as bare digits, which fails the E.164 check every
	// identity write runs. Converting here — and using the same value for the
	// lookup and the write, or the lookup misses the row it just wrote.
	identityValue := domain.IdentityValueFromChannelSender(kind, sender)

	contact := &domain.Contact{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ExternalID: sender,
	}
	channelType := channel.Type
	contact.ChannelType = &channelType
	if name := strings.TrimSpace(dto.SenderName); name != "" {
		contact.Name = &name
	}
	// An inbound message IS proof the identifier reaches them.
	opts := contactResolveOpts{VerifiedAt: ptrTime(time.Now())}
	switch kind {
	case domain.IdentityKindPhone:
		// From the qualified form, not the raw sender: NormalizeE164 refuses
		// bare digits, so every WhatsApp contact was created with no phone at
		// all — the column the dashboard and every outbound confirmation read.
		if phone, ok := domain.NormalizeE164(identityValue); ok {
			contact.Phone = &phone
			if key, ok := domain.ContactKey(phone, ""); ok {
				contact.ExternalID = key
			}
		}
	case domain.IdentityKindEmail:
		email := strings.ToLower(sender)
		contact.Email = &email
		contact.ExternalID = email
	default:
		// A chat id or a handle: not a phone, not an e-mail, but still an
		// identity the resolver looks up and registers.
		if identityValue != "" {
			opts.Extra = []identityProbe{{Kind: kind, Value: identityValue}}
		}
	}

	// The same resolver every door uses: the person is found by any address
	// under any spelling, and the identity registered here is what makes the
	// NEXT inbound (on this or any other channel) resolve to them.
	saved, _, err := newContactResolver(uc.contactRepo, uc.identityRepo).resolve(ctx, contact, opts)
	if err != nil {
		slog.WarnContext(ctx, "inbound: failed to upsert contact",
			"channel_id", channel.ID, "error", err)
		return nil
	}
	return saved
}

func ptrTime(t time.Time) *time.Time { return &t }

// conversationByThread matches an inbound e-mail to the thread it answers,
// by the Message-IDs its client echoed back: In-Reply-To first, then the
// References chain newest-first. Returns the thread and the contact behind
// it, or nils.
//
// This is what keeps a reply from a colleague's address in the thread it
// belongs to. We wrote to contato@hospital; the purchasing manager answered
// from joao@hospital. Keyed on the sender alone that answer was a new
// conversation with a new contact and a new inbound lead, while the lead we
// actually worked stayed "contacted" forever. The threading headers were
// stored in the message metadata and never read.
func (uc *InboundWebhookUseCase) conversationByThread(ctx context.Context, channel *domain.Channel, dto InboundMessageDTO) (*domain.Conversation, *domain.Contact) {
	if uc.msgRepo == nil || uc.convRepo == nil || channel == nil || len(dto.Metadata) == 0 {
		return nil, nil
	}
	var meta struct {
		InReplyTo  string `json:"in_reply_to"`
		References string `json:"references"`
	}
	if err := json.Unmarshal(dto.Metadata, &meta); err != nil {
		return nil, nil
	}
	for _, id := range threadMessageIDs(meta.InReplyTo, meta.References) {
		parent, err := uc.msgRepo.GetByExternalID(ctx, channel.ID, id)
		if err != nil || parent == nil || parent.ConversationID == nil {
			continue
		}
		conv, err := uc.convRepo.GetByID(ctx, *parent.ConversationID, channel.AccountID)
		if err != nil || conv == nil {
			continue
		}
		var contact *domain.Contact
		if conv.ContactRefID != nil && uc.contactRepo != nil {
			contact, _ = uc.contactRepo.GetByID(ctx, *conv.ContactRefID, channel.AccountID)
		}
		slog.InfoContext(ctx, "inbound: matched to its thread by message id",
			"conversation_id", conv.ID, "message_id", id,
			"same_sender", strings.EqualFold(strings.TrimSpace(conv.ContactID), strings.TrimSpace(dto.SenderPhone)))
		return conv, contact
	}
	return nil, nil
}

// threadMessageIDs orders the ids to try: In-Reply-To (the direct parent),
// then References from the newest back — the header lists the chain oldest
// first. Deduplicated, since the parent is normally the last reference too.
func threadMessageIDs(inReplyTo, references string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	add(inReplyTo)
	refs := strings.Fields(references)
	for i := len(refs) - 1; i >= 0; i-- {
		add(refs[i])
	}
	return out
}

// OptOutConfirmationSender is the narrow port SaveInbound uses to send the
// short acknowledgment back to a contact that just opted out via keyword
// (US-025). Satisfied by the WhatsApp adapter's SendMessage. Optional — when
// unset, the contact is still flagged, only the confirmation reply is skipped.
type OptOutConfirmationSender interface {
	SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error
}

// SetOptOutSender wires the channel adapter used to send the opt-out
// confirmation reply. The send happens BEFORE the contact is flagged so the
// adapter's own opt-out send-guard (which keys on opted_out_at) still lets the
// acknowledgment through.
func (uc *InboundWebhookUseCase) SetOptOutSender(s OptOutConfirmationSender) {
	uc.optOutSender = s
}

// SetAutoAssign enables round-robin auto-assignment for new conversations.
func (uc *InboundWebhookUseCase) SetAutoAssign(aa *AutoAssignUseCase) {
	uc.autoAssign = aa
}

// SetUsageRepo wires the usage repo so SaveInbound can bump
// conversations_count each time a brand-new conversation is created.
// Without it the counter stays frozen and plan-limit enforcement
// on the /conversations dimension becomes ineffective.
func (uc *InboundWebhookUseCase) SetUsageRepo(r domain.UsageRepository) {
	uc.usageRepo = r
}

// SetCSATRepo lets SaveInbound recognize numeric CSAT replies and
// record them as answers instead of letting the bot respond to a
// bare "5". Without this, CSAT still works via the public recording
// endpoint — the pipeline just doesn't short-circuit.
func (uc *InboundWebhookUseCase) SetCSATRepo(r domain.CSATRepository) {
	uc.csatRepo = r
}

// SetSentimentAnalyzer wires the LLM-backed tone classifier.
// SaveInbound spawns a detached goroutine after the message lands so
// classification + DB write never block the request path. Without
// this set, sentiment stays nil and the dashboard tile renders no
// badge.
func (uc *InboundWebhookUseCase) SetSentimentAnalyzer(s SentimentAnalyzer) {
	uc.sentiment = s
}

// SetIntentClassifier wires the LLM-backed intent classifier + tag
// repo. Both must be non-nil for auto-tagging to fire — without
// the tag repo the classifier has no place to persist the result.
func (uc *InboundWebhookUseCase) SetIntentClassifier(c IntentClassifier, r domain.TagRepository) {
	uc.intent = c
	uc.tagRepo = r
}

// parseCSATReply returns (rating, true) if the trimmed content is a
// bare 1..5 — nothing else. Keeps the detection strict: anything
// with additional text ("5 estrelas obrigado") falls through to the
// AI so legitimate messages aren't accidentally swallowed.
func parseCSATReply(content string) (int, bool) {
	s := strings.TrimSpace(content)
	if len(s) != 1 {
		return 0, false
	}
	if s[0] < '1' || s[0] > '5' {
		return 0, false
	}
	return int(s[0] - '0'), true
}

// SetSalesLead wires the funnel + lead repositories so SaveInbound can mark a
// lead as "responded" when the contact replies on a funnel-bound channel
// (US-015). Both must be non-nil for the hook to fire; without it the inbound
// pipeline behaves exactly as before.
func (uc *InboundWebhookUseCase) SetSalesLead(funnelRepo domain.SalesFunnelRepository, leadRepo domain.LeadRepository) {
	uc.salesFunnelRepo = funnelRepo
	uc.leadRepo = leadRepo
}

// markLeadResponded advances (or creates) the lead for this contact on the
// channel's active funnel to status=responded. Best-effort and silent: any
// lookup/write failure logs and returns so a CRM hiccup never blocks a real
// customer message. Idempotent — once a lead is past the pre-response stages
// (responded/qualified/won/lost) it's left untouched so an inbound reply can't
// drag a qualified lead backwards.
//
// The lead is keyed by the RESOLVED contact (leads.contact_id is a real FK to
// contacts since the identity migration) — the same resolution that stamps
// conversations.contact_ref_id. The raw sender is used only to MATCH legacy
// rows written when contact_id still held the phone; it is never used to
// create one. An unresolved sender refuses instead of guessing: forcing the
// raw string through the FK is exactly the write that used to WARN on every
// email reply while the funnel silently stayed at "contacted".
func (uc *InboundWebhookUseCase) markLeadResponded(ctx context.Context, channel *domain.Channel, contact *domain.Contact, rawSender string, dto InboundMessageDTO) {
	if uc.salesFunnelRepo == nil || uc.leadRepo == nil {
		return
	}
	funnel, err := funnelForChannel(ctx, uc.salesFunnelRepo, channel)
	if err != nil {
		slog.Warn("lead responded: funnel lookup failed", "channel_id", channel.ID, "error", err)
		return
	}
	if funnel == nil {
		return
	}
	var lead *domain.Lead
	if contact != nil {
		lead, err = uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contact.ID)
		if err != nil {
			slog.Warn("lead responded: lead lookup failed", "funnel_id", funnel.ID, "error", err)
			return
		}
	}
	if lead == nil && rawSender != "" {
		// Legacy shape: a lead written before the identity migration carries
		// the sender verbatim in contact_id. Matching it keeps those funnels
		// honest; creating with it would violate the FK.
		lead, err = uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, rawSender)
		if err != nil {
			slog.Warn("lead responded: legacy lead lookup failed", "funnel_id", funnel.ID, "error", err)
			return
		}
	}
	if lead == nil {
		if contact == nil {
			// No contact row means no valid FK target. Refuse rather than
			// guess — the message itself is already persisted and visible.
			slog.Warn("lead responded: sender did not resolve to a contact — not creating a lead",
				"channel_id", channel.ID, "funnel_id", funnel.ID)
			return
		}
		// Inbound-first lead: the contact reached out without a prior
		// outbound approach. Create it directly in the responded state —
		// but ONLY when the sender is plausibly a person.
		//
		// Without this check every message a connected mailbox receives
		// becomes a prospect who "responded": production had Stripe,
		// GoDaddy and a domain renewal sitting in the funnel as four of
		// the five replies the account had ever earned. The message is
		// still persisted and still visible in the inbox — what is
		// refused is the CLAIM that a prospect answered.
		if !domain.IsProspectInbound(rawSender, dto.Subject, dto.Content, dto.ListUnsubscribe) {
			slog.InfoContext(ctx, "lead responded: inbound looks transactional — no lead minted",
				"channel_id", channel.ID, "funnel_id", funnel.ID, "contact_id", contact.ID)
			return
		}
		newLead := &domain.Lead{
			ID:                ulid.Make().String(),
			AccountID:         channel.AccountID,
			FunnelID:          funnel.ID,
			ContactID:         contact.ID,
			Status:            domain.LeadStatusResponded,
			Source:            "inbound",
			CollectedMetadata: json.RawMessage("{}"),
			Enrichment:        json.RawMessage("{}"),
		}
		if err := uc.leadRepo.Upsert(ctx, newLead); err != nil {
			slog.Warn("lead responded: create failed", "funnel_id", funnel.ID, "error", err)
			return
		}
		// An inbound-first lead is BOTH a lead we generated and a reply, and
		// the case report needs both: the source breakdown would otherwise
		// under-count inbound entirely.
		uc.cases.LeadCreated(ctx, channel.AccountID, newLead.ID, "inbound")
		uc.cases.ReplyReceived(ctx, channel.AccountID, newLead.ID, channel.Type)
		return
	}
	// The cadence stops HERE, before the status check, and on every inbound
	// rather than only on the first.
	//
	// A cadence that keeps firing after a human answered is not persistence,
	// it is a machine talking over somebody — and it is the fastest way to
	// lose a deal that was working. The status check below is about
	// COUNTING a reply once; stopping the follow-up is about not writing to
	// somebody who is already talking to you, and those are different
	// questions.
	uc.stopSequence(ctx, channel.AccountID, lead.ID)

	if !isPreResponseLeadStatus(lead.Status) {
		// Already past the first reply: counting every later message would
		// turn one conversation into a stream of "replies".
		return
	}
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, domain.LeadStatusResponded); err != nil {
		slog.Warn("lead responded: status update failed", "lead_id", lead.ID, "error", err)
		return
	}
	// Recorded on the TRANSITION only, so reply count means "prospects who
	// answered", not "messages received".
	uc.cases.ReplyReceived(ctx, channel.AccountID, lead.ID, channel.Type)
}

// funnelForChannel finds the funnel a reply on this channel belongs to.
//
// The channel-bound funnel when there is one; otherwise the account's DEFAULT
// funnel, which is where every import and every find lands when nobody chose
// one. The fallback is the whole point: in production the default funnels
// have no channel_id at all, so keyed on the channel alone this returned nil
// for every account — and no reply ever marked a lead as responded, no
// bounce ever marked one as lost, silently.
//
// Shared by the inbound reply hook and the bounce use case so the two cannot
// disagree about which funnel "the lead" is in.
func funnelForChannel(ctx context.Context, repo domain.SalesFunnelRepository, channel *domain.Channel) (*domain.SalesFunnel, error) {
	if repo == nil || channel == nil {
		return nil, nil
	}
	funnel, err := repo.GetByChannelID(ctx, channel.ID)
	if err != nil || funnel != nil {
		return funnel, err
	}
	return repo.EnsureDefault(ctx, channel.AccountID)
}

// SequenceStopper ends a lead's cadence. A narrow port rather than the whole
// SequenceUseCase: the inbound pipeline only ever needs this one verb.
type SequenceStopper interface {
	StopOnReply(ctx context.Context, accountID, leadID string) error
}

// SetSequenceStopper wires cadence stop-on-reply (§1.5 E1).
func (uc *InboundWebhookUseCase) SetSequenceStopper(s SequenceStopper) {
	uc.sequences = s
}

// stopSequence ends any cadence this lead is walking through.
//
// Best effort by position — the message is already persisted and delivered —
// but logged at ERROR rather than WARN, because the consequence is not a lost
// metric: it is the product writing to somebody who already answered.
func (uc *InboundWebhookUseCase) stopSequence(ctx context.Context, accountID, leadID string) {
	if uc.sequences == nil || leadID == "" {
		return
	}
	if err := uc.sequences.StopOnReply(ctx, accountID, leadID); err != nil {
		slog.ErrorContext(ctx, "could not stop the cadence after a reply — follow-ups may keep going out",
			"account_id", accountID, "lead_id", leadID, "error", err)
	}
}

// sendOptOutConfirmation delivers the short opt-out acknowledgment back to the
// contact (US-025). Best-effort and silent: a missing sender or a send error
// only logs — the opt-out flag itself is what protects the number, the reply
// is a courtesy. Routed only for channels the wired sender can actually serve
// (the WhatsApp adapter); other channel types skip it.
func (uc *InboundWebhookUseCase) sendOptOutConfirmation(ctx context.Context, channel *domain.Channel, contactID string) {
	if uc.optOutSender == nil || channel == nil || channel.Type != "whatsapp" || contactID == "" {
		return
	}
	body := optOutConfirmationText
	msg := &domain.Message{Type: "text", Content: &body}
	if err := uc.optOutSender.SendMessage(ctx, channel, contactID, msg); err != nil {
		slog.Warn("opt-out confirmation send failed",
			"channel_id", channel.ID, "error", err)
	}
}

// markLeadLost flips the contact's lead (on the channel's active funnel) to
// status=lost when they opt out, so it disappears from batch-approach lists
// (US-025). Best-effort and silent — a CRM hiccup must not block the inbound
// pipeline. A lead already won/lost is left untouched.
// Same design as markLeadResponded (Item 3 / commit 065b373f): the lead is
// keyed by the RESOLVED contact — leads.contact_id is a FK to contacts since
// the identity migration — and the raw sender is consulted only to MATCH
// legacy rows. On email channels the raw address never matched a ULID-keyed
// lead, so the lead of someone who asked to STOP stayed active in the funnel.
// There is no creation here, so an unresolved contact simply logs and leaves.
func (uc *InboundWebhookUseCase) markLeadLost(ctx context.Context, channel *domain.Channel, contact *domain.Contact, rawSender string) {
	if uc.salesFunnelRepo == nil || uc.leadRepo == nil || channel == nil {
		return
	}
	// Same resolution as markLeadResponded: the channel's funnel, else the
	// account's default — an opt-out on a default funnel used to be lost.
	funnel, err := funnelForChannel(ctx, uc.salesFunnelRepo, channel)
	if err != nil || funnel == nil {
		return
	}
	var lead *domain.Lead
	if contact != nil {
		lead, err = uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contact.ID)
		if err != nil {
			slog.Warn("opt-out: lead lookup failed", "funnel_id", funnel.ID, "error", err)
			return
		}
	}
	if lead == nil && rawSender != "" {
		lead, err = uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, rawSender)
		if err != nil {
			slog.Warn("opt-out: legacy lead lookup failed", "funnel_id", funnel.ID, "error", err)
			return
		}
	}
	if lead == nil {
		if contact == nil {
			slog.Warn("opt-out: sender did not resolve to a contact — no lead to mark lost",
				"channel_id", channel.ID, "funnel_id", funnel.ID)
		}
		return
	}
	if lead.Status == domain.LeadStatusLost || lead.Status == domain.LeadStatusWon {
		return
	}
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, domain.LeadStatusLost); err != nil {
		slog.Warn("opt-out: mark lead lost failed", "lead_id", lead.ID, "error", err)
	}
}

// isPreResponseLeadStatus reports whether a lead is still in a stage before it
// has replied, so an inbound message should advance it to responded.
func isPreResponseLeadStatus(s domain.LeadStatus) bool {
	switch s {
	case domain.LeadStatusNew, domain.LeadStatusProspected, domain.LeadStatusEnriching,
		domain.LeadStatusEnriched, domain.LeadStatusPendingApproval, domain.LeadStatusContacted:
		return true
	default:
		return false
	}
}

// SetAccountRepo enables the "first inbound message ever" onboarding
// milestone. When set, SaveInbound writes accounts.onboarding_first_inbound_at
// once per account (the SQL itself enforces idempotency).
// SetEmailTemplateStats wires the counters that credit a reply back to the
// template that earned it.
func (uc *InboundWebhookUseCase) SetEmailTemplateStats(r domain.EmailTemplateRepository) {
	uc.templateStats = r
}

func (uc *InboundWebhookUseCase) SetAccountRepo(r domain.AccountRepository) {
	uc.accountRepo = r
}

// SaveInbound creates/finds the conversation and persists the inbound message.
// It does NOT run AI processing. The channel must already be resolved by the caller.
// Returns a partial result suitable for broadcasting the inbound event immediately.
func (uc *InboundWebhookUseCase) SaveInbound(ctx context.Context, channel *domain.Channel, dto InboundMessageDTO) (*InboundPartialResult, error) {
	// Account is always available directly from the channel (no agent needed).
	accountID := channel.AccountID

	// Webhook dedupe: Meta retries the same wamid when our handler takes too
	// long or returns non-2xx. The partial unique index on messages
	// (channel_id, external_message_id) would already reject the insert, but
	// checking here lets us skip the rest of the pipeline (conversation
	// create, opt-out check, AI fanout) on a retry instead of hitting the DB
	// only to fail at the last step.
	if dto.ExternalMessageID != "" {
		existing, err := uc.msgRepo.GetByExternalID(ctx, channel.ID, dto.ExternalMessageID)
		if err != nil {
			return nil, fmt.Errorf("dedupe lookup failed: %w", err)
		}
		if existing != nil {
			return nil, ErrDuplicateInbound
		}
	}

	// Find or Create Conversation for this sender on this channel.
	// `convCreatedNow` lets the caller emit conversation.created on
	// the outbound webhook stream — only the first inbound for a
	// (channel, sender) pair counts as a fresh conversation.
	//
	// A reply that names the message it answers (e-mail threading headers)
	// belongs to THAT thread and that person, even from another address —
	// see conversationByThread. Only then is the sender the key.
	conv, resolvedContact := uc.conversationByThread(ctx, channel, dto)
	if conv == nil {
		var err error
		conv, err = uc.convRepo.FindByContact(ctx, channel.ID, dto.SenderPhone)
		if err != nil {
			return nil, fmt.Errorf("error looking up conversation: %w", err)
		}
		if conv == nil {
			// No open thread: a resolved one for the same sender comes back
			// instead of a fresh, history-less conversation.
			conv, err = uc.reopenIfClosed(ctx, channel.ID, dto.SenderPhone)
			if err != nil {
				return nil, err
			}
		}
	}
	convCreatedNow := conv == nil

	// US-F007: resolve (or register) the person behind this sender. Runs
	// before the AI/lead hooks so a lead bound later in this same call sees
	// an existing contact. Best-effort by design — see resolveContact.
	if resolvedContact == nil {
		resolvedContact = uc.resolveContact(ctx, channel, dto)
	}

	if conv == nil {
		now := time.Now()
		var contactName *string
		if dto.SenderName != "" {
			contactName = &dto.SenderName
		}
		var contactUsername *string
		if dto.SenderUsername != "" {
			contactUsername = &dto.SenderUsername
		}
		var contactAvatarURL *string
		var contactAvatarFetchedAt *time.Time
		if dto.SenderAvatarURL != "" {
			contactAvatarURL = &dto.SenderAvatarURL
			// Mirror the SQL default: when we save an avatar at creation
			// time, the conversations.contact_avatar_fetched_at column
			// will be NOW(), so the partial must reflect that too or
			// the handler will immediately try to re-fetch.
			contactAvatarFetchedAt = &now
		}
		// The FK to the person, when we managed to resolve one. Left NULL
		// otherwise rather than guessed: contact_ref_id is a real FK and
		// everything that reads it (lead panel, timeline, opt-out guard) treats
		// a value as proof this thread IS that contact.
		var contactRefID *string
		if resolvedContact != nil {
			contactRefID = &resolvedContact.ID
		}
		conv = &domain.Conversation{
			ID:                     ulid.Make().String(),
			AccountID:              accountID,
			ChannelID:              channel.ID,
			AgentID:                channel.AgentID, // may be nil (human-only channel)
			ContactID:              dto.SenderPhone,
			ContactRefID:           contactRefID,
			ContactName:            contactName,
			ContactUsername:        contactUsername,
			ContactAvatarURL:       contactAvatarURL,
			ContactAvatarFetchedAt: contactAvatarFetchedAt,
			Status:                 domain.ConversationOpen,
			IsBotPaused:            false,
			LastCustomerMessageAt:  &now,
			WaitingSince:           &now,
		}
		if err := uc.convRepo.Create(ctx, conv); err != nil {
			// Race recovery: a parallel inbound webhook for the same
			// (channel, contact) pair won the INSERT and the partial
			// unique index uniq_conversations_open_per_contact rejected
			// ours. The repository translates the PG 23505 into the
			// domain sentinel; we re-read the winning row and continue
			// as if FindByContact had returned it. Without this branch
			// each parallel retry would 5xx and Meta would re-fire,
			// piling up duplicate counter bumps and empty conversations.
			if err == domain.ErrConversationAlreadyOpen {
				existing, fetchErr := uc.convRepo.FindByContact(ctx, channel.ID, dto.SenderPhone)
				if fetchErr != nil || existing == nil {
					return nil, fmt.Errorf("conversation create lost a race but re-read failed: %w", fetchErr)
				}
				conv = existing
				convCreatedNow = false
			} else {
				return nil, fmt.Errorf("failed to create conversation: %w", err)
			}
		}

		// Bump the monthly conversations counter for plan enforcement.
		// Tokens pass as zero — AddUsage only advances the counters that
		// are non-zero. Best-effort: a failure here must not block a
		// real customer message, the count simply drifts by one.
		// Skipped on the race-recovery path because the winning thread
		// is the one that bumps the counter.
		if convCreatedNow && uc.usageRepo != nil {
			if err := uc.usageRepo.AddUsage(ctx, accountID, 0, 0, 0, 1); err != nil {
				slog.Warn("failed to bump conversations_count on new conversation",
					"account_id", accountID, "conversation_id", conv.ID, "error", err)
			}
		}

		// Auto-assign new conversation to an attendant (round-robin).
		// Skipped on the race-recovery path because the winner already
		// assigned it.
		if convCreatedNow && uc.autoAssign != nil {
			if assignee, err := uc.autoAssign.Assign(ctx, conv.ID, accountID); err == nil && assignee != nil {
				conv.AssignedTo = assignee
			}
		}

		// Detect language from first inbound text. Best-effort: empty
		// content, media-only, or too-short strings fall back to "pt",
		// which matches the product's default market. A failure here
		// must not block the message pipeline — the detector loads 50MB
		// of lingua models on first call and we've seen it panic on
		// memory-pinched containers, so wrap the whole thing in recover.
		// Skipped on race recovery — winner already detected the language.
		if convCreatedNow && dto.Content != "" {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Warn("language detection panicked",
							"conversation_id", conv.ID, "recover", r)
					}
				}()
				lang := services.DetectReplyLanguage(dto.Content)
				if err := uc.convRepo.SetLanguage(ctx, conv.ID, lang); err != nil {
					slog.Warn("failed to persist detected language",
						"conversation_id", conv.ID, "lang", lang, "error", err)
					return
				}
				conv.Language = &lang
			}()
		}
	} else {
		// Update SLA timestamp on existing conversation. Best-effort,
		// but log on failure — a silent miss drifts the dashboard's
		// "waiting longest" ordering with no trace for ops.
		if err := uc.convRepo.UpdateLastCustomerMessageAt(ctx, conv.ID, accountID); err != nil {
			slog.Warn("failed to update conversation SLA timestamp",
				"conversation_id", conv.ID, "account_id", accountID, "error", err)
		}
	}

	// Save the inbound message
	msgID := ulid.Make().String()
	convID := conv.ID
	var senderName *string
	if dto.SenderName != "" {
		senderName = &dto.SenderName
	}
	var mediaURL *string
	if dto.MediaURL != "" {
		mediaURL = &dto.MediaURL
	}
	var externalID *string
	if dto.ExternalMessageID != "" {
		id := dto.ExternalMessageID
		externalID = &id
	}
	inboundMsg := &domain.Message{
		ID:                msgID,
		ChannelID:         channel.ID,
		ConversationID:    &convID,
		From:              dto.SenderPhone,
		SenderName:        senderName,
		Type:              dto.MessageType,
		Content:           &dto.Content,
		MediaURL:          mediaURL,
		Metadata:          dto.Metadata,
		Direction:         "inbound",
		ExternalMessageID: externalID,
		Timestamp:         time.Now(),
	}
	inserted, err := uc.msgRepo.Create(ctx, inboundMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to save inbound message: %w", err)
	}
	if !inserted {
		// Race winner already persisted this wamid; drop the duplicate
		// before any AI/billing/websocket fanout fires twice.
		return nil, ErrDuplicateInbound
	}

	// Sentiment classification on inbound text. Detached goroutine so
	// a slow LLM doesn't push Meta past its 20s webhook budget. We
	// intentionally skip media-only messages (no signal) and short
	// strings (<8 chars — "ok", "obg" etc. give the model nothing).
	// All errors logged at WARN; the dashboard just renders no badge.
	//
	// E-mail is EXCLUDED: that path never auto-answers, the classifiers'
	// vocabulary is customer-service (cobrança/suporte), and every inbound
	// mail was costing a model call for a badge nobody reads there.
	classifyInbound := channel.Type != "email"
	if classifyInbound && uc.sentiment != nil && len(strings.TrimSpace(dto.Content)) >= 8 {
		go uc.classifySentimentAsync(accountID, conv.ID, dto.Content)
	}

	// Intent classification on the FIRST inbound text only. The
	// rationale: a thread's intent is set by the customer's opening
	// message ("quero cancelar" / "ainda não recebi") — re-running
	// per message would just thrash tags. We gate on convCreatedNow
	// to enforce that.
	if classifyInbound && uc.intent != nil && uc.tagRepo != nil && convCreatedNow &&
		len(strings.TrimSpace(dto.Content)) >= 8 {
		go uc.classifyIntentAsync(accountID, conv.ID, dto.Content)
	}

	// Mission 6 analytics: stamp first_inbound_at on the conversation
	// if not already set. Idempotent at the SQL level; best-effort so
	// a failure here never blocks the real customer message.
	if err := uc.convRepo.SetFirstInboundAt(ctx, conv.ID, accountID); err != nil {
		slog.Warn("set first inbound failed",
			"conversation_id", conv.ID, "error", err)
	}

	// Mission 6 CSAT: if a survey is pending on this conversation and
	// the customer's message is a bare 1..5, record it as the answer
	// and short-circuit the pipeline. The bot's canned response would
	// otherwise reply to "5" as if it were a new question.
	if uc.csatRepo != nil {
		if rating, ok := parseCSATReply(dto.Content); ok {
			pending, _ := uc.csatRepo.GetPending(ctx, conv.ID)
			if pending != nil {
				if err := uc.csatRepo.RecordAnswer(ctx, conv.ID, rating, nil); err == nil {
					slog.Info("csat answer recorded",
						"conversation_id", conv.ID, "rating", rating)
					// Flag as "paused" in the partial so ProcessAI skips
					// AI for this message. The customer already ended
					// the conversation with the score; silence is the
					// correct response.
					conv.IsBotPaused = true
				}
			}
		}
	}

	// LGPD opt-out: if the customer sent a stop keyword, flag the contact
	// so broadcast campaigns never enqueue a send for them again. Best-
	// effort — a failure here must not block the inbound pipeline.
	//
	// On a successful flag we also pause the bot for THIS conversation
	// so the AI doesn't reply to the STOP message itself — replying
	// would defeat the purpose of opting out and is a CAN-SPAM /
	// LGPD foot-gun. The attendant can manually re-engage if needed
	// (e.g. a wrong-number scenario).
	if uc.contactRepo != nil && IsOptOutMessage(dto.Content) {
		// Send the confirmation reply BEFORE flagging the contact: the
		// adapter's outbound opt-out guard keys on opted_out_at, so doing it
		// first means the acknowledgment isn't blocked by the very flag we're
		// about to set. Best-effort — a send failure never blocks the flag.
		uc.sendOptOutConfirmation(ctx, channel, dto.SenderPhone)

		// The flag goes on the PERSON we resolved, addressed by their own
		// key (external_id is unique per account). Keyed on the raw sender
		// it updated zero rows for a contact keyed by the other address —
		// "sair" by e-mail from someone the base knows by phone.
		optOutKey := dto.SenderPhone
		if resolvedContact != nil && resolvedContact.ExternalID != "" {
			optOutKey = resolvedContact.ExternalID
		}
		if _, err := uc.contactRepo.SetOptedOut(ctx, accountID, optOutKey); err != nil {
			slog.Warn("opt-out flag failed", "account_id", accountID, "sender", dto.SenderPhone, "error", err)
		} else {
			slog.Info("contact opted out via inbound keyword",
				"account_id", accountID, "sender", dto.SenderPhone)
			conv.IsBotPaused = true
			if err := uc.convRepo.SetBotPaused(ctx, conv.ID, accountID, true); err != nil {
				slog.Warn("opt-out: pause bot failed",
					"conversation_id", conv.ID, "error", err)
			}
			// The contact's lead (if any) is dead for outbound purposes:
			// move it to status=lost so it drops out of batch-approach lists.
			uc.markLeadLost(ctx, channel, resolvedContact, dto.SenderPhone)
		}
	}

	// Onboarding milestone: stamp accounts.onboarding_first_inbound_at on
	// the first inbound for this account. Idempotent at the SQL level.
	// Best-effort — never block a real customer message because of an
	// onboarding side-effect. The emitted info log is the analytics
	// signal for the "onboarding.first_inbound_received" event (see
	// Fase 6 spec): fires on every call but the downstream pipeline
	// can dedupe with the account_id.
	if uc.accountRepo != nil {
		if err := uc.accountRepo.MarkFirstInboundReceived(ctx, accountID, time.Now()); err != nil {
			slog.Warn("mark first inbound failed", "account_id", accountID, "error", err)
		} else {
			slog.Info("onboarding.first_inbound_received",
				"account_id", accountID,
				"channel_id", channel.ID)
		}
	}

	// Outbound-sales CRM (US-015): a reply on a funnel-bound channel advances
	// (or creates) the contact's lead to status=responded. Best-effort, after
	// the message is persisted, so a CRM hiccup never blocks the reply. An
	// opt-out message is a termination, not a genuine response — the opt-out
	// block above already moved the lead to lost, so skip the advance here.
	if !IsOptOutMessage(dto.Content) {
		uc.markLeadResponded(ctx, channel, resolvedContact, dto.SenderPhone, dto)
		// A reply is the signal that actually matters — above a click, far
		// above an open. Counted against the template VERSION that earned
		// it, which is why the version travels on the outbound message.
		uc.creditTemplateReply(ctx, channel.ID, dto.Metadata)
	}

	return &InboundPartialResult{
		AccountID:              accountID,
		ConversationID:         conv.ID,
		ChannelID:              channel.ID,
		AgentID:                channel.AgentID,
		IsBotPaused:            conv.IsBotPaused,
		ContactID:              dto.SenderPhone,
		Message:                inboundMsg,
		IsNewConversation:      convCreatedNow,
		ContactAvatarFetchedAt: conv.ContactAvatarFetchedAt,
	}, nil
}

// ProcessAIResult holds the AI response message and optional usage alert info.
type ProcessAIResult struct {
	Message      *domain.Message
	UsageAlert   *UsageAlertInfo
	LimitReached bool // when true, the message is a system notice — do NOT send to external channel
	// HandoffRequested signals that the bot emitted the HandoffSentinel
	// because the agent is configured with SilentOnUnknown=true and
	// couldn't answer. The caller MUST skip outbound send + bot
	// message save and instead flip the conversation into
	// human-attendance mode.
	HandoffRequested bool
}

// ProcessAI runs the AI agent for the given conversation and saves the AI response message.
// Returns nil if the bot is paused or AI processing fails (failure is logged, not returned as error).
func (uc *InboundWebhookUseCase) ProcessAI(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult {
	if partial.IsBotPaused {
		return nil
	}

	// No agent assigned → no AI processing (human-only channel)
	if partial.AgentID == nil || *partial.AgentID == "" {
		return nil
	}

	// K1: wait for the customer to finish typing.
	//
	// Checked BEFORE the serializer lock on purpose. The lock makes a burst
	// reply in ORDER; the debounce makes it reply ONCE. Holding the lock while
	// waiting would serialize the whole burst behind a timer and hold a
	// connection for a minute per conversation.
	if deferred := uc.deferInbound(ctx, partial, content); deferred {
		return nil
	}

	return uc.processAIAfterDefer(ctx, partial, content)
}

// processAIAfterDefer runs the AI pipeline once the debounce decision is
// already made — either the webhook entry path chose not to wait, or the
// debounce worker's timer fired and it is replaying the burst (RunPending).
//
// It MUST NOT call deferInbound. RunPending deletes the pending row before
// replaying; a deferInbound call here would see no row, re-Enqueue a fresh one
// (respond_at = now + window) with a reset first_queued_at, and the worker
// would claim/delete/re-enqueue it every window forever — a non-buy-intent
// burst would loop and never be answered, while the MaxTotalWait ceiling never
// accumulates. So the defer decision lives only at the top of ProcessAI, and
// this method is the single pipeline both paths share.
func (uc *InboundWebhookUseCase) processAIAfterDefer(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult {
	// Re-checked here because RunPending reaches this method directly, not
	// through ProcessAI's guards.
	if partial.IsBotPaused {
		return nil
	}
	if partial.AgentID == nil || *partial.AgentID == "" {
		return nil
	}

	// US-046: serialize AI processing per conversation. The webhook
	// handlers fire ProcessAI on detached goroutines, so without this a
	// burst of inbounds on one conversation would run concurrently and
	// reply out of order. Hold the lock for the whole pipeline so the
	// next message only starts once this reply is persisted/dispatched.
	// A cancelled ctx (takeover/disconnect) while waiting drops the reply
	// rather than parking the goroutine forever.
	if uc.serializer != nil && partial.ConversationID != "" {
		release, ok := uc.serializer.Lock(ctx, partial.ConversationID)
		if !ok {
			slog.Info("AI processing canceled while waiting for conversation lock; dropping reply",
				"conversation_id", partial.ConversationID)
			return nil
		}
		defer release()
	}

	agent, err := uc.agentRepo.GetByID(ctx, *partial.AgentID)
	if err != nil || agent == nil {
		slog.Error("agent not found for AI processing", "error", err)
		return nil
	}

	// Business hours: skip AI outside the configured window so the bot
	// doesn't reply at 3am (a dead giveaway vs a human attendant). The
	// inbound message is already saved — a human picking up the
	// conversation later still sees it, and the returning customer just
	// sees "read, no reply yet" as if waiting on a human team.
	if !agent.IsWithinBusinessHours(time.Now()) {
		slog.Debug("AI skipped: outside business hours",
			"agent_id", agent.ID, "conversation_id", partial.ConversationID)
		return nil
	}

	// US-044: register a cancelable context for this generation so a human
	// assuming the conversation mid-stream cancels the in-flight AI before
	// its reply is persisted/dispatched. The bot-pause check at the top of
	// the pipeline only catches a takeover that landed BEFORE we started;
	// this catches the race where it lands while we're already streaming.
	if uc.inflight != nil && partial.ConversationID != "" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		cleanup := uc.inflight.Register(partial.ConversationID, cancel)
		defer cleanup()
		defer cancel()
	}

	result, err := uc.processMsg.Execute(ctx, ProcessMessageDTO{
		AccountID:      partial.AccountID,
		AgentID:        *partial.AgentID,
		ConversationID: partial.ConversationID,
		Message:        content,
	})
	if err != nil {
		// US-044: a canceled context means a human assumed the conversation
		// while we were building the reply (caught at a tool-call boundary
		// before streaming). Drop silently — no fallback, no alert.
		if errors.Is(err, context.Canceled) {
			slog.Info("AI generation canceled before stream (takeover); dropping reply",
				"conversation_id", partial.ConversationID)
			return nil
		}
		// Account suspended (past_due grace expired): same treatment as
		// usage limit — save a dashboard message, do not send to channel.
		if err == domain.ErrAccountSuspended {
			fallback := "Conta suspensa por fatura não paga. Atualize seu método de pagamento para retomar o atendimento automático."
			convID := partial.ConversationID
			botName := "Replider"
			msg := &domain.Message{
				ID:             ulid.Make().String(),
				ChannelID:      partial.ChannelID,
				ConversationID: &convID,
				From:           "bot",
				SenderName:     &botName,
				Type:           "text",
				Content:        &fallback,
				Direction:      "outbound",
				Timestamp:      time.Now(),
			}
			if _, saveErr := uc.msgRepo.Create(ctx, msg); saveErr != nil {
				slog.Error("failed to save account-suspended fallback", "error", saveErr)
			}
			return &ProcessAIResult{Message: msg, LimitReached: true}
		}
		// Usage limit reached: save a system message visible on the
		// dashboard but NOT sent to the external channel. The handler
		// broadcasts a usage_limit_reached WS event so the UI can
		// surface an upgrade prompt.
		if err == domain.ErrUsageLimitExceeded {
			fallback := "Limite de uso mensal atingido. Um atendente humano retornará em breve."
			convID := partial.ConversationID
			botName := "Replider"
			msg := &domain.Message{
				ID:             ulid.Make().String(),
				ChannelID:      partial.ChannelID,
				ConversationID: &convID,
				From:           "bot",
				SenderName:     &botName,
				Type:           "text",
				Content:        &fallback,
				Direction:      "outbound",
				Timestamp:      time.Now(),
			}
			if _, saveErr := uc.msgRepo.Create(ctx, msg); saveErr != nil {
				slog.Error("failed to save usage-limit fallback", "error", saveErr)
			}
			return &ProcessAIResult{Message: msg, LimitReached: true}
		}
		// BYOK contract account hasn't configured an API key yet —
		// dashboard-only notice (do not send to channel) so the admin
		// is prompted to fill it in via Settings instead of the
		// customer being told the bot has a configuration problem.
		if isAppErr(err, "BYOK_KEY_MISSING") {
			fallback := "Chave de API não configurada. Configure em Configurações para retomar o atendimento automático."
			convID := partial.ConversationID
			botName := "Replider"
			msg := &domain.Message{
				ID:             ulid.Make().String(),
				ChannelID:      partial.ChannelID,
				ConversationID: &convID,
				From:           "bot",
				SenderName:     &botName,
				Type:           "text",
				Content:        &fallback,
				Direction:      "outbound",
				Timestamp:      time.Now(),
			}
			if _, saveErr := uc.msgRepo.Create(ctx, msg); saveErr != nil {
				slog.Error("failed to save BYOK-key-missing fallback", "error", saveErr)
			}
			return &ProcessAIResult{Message: msg, LimitReached: true}
		}
		// Out-of-credits fallback (legacy token-only path): send a short
		// message instead of ghosting the end-user.
		if err == domain.ErrNoCredits {
			fallback := "No momento não consigo responder automaticamente. Em breve um atendente humano retornará seu contato."
			convID := partial.ConversationID
			botName := "Replider"
			msg := &domain.Message{
				ID:             ulid.Make().String(),
				ChannelID:      partial.ChannelID,
				ConversationID: &convID,
				From:           "bot",
				SenderName:     &botName,
				Type:           "text",
				Content:        &fallback,
				Direction:      "outbound",
				Timestamp:      time.Now(),
			}
			if _, saveErr := uc.msgRepo.Create(ctx, msg); saveErr != nil {
				slog.Error("failed to save out-of-credits fallback", "error", saveErr)
			}
			return &ProcessAIResult{Message: msg}
		}
		// Control-plane errors (ownership / agent missing / race / paused)
		// are misconfiguration or state problems, not an LLM failure. Stay
		// silent so we never message the customer about an internal state
		// issue, and don't pause the bot on a transient ownership glitch.
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrAgentNotFound) ||
			errors.Is(err, domain.ErrConversationNotFound) || errors.Is(err, domain.ErrBotPaused) {
			slog.Error("AI processing failed (control error; no customer fallback)",
				"conversation_id", partial.ConversationID, "error", err)
			if uc.alerter != nil {
				uc.alerter.Record("ai_provider", err.Error())
			}
			return nil
		}
		// US-045: a genuine AI/provider failure (stream error, provider init,
		// prompt build, infra). Don't ghost the customer — send the fallback
		// and raise the human-takeover flag.
		slog.Error("AI processing failed; sending fallback", "conversation_id", partial.ConversationID, "error", err)
		return uc.aiFailureFallback(ctx, partial, err.Error())
	}

	// Drain the streaming response while respecting ctx. Without the
	// ctx.Done arm, a provider that stops emitting chunks without
	// closing the channel (seen on stalled Gemini/OpenAI streams)
	// wedges this goroutine forever — callers never see ProcessAI
	// return, so the "typing" WS event never gets its matching
	// "typing_stopped" and the dashboard shows the bot typing
	// indefinitely.
	var aiText string
readLoop:
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break readLoop
			}
			aiText += chunk
		case <-ctx.Done():
			// A plain Canceled is a deliberate stop — a human took over the
			// conversation (US-044) or the client disconnected — NOT a
			// provider fault, so don't save/send a partial reply and don't
			// fire the AI-failure alert. A DeadlineExceeded is a genuine
			// stall worth alerting on.
			if errors.Is(ctx.Err(), context.Canceled) {
				slog.Info("AI stream canceled (takeover/disconnect); dropping reply",
					"conversation_id", partial.ConversationID)
				return nil
			}
			// US-045: a genuine provider stall — send the fallback + raise
			// human takeover instead of leaving the customer ghosted.
			slog.Warn("AI stream stalled before completion; sending fallback",
				"conversation_id", partial.ConversationID, "error", ctx.Err())
			return uc.aiFailureFallback(ctx, partial, "stream stalled: "+ctx.Err().Error())
		}
	}

	// Empty completion (provider closed the stream without emitting
	// anything). Saving and broadcasting a blank bot message just
	// creates a ghost bubble in the dashboard and a failed send on
	// the channel — treat it as an AI failure instead.
	if aiText == "" {
		// US-045: an empty completion is an AI failure — send the fallback +
		// raise human takeover rather than ghosting the customer.
		slog.Warn("AI returned empty response; sending fallback",
			"conversation_id", partial.ConversationID)
		return uc.aiFailureFallback(ctx, partial, "empty response")
	}

	// Silent hand-off: when the agent has SilentOnUnknown=true the
	// prompt tells the model to emit the HandoffSentinel token alone
	// in lieu of "I don't know" text. Intercept it here — do NOT send
	// anything to the customer, pause the bot and surface the thread
	// to the dashboard so an attendant takes over.
	if strings.Contains(aiText, services.HandoffSentinel) {
		slog.Info("silent handoff requested",
			"conversation_id", partial.ConversationID,
			"raw", aiText)
		if err := uc.convRepo.SetBotPaused(ctx, partial.ConversationID, partial.AccountID, true); err != nil {
			slog.Warn("failed to pause bot on handoff",
				"conversation_id", partial.ConversationID, "error", err)
		}
		return &ProcessAIResult{HandoffRequested: true}
	}

	// Read usage alert (non-blocking — channel is buffered with 1 element)
	var usageAlert *UsageAlertInfo
	select {
	case alert := <-result.UsageAlert:
		usageAlert = alert
	default:
	}

	aiMsgID := ulid.Make().String()
	botName := agent.Name
	convID := partial.ConversationID

	// US-030: per-message token accounting + estimated provider cost.
	// Prefer the provider's reported usage; when it's missing (some
	// streamed responses omit usage) fall back to the len(text)/4
	// heuristic so cost attribution is never silently zero.
	var inTok, outTok int64
	if usageAlert != nil {
		inTok = usageAlert.InputTokensThisCall
		outTok = usageAlert.OutputTokensThisCall
	}
	// US-042: flag the row when either count had to be estimated so A/B
	// attribution + billing can discount it instead of trusting it as an
	// exact provider count.
	tokenEstimated := false
	if inTok <= 0 {
		inTok = services.EstimateTokens(content)
		tokenEstimated = true
	}
	if outTok <= 0 {
		outTok = services.EstimateTokens(aiText)
		tokenEstimated = true
	}
	costUSD := services.ProviderCostUSD(agent.Provider, agent.Model, inTok, outTok)
	inTok32 := int32(inTok)
	outTok32 := int32(outTok)

	aiMsg := &domain.Message{
		ID:              aiMsgID,
		ChannelID:       partial.ChannelID,
		ConversationID:  &convID,
		From:            "bot",
		SenderName:      &botName,
		Type:            "text",
		Content:         &aiText,
		Direction:       "outbound",
		Timestamp:       time.Now(),
		InputTokens:     &inTok32,
		OutputTokens:    &outTok32,
		ProviderCostUSD: &costUSD,
		TokenEstimated:  &tokenEstimated,
	}
	// Stamp A/B routing attribution on the outbound message metadata
	// when the conversation was bucketed into an experiment variant.
	// Token breakdown lands here too so the per-variant aggregation can
	// sum tokens directly off `messages` without a separate journal.
	if result.Variant != nil && result.ExperimentID != nil {
		meta := map[string]any{
			"experiment_id": *result.ExperimentID,
			"variant":       *result.Variant,
		}
		if usageAlert != nil {
			if usageAlert.InputTokensThisCall > 0 {
				meta["tokens_in"] = usageAlert.InputTokensThisCall
			}
			if usageAlert.OutputTokensThisCall > 0 {
				meta["tokens_out"] = usageAlert.OutputTokensThisCall
			}
		}
		if raw, err := json.Marshal(meta); err == nil {
			aiMsg.Metadata = raw
		}
	}
	// Final takeover guard: SaveInbound captured IsBotPaused before the
	// AI ran. If a human attendant assumed the conversation during that
	// AI round, the snapshot is stale and persisting + sending this
	// reply would race with the human's first message. Re-read the row
	// and drop the bot turn silently when it's now paused.
	if current, err := uc.convRepo.GetByID(ctx, partial.ConversationID, partial.AccountID); err == nil && current != nil && current.IsBotPaused {
		slog.Info("bot reply dropped: human took over during AI run",
			"conversation_id", partial.ConversationID, "account_id", partial.AccountID)
		return &ProcessAIResult{HandoffRequested: true}
	}

	if _, err := uc.msgRepo.Create(ctx, aiMsg); err != nil {
		slog.Error("failed to save AI response message", "error", err)
	}

	// US-030: fold this turn's estimated cost into the conversation total
	// so the cost-attribution dashboard stays current. Best-effort — a
	// failure here must not block the reply pipeline.
	if costUSD > 0 {
		if err := uc.convRepo.IncrementCost(ctx, partial.ConversationID, partial.AccountID, costUSD); err != nil {
			slog.Warn("failed to increment conversation cost",
				"conversation_id", partial.ConversationID, "error", err)
		}
	}

	// Mission 6 analytics: stamp first_response_at + first_response_by
	// in a single idempotent UPDATE. Replaces the previous Get+Update
	// pattern so the first-response marker doesn't race with human
	// takeover (both paths now funnel into the same guarded query).
	if err := uc.convRepo.SetFirstResponse(ctx, partial.ConversationID, partial.AccountID, "bot"); err != nil {
		slog.Warn("set first response (bot) failed",
			"conversation_id", partial.ConversationID, "error", err)
	}

	return &ProcessAIResult{
		Message:    aiMsg,
		UsageAlert: usageAlert,
	}
}

// aiFailureFallback handles a genuine AI/provider failure (US-045): instead of
// ghosting the customer, it sends a configurable fallback reply
// (services.AIFailureFallbackMessage) AND raises the human-takeover flag by
// pausing the bot for the conversation, so an attendant picks up and the next
// inbound doesn't retry straight into the same failure. The returned
// ProcessAIResult carries the fallback as a normal outbound message, so the
// handler sends it to the channel (LimitReached stays false).
//
// Writes use a fresh detached context: the failing path may carry a cancelled
// or deadline-exceeded request ctx (stall case), which would otherwise reject
// the very writes that rescue the customer.
func (uc *InboundWebhookUseCase) aiFailureFallback(ctx context.Context, partial *InboundPartialResult, reason string) *ProcessAIResult {
	if uc.alerter != nil {
		uc.alerter.Record("ai_provider", reason)
	}
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Raise human-takeover flag. Best-effort: a customer reply is more
	// important than the pause, so a failure here still sends the fallback.
	if err := uc.convRepo.SetBotPaused(writeCtx, partial.ConversationID, partial.AccountID, true); err != nil {
		slog.Warn("ai-failure fallback: pause bot failed",
			"conversation_id", partial.ConversationID, "error", err)
	}

	fallback := services.AIFailureFallbackMessage
	convID := partial.ConversationID
	botName := "Replider"
	msg := &domain.Message{
		ID:             ulid.Make().String(),
		ChannelID:      partial.ChannelID,
		ConversationID: &convID,
		From:           "bot",
		SenderName:     &botName,
		Type:           "text",
		Content:        &fallback,
		Direction:      "outbound",
		Timestamp:      time.Now(),
	}
	if _, saveErr := uc.msgRepo.Create(writeCtx, msg); saveErr != nil {
		slog.Error("failed to save AI-failure fallback", "error", saveErr)
	}
	return &ProcessAIResult{Message: msg}
}

// UpdateContactAvatar persists freshly-fetched contact profile fields
// (avatar URL + bio) on the conversation. Empty values are allowed —
// they clear a stale field (e.g. the user removed their photo or bio)
// and still bump the fetched-at cursor so we don't retry on every
// inbound.
func (uc *InboundWebhookUseCase) UpdateContactAvatar(ctx context.Context, conversationID, accountID, url, bio string) error {
	return uc.convRepo.RefreshContactAvatar(ctx, conversationID, accountID, url, bio)
}

// Execute is the full inbound flow (backwards-compatible). Internally calls SaveInbound + ProcessAI.
func (uc *InboundWebhookUseCase) Execute(ctx context.Context, dto InboundMessageDTO) (*InboundMessageResult, error) {
	channel, err := uc.channelRepo.GetByWebhookToken(ctx, dto.ChannelToken)
	if err != nil || channel == nil {
		return nil, fmt.Errorf("channel not found for token")
	}

	partial, err := uc.SaveInbound(ctx, channel, dto)
	if err != nil {
		return nil, err
	}

	aiResult := uc.ProcessAI(ctx, partial, dto.Content)
	var aiResponse *domain.Message
	if aiResult != nil {
		aiResponse = aiResult.Message
	}

	result := &InboundMessageResult{
		AccountID:      partial.AccountID,
		ConversationID: partial.ConversationID,
		Message:        partial.Message,
		AIResponse:     aiResponse,
	}
	return result, nil
}

// classifySentimentAsync runs the LLM classifier off the request path.
// Uses a fresh detached context (NOT the request's) so the goroutine
// outlives the inbound webhook ack — Meta's 20s budget is unrelated
// to how long the analyzer takes. 30s cap matches the LLM SDK's own
// timeout so we don't leak goroutines on hung providers.
//
// Best-effort throughout: if the LLM fails or returns garbage we just
// don't update the row, leaving the previous classification (or nil)
// in place. The dashboard tile renders gracefully either way.
func (uc *InboundWebhookUseCase) classifySentimentAsync(accountID, conversationID, text string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("sentiment classification panicked",
				"conversation_id", conversationID, "recover", r)
		}
	}()
	// US-033 justified-Background: launched with `go` off the request
	// path (see doc comment); best-effort, must not be cancelled when the
	// inbound webhook handler returns its 200.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	label, score, err := uc.sentiment.Analyze(ctx, accountID, text)
	if err != nil {
		slog.Debug("sentiment analyze failed",
			"conversation_id", conversationID, "error", err)
		return
	}
	if err := uc.convRepo.SetSentiment(ctx, conversationID, accountID, label, float32(score)); err != nil {
		slog.Warn("sentiment persist failed",
			"conversation_id", conversationID, "error", err)
	}
}

// intentColors mirrors services.IntentColors. Duplicated here as a
// const-style map so the usecases layer doesn't import services and
// invert the arch direction. Keep these in sync with the service.
var intentColors = map[string]struct {
	Color string
	Label string
}{
	"cobranca":     {"ef4444", "Cobrança"},
	"compra":       {"22c55e", "Compra"},
	"cancelamento": {"f59e0b", "Cancelamento"},
	"suporte":      {"3b82f6", "Suporte"},
	"duvida":       {"a855f7", "Dúvida"},
	"outro":        {"6b7280", "Outro"},
}

// classifyIntentAsync runs the LLM intent classifier off the request
// path, finds-or-creates the matching tag for the account, and
// attaches it to the conversation. Best-effort all the way down —
// any failure leaves the conversation untagged and logs at WARN.
//
// We skip the "outro" bucket: persisting a tag that says "we
// classified this and it didn't fit anywhere" adds noise to the
// dashboard without helping anyone filter.
func (uc *InboundWebhookUseCase) classifyIntentAsync(accountID, conversationID, text string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("intent classification panicked",
				"conversation_id", conversationID, "recover", r)
		}
	}()
	// US-033 justified-Background: launched with `go` off the request
	// path (see doc comment); best-effort, must not be cancelled when the
	// inbound webhook handler returns its 200.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	label, score, err := uc.intent.Classify(ctx, accountID, text)
	if err != nil {
		slog.Debug("intent classify failed",
			"conversation_id", conversationID, "error", err)
		return
	}
	if label == "outro" || label == "" {
		return
	}
	meta, ok := intentColors[label]
	if !ok {
		return
	}
	tag, err := uc.tagRepo.GetByAccountAndName(ctx, accountID, meta.Label)
	if err != nil {
		slog.Warn("intent tag lookup failed",
			"conversation_id", conversationID, "error", err)
		return
	}
	if tag == nil {
		tag = &domain.Tag{
			ID:        ulid.Make().String(),
			AccountID: accountID,
			Name:      meta.Label,
			Color:     meta.Color,
		}
		if err := uc.tagRepo.Create(ctx, tag); err != nil {
			slog.Warn("intent tag create failed",
				"conversation_id", conversationID, "error", err)
			return
		}
	}
	if err := uc.tagRepo.AddToConversation(ctx, conversationID, tag.ID); err != nil {
		slog.Warn("intent tag attach failed",
			"conversation_id", conversationID, "error", err)
		return
	}
	slog.Info("auto-tagged conversation by intent",
		"conversation_id", conversationID,
		"intent", label, "score", score, "tag_id", tag.ID)
}

// creditTemplateReply counts a reply against the template that provoked it.
//
// The link is the threading header the customer's mail client echoed back:
// the outbound message recorded which template it used, and In-Reply-To
// points at it. Without that hop a reply is just a message, and every
// template's reply rate stays at zero forever.
//
// Best effort in every direction. This runs after the reply is already
// persisted, and losing a counter must never cost the message.
func (uc *InboundWebhookUseCase) creditTemplateReply(ctx context.Context, channelID string, metadata []byte) {
	if uc.templateStats == nil || uc.msgRepo == nil || len(metadata) == 0 {
		return
	}
	var meta struct {
		InReplyTo string `json:"in_reply_to"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil || meta.InReplyTo == "" {
		return
	}
	parent, err := uc.msgRepo.GetByExternalID(ctx, channelID, meta.InReplyTo)
	if err != nil || parent == nil {
		return
	}
	templateID, version := templateFromMetadata(parent.Metadata)
	if templateID == "" {
		return
	}
	if err := uc.templateStats.RecordReply(ctx, templateID, version); err != nil {
		slog.WarnContext(ctx, "could not credit the reply to its template",
			"template_id", templateID, "error", err)
	}
}

// templateFromMetadata reads the template a sent message used, whatever shape
// the driver handed the metadata back in — a fresh struct carries a map, a
// row read from Postgres carries raw JSONB bytes.
func templateFromMetadata(meta any) (string, int) {
	var m map[string]any
	switch v := meta.(type) {
	case map[string]any:
		m = v
	case json.RawMessage:
		// A named []byte does NOT match a `case []byte` — and json.RawMessage
		// is what the message read constructors actually carry.
		if json.Unmarshal(v, &m) != nil {
			return "", 0
		}
	case []byte:
		if json.Unmarshal(v, &m) != nil {
			return "", 0
		}
	case string:
		if json.Unmarshal([]byte(v), &m) != nil {
			return "", 0
		}
	default:
		return "", 0
	}
	id, _ := m["template_id"].(string)
	version := 1
	if f, ok := m["template_version"].(float64); ok && f > 0 {
		version = int(f)
	}
	return strings.TrimSpace(id), version
}
