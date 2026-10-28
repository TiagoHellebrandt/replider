package services

import "testing"

func TestDetectReplyLanguage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"pt sentence", "Oi, qual é o horário de atendimento hoje?", "pt"},
		{"en sentence", "Hi, what are your opening hours today?", "en"},
		{"es sentence", "Hola, cuál es el horario de atención hoy?", "es"},
		{"too short pt", "oi", "pt"},
		{"empty", "", "pt"},
		{"emoji only", "🤔", "pt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectReplyLanguage(c.in)
			if got != c.want {
				t.Fatalf("DetectReplyLanguage(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
