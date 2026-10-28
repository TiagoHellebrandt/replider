package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// FunnelChannelLinker binds an account's default funnel to its email channel
// the moment that channel exists.
//
// Why (analysis 2026-09-01, break #1): the default funnel is created with the
// account, before any channel, and nothing ever came back to bind one — in
// prod 7 of 7 default funnels had channel_id NULL, so every approval answered
// LEAD_FUNNEL_NO_CHANNEL. The send path has a runtime fallback to the
// account's active email channel (see ActiveEmailChannel); this is the
// DURABLE half, so the funnel screen, the MCP tools and the e2e journey all
// see the same channel the sender will use.
//
// Rules:
//   - Only the account's own, active, email channel qualifies.
//   - A funnel already bound to a channel is a decision: never overridden,
//     even by a newer mailbox. Idempotent on repeat calls.
//   - Best-effort for callers: they WARN and never fail channel creation over
//     it — the channel is the thing the customer just made; the link is ours.
type FunnelChannelLinker struct {
	funnels  domain.SalesFunnelRepository
	channels domain.ChannelRepository
}

func NewFunnelChannelLinker(funnels domain.SalesFunnelRepository, channels domain.ChannelRepository) *FunnelChannelLinker {
	return &FunnelChannelLinker{funnels: funnels, channels: channels}
}

var (
	errFunnelLinkNotEmail = &domain.AppError{Code: "FUNNEL_LINK_NOT_EMAIL", Message: "only an email channel can back the default funnel"}
	errFunnelLinkInactive = &domain.AppError{Code: "FUNNEL_LINK_CHANNEL_INACTIVE", Message: "an inactive channel cannot back the default funnel"}
)

// LinkDefaultToChannel points the account's default funnel at channelID when
// the funnel has no channel yet. Safe on a nil receiver (returns nil) so
// optional wiring needs no guard at the call site.
func (l *FunnelChannelLinker) LinkDefaultToChannel(ctx context.Context, accountID, channelID string) error {
	if l == nil || l.funnels == nil || l.channels == nil {
		return nil
	}
	if accountID == "" || channelID == "" {
		return domain.ErrChannelNotFound
	}

	ch, err := l.channels.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if ch == nil || ch.AccountID != accountID {
		return domain.ErrChannelNotFound
	}
	if ch.Type != "email" {
		return errFunnelLinkNotEmail
	}
	if !ch.IsActive {
		return errFunnelLinkInactive
	}

	f, err := l.funnels.EnsureDefault(ctx, accountID)
	if err != nil {
		return err
	}
	if f == nil {
		return domain.ErrSalesFunnelNotFound
	}
	if f.ChannelID != nil && *f.ChannelID != "" {
		// Already bound (to this channel or another): leave the decision alone.
		return nil
	}

	id := channelID
	f.ChannelID = &id
	f.UpdatedAt = time.Now()
	return l.funnels.Update(ctx, f)
}
