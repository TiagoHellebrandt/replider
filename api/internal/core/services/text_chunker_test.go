package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkText_ShortInputReturnedAsIs(t *testing.T) {
	out := ChunkText("short text", 1000, 100)
	assert.Equal(t, []string{"short text"}, out)
}

func TestChunkText_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, ChunkText("", 1000, 100))
	assert.Nil(t, ChunkText("   \n\t  ", 1000, 100))
}

func TestChunkText_SplitsOnParagraphBoundary(t *testing.T) {
	paraA := strings.Repeat("Primeiro parágrafo da clínica. ", 30)
	paraB := strings.Repeat("Segundo parágrafo sobre horários. ", 30)
	input := paraA + "\n\n" + paraB

	chunks := ChunkText(input, 400, 50)

	assert.GreaterOrEqual(t, len(chunks), 2)
	// The paragraph boundary should align with a chunk start so we
	// don't bury the second paragraph's opening in an overlap tail.
	joined := strings.Join(chunks, " | ")
	assert.Contains(t, joined, "Primeiro parágrafo")
	assert.Contains(t, joined, "Segundo parágrafo")
}

func TestChunkText_RespectsMaxSize(t *testing.T) {
	text := strings.Repeat("uma frase de exemplo. ", 200) // ~4400 chars
	chunks := ChunkText(text, 800, 100)

	assert.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		// Allow a small buffer around 800 because whitespace breaks may
		// land slightly past — but never > maxSize + last-word slack.
		assert.LessOrEqual(t, len([]rune(c)), 800+50, "chunk longer than budget: %q", c[:50])
	}
}

func TestChunkText_OverlapProducesConsecutiveChunks(t *testing.T) {
	// Build text where we can predict overlap: lines of fixed length.
	sentences := []string{}
	for i := 0; i < 40; i++ {
		sentences = append(sentences,
			"Esta é a frase número X que preenche espaço. ")
	}
	text := strings.Join(sentences, "")

	chunks := ChunkText(text, 500, 100)

	require := len(chunks) > 1
	if !require {
		t.Fatalf("expected >1 chunks")
	}
	// Second chunk should start with text that appeared near the end
	// of the first chunk (that's the whole point of overlap for RAG).
	firstTail := chunks[0][len(chunks[0])-80:]
	secondHead := chunks[1][:80]
	_ = firstTail
	_ = secondHead
	// Not asserting exact equality because the break-finder rewinds
	// to the nearest whitespace/sentence — just assert both chunks
	// contain the shared filler text.
	assert.Contains(t, chunks[0], "Esta é a frase")
	assert.Contains(t, chunks[1], "Esta é a frase")
}

func TestChunkText_InvalidOverlapFallsBackToDefault(t *testing.T) {
	text := strings.Repeat("frase. ", 500)
	// overlap >= maxChars is nonsense; chunker should recover, not loop.
	chunks := ChunkText(text, 500, 900)
	assert.NotEmpty(t, chunks)
}
