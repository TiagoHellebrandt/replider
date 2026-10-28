package usecases

import "testing"

// The model answers structured JSON; the parser labels the three fields and
// renders the flowing form. Anything else degrades to plain text.
func TestParseBriefJSON(t *testing.T) {
	agreed, pending, next, ok := parseBriefJSON(
		"```json\n{\"acordado\": \"foto enviada\", \"pendente\": \"orçamento\", \"proximo_passo\": \"seu: responder\"}\n```")
	if !ok {
		t.Fatal("fenced JSON must parse")
	}
	if agreed != "foto enviada" || pending != "orçamento" || next != "seu: responder" {
		t.Fatalf("wrong fields: %q %q %q", agreed, pending, next)
	}

	if _, _, _, ok := parseBriefJSON("um resumo em texto corrido, sem JSON"); ok {
		t.Fatal("plain prose must NOT parse as structured")
	}
	if _, _, _, ok := parseBriefJSON(`{"acordado": "", "pendente": "", "proximo_passo": ""}`); ok {
		t.Fatal("three empty fields is a non-answer")
	}

	got := briefFlowingText("a", "", "c")
	want := "Acordado: a\nPróximo passo: c"
	if got != want {
		t.Fatalf("flowing text: got %q want %q", got, want)
	}
}
