package usecases

import (
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// makeMsgs builds n messages newest-first (as ListByConversationID
// returns them), alternating inbound/outbound, with non-empty content.
func makeMsgs(n int) []*domain.Message {
	msgs := make([]*domain.Message, 0, n)
	for i := n; i >= 1; i-- {
		dir := "inbound"
		if i%2 == 0 {
			dir = "outbound"
		}
		msgs = append(msgs, &domain.Message{
			Content:   strPtr("msg-" + string(rune('a'+(i%26)))),
			Direction: dir,
		})
	}
	return msgs
}

// TestBuildBoundedHistory_BoundsPromptSize is the core US-024 guarantee:
// no matter how many prior messages exist, the replayed history is capped
// at maxHistory recent turns (+ optional summary + current message).
func TestBuildBoundedHistory_BoundsPromptSize(t *testing.T) {
	maxHistory := 20
	// 500 prior messages — far beyond the window.
	prev := makeMsgs(500)

	got := buildBoundedHistory(prev, "current question", nil, maxHistory)

	// Without a summary: at most maxHistory recent + 1 current.
	assert.LessOrEqual(t, len(got), maxHistory+1,
		"history must be bounded to the window size")
	// The last entry is always the current message.
	assert.Equal(t, "current question", got[len(got)-1].Content)
	assert.Equal(t, "user", got[len(got)-1].Role)
}

// TestBuildBoundedHistory_InjectsSummary verifies the rolling summary is
// prepended as a single leading context line when present.
func TestBuildBoundedHistory_InjectsSummary(t *testing.T) {
	prev := makeMsgs(20)
	summary := "Cliente quer reservar mesa para 4 pessoas no sábado."

	got := buildBoundedHistory(prev, "current", strPtr(summary), 20)

	assert.Equal(t, "user", got[0].Role)
	assert.True(t, strings.Contains(got[0].Content, summary),
		"first entry must carry the summary")
	assert.True(t, strings.HasPrefix(got[0].Content, "System: "),
		"summary uses the System: injected-context convention")
	// summary + 20 recent + current.
	assert.LessOrEqual(t, len(got), 1+20+1)
}

// TestBuildBoundedHistory_EmptySummaryNotInjected ensures a blank/whitespace
// summary doesn't add a useless context line.
func TestBuildBoundedHistory_EmptySummaryNotInjected(t *testing.T) {
	got := buildBoundedHistory(makeMsgs(2), "hi", strPtr("   "), 20)
	for _, m := range got {
		assert.False(t, strings.HasPrefix(m.Content, "System: Resumo"),
			"empty summary must not be injected")
	}
}

// TestBuildBoundedHistory_ChronologicalOrderAndSkipsEmpty verifies recent
// turns are replayed oldest-first and media-only (empty content) rows are
// dropped.
func TestBuildBoundedHistory_ChronologicalOrderAndSkipsEmpty(t *testing.T) {
	// newest-first input: [m3(out), <empty>, m1(in)]
	prev := []*domain.Message{
		{Content: strPtr("third"), Direction: "outbound"},
		{Content: nil, Direction: "inbound"}, // media-only, must be skipped
		{Content: strPtr("first"), Direction: "inbound"},
	}

	got := buildBoundedHistory(prev, "now", nil, 20)

	// Expected chronological: first(user), third(model), now(user).
	assert.Equal(t, 3, len(got))
	assert.Equal(t, "first", got[0].Content)
	assert.Equal(t, "user", got[0].Role)
	assert.Equal(t, "third", got[1].Content)
	assert.Equal(t, "model", got[1].Role)
	assert.Equal(t, "now", got[2].Content)
}

// TestEffectiveMaxHistoryMessages locks the plan-configurable cap + default.
func TestEffectiveMaxHistoryMessages(t *testing.T) {
	assert.Equal(t, domain.DefaultMaxHistoryMessages, (&domain.Plan{}).EffectiveMaxHistoryMessages())
	assert.Equal(t, domain.DefaultMaxHistoryMessages, (*domain.Plan)(nil).EffectiveMaxHistoryMessages())
	assert.Equal(t, 50, (&domain.Plan{MaxHistoryMessages: 50}).EffectiveMaxHistoryMessages())
}
