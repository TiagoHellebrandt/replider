package domain

import "strings"

// Prompt layering for provider-side caching (§6.16, K3).
//
// Both providers cache a prompt by matching an EXACT PREFIX of the tokens they
// already saw. Nothing else about the request matters: the moment one byte
// differs, everything from that byte onward is a cache miss and is billed at full
// rate.
//
// Which makes the whole feature a question of ORDER, not of API calls. Everything
// stable — the persona, the company identity, the agent's instructions, the
// booking rules — has to come first and be byte-identical across every turn of
// every conversation for that agent. Everything that changes per message — the
// language tag, the retrieved knowledge — has to come after it.
//
// The bug this file exists to prevent was real: `REPLY LANGUAGE: pt` sat about
// thirty bytes into the prompt, ahead of a few thousand tokens of persona and
// instructions. Two conversations in different languages — or one customer
// switching mid-thread — shared almost no cacheable prefix at all. The content
// was already right; it was in the wrong order.

// The outbound composer already layers its own prompt for the same reason (see
// ComposeSystemPrompt / the brand kit's PromptLayers, C1). This file is the
// CONVERSATION side, which was never layered: an agent answering a customer
// assembled its prompt in whatever order the features were added in.

// PromptSection is one labelled block of the system prompt.
type PromptSection struct {
	// Title is the heading rendered above the body. Empty renders the body
	// alone (used by the opening persona, which is not a titled section).
	Title string
	Body  string
}

// SystemPromptLayers is a system prompt split by how often each part changes.
//
// The split is the contract. A section in the wrong layer does not break
// anything visibly — it just quietly costs money on every single turn, which is
// exactly the kind of bug nobody notices for a year.
type SystemPromptLayers struct {
	// Static changes only when the account or agent is edited. This is the
	// cacheable prefix, and it must not contain anything derived from the
	// current message, the current conversation, or the clock.
	Static []PromptSection
	// Variable changes per message. Always rendered last, so a change here
	// never invalidates the prefix above it.
	Variable []PromptSection
}

// AddStatic appends a stable section, ignoring an empty body.
//
// Ignoring empties matters more than it looks: rendering a titled section with
// nothing under it would put a different number of newlines in the prefix
// depending on whether an optional field happens to be filled in, and a
// whitespace difference is a cache miss like any other.
func (l *SystemPromptLayers) AddStatic(title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	l.Static = append(l.Static, PromptSection{Title: title, Body: body})
}

// AddVariable appends a per-message section.
func (l *SystemPromptLayers) AddVariable(title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	l.Variable = append(l.Variable, PromptSection{Title: title, Body: body})
}

// promptSectionSeparator is what goes between sections. One constant, because a
// separator that differs by a byte between two call sites is a cache miss that
// looks like nothing in a diff.
const promptSectionSeparator = "\n\n---\n\n"

// StaticPrefix renders the cacheable half.
//
// Exposed separately from Render so a test can assert that two different
// conversations of the same agent produce an identical prefix — which is the only
// way to keep this property from rotting the next time somebody appends a section
// in a hurry.
func (l SystemPromptLayers) StaticPrefix() string {
	return renderSections(l.Static)
}

// Render assembles the whole prompt: stable half, then the per-message half.
func (l SystemPromptLayers) Render() string {
	static := renderSections(l.Static)
	variable := renderSections(l.Variable)
	switch {
	case variable == "":
		return static
	case static == "":
		return variable
	default:
		return static + promptSectionSeparator + variable
	}
}

func renderSections(sections []PromptSection) string {
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString(promptSectionSeparator)
		}
		if section.Title != "" {
			b.WriteString(section.Title)
			b.WriteString("\n")
		}
		b.WriteString(section.Body)
	}
	return b.String()
}
