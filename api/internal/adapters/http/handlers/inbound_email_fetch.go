package handlers

import (
	"context"
	"log/slog"
	"strings"

	emailchannel "github.com/replider/api/internal/adapters/channels/email"
	"github.com/replider/api/internal/adapters/observability"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Ingesting mail that nobody pushed to us (IMAP inbound).
//
// This file exists so the mailbox poller does NOT get its own copy of the
// inbound pipeline. A message read off IMAP and a message POSTed by a webhook
// have to be stored, attributed, checked for impersonation, triaged and
// broadcast identically — and the only way to keep that true a year from now is
// for both to call `ingestInbound`.
//
// It lives in the handlers package rather than in services because that is where
// the pipeline already is: moving the pipeline to reach the worker would be a
// far larger change than letting the worker reach the pipeline.

// IngestFetchedEmail parses one raw message and runs it through the same path a
// webhook takes.
//
// Returns false with a nil error for a message that was already handled — a
// mailbox poll legitimately re-reads a UID after a partial ingest, and a
// duplicate is the system working, not a failure.
func (h *InboundWebhookHandler) IngestFetchedEmail(ctx context.Context, channel *domain.Channel, raw []byte) (bool, error) {
	parsed, err := emailchannel.ParseRawInbound(raw)
	if err != nil {
		// Unparseable mail is DROPPED rather than retried: the bytes will not
		// improve, and returning an error would park the cursor on this UID and
		// block every reply behind it forever.
		slog.WarnContext(ctx, "imap: skipping unparseable message",
			"channel_id", channel.ID, "error", err)
		return false, nil
	}

	// Our own outbound, coming back to us. The mailbox holds both sides of the
	// conversation when the provider files sent mail into INBOX (or when the
	// salesperson CCs themselves), and ingesting it would create a contact for
	// the account's own address and let the agent answer itself.
	if selfAddressed(channel, parsed.From) {
		return false, nil
	}

	// Mail no person wrote: a delivery report or an autoresponder. The same
	// gate the webhook runs — before this, only the webhook ran it, and in
	// production IMAP is the path that actually carries mail, so every
	// MAILER-DAEMON and every "estou de férias" became a contact, a lead in
	// "responded", and the end of a cadence.
	if verdict, _ := h.emailDeliveryGate(ctx, channel, parsed); verdict != emailGatePass {
		return false, nil
	}

	// Plenty of hosted providers stamp no Authentication-Results at all
	// (GoDaddy among them), and "unknown" renders as "remetente não
	// verificado" on every legitimate message — including mail Gmail itself
	// signed. IMAP preserves the message as the sender wrote it, so when the
	// receiving server answered nothing we run the DKIM check it skipped.
	// Only when it answered NOTHING: a verdict the customer's own MX reached
	// (it saw the connecting IP; we did not) always outranks ours.
	if parsed.Auth.DKIM == emailchannel.AuthUnknown && parsed.Auth.DMARC == emailchannel.AuthUnknown &&
		parsed.Auth.SPF == emailchannel.AuthUnknown {
		parsed.Auth.DKIM = emailchannel.VerifyDKIM(ctx, raw, emailchannel.EmailDomainOf(parsed.From))
	}

	item, ok := h.emailInboundItem(ctx, channel, parsed)
	if !ok {
		// No text after the quoted history is stripped, and no subject: there
		// is nothing to show the operator. Dropped rather than retried.
		return false, nil
	}

	partial, runAI, err := h.ingestInbound(ctx, channel, channel.WebhookToken, item)
	if err != nil {
		if err == usecases.ErrDuplicateInbound {
			return false, nil
		}
		return false, err
	}
	if runAI {
		// Detached exactly like the webhook path: the reply goes out through
		// the same broadcast + send + failure-reporting ladder, and a slow model
		// must not hold an IMAP session open.
		//
		// WithoutCancel rather than a bare goroutine over ctx: the poll's
		// context is about to be reused for the next message and would cancel a
		// reply mid-generation, while dropping it entirely would lose the trace
		// and logging values that make an async failure findable.
		detached := context.WithoutCancel(ctx)
		go h.processAIAsyncCtx(detached, channel, partial, item.from, item.content)
	}
	return true, nil
}

// selfAddressed reports whether a fetched message was sent BY this channel.
func selfAddressed(channel *domain.Channel, from string) bool {
	own := strings.ToLower(strings.TrimSpace(domain.ChannelConfigString(channel, "from_address")))
	return own != "" && own == strings.ToLower(strings.TrimSpace(from))
}

// emailGateVerdict is what emailDeliveryGate decided about a parsed message.
type emailGateVerdict int

const (
	// emailGatePass: a person wrote this; carry on into the pipeline.
	emailGatePass emailGateVerdict = iota
	// emailGateBounce: a delivery report, and the failure was recorded.
	emailGateBounce
	// emailGateDropped: machine mail we could not or should not act on — an
	// unreadable report, or an autoresponder. Nothing stored.
	emailGateDropped
)

// emailDeliveryGate keeps mail nobody wrote out of the inbox.
//
// ONE gate for both arrival routes (webhook and IMAP), because the failure
// mode of having two is exactly what happened: the webhook checked for
// delivery reports and the poller did not, and the poller is the route
// production mail actually takes.
//
// Order matters. A DSN is checked before the auto-reply markers because a
// report carries `Auto-Submitted: auto-generated` too — stopping at the
// marker would drop the report without suppressing the dead address, which
// is the one thing a hard bounce must do.
//
// Returns the bounce alongside the verdict so a webhook can echo its kind in
// the ack; the poller ignores it.
func (h *InboundWebhookHandler) emailDeliveryGate(
	ctx context.Context,
	channel *domain.Channel,
	parsed emailchannel.InboundEmail,
) (emailGateVerdict, domain.EmailBounce) {
	if emailchannel.LooksLikeDSN(parsed) {
		if bounce, ok := emailchannel.ParseDSN(parsed); ok {
			h.recordBounceCtx(ctx, channel, bounce)
			return emailGateBounce, bounce
		}
		// A report we could not read the failed address out of. It is
		// still not a message from a person — storing it would put a
		// mail server in the inbox and point the agent at it.
		slog.WarnContext(ctx, "email: unreadable delivery report, dropped",
			"channel_id", channel.ID, "from", parsed.From)
		observability.WebhookFailuresTotal.WithLabelValues("email", "unreadable_dsn").Inc()
		return emailGateDropped, domain.EmailBounce{}
	}
	if emailchannel.IsAutoSubmitted(parsed) {
		// INFO, not WARN: an out-of-office is ordinary, and this line is
		// how "why didn't their reply show up?" gets answered.
		slog.InfoContext(ctx, "email: auto-reply dropped",
			"channel_id", channel.ID, "from", parsed.From, "subject", parsed.Subject,
			"auto_submitted", parsed.AutoSubmitted, "precedence", parsed.Precedence)
		return emailGateDropped, domain.EmailBounce{}
	}
	return emailGatePass, domain.EmailBounce{}
}

// recordBounceCtx hands a delivery failure to the bounce use case.
//
// Unwired, it logs and moves on rather than failing: neither a provider
// retry nor an IMAP re-read can supply a missing use case.
func (h *InboundWebhookHandler) recordBounceCtx(ctx context.Context, channel *domain.Channel, b domain.EmailBounce) {
	if h.bounceUC == nil {
		slog.WarnContext(ctx, "email bounce received with no handler wired — not suppressed",
			"channel_id", channel.ID, "kind", string(b.Kind))
		return
	}
	h.bounceUC.Record(ctx, channel, b)
}
