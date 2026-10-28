package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/replider/api/internal/config"
)

// The breaker is keyed per (provider, account): with BYOK every tenant runs
// on its own key, so one customer's revoked key must trip only THAT
// customer's breaker.

func TestAIFactory_BreakerIsKeyedPerProviderAndAccount(t *testing.T) {
	f := NewAIFactory(&config.Config{AIProvider: "gemini"})

	a := f.breakerFor("gemini", "acc-a")
	b := f.breakerFor("gemini", "acc-b")
	aAgain := f.breakerFor("gemini", "acc-a")
	aOpenAI := f.breakerFor("openai", "acc-a")

	assert.Same(t, a, aAgain, "the same pair reuses its breaker, or it would never trip")
	assert.NotSame(t, a, b, "two tenants never share a breaker")
	assert.NotSame(t, a, aOpenAI, "two vendors never share a breaker")
}

func TestAIFactory_OneTenantsBadKeyDoesNotTripTheOthers(t *testing.T) {
	f := NewAIFactory(&config.Config{AIProvider: "gemini"})
	bad := f.breakerFor("gemini", "acc-bad")
	good := f.breakerFor("gemini", "acc-good")

	for i := 0; i < 5; i++ {
		bad.Record(errors.New("401 invalid api key"))
	}

	assert.Equal(t, CircuitOpen, bad.State())
	assert.Equal(t, CircuitClosed, good.State())
	assert.NoError(t, good.Allow())
}

func TestAIFactory_PlatformKeyHasItsOwnBreakerSlot(t *testing.T) {
	f := NewAIFactory(&config.Config{AIProvider: "gemini"})

	platform := f.breakerFor("gemini", "")
	assert.Same(t, platform, f.breakerFor("gemini", breakerPlatformKey))
	assert.NotSame(t, platform, f.breakerFor("gemini", "acc-a"))
}
