package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

func linkerEmailChannel(id, accountID string, active bool) *domain.Channel {
	return &domain.Channel{ID: id, AccountID: accountID, Type: "email", IsActive: active}
}

func TestFunnelChannelLinker_BindsAnUnboundDefaultFunnel(t *testing.T) {
	ctx := context.Background()
	funnels := new(MockSalesFunnelRepo)
	channels := new(MockChannelRepo)
	channels.On("GetByID", ctx, "ch-1").Return(linkerEmailChannel("ch-1", "acc-1", true), nil)
	funnels.On("EnsureDefault", ctx, "acc-1").Return(&domain.SalesFunnel{ID: "fun-1", AccountID: "acc-1", IsDefault: true}, nil)
	funnels.On("Update", ctx, mock.MatchedBy(func(f *domain.SalesFunnel) bool {
		return f.ID == "fun-1" && f.ChannelID != nil && *f.ChannelID == "ch-1"
	})).Return(nil)

	err := NewFunnelChannelLinker(funnels, channels).LinkDefaultToChannel(ctx, "acc-1", "ch-1")
	require.NoError(t, err)
	funnels.AssertExpectations(t)
}

func TestFunnelChannelLinker_NeverOverridesAnExistingLink(t *testing.T) {
	ctx := context.Background()
	funnels := new(MockSalesFunnelRepo)
	channels := new(MockChannelRepo)
	other := "ch-old"
	channels.On("GetByID", ctx, "ch-new").Return(linkerEmailChannel("ch-new", "acc-1", true), nil)
	funnels.On("EnsureDefault", ctx, "acc-1").Return(&domain.SalesFunnel{ID: "fun-1", AccountID: "acc-1", IsDefault: true, ChannelID: &other}, nil)

	err := NewFunnelChannelLinker(funnels, channels).LinkDefaultToChannel(ctx, "acc-1", "ch-new")
	require.NoError(t, err)
	funnels.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestFunnelChannelLinker_IsIdempotentOnTheSameChannel(t *testing.T) {
	ctx := context.Background()
	funnels := new(MockSalesFunnelRepo)
	channels := new(MockChannelRepo)
	same := "ch-1"
	channels.On("GetByID", ctx, "ch-1").Return(linkerEmailChannel("ch-1", "acc-1", true), nil)
	funnels.On("EnsureDefault", ctx, "acc-1").Return(&domain.SalesFunnel{ID: "fun-1", AccountID: "acc-1", IsDefault: true, ChannelID: &same}, nil)

	linker := NewFunnelChannelLinker(funnels, channels)
	require.NoError(t, linker.LinkDefaultToChannel(ctx, "acc-1", "ch-1"))
	require.NoError(t, linker.LinkDefaultToChannel(ctx, "acc-1", "ch-1"))
	funnels.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestFunnelChannelLinker_RefusesAnotherAccountsChannel(t *testing.T) {
	ctx := context.Background()
	funnels := new(MockSalesFunnelRepo)
	channels := new(MockChannelRepo)
	channels.On("GetByID", ctx, "ch-1").Return(linkerEmailChannel("ch-1", "acc-OTHER", true), nil)

	err := NewFunnelChannelLinker(funnels, channels).LinkDefaultToChannel(ctx, "acc-1", "ch-1")
	assert.ErrorIs(t, err, domain.ErrChannelNotFound)
	funnels.AssertNotCalled(t, "EnsureDefault", mock.Anything, mock.Anything)
}

func TestFunnelChannelLinker_RefusesNonEmailAndInactiveChannels(t *testing.T) {
	ctx := context.Background()
	funnels := new(MockSalesFunnelRepo)
	channels := new(MockChannelRepo)
	channels.On("GetByID", ctx, "wa").Return(&domain.Channel{ID: "wa", AccountID: "acc-1", Type: "whatsapp", IsActive: true}, nil)
	channels.On("GetByID", ctx, "off").Return(linkerEmailChannel("off", "acc-1", false), nil)

	linker := NewFunnelChannelLinker(funnels, channels)
	assert.ErrorIs(t, linker.LinkDefaultToChannel(ctx, "acc-1", "wa"), errFunnelLinkNotEmail)
	assert.ErrorIs(t, linker.LinkDefaultToChannel(ctx, "acc-1", "off"), errFunnelLinkInactive)
	funnels.AssertNotCalled(t, "EnsureDefault", mock.Anything, mock.Anything)
}

func TestFunnelChannelLinker_NilReceiverIsANoOp(t *testing.T) {
	var linker *FunnelChannelLinker
	require.NoError(t, linker.LinkDefaultToChannel(context.Background(), "acc-1", "ch-1"))
}
