package usecases

import (
	"context"
	"sort"

	"github.com/replider/api/internal/core/domain"
)

// ActiveEmailChannel resolves the ONE email channel an account sends and
// receives through when nothing more specific was configured.
//
// Why this exists (analysis 2026-09-01, break #1): the default funnel is
// created without a channel and no screen ever bound one, so every approval
// answered LEAD_FUNNEL_NO_CHANNEL and every reply failed to mark its lead.
// The product has exactly one email door per account in practice (the Gmail
// the company already uses), so "the account's active email channel" is the
// right default everywhere a funnel's channel_id is nil.
//
// Deterministic: the OLDEST active email channel wins, so two callers agree
// and a reconnect never silently switches the account to a newer mailbox.
// Returns (nil, nil) when the account has no active email channel — callers
// decide whether that is an error.
func ActiveEmailChannel(ctx context.Context, channels domain.ChannelRepository, accountID string) (*domain.Channel, error) {
	if channels == nil || accountID == "" {
		return nil, nil
	}
	list, err := channels.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var candidates []*domain.Channel
	for _, ch := range list {
		if ch != nil && ch.Type == "email" && ch.IsActive {
			candidates = append(candidates, ch)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})
	return candidates[0], nil
}
