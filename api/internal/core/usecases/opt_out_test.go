package usecases

import "testing"

func TestIsOptOutMessage(t *testing.T) {
	positive := []string{
		"PARAR",
		"parar",
		"Parar",
		" parar ",
		"PARAR.",
		"SAIR",
		"saír", // accent-folded
		"stop",
		"STOP!",
		"unsubscribe",
		"UNSUBSCRIBE?",
		"cancelar",
		"SAI",
		"descadastrar",
		"DESCADASTRAR",
		"Descadastrar.",
	}
	for _, tc := range positive {
		if !IsOptOutMessage(tc) {
			t.Errorf("expected %q to be opt-out", tc)
		}
	}

	negative := []string{
		"",
		"   ",
		"parar de receber mensagens às 18h?", // substring — NOT an opt-out
		"não quero parar",                    // conversational
		"oi",
		"boa tarde, preciso cancelar meu pedido", // conversational
		"stop doing this please",
		"cancelar pedido #123",
		"parabéns",
		"saint",
	}
	for _, tc := range negative {
		if IsOptOutMessage(tc) {
			t.Errorf("expected %q NOT to be opt-out", tc)
		}
	}
}
