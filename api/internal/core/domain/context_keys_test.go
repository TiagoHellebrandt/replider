package domain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImpersonatorContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetImpersonator(ctx), "fresh context returns empty")

	ctx2 := WithImpersonator(ctx, "ops@x.com")
	assert.Equal(t, "ops@x.com", GetImpersonator(ctx2))

	// Original context isn't mutated.
	assert.Equal(t, "", GetImpersonator(ctx))
}

func TestImpersonatorContext_EmptyIsNoOp(t *testing.T) {
	ctx := WithImpersonator(context.Background(), "")
	assert.Equal(t, "", GetImpersonator(ctx))
}
