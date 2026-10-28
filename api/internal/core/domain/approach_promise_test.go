package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Real sentences produced by the model, collected from a pilot run. Each is
// a commitment made in the sender's name before any human read it.
func TestApproachPromises_CatchesWhatTheModelActuallyWrote(t *testing.T) {
	for _, text := range []string{
		"podemos otimizar a manutenção e garantir a continuidade de equipamentos",
		"nossos 6 técnicos garantem o atendimento corretivo rápido",
		"evitar paradas e garantir agilidade no atendimento",
		"atendimento em 24 horas para toda a região",
		"retorno em 2 horas, sempre",
		"resolvemos qualquer problema no seu equipamento",
		"Soube que vocês estão buscando monitores multiparâmetro",
		"Vi que a clínica está trocando de fornecedor",
	} {
		assert.True(t, HasApproachPromise(text), "não sinalizou: %q", text)
	}
}

// A consultative message that promises nothing must pass clean, or the
// warning becomes noise and the person approving stops reading it.
func TestApproachPromises_LeavesAnHonestMessageAlone(t *testing.T) {
	for _, text := range []string{
		"Olá Dra. Marlene, aqui é a Rita da AssistMed. Vi seu contrato vencido e queria entender como estão fazendo a manutenção hoje. Podemos conversar?",
		"Trabalho com bombas de infusão e queria saber se faz sentido para vocês.",
		"Lembro que o Centro de Imagem já foi nosso cliente. Que tal retomarmos?",
		"",
	} {
		assert.False(t, HasApproachPromise(text), "falso positivo em: %q", text)
	}
}

func TestApproachPromises_ListsEachFragmentOnce(t *testing.T) {
	got := ApproachPromises("garantimos o prazo e garantimos a qualidade")
	assert.Len(t, got, 1, "repetiu o mesmo trecho: %v", got)
}

// A pattern without a declared kind would silently inherit "promise" and send
// the reader to fix the wrong sentence: "soube que vocês" is not a promise,
// it is a claim nobody supplied.
func TestApproachFindings_KindMatchesThePattern(t *testing.T) {
	assert.Len(t, patternKinds, len(promisePatterns),
		"cada padrão precisa de um kind — senão herda 'promise' calado")

	got := ApproachFindings("garantimos o prazo e soube que vocês trocaram de fornecedor")
	kinds := map[string]string{}
	for _, f := range got {
		kinds[f.Kind] = f.Quote
	}
	assert.Contains(t, kinds, "promise")
	assert.Contains(t, kinds, "invented_fact")
	assert.Contains(t, kinds["invented_fact"], "soube que voc")
}

// A clean message produces nothing, or the warning becomes noise.
func TestApproachFindings_QuietOnAnHonestMessage(t *testing.T) {
	assert.Empty(t, ApproachFindings(
		"Olá Dra. Marlene, aqui é a Rita da AssistMed. Podemos conversar sobre a manutenção?"))
}

// ── Placeholders ──────────────────────────────────────────────────────────
//
// Unlike a promise, which the approver may legitimately have meant, an
// unfilled placeholder is never intentional. "Olá {{nome}}" is a broken
// message and it burns a lead for a reason nobody chose.

func TestPlaceholderFindings_CatchesTheCommonShapes(t *testing.T) {
	cases := []string{
		"Olá {{nome}}, tudo bem?",
		"Olá {{ nome }}, tudo bem?",
		"Olá {nome}, tudo bem?",
		"Atenciosamente, [Seu Nome]",
		"Prezado [nome do cliente]",
		"Falo com <nome>?",
		"Ligue para XXXX",
		"TODO: personalizar",
		"PREENCHER antes de enviar",
	}
	for _, in := range cases {
		assert.NotEmpty(t, PlaceholderFindings(in), "must be caught: %q", in)
	}
}

// Over-flagging would block real messages, and a block is the harshest thing
// this system does.
func TestPlaceholderFindings_LeavesOrdinaryTextAlone(t *testing.T) {
	cases := []string{
		"Bom dia, dr. Exemplo. Posso mandar duas opções de preço?",
		"O equipamento custa R$ 12.500 e temos assistência em Sorocaba.",
		"Enviei o orçamento (2 páginas) por e-mail.",
		"Trabalhamos com monitores multiparâmetro.",
	}
	for _, in := range cases {
		assert.Empty(t, PlaceholderFindings(in), "must NOT be flagged: %q", in)
	}
}

func TestPlaceholderFindings_ReportsEachOnce(t *testing.T) {
	got := PlaceholderFindings("Olá {{nome}}, tudo bem {{nome}}?")

	assert.Len(t, got, 1)
	assert.Equal(t, "placeholder", got[0].Kind)
}

// ── Attachment promised but absent ────────────────────────────────────────
//
// "Segue em anexo o termo de referência" with nothing attached gets answered
// with "não veio nada", and the thread starts on an apology.

func TestPromisesAnAttachment_CatchesTheClaim(t *testing.T) {
	cases := []string{
		"Segue em anexo o termo de referência.",
		"Seguem anexo os dois orçamentos.",
		"Envio em anexo a proposta.",
		"Anexei a tabela de preços.",
		"Estou anexando o catálogo.",
		"Please find attached our quote.",
		"See attached.",
	}
	for _, in := range cases {
		assert.True(t, PromisesAnAttachment(in), "must be caught: %q", in)
	}
}

// Sentences ABOUT an attachment that is correctly absent must not fire, or
// the warning appears on messages that are fine and stops being read.
func TestPromisesAnAttachment_IgnoresTalkAboutAttachments(t *testing.T) {
	cases := []string{
		"Aguardo o anexo que você mencionou.",
		"Não recebi nenhum anexo.",
		"Posso mandar o catálogo por WhatsApp se preferir.",
		"Bom dia, tudo bem?",
	}
	for _, in := range cases {
		assert.False(t, PromisesAnAttachment(in), "must NOT be flagged: %q", in)
	}
}
