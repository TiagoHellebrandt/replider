package ai

import (
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
)

// Streaming usage: every chunk's UsageMetadata is cumulative, so summing the
// chunks multiplied the prompt by the chunk count and lost the thought tokens
// a reasoning model bills as output.

func TestGeminiStreamUsage_TakesTheLastChunkNotTheSum(t *testing.T) {
	var u geminiStreamUsage
	// Three chunks of one reply: prompt repeated, candidates running total.
	u.observe(&genai.UsageMetadata{PromptTokenCount: 700, CandidatesTokenCount: 10, TotalTokenCount: 710})
	u.observe(&genai.UsageMetadata{PromptTokenCount: 700, CandidatesTokenCount: 30, TotalTokenCount: 730})
	u.observe(&genai.UsageMetadata{PromptTokenCount: 700, CandidatesTokenCount: 46, TotalTokenCount: 746})

	prompt, completion := u.totals()

	assert.Equal(t, int64(700), prompt, "the prompt is charged once, not once per chunk")
	assert.Equal(t, int64(46), completion)
}

func TestGeminiStreamUsage_BillsThoughtTokensAsOutput(t *testing.T) {
	var u geminiStreamUsage
	// The real approach draft from the CLAUDE.md note: prompt 729, candidates
	// 46, total 1875 — 1100 tokens of reasoning Google bills as output.
	u.observe(&genai.UsageMetadata{PromptTokenCount: 729, CandidatesTokenCount: 46, TotalTokenCount: 1875})

	prompt, completion := u.totals()

	assert.Equal(t, int64(729), prompt)
	assert.Equal(t, int64(1146), completion, "total − prompt, same rule as the sync path")
}

func TestGeminiStreamUsage_IgnoresChunksWithoutMetadataAndHandlesNone(t *testing.T) {
	var u geminiStreamUsage
	u.observe(nil)
	prompt, completion := u.totals()
	assert.Zero(t, prompt)
	assert.Zero(t, completion)

	u.observe(&genai.UsageMetadata{PromptTokenCount: 5, CandidatesTokenCount: 2, TotalTokenCount: 7})
	u.observe(nil)
	prompt, completion = u.totals()
	assert.Equal(t, int64(5), prompt)
	assert.Equal(t, int64(2), completion)
}
