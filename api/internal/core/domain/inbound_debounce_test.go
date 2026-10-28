package domain

import (
	"testing"
	"time"
)

// Inbound debounce (K1).
//
// The delay decision is the feature. What these pin is the asymmetry: waiting on
// someone ready to buy loses the sale, so buy intent never waits; answering a
// bare "oi" costs a call and a second reply, so small talk waits longer.

func seconds(n int) *int { return &n }

func TestDebounceDelay_BuyIntentNeverWaits(t *testing.T) {
	cfg := DebounceConfig{Seconds: seconds(60)}

	for _, text := range []string{
		"quanto custa o comodato?",
		"Qual o preço",
		"quero fechar hoje",
		"me manda o orçamento por favor",
		"tem disponível para entrega essa semana?",
		"como pago?",
	} {
		if got := DebounceDelay(text, cfg); got != 0 {
			t.Fatalf("%q waited %v — every second of silence on a buyer is a "+
				"chance for them to close the tab", text, got)
		}
	}
}

func TestDebounceDelay_BuyIntentSurvivesHowPeopleActuallyType(t *testing.T) {
	cfg := DebounceConfig{Seconds: seconds(60)}

	// Accents, caps, punctuation, emoji: all of it is how a real message
	// arrives, and none of it may hide the signal.
	for _, text := range []string{
		"QUANTO CUSTA???",
		"quanto  custa",
		"Quanto custa!! 😅",
		"me manda o orçamento",
	} {
		if got := DebounceDelay(text, cfg); got != 0 {
			t.Fatalf("%q waited %v, want immediate", text, got)
		}
	}
}

func TestDebounceDelay_SmallTalkWaitsLonger(t *testing.T) {
	cfg := DebounceConfig{Seconds: seconds(40)}

	// A bare greeting is almost always followed by the real message. Answering
	// it costs a call and then has to answer again.
	got := DebounceDelay("bom dia", cfg)
	want := 40 * DebounceIdleMultiplier * time.Second
	if got != want {
		t.Fatalf("small talk waited %v, want %v", got, want)
	}
}

func TestDebounceDelay_AQuestionIsNeverSmallTalk(t *testing.T) {
	cfg := DebounceConfig{Seconds: seconds(40)}

	// "bom dia" waits long; "bom dia, vocês entregam?" is a question and gets
	// the normal window. Collapsing the two would make somebody who asked
	// something wait almost three minutes.
	if got := DebounceDelay("bom dia, vocês entregam em Cuiabá?", cfg); got != 40*time.Second {
		t.Fatalf("delay = %v, want the normal 40s window", got)
	}
}

func TestDebounceDelay_ARealMessageGetsTheConfiguredWindow(t *testing.T) {
	if got := DebounceDelay("preciso de 200 unidades para o hospital", DebounceConfig{Seconds: seconds(30)}); got != 30*time.Second {
		t.Fatalf("delay = %v, want 30s", got)
	}
}

func TestDebounceConfig_ZeroIsExplicitlyOffAndNilIsTheDefault(t *testing.T) {
	// The distinction is the whole reason the field is a pointer: an account
	// that wants instant replies must be able to say so without the column
	// reading as unset.
	off := DebounceConfig{Seconds: seconds(0)}
	if off.Enabled() {
		t.Fatal("an explicit 0 must disable the debounce")
	}
	if got := DebounceDelay("bom dia", off); got != 0 {
		t.Fatalf("delay = %v with the debounce off, want 0", got)
	}

	unset := DebounceConfig{}
	if !unset.Enabled() {
		t.Fatal("nil must mean the platform default, not off")
	}
	if got := DebounceDelay("preciso de um orçamento formal", unset); got != DebounceDefaultSeconds*time.Second {
		t.Fatalf("delay = %v, want the %ds default", got, DebounceDefaultSeconds)
	}
}

func TestDebounceConfig_ClampsAnAbsurdWindow(t *testing.T) {
	// A misconfigured account must not be able to park its customers for an
	// hour. Negative is treated as off rather than as a time machine.
	if got := DebounceDelay("uma pergunta qualquer", DebounceConfig{Seconds: seconds(99999)}); got != DebounceMaxSeconds*time.Second {
		t.Fatalf("delay = %v, want the %ds ceiling", got, DebounceMaxSeconds)
	}
	if got := DebounceDelay("uma pergunta qualquer", DebounceConfig{Seconds: seconds(-5)}); got != 0 {
		t.Fatalf("delay = %v, want 0 for a negative window", got)
	}
}

func TestDebounceDelay_SmallTalkNeverExceedsTheCeiling(t *testing.T) {
	// 300s × 4 would be twenty minutes.
	if got := DebounceDelay("obrigado", DebounceConfig{Seconds: seconds(DebounceMaxSeconds)}); got != DebounceMaxSeconds*time.Second {
		t.Fatalf("delay = %v, want the ceiling", got)
	}
}

func TestDebounceDelay_AnEmptyMessageIsNotSmallTalk(t *testing.T) {
	// An empty or media-only message (no text) is not a greeting: it gets the
	// normal window rather than the long one, because there is no evidence the
	// customer is still typing.
	if got := DebounceDelay("", DebounceConfig{Seconds: seconds(40)}); got != 40*time.Second {
		t.Fatalf("delay = %v, want the normal window", got)
	}
}

func TestPendingInbound_ClampsTheWaitToTheBurstCeiling(t *testing.T) {
	first := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	p := PendingInbound{FirstQueuedAt: first}

	// A customer typing continuously would push the answer out forever.
	proposed := first.Add(10 * time.Minute)
	got := p.ClampRespondAt(proposed)

	if !got.Equal(first.Add(DebounceMaxTotalWait)) {
		t.Fatalf("clamped to %v, want the %v ceiling", got, DebounceMaxTotalWait)
	}
	// A wait inside the ceiling passes through untouched.
	inside := first.Add(30 * time.Second)
	if got := p.ClampRespondAt(inside); !got.Equal(inside) {
		t.Fatalf("clamped %v unnecessarily to %v", inside, got)
	}
}

func TestFoldAccents_CoversWhatBrazilianCustomersType(t *testing.T) {
	if got := FoldAccents("Ação Não Está Válido Ótimo Última"); got != "ACAO NAO ESTA VALIDO OTIMO ULTIMA" {
		t.Fatalf("folded = %q", got)
	}
}
