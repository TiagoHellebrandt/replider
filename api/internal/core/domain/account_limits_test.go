package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccount_EffectiveMaxAgents_OverrideWins(t *testing.T) {
	override := 99
	plan := &Plan{MaxAgents: 5}
	a := &Account{MaxAgentsOverride: &override}
	assert.Equal(t, 99, a.EffectiveMaxAgents(plan))
}

func TestAccount_EffectiveMaxAgents_FallsBackToPlan(t *testing.T) {
	plan := &Plan{MaxAgents: 5}
	a := &Account{}
	assert.Equal(t, 5, a.EffectiveMaxAgents(plan))
}

// Contract plans (StripePriceID == nil) seed limits at 0 so admins can
// set per-account overrides per deal. Sem override, o valor não pode ser zero
// — travaria a conta no primeiro agente — e também não pode ser infinito: era
// daí que saía o "ilimitado" que a tela mostrava ao cliente sem ninguém ter
// prometido nada. O caso não-negociado responde com o teto de plataforma.
func TestAccount_EffectiveMaxAgents_ContractPlanZeroIsCeiling(t *testing.T) {
	plan := &Plan{Name: "Enterprise", MaxAgents: 0, StripePriceID: nil}
	a := &Account{}
	assert.Equal(t, CeilingAgents, a.EffectiveMaxAgents(plan))
}

// Self-serve plans treat 0 as the documented "feature disabled"
// — the Trial plan has 0 broadcast caps for that exact reason.
// Don't quietly turn that into unlimited just because the account is
// missing an override.
func TestAccount_EffectiveMaxAgents_SelfServeZeroStaysZero(t *testing.T) {
	stripeID := "price_solo"
	plan := &Plan{Name: "Solo", MaxAgents: 0, StripePriceID: &stripeID}
	a := &Account{}
	assert.Equal(t, 0, a.EffectiveMaxAgents(plan))
}

func TestAccount_IsContractPlan(t *testing.T) {
	stripeID := "price_pro"
	selfServe := &Plan{StripePriceID: &stripeID}
	contract := &Plan{StripePriceID: nil}
	a := &Account{}
	assert.False(t, a.IsContractPlan(selfServe))
	assert.True(t, a.IsContractPlan(contract))
	assert.False(t, a.IsContractPlan(nil), "nil plan must not be treated as contract")
}
