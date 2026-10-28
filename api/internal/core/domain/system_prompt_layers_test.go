package domain

import (
	"strings"
	"testing"
)

// Prompt layering (K3).
//
// Providers cache by exact prefix, so the property worth pinning is that the
// static half is byte-stable no matter what the variable half does.

func TestSystemPromptLayers_StaticComesFirst(t *testing.T) {
	var l SystemPromptLayers
	l.AddStatic("PERSONA", "you are helpful")
	l.AddVariable("REPLY LANGUAGE: pt", "reply in Portuguese")
	l.AddStatic("COMPANY", "Clínica Bem Viver")

	rendered := l.Render()

	// Both static sections, in the order they were added, before anything
	// variable. A static block after a variable one is billed at full rate on
	// every single turn.
	personaIdx := strings.Index(rendered, "you are helpful")
	companyIdx := strings.Index(rendered, "Clínica Bem Viver")
	langIdx := strings.Index(rendered, "REPLY LANGUAGE")
	if !(personaIdx < companyIdx && companyIdx < langIdx) {
		t.Fatalf("order = persona %d, company %d, language %d", personaIdx, companyIdx, langIdx)
	}
}

func TestSystemPromptLayers_PrefixDoesNotMoveWhenTheVariableHalfChanges(t *testing.T) {
	build := func(lang, knowledge string) SystemPromptLayers {
		var l SystemPromptLayers
		l.AddStatic("PERSONA", "you are helpful")
		l.AddStatic("COMPANY", "Clínica Bem Viver")
		l.AddVariable("REPLY LANGUAGE: "+lang, "follow the customer")
		l.AddVariable("KNOWLEDGE", knowledge)
		return l
	}

	first := build("pt", "tabela de preços")
	second := build("en", "price list")

	// One differing byte in the prefix and the provider bills all of it.
	if first.StaticPrefix() != second.StaticPrefix() {
		t.Fatalf("prefix drifted:\n%q\nvs\n%q", first.StaticPrefix(), second.StaticPrefix())
	}
	// And the prefix really is a prefix of the whole thing.
	if !strings.HasPrefix(first.Render(), first.StaticPrefix()) {
		t.Fatal("StaticPrefix must be a literal prefix of Render")
	}
}

func TestSystemPromptLayers_AnEmptySectionChangesNothing(t *testing.T) {
	var withEmpty SystemPromptLayers
	withEmpty.AddStatic("PERSONA", "you are helpful")
	// An optional field nobody filled in.
	withEmpty.AddStatic("PERSONALITY", "   ")
	withEmpty.AddStatic("COMPANY", "Clínica Bem Viver")

	var without SystemPromptLayers
	without.AddStatic("PERSONA", "you are helpful")
	without.AddStatic("COMPANY", "Clínica Bem Viver")

	// Rendering an empty titled section would put a different number of newlines
	// in the prefix depending on whether an optional field happens to be set —
	// and a whitespace difference is a cache miss like any other.
	if withEmpty.StaticPrefix() != without.StaticPrefix() {
		t.Fatalf("an empty section changed the prefix:\n%q\nvs\n%q",
			withEmpty.StaticPrefix(), without.StaticPrefix())
	}
}

func TestSystemPromptLayers_ATitlelessSectionRendersItsBodyAlone(t *testing.T) {
	var l SystemPromptLayers
	l.AddStatic("", "raw persona text")

	if got := l.Render(); got != "raw persona text" {
		t.Fatalf("rendered = %q, want the body alone", got)
	}
}

func TestSystemPromptLayers_NoStraySeparators(t *testing.T) {
	var onlyStatic SystemPromptLayers
	onlyStatic.AddStatic("", "persona")
	if got := onlyStatic.Render(); got != "persona" {
		t.Fatalf("static-only rendered = %q, want no trailing separator", got)
	}

	var onlyVariable SystemPromptLayers
	onlyVariable.AddVariable("", "context")
	if got := onlyVariable.Render(); got != "context" {
		t.Fatalf("variable-only rendered = %q, want no leading separator", got)
	}

	var empty SystemPromptLayers
	if got := empty.Render(); got != "" {
		t.Fatalf("empty rendered = %q", got)
	}
}

func TestSystemPromptLayers_SectionsAreSeparatedExactlyOnce(t *testing.T) {
	var l SystemPromptLayers
	l.AddStatic("", "a")
	l.AddStatic("", "b")
	l.AddVariable("", "c")

	// Two static plus one variable = three sections, two separators. A block that
	// carries its own separator would double one of them and shift every byte
	// after it.
	if got := strings.Count(l.Render(), promptSectionSeparator); got != 2 {
		t.Fatalf("separators = %d, want 2 in %q", got, l.Render())
	}
}
