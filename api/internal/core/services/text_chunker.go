package services

import (
	"strings"
	"unicode"
)

// DefaultChunkSize and DefaultChunkOverlap are the character budgets we
// use by default for knowledge ingestion. Numbers chosen after looking
// at OpenAI's `text-embedding-3-small` input ceiling (~8k tokens) and
// leaving generous headroom: 1500 chars ≈ 350-400 tokens, so a single
// chunk fits comfortably and the overlap carries enough context across
// boundaries to keep retrieval stable when a query straddles two
// paragraphs.
const (
	DefaultChunkSize    = 1500
	DefaultChunkOverlap = 200
)

// ChunkText splits a long input into overlapping segments no larger than
// maxChars, preferring natural breaks (paragraph → sentence → whitespace)
// over hard cuts mid-word. Callers with knowledge that the input is
// short can still use it — a text below maxChars is returned as-is in a
// single-element slice.
//
// Returns a nil slice when text is empty or contains only whitespace.
// Caller is responsible for embedding each chunk — this is pure CPU.
func ChunkText(text string, maxChars, overlap int) []string {
	if maxChars <= 0 {
		maxChars = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= maxChars {
		// Nonsensical config — collapse to the default overlap so we
		// don't loop forever advancing by zero characters.
		overlap = DefaultChunkOverlap
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	runes := []rune(trimmed)
	if len(runes) <= maxChars {
		return []string{trimmed}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + maxChars
		if end >= len(runes) {
			chunks = append(chunks, strings.TrimSpace(string(runes[start:])))
			break
		}
		// Walk backward from end looking for a good break. Priority:
		//   1. Double newline (paragraph)
		//   2. Sentence terminator (.!?) followed by whitespace
		//   3. Any whitespace
		// We only rewind up to maxChars/4 characters — further than
		// that and the chunks get pathologically small.
		breakAt := findBreakPoint(runes, start, end, maxChars/4)
		if breakAt <= start {
			breakAt = end
		}
		chunks = append(chunks, strings.TrimSpace(string(runes[start:breakAt])))
		next := breakAt - overlap
		if next <= start {
			// Overlap would stall progress (tiny chunk). Advance to the
			// break point regardless.
			next = breakAt
		}
		start = next
	}

	// Drop any empty chunks left by aggressive trimming.
	out := chunks[:0]
	for _, c := range chunks {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func findBreakPoint(runes []rune, start, end, maxRewind int) int {
	minAccept := end - maxRewind
	if minAccept < start+1 {
		minAccept = start + 1
	}
	// 1. Paragraph break.
	for i := end - 1; i >= minAccept; i-- {
		if i > 0 && runes[i] == '\n' && runes[i-1] == '\n' {
			return i + 1
		}
	}
	// 2. Sentence terminator followed by whitespace.
	for i := end - 1; i >= minAccept; i-- {
		if isSentenceTerminator(runes[i]) && i+1 < len(runes) &&
			unicode.IsSpace(runes[i+1]) {
			return i + 1
		}
	}
	// 3. Any whitespace.
	for i := end - 1; i >= minAccept; i-- {
		if unicode.IsSpace(runes[i]) {
			return i + 1
		}
	}
	return end
}

func isSentenceTerminator(r rune) bool {
	return r == '.' || r == '!' || r == '?'
}
