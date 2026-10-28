package domain

import "strings"

// The lifecycle event catalogue (§6.19, J2).
//
// The dispatcher, the HMAC signing, the retry ladder and the auto-disable have
// existed since the conversation events shipped. What was missing was the rest of
// the lifecycle: a customer could be told when a conversation closed, but not when
// a deal was won, a contract was signed or a commission cycle was settled — which
// are the events anybody integrating actually wants.
//
// A CATALOGUE rather than a free-form string because two different things need to
// agree on the names: the subscription (what a customer asked for) and the emit
// site (what the code sends). A typo on either side is a webhook that silently
// never fires, which is the hardest integration bug to notice — everything looks
// configured and nothing arrives.

// Webhook event names. Renaming one breaks live subscriptions, so treat them as
// API.
const (
	// Message and conversation events (pre-existing — the dashboard has
	// exposed these since the first webhook shipped, so they stay in the
	// catalogue or live subscriptions break).
	EventMessageReceived      = "message.received"
	EventMessageSent          = "message.sent"
	EventConversationCreated  = "conversation.created"
	EventConversationAssigned = "conversation.assigned"
	EventConversationClosed   = "conversation.closed"

	// WhatsApp health events (pre-existing).
	EventWhatsAppQualityChanged        = "whatsapp.quality_changed"
	EventWhatsAppTemplateStatusChanged = "whatsapp.template_status_changed"

	// Funnel events, delivered to the funnel's own endpoint (US-016) and now
	// also subscribable account-wide.
	EventLeadQualified = "lead.qualified"
	EventLeadBuyIntent = "lead.buy_intent"

	// The lifecycle events this story adds.
	EventLeadWon          = "lead.won"
	EventSaleRegistered   = "sale.registered"
	EventContractSigned   = "contract.signed"
	EventReorderPredicted = "reorder.predicted"
	EventCommissionClosed = "commission.closed"
)

// WebhookEventInfo describes one event for the catalogue endpoint.
//
// The description is part of the product: somebody wiring an integration reads
// this list to decide what to subscribe to, and "sale.registered" alone does not
// say whether it fires on an AI-extracted sale nobody confirmed yet.
type WebhookEventInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// WebhookEventCatalogue is every event a customer may subscribe to.
func WebhookEventCatalogue() []WebhookEventInfo {
	return []WebhookEventInfo{
		{EventMessageReceived, "Uma mensagem chegou de um contato."},
		{EventMessageSent, "Uma mensagem saiu para um contato."},
		{EventConversationCreated, "Uma conversa nova foi aberta por um contato."},
		{EventConversationAssigned, "Uma conversa foi assumida por um atendente."},
		{EventConversationClosed, "Uma conversa foi encerrada."},
		{EventLeadQualified, "Um lead preencheu todos os campos obrigatórios do funil."},
		{EventLeadBuyIntent, "Um lead demonstrou intenção de compra."},
		{EventLeadWon, "Um lead foi marcado como ganho."},
		{EventSaleRegistered, "Uma venda foi registrada. Dispara também para venda extraída por IA — o campo `confirmed` diz se um humano já validou."},
		{EventContractSigned, "Um contrato foi assinado por todas as partes."},
		{EventReorderPredicted, "Um cliente chegou na data prevista de recompra."},
		{EventCommissionClosed, "Um ciclo de comissão foi fechado e pago."},
		{EventWhatsAppQualityChanged, "A qualidade do número de WhatsApp mudou (risco de banimento)."},
		{EventWhatsAppTemplateStatusChanged, "Um template de WhatsApp foi aprovado ou rejeitado pela Meta."},
	}
}

// ValidWebhookEvent guards a name arriving from a client.
//
// Refusing an unknown name at subscription time is the whole point: a typo
// accepted here becomes a webhook that is configured, looks healthy and never
// fires.
func ValidWebhookEvent(name string) bool {
	name = strings.TrimSpace(name)
	for _, info := range WebhookEventCatalogue() {
		if info.Name == name {
			return true
		}
	}
	return false
}

// ValidateWebhookEvents checks a subscription's whole list, naming the offender.
//
// "evento inválido" with no name makes somebody compare two lists by eye; naming
// it turns a support thread into a typo fix.
func ValidateWebhookEvents(events []string) error {
	if len(events) == 0 {
		// A subscription to nothing is almost always a mistake — and it is a
		// mistake that presents as "the webhook never fires".
		return &AppError{
			Code:    "WEBHOOK_NO_EVENTS",
			Message: "assine ao menos um evento",
		}
	}
	for _, name := range events {
		if !ValidWebhookEvent(name) {
			return &AppError{
				Code:    "WEBHOOK_UNKNOWN_EVENT",
				Message: "evento desconhecido: " + strings.TrimSpace(name),
			}
		}
	}
	return nil
}

// LifecyclePayload is the shape every lifecycle event carries.
//
// Deliberately flat and small: the id, who it is about, what it is worth. An
// integration that needs more calls the API with the id — embedding the whole
// entity would make the payload a second API surface to version, and would ship
// fields (a contact's phone, a contract's clauses) to a third party the customer
// only meant to notify.
type LifecyclePayload struct {
	// ID is the entity the event is about.
	ID string `json:"id"`
	// ContactID and ContactName say who, when there is a who.
	ContactID   string `json:"contact_id,omitempty"`
	ContactName string `json:"contact_name,omitempty"`
	// AmountCents is what it is worth, when the event has an amount.
	AmountCents int64  `json:"amount_cents,omitempty"`
	Currency    string `json:"currency,omitempty"`
	// Confirmed distinguishes a machine-made record from a human-validated one
	// (an AI-extracted sale fires this event before anybody confirms it).
	Confirmed bool `json:"confirmed"`
	// OccurredAt is when it happened, not when it was delivered — a retry an
	// hour later must not look like a second event.
	OccurredAt string `json:"occurred_at"`
}
