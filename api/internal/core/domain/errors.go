package domain

import (
	"fmt"
	"strings"
)

// AppError provides structured error information for the API.
// Metadata is an opaque bag the handler serialises back to the client
// so UI code can render precise copy (e.g. "2/1 agentes usados")
// without having to re-fetch the plan or re-derive numbers from the
// generic Message string. Optional; most errors don't need it.
type AppError struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewPlanLimitError is the canonical constructor for a
// PLAN_LIMIT_EXCEEDED response. Every check on user-facing counted
// resources (agents, channels, seats, etc.) returns this so the
// Flutter client can intercept one error code, read Metadata.resource
// to pick the right copy, and route into the shared upgrade modal.
func NewPlanLimitError(resource string, current, limit int) *AppError {
	return &AppError{
		Code:    "PLAN_LIMIT_EXCEEDED",
		Message: fmt.Sprintf("%s limit reached on current plan", resource),
		Metadata: map[string]any{
			"resource": resource,
			"current":  current,
			"limit":    limit,
		},
	}
}

var (
	// Auth Errors
	ErrEmailAlreadyExists       = &AppError{Code: "AUTH_EMAIL_ALREADY_EXISTS", Message: "email already exists"}
	ErrInvalidCredentials       = &AppError{Code: "AUTH_INVALID_CREDENTIALS", Message: "invalid email or password"}
	ErrUnauthorized             = &AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"}
	ErrInvalidResetToken        = &AppError{Code: "AUTH_INVALID_RESET_TOKEN", Message: "invalid or expired reset token"}
	ErrSocialAuthFailed         = &AppError{Code: "AUTH_SOCIAL_FAILED", Message: "social authentication failed"}
	ErrUserNotFound             = &AppError{Code: "AUTH_USER_NOT_FOUND", Message: "user not found"}
	ErrInviteAlreadyAccepted    = &AppError{Code: "TEAM_INVITE_ALREADY_ACCEPTED", Message: "user has already verified their email — use password reset instead"}
	ErrEmailNotVerified         = &AppError{Code: "AUTH_EMAIL_NOT_VERIFIED", Message: "please verify your email before logging in"}
	ErrInvalidVerificationToken = &AppError{Code: "AUTH_INVALID_VERIFICATION_TOKEN", Message: "invalid or expired verification token"}
	// ErrRegistrationPaused gates every account-creating path (password
	// signup + social auth) while REGISTRATION_ENABLED=false (US-F001).
	// Existing users keep logging in — only NEW accounts are refused.
	ErrRegistrationPaused = &AppError{Code: "REGISTRATION_PAUSED", Message: "new registrations are temporarily paused"}

	// MCP / API token errors
	ErrPATNotFound     = &AppError{Code: "MCP_TOKEN_NOT_FOUND", Message: "access token not found"}
	ErrPATLimitReached = &AppError{Code: "MCP_TOKEN_LIMIT_REACHED", Message: "maximum number of active access tokens reached"}

	// Usage & Plan Errors
	ErrInsufficientTokens = &AppError{Code: "USAGE_INSUFFICIENT_TOKENS", Message: "insufficient AI tokens"}
	ErrNoCredits          = &AppError{Code: "USAGE_NO_CREDITS", Message: "AI limits reached. Please reply manually."}
	ErrPlanNotFound       = &AppError{Code: "PLAN_NOT_FOUND", Message: "plan not found"}
	// ErrPlanMissing (US-043) fires when an account references a PlanID but
	// the plan row can't be fetched (transient DB error or dangling FK).
	// We refuse to silently downgrade a paying account to trial limits —
	// the token-cap logic dereferences plan, so a nil plan would panic.
	// Mapped to 500 so the inbound pipeline fails loud instead of guessing.
	ErrPlanMissing        = &AppError{Code: "BILLING_PLAN_MISSING", Message: "account plan could not be loaded"}
	ErrPlanNotPurchasable = &AppError{Code: "PLAN_NOT_PURCHASABLE", Message: "plan cannot be purchased online — contact sales"}
	// ErrPlanCurrencyMismatch blocks a checkout for a plan priced in a
	// different currency than the account (US-003). Accounts are pinned to
	// one currency at signup; the /plans listing only offers matching
	// plans, so hitting this means a hand-crafted request.
	ErrPlanCurrencyMismatch = &AppError{Code: "PLAN_CURRENCY_MISMATCH", Message: "plan currency does not match account currency"}
	// ErrAddonCurrencyUnsupported blocks addon credit packs for non-BRL
	// accounts — packs are priced in centavos BRL only; charging the same
	// number of cents in USD would ~5x the price (US-003).
	ErrAddonCurrencyUnsupported = &AppError{Code: "ADDON_CURRENCY_UNSUPPORTED", Message: "addon packs are not available for this account's currency"}
	ErrUsageLimitExceeded       = &AppError{Code: "USAGE_LIMIT_EXCEEDED", Message: "monthly usage limit reached"}
	// ErrDailyTokenLimitExceeded is returned when an account exhausts its
	// per-day AI token cap (US-025); the AI reply is skipped for the rest
	// of the day, bounding a runaway integration's blast radius.
	ErrDailyTokenLimitExceeded = &AppError{Code: "USAGE_DAILY_LIMIT_EXCEEDED", Message: "daily AI token limit reached — resets tomorrow"}
	ErrAccountSuspended        = &AppError{Code: "ACCOUNT_SUSPENDED", Message: "account suspended due to unpaid invoice — update your payment method"}

	// AI Provider Errors
	// ErrAIProviderUnavailable is returned when we fail to reach the
	// upstream LLM (OpenAI/Gemini) — wrong model name, missing API key,
	// Meta-side outage. The handler surfaces this as 400 with a clear
	// code instead of a generic 500 so the UI can render a useful
	// snackbar and ops can grep for it.
	ErrAIProviderUnavailable = &AppError{Code: "AI_PROVIDER_UNAVAILABLE", Message: "AI provider is unavailable"}

	// ErrAIDocumentUnsupported is returned by a decorator that DOES expose
	// GenerateFromDocument but wraps a provider that cannot read files.
	//
	// The decorators (breaker/retry/tracing) must declare the method
	// unconditionally — a Go method set is static, so a decorator cannot
	// implement DocumentReader only when its inner does. Declaring it is
	// what keeps the capability visible through the chain; this error is
	// how the chain stays HONEST when the capability isn't really there.
	// Callers that type-assert map it back to their own "this provider
	// cannot read documents" message.
	ErrAIDocumentUnsupported = &AppError{Code: "AI_DOCUMENT_UNSUPPORTED", Message: "AI provider cannot read documents"}

	// Domain Entity Errors
	ErrAccountNotFound   = &AppError{Code: "ACCOUNT_NOT_FOUND", Message: "account not found"}
	ErrAgentNotFound     = &AppError{Code: "AGENT_NOT_FOUND", Message: "agent not found"}
	ErrAgentLimitReached = &AppError{Code: "AGENT_LIMIT_REACHED", Message: "account already has the maximum number of agents"}

	// Billing errors
	ErrNoCustomer = &AppError{Code: "BILLING_NO_CUSTOMER", Message: "account has no Stripe customer — complete at least one checkout first"}

	// Skill Errors
	ErrInvalidSkillType   = &AppError{Code: "SKILL_INVALID_TYPE", Message: "invalid skill type"}
	ErrInvalidSkillConfig = &AppError{Code: "SKILL_INVALID_CONFIG", Message: "invalid skill configuration"}
	ErrSkillNotFound      = &AppError{Code: "SKILL_NOT_FOUND", Message: "skill not found"}

	// Knowledge Errors
	ErrKnowledgeNotFound = &AppError{Code: "KNOWLEDGE_NOT_FOUND", Message: "knowledge chunk not found"}

	// Calendar Errors
	ErrCalendarNotFound = &AppError{Code: "CALENDAR_NOT_FOUND", Message: "calendar not found"}

	// Channel Errors
	ErrChannelNotFound = &AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"}
	// ErrWAPhoneInUse fires when two parallel signups try to register
	// the same WhatsApp phone_number_id and the second loses the race
	// against the partial unique index from migration 000048. Without
	// this typed error, the loser saw a generic 5xx and the curated
	// "número já está conectado" 409 message never reached the UI.
	ErrWAPhoneInUse = &AppError{Code: "WA_PHONE_OWNED_ELSEWHERE", Message: "Este número já está conectado a outra conta Replider. Solicite a liberação para o suporte."}

	// Seller convert-lead errors (US-004, vendedores). ALREADY_LINKED is a
	// 409 (never overwrite an existing attribution — the owner panel is the
	// only place that reallocates); NOT_ASSIGNEE is 403; the rest keep the
	// default 400. Register new codes in httpStatusForCode (api) AND
	// parseApiError (app) when adding here.
	ErrSellerConvUnassigned        = &AppError{Code: "SELLER_CONV_UNASSIGNED", Message: "conversation has no assigned attendant — take it over before converting"}
	ErrSellerNotConvAssignee       = &AppError{Code: "SELLER_NOT_CONV_ASSIGNEE", Message: "only the assigned attendant or an admin can convert this lead"}
	ErrSellerAccountAlreadyLinked  = &AppError{Code: "SELLER_ACCOUNT_ALREADY_LINKED", Message: "this account is already linked to a seller"}
	ErrSellerCannotConvertPlatform = &AppError{Code: "SELLER_CANNOT_CONVERT_PLATFORM", Message: "the platform account itself cannot be converted"}
	// US-014 review fixes. NOT_FLAGGED is 403 (platform user without the
	// explicit owner-granted seller flag); TARGET_UNRELATED is 409 (an
	// organically paying account can only be converted when the lead
	// conversation's contact provably belongs to it).
	ErrSellerNotFlagged          = &AppError{Code: "SELLER_NOT_FLAGGED", Message: "user is not a registered seller — ask the owner to grant seller access"}
	ErrSellerConvTargetUnrelated = &AppError{Code: "SELLER_CONV_TARGET_UNRELATED", Message: "this account already has payment history and the conversation's contact does not match it — conversion blocked"}

	// Conversation Errors
	ErrBotPaused            = &AppError{Code: "CONV_BOT_PAUSED", Message: "bot is paused for this conversation, awaiting human reply"}
	ErrConversationNotFound = &AppError{Code: "CONV_NOT_FOUND", Message: "conversation not found"}
	ErrOutside24hWindow     = &AppError{Code: "CONV_OUTSIDE_24H", Message: "outside 24h messaging window"}
	// ErrConversationAlreadyOpen is the sentinel the postgres repo returns
	// when CreateConversation hits the partial unique index
	// uniq_conversations_open_per_contact (channel_id, contact_id WHERE
	// status != 'closed'). Two parallel inbound webhooks for the same
	// (channel, contact) used to both pass FindByContact and both INSERT
	// fresh rows; the index makes the database the source of truth.
	// SaveInbound's race-recovery path catches this error and re-reads
	// the existing row instead of treating it as a generic failure.
	ErrConversationAlreadyOpen = &AppError{Code: "CONV_ALREADY_OPEN", Message: "an open conversation already exists for this contact"}
	// ErrConversationNotAssumed fires when a human tries to send a
	// message on a conversation that no human has taken over yet. The
	// UI gates the input on this state; the API enforces it so a stale
	// client that kept the send button enabled can't bypass takeover.
	ErrConversationNotAssumed = &AppError{Code: "CONV_NOT_ASSUMED", Message: "conversation is not under human takeover"}
	// ErrMessageSendFailed fires when an attendant reply was persisted
	// but the channel adapter (WhatsApp/Telegram/SMS/IG) failed to
	// deliver it. The message row is stamped failed_at/failed_reason
	// and the API returns this error instead of a silent 200 so the
	// attendant knows the customer never received the reply (review-fixes
	// US-009).
	ErrMessageSendFailed = &AppError{Code: "MESSAGE_SEND_FAILED", Message: "message saved but could not be delivered to the channel"}

	// ErrMessageNotFound also covers "belongs to another account" and "has
	// no stored original": one answer for all three, so probing ids cannot
	// distinguish a message that does not exist from one the caller may not
	// see.
	ErrMessageNotFound = &AppError{Code: "MESSAGE_NOT_FOUND", Message: "mensagem não encontrada"}

	// ErrApproachHasPlaceholder refuses a send outright.
	//
	// Unlike a promise, which is a judgement the approver may legitimately
	// make, an unfilled placeholder is never intentional: "Olá {{nome}}" is a
	// broken message, and letting it out burns a lead for a reason nobody
	// chose. This is the one outbound check that blocks rather than warns.
	ErrApproachHasPlaceholder = &AppError{
		Code:    "APPROACH_HAS_PLACEHOLDER",
		Message: "a mensagem ainda tem um campo por preencher",
	}

	// ErrApproachNeedsProducts refuses to DRAFT at all when the catalogue is
	// empty. With nothing true to offer, the model fills the gap with generic
	// filler ("possíveis sinergias…") — exactly the spam this product
	// promises not to send. Forty of those reached a real approval queue
	// before this gate existed.
	ErrApproachNeedsProducts = &AppError{
		Code:    "APPROACH_NEEDS_PRODUCTS",
		Message: "cadastre seus produtos antes de gerar abordagens — sem catálogo o rascunho sai genérico",
	}

	// ErrApproachNeedsChannel refuses to DRAFT when the account has nowhere
	// to send from.
	//
	// Same shape as the catalogue gate above, and for a sharper reason. On
	// 2026-09-09 an account had 89 finished offers — written, reviewed,
	// objections anticipated — and zero channels. The nightly hunt kept
	// adding forty a night, spending AI on messages that could not leave and
	// burying the ones that mattered under ones nobody could act on.
	//
	// An offer with no door does not wait, it ages. Refusing costs one click
	// (connecting a mailbox); not refusing costs the queue.
	ErrApproachNeedsChannel = &AppError{
		Code:    "APPROACH_NEEDS_CHANNEL",
		Message: "conecte um canal antes de gerar abordagens — sem porta de saída a oferta só envelhece",
	}

	// ErrApproachNeedsAddress refuses to DRAFT for a contact none of the
	// account's channels can deliver to.
	//
	// The door gate above is about the company; this one is about the envelope,
	// and skipping it produced the same waste one lead at a time. Production:
	// 70 contacts, 52 with a phone, ZERO with an e-mail — and a queue of
	// finished offers addressed to none of them. Thirteen carried no address of
	// any kind.
	//
	// The message names the repair because it is a real, finishable task: find
	// the buyer's contact. "Aprove ou descarte" was the advice the product gave
	// instead, and it was impossible to follow.
	ErrApproachNeedsAddress = &AppError{
		Code:    "APPROACH_NEEDS_ADDRESS",
		Message: "este contato não tem e-mail nem telefone que os seus canais alcancem — encontre o contato de compras antes de gerar a oferta",
	}

	// ErrContractCalendarUnavailable is the unwired answer for the buying
	// calendar, matching the optional-setter convention used everywhere else:
	// a missing dependency is a 503 the client can explain, never a panic.
	ErrContractCalendarUnavailable = &AppError{
		Code:    "CONTRACT_CALENDAR_UNAVAILABLE",
		Message: "o calendário de vencimentos não está disponível nesta instalação",
	}

	// ErrContractWindowNotFound covers "no such contract" AND "not one this
	// account's calendar would show" — one answer, or the endpoint becomes a
	// way to enumerate a shared public table one id at a time.
	ErrContractWindowNotFound = &AppError{
		Code:    "CONTRACT_WINDOW_NOT_FOUND",
		Message: "contrato não encontrado no seu calendário",
	}

	// ErrSignalNotFound covers "no such signal" AND "not one of yours" — one
	// answer, for the same reason the contract one gives: signals ride a
	// per-account row but the underlying tender is public, and two distinct
	// answers would let an id be probed for existence.
	ErrSignalNotFound = &AppError{
		Code:    "SIGNAL_NOT_FOUND",
		Message: "licitação não encontrada no seu radar",
	}

	// ErrProposalHasNoLines refuses to print a priced offer with no prices.
	//
	// A letterhead over an empty table is worse than no file: the salesperson
	// forwards it before reading it, and the customer receives a blank quote.
	ErrProposalHasNoLines = &AppError{
		Code:    "PROPOSAL_HAS_NO_LINES",
		Message: "adicione ao menos um item antes de gerar o PDF",
	}

	// ErrTemplateNotFound also covers "belongs to another account".
	ErrTemplateNotFound = &AppError{Code: "EMAIL_TEMPLATE_NOT_FOUND", Message: "modelo não encontrado"}

	// Team Errors
	ErrCannotRemoveSelf      = &AppError{Code: "TEAM_CANNOT_REMOVE_SELF", Message: "cannot remove yourself from the team"}
	ErrCannotRemoveLastAdmin = &AppError{Code: "TEAM_CANNOT_REMOVE_LAST_ADMIN", Message: "cannot remove the last admin of the account"}
	ErrInvalidRole           = &AppError{Code: "TEAM_INVALID_ROLE", Message: "role must be 'admin' or 'attendant'"}
	ErrMemberNotFound        = &AppError{Code: "TEAM_MEMBER_NOT_FOUND", Message: "team member not found"}
	// US-016 auto-assign config: participant must be an active member of
	// the account. Unmapped in httpStatusForCode on purpose → default 400.
	ErrInvalidAutoAssignParticipant = &AppError{Code: "TEAM_INVALID_PARTICIPANT", Message: "participant is not an active member of this account"}
)

// IsAccountLevelDraftRefusal reports a refusal that no other lead will get
// past either: the account has no catalogue, no channel, or no allowance
// left.
//
// A batch must stop on the FIRST of these rather than counting to forty.
// Production spent nine consecutive nights logging 63-70 identical failures
// and recording the count as if it were seventy separate problems.
//
// One predicate, three callers (the nightly batch, the redraft batch, and
// whatever comes next) so a new account-level refusal is honoured everywhere
// the moment it is added here.
func IsAccountLevelDraftRefusal(code string) bool {
	switch code {
	case ErrApproachNeedsProducts.Code, ErrApproachNeedsChannel.Code:
		return true
	}
	// The spend caps are account-level by construction: an allowance that
	// ran out for one lead ran out for all of them.
	return strings.HasPrefix(code, "SPEND_")
}
