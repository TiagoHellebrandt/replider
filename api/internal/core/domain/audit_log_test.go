package domain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditMetaContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, AuditMeta{}, GetAuditMeta(ctx), "fresh context returns zero meta")

	ctx2 := WithAuditMeta(ctx, "203.0.113.7", "Mozilla/5.0")
	got := GetAuditMeta(ctx2)
	assert.Equal(t, "203.0.113.7", got.IP)
	assert.Equal(t, "Mozilla/5.0", got.UserAgent)

	// Original context isn't mutated.
	assert.Equal(t, AuditMeta{}, GetAuditMeta(ctx))
}

func TestAuditMetaContext_EmptyIsNoOp(t *testing.T) {
	ctx := WithAuditMeta(context.Background(), "", "")
	assert.Equal(t, AuditMeta{}, GetAuditMeta(ctx))
}

func TestAuditLog_FillMetaFromContext_StampsFromContext(t *testing.T) {
	ctx := WithAuditMeta(context.Background(), "198.51.100.4", "curl/8.0")
	l := &AuditLog{Action: "account.trial_bypass.set"}

	l.FillMetaFromContext(ctx)

	if assert.NotNil(t, l.IP) {
		assert.Equal(t, "198.51.100.4", *l.IP)
	}
	if assert.NotNil(t, l.UserAgent) {
		assert.Equal(t, "curl/8.0", *l.UserAgent)
	}
}

func TestAuditLog_FillMetaFromContext_DoesNotOverrideExplicit(t *testing.T) {
	explicitIP := "10.0.0.1"
	explicitUA := "explicit-agent"
	l := &AuditLog{IP: &explicitIP, UserAgent: &explicitUA}

	l.FillMetaFromContext(WithAuditMeta(context.Background(), "198.51.100.4", "curl/8.0"))

	assert.Equal(t, "10.0.0.1", *l.IP, "explicit IP wins over context")
	assert.Equal(t, "explicit-agent", *l.UserAgent, "explicit UA wins over context")
}

func TestAuditLog_FillMetaFromContext_NoMetaLeavesNil(t *testing.T) {
	l := &AuditLog{}
	l.FillMetaFromContext(context.Background())
	assert.Nil(t, l.IP)
	assert.Nil(t, l.UserAgent)
}
