package ai

import (
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
)

// Reasoning models spend tokens thinking. Google bills those as output and
// reports them in neither PromptTokenCount nor CandidatesTokenCount — only
// in the difference against TotalTokenCount.
//
// The numbers below are from a real approach draft: recording 46 output
// tokens instead of 1146 misses 59% of what that call cost. Every token
// allowance, plan limit and margin figure in the product is computed from
// this, so under-counting prices the product against spend that never
// happened.
func TestGeminiUsage_CountsTheThinkingGoogleBillsFor(t *testing.T) {
	prompt, completion := geminiUsage(&genai.UsageMetadata{
		PromptTokenCount:     729,
		CandidatesTokenCount: 46,
		TotalTokenCount:      1875,
	})

	assert.Equal(t, int64(729), prompt)
	assert.Equal(t, int64(1146), completion, "os tokens de raciocínio sumiram da conta")
}

// A model that does not think reports a total equal to the parts — nothing
// to add, and the visible answer must not be inflated.
func TestGeminiUsage_LeavesANonThinkingCallAlone(t *testing.T) {
	prompt, completion := geminiUsage(&genai.UsageMetadata{
		PromptTokenCount:     100,
		CandidatesTokenCount: 40,
		TotalTokenCount:      140,
	})

	assert.Equal(t, int64(100), prompt)
	assert.Equal(t, int64(40), completion)
}

// Older responses may omit the total. Falling back to the visible count is
// wrong-but-bounded; inventing a negative would corrupt the ledger.
func TestGeminiUsage_SurvivesAMissingTotal(t *testing.T) {
	prompt, completion := geminiUsage(&genai.UsageMetadata{
		PromptTokenCount:     100,
		CandidatesTokenCount: 40,
	})
	assert.Equal(t, int64(100), prompt)
	assert.Equal(t, int64(40), completion)

	p, c := geminiUsage(nil)
	assert.Zero(t, p)
	assert.Zero(t, c)
}
