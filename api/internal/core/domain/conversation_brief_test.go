package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A brief that silently describes a negotiation which has since moved on is
// worse than no brief: the reader acts on it. So staleness is computed from
// the data, never assumed from a TTL.

func TestBrief_IsStaleWhenTheThreadMoved(t *testing.T) {
	at := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	b := &ConversationBrief{CoversUntil: at}

	assert.True(t, b.IsStale(at.Add(time.Minute)), "one new message makes it stale")
	assert.False(t, b.IsStale(at), "nothing new, still correct")
}

// A quiet thread's brief is correct a month later. Expiring it on a clock
// would pay for a model call to rewrite the same sentence.
func TestBrief_QuietThreadStaysFresh(t *testing.T) {
	at := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	b := &ConversationBrief{CoversUntil: at, GeneratedAt: at}

	assert.False(t, b.IsStale(at))
}

// No brief is always stale — that is what makes the first read generate one.
func TestBrief_AbsentIsStale(t *testing.T) {
	var b *ConversationBrief

	assert.True(t, b.IsStale(time.Now()))
}
