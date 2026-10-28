package services

import (
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// Humanization tunables. Values chosen to hit the sweet spot where the bot
// feels like a real attendant on mobile: not so slow that users give up, not
// so fast that the response looks automated. Based on typical WhatsApp
// conversation cadence (humans take 5-15s even for short replies).
const (
	// First-char delay: before ANY typing indicator starts. Emulates the
	// "someone opened the chat and is reading" phase.
	humanReadDelayMin = 1500 * time.Millisecond
	humanReadDelayMax = 3500 * time.Millisecond

	// Per-char typing delay. 35ms ≈ 50 WPM, slightly above casual mobile
	// typing to avoid making multi-sentence replies painfully slow.
	humanTypingMsPerChar = 35

	// Hard cap so very long replies don't hold the customer for a full
	// minute. If the AI returns 2000 chars, we still ship in under 15s.
	humanTypingCeiling = 12 * time.Second

	// Inter-message pause when we split a long reply into several shorter
	// messages — humans send a burst of 2-3 lines with short beats between.
	humanBetweenMsgMin = 700 * time.Millisecond
	humanBetweenMsgMax = 1800 * time.Millisecond

	// Split thresholds. We only split responses longer than this; keeping
	// short answers as one bubble matches how humans actually write.
	humanSplitMinLen = 160
	// Prefer not to split pieces below this, to avoid 1-word bubbles.
	humanSplitTargetMin = 40
)

// ReadDelay returns a randomized "time to read the customer's message"
// pause. Call this before showing the typing indicator.
func ReadDelay() time.Duration {
	return randomDuration(humanReadDelayMin, humanReadDelayMax)
}

// TypingDelay returns how long the typing indicator should stay visible
// before the given message content lands. Proportional to length, capped.
func TypingDelay(content string) time.Duration {
	chars := len([]rune(content))
	d := time.Duration(chars*humanTypingMsPerChar) * time.Millisecond
	if d > humanTypingCeiling {
		d = humanTypingCeiling
	}
	// Minimum 800ms so even a one-word reply has a visible "typing" beat.
	if d < 800*time.Millisecond {
		d = 800 * time.Millisecond
	}
	// ±20% jitter so two replies in a row don't feel mechanical.
	jitter := time.Duration(rand.Int63n(int64(d) / 5))
	return d - d/10 + jitter
}

// BetweenMessagesDelay returns the pause between two message bubbles of a
// split reply. Small because the user is already "in" the conversation.
func BetweenMessagesDelay() time.Duration {
	return randomDuration(humanBetweenMsgMin, humanBetweenMsgMax)
}

// SplitForHuman breaks a long AI reply into bubble-sized chunks matching
// how humans write on WhatsApp: multiple short lines instead of a wall of
// text. Returns the input unchanged when it's already short enough.
//
// Strategy: prefer paragraph breaks (blank lines), then sentence
// boundaries, never mid-word. We aim for 2-4 chunks max — more than that
// and the user starts getting annoyed by notifications.
func SplitForHuman(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len([]rune(text)) <= humanSplitMinLen {
		return []string{text}
	}

	// Paragraph split first (most signal about intent from the model).
	paragraphs := splitParagraphs(text)
	if len(paragraphs) >= 2 {
		return coalesceChunks(paragraphs)
	}

	// Fall back to sentence splitting.
	sentences := splitSentences(text)
	if len(sentences) >= 2 {
		return coalesceChunks(sentences)
	}

	// Single long sentence with no paragraph/sentence cues — ship as one
	// bubble rather than chopping mid-clause. Better a long bubble than
	// a nonsensical burst.
	return []string{text}
}

func splitParagraphs(text string) []string {
	parts := regexp.MustCompile(`\n\s*\n+`).Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// splitSentences breaks on ". ", "! ", "? " while keeping the punctuation
// attached to the left side. URLs and abbreviations (Sr., Dr.) aren't
// perfectly handled — acceptable trade-off for a messaging UX where a
// slightly wrong split is invisible.
func splitSentences(text string) []string {
	re := regexp.MustCompile(`([.!?])\s+`)
	idx := re.FindAllStringIndex(text, -1)
	if len(idx) == 0 {
		return []string{text}
	}
	var out []string
	start := 0
	for _, m := range idx {
		end := m[0] + 1 // include punctuation
		s := strings.TrimSpace(text[start:end])
		if s != "" {
			out = append(out, s)
		}
		start = m[1]
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		out = append(out, tail)
	}
	return out
}

// coalesceChunks merges adjacent pieces while staying under a soft ceiling,
// and also avoids emitting tiny fragments. End result: 2-4 balanced bubbles
// in the 60-200 char range, which reads most naturally.
func coalesceChunks(parts []string) []string {
	const softMax = 220
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	for _, p := range parts {
		if cur.Len() == 0 {
			cur.WriteString(p)
			continue
		}
		// Merge into current chunk if the result stays under softMax OR if
		// the incoming piece is below humanSplitTargetMin (avoid tiny tails).
		combinedLen := cur.Len() + 1 + len(p)
		if combinedLen <= softMax || len([]rune(p)) < humanSplitTargetMin {
			cur.WriteString(" ")
			cur.WriteString(p)
			continue
		}
		flush()
		cur.WriteString(p)
	}
	flush()
	// Cap to 4 bubbles: everything left goes into the last one.
	const maxBubbles = 4
	if len(out) > maxBubbles {
		tail := strings.Join(out[maxBubbles-1:], " ")
		out = append(out[:maxBubbles-1], strings.TrimSpace(tail))
	}
	return out
}

func randomDuration(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	return min + time.Duration(rand.Int63n(int64(max-min)))
}
