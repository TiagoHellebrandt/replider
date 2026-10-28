package whatsapp

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// OptOutChecker reports whether a given recipient has opted out of messaging
// for an account. It is the narrow port the WhatsApp adapter uses to enforce
// the anti-ban opt-out guard (US-025) without importing the repository layer
// — the postgres ContactRepository satisfies it via IsOptedOut.
type OptOutChecker interface {
	IsOptedOut(ctx context.Context, accountID, externalID string) (bool, error)
}

// SetOptOutChecker wires the opt-out guard. Once set, every outbound send
// (SendMessageAndGetID for text/media/template + SendTemplateAndGetID for the
// low-level campaign path) first checks the recipient and aborts before any
// call to Meta when they've opted out. Nil-safe: leaving it unset disables
// the guard (the default for tests that don't exercise opt-out).
func (a *WhatsAppAdapter) SetOptOutChecker(c OptOutChecker) {
	a.optOut = c
}

// ensureNotOptedOut returns domain.ErrContactOptedOut when the recipient has
// opted out, so the caller aborts the send BEFORE reaching the Cloud API.
// Fail-OPEN on a checker error or when no checker is wired: a transient DB
// blip must not silently swallow every legitimate outbound message — the
// opt-out keyword path is the durable enforcement, this guard is the
// belt-and-suspenders pre-send check. The block is logged with the
// CONTACT_OPTED_OUT code for the anti-ban audit trail.
func (a *WhatsAppAdapter) ensureNotOptedOut(ctx context.Context, channel *domain.Channel, to string) error {
	if a.optOut == nil || channel == nil {
		return nil
	}
	optedOut, err := a.optOut.IsOptedOut(ctx, channel.AccountID, to)
	if err != nil {
		slog.Warn("opt-out pre-send check failed; allowing send",
			"channel_id", channel.ID, "account_id", channel.AccountID, "error", err)
		return nil
	}
	if optedOut {
		slog.Warn("outbound blocked: contact opted out",
			"code", domain.ErrContactOptedOut.Code,
			"channel_id", channel.ID,
			"account_id", channel.AccountID)
		return domain.ErrContactOptedOut
	}
	return nil
}
