package domain

// VerticalDefaults captures the per-vertical hints the synthesizer (and
// the wizard UI) fall back on when the user leaves a v2 field blank.
// Treat the values as soft defaults, not invariants — the user can
// always override them in the wizard. Keep entries terse: this is not
// the place for full sentences, only seed values the LLM can stitch
// into the final body.
type VerticalDefaults struct {
	// BusinessHours is a typical operating window for the vertical in
	// the BR market. Empty fields mean "no opinion" — the LLM is told
	// to ask the operator if the field stays empty in the final body.
	BusinessHours BusinessHours

	// ScopeOutBullets are topics the synthesizer should treat as MUST
	// NOT ANSWER unless the operator explicitly opts in. Vertical-
	// specific safety rails (clinic → diagnostics; legal → counsel).
	ScopeOutBullets []string

	// ComplianceFlags pre-seeds the regulated-industry posture so the
	// synthesizer adds the right disclaimers automatically.
	ComplianceFlags []string

	// CommonScenarios are 5 archetypal questions the test-scenarios run
	// will exercise the prompt against. Phrased as user messages.
	CommonScenarios []string

	// DataCollection is the typical PII set this vertical's businesses
	// ask for. Pre-checks the boxes; user can untick.
	DataCollection []string

	// EmojiUsage is the default tone-emoji posture (clinic/legal lean
	// none; ecommerce/restaurant lean contextual).
	EmojiUsage string

	// PrimaryGoal is the most common service objective for the vertical.
	PrimaryGoal string
}

// VerticalDefaultsByID returns the canonical map of seed values per
// vertical id (matches the wizard's Step 0 enum). Keep the map keys in
// sync with `app/lib/features/prompt_builder/presentation/widgets/step_vertical.dart`.
func VerticalDefaultsByID() map[string]VerticalDefaults {
	return map[string]VerticalDefaults{
		"clinic": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "08:00", WeekdaysTo: "18:00",
				SaturdayFrom: "08:00", SaturdayTo: "12:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"diagnose conditions",
				"prescribe medication or change dosages",
				"interpret medical exam results",
				"give clinical advice the practitioner hasn't approved",
			},
			ComplianceFlags: []string{"medical_no_diagnosis"},
			CommonScenarios: []string{
				"Qual remédio devo tomar pra dor de cabeça?",
				"Qual o preço da consulta?",
				"Estou com sintoma X, é grave?",
				"Qual o horário de atendimento?",
				"Quero remarcar minha consulta amanhã",
			},
			DataCollection: []string{"name", "phone"},
			EmojiUsage:     "none",
			PrimaryGoal:    "book_meeting",
		},
		"realestate": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "19:00",
				SaturdayFrom: "09:00", SaturdayTo: "15:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"commit to a final price",
				"sign or e-sign contracts",
				"give legal advice on the deed or paperwork",
			},
			CommonScenarios: []string{
				"Quanto custa o imóvel da rua X?",
				"Aceita financiamento pelo banco Y?",
				"Posso visitar amanhã?",
				"Qual o valor mínimo que vocês negociam?",
				"Vocês trabalham com aluguel ou só venda?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "sparse",
			PrimaryGoal:    "qualify_lead",
		},
		"ecommerce": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "00:00", WeekdaysTo: "23:59",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"refund without checking the order status first",
				"promise delivery dates the carrier hasn't confirmed",
			},
			CommonScenarios: []string{
				"Onde está meu pedido?",
				"Posso trocar a cor/tamanho?",
				"Quanto é o frete pro CEP X?",
				"Vocês parcelam em quantas vezes?",
				"Como faço pra cancelar?",
			},
			DataCollection: []string{"name", "email"},
			EmojiUsage:     "contextual",
			PrimaryGoal:    "support",
		},
		"restaurant": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "11:30", WeekdaysTo: "23:00",
				SaturdayFrom: "11:30", SaturdayTo: "23:30",
				SundayFrom: "11:30", SundayTo: "22:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"guarantee an exact delivery time when the kitchen is busy",
			},
			CommonScenarios: []string{
				"Vocês entregam aqui no bairro X?",
				"Qual o cardápio do dia?",
				"Tem opção vegetariana / sem glúten?",
				"Aceita reserva pra hoje à noite?",
				"Quanto é o couvert?",
			},
			DataCollection: []string{"name", "phone", "address"},
			EmojiUsage:     "contextual",
			PrimaryGoal:    "close_sale",
		},
		"beauty": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "20:00",
				SaturdayFrom: "09:00", SaturdayTo: "18:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"recommend dermatological treatments without a professional consult",
			},
			CommonScenarios: []string{
				"Tem horário pra hoje?",
				"Quanto custa a escova progressiva?",
				"Vocês fazem unha em gel?",
				"Aceita pix?",
				"Quero remarcar pra outro dia",
			},
			DataCollection: []string{"name", "phone"},
			EmojiUsage:     "contextual",
			PrimaryGoal:    "book_meeting",
		},
		"fitness": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "06:00", WeekdaysTo: "22:00",
				SaturdayFrom: "08:00", SaturdayTo: "14:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"prescribe a personalized training routine without an in-person assessment",
				"give nutritional advice without a registered nutritionist",
			},
			ComplianceFlags: []string{"medical_no_diagnosis"},
			CommonScenarios: []string{
				"Quanto custa a mensalidade?",
				"Tem aula experimental?",
				"Quais horários de aula coletiva?",
				"Tem personal trainer incluso?",
				"Como faço pra cancelar a matrícula?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "sparse",
			PrimaryGoal:    "qualify_lead",
		},
		"pet": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "08:00", WeekdaysTo: "19:00",
				SaturdayFrom: "08:00", SaturdayTo: "16:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"diagnose pet illnesses",
				"prescribe medication without veterinary consult",
			},
			ComplianceFlags: []string{"medical_no_diagnosis"},
			CommonScenarios: []string{
				"Quanto custa o banho e tosa?",
				"Tem horário pra hoje?",
				"Vocês atendem em domicílio?",
				"Aceitam plano pet?",
				"Tem ração marca X?",
			},
			DataCollection: []string{"name", "phone"},
			EmojiUsage:     "contextual",
			PrimaryGoal:    "book_meeting",
		},
		"hotel": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "00:00", WeekdaysTo: "23:59",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Tem disponibilidade no fim de semana X?",
				"Qual o valor da diária pra 2 pessoas?",
				"Aceita pet?",
				"Tem café da manhã incluso?",
				"Como funciona o cancelamento?",
			},
			DataCollection: []string{"name", "email", "phone"},
			EmojiUsage:     "contextual",
			PrimaryGoal:    "close_sale",
		},
		"automotive": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "08:00", WeekdaysTo: "18:00",
				SaturdayFrom: "08:00", SaturdayTo: "12:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"give a final repair quote without inspecting the vehicle",
			},
			CommonScenarios: []string{
				"Quanto custa a revisão dos 20 mil km?",
				"Tem horário pra hoje?",
				"Vocês fazem alinhamento e balanceamento?",
				"Aceita peça que eu compre por fora?",
				"Em quanto tempo fica pronto?",
			},
			DataCollection: []string{"name", "phone"},
			EmojiUsage:     "none",
			PrimaryGoal:    "book_meeting",
		},
		"education": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "08:00", WeekdaysTo: "20:00",
				SaturdayFrom: "08:00", SaturdayTo: "12:00",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Quanto custa a mensalidade?",
				"Tem desconto pra primeiro ano?",
				"Quais turnos vocês têm?",
				"Como funciona a matrícula?",
				"Vocês têm bolsa de estudo?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "sparse",
			PrimaryGoal:    "qualify_lead",
		},
		"legal": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "18:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"give legal counsel or interpret laws for the client's case",
				"predict outcomes of a lawsuit",
				"draft contracts the lawyer hasn't reviewed",
			},
			ComplianceFlags: []string{"legal_no_consult"},
			CommonScenarios: []string{
				"Tenho um problema com o INSS, vocês resolvem?",
				"Quanto custa uma consulta?",
				"Vocês trabalham com direito trabalhista?",
				"Fui demitido sem justa causa, posso processar?",
				"Como agendo uma consulta?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "none",
			PrimaryGoal:    "book_meeting",
		},
		"events": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "19:00",
				SaturdayFrom: "09:00", SaturdayTo: "14:00",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Quanto custa o pacote pra 100 convidados?",
				"Tem disponibilidade no dia X?",
				"O que está incluso no pacote?",
				"Como funciona o sinal e o pagamento?",
				"Vocês fazem casamento ao ar livre?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "contextual",
			PrimaryGoal:    "qualify_lead",
		},
		"agency": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "18:00",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Quanto custa pra rodar tráfego pago?",
				"Vocês fazem gestão de redes sociais?",
				"Em quanto tempo vejo resultado?",
				"Como funciona o contrato?",
				"Vocês trabalham com B2B?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "sparse",
			PrimaryGoal:    "qualify_lead",
		},
		"finance": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "18:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"recommend specific investments or guarantee returns",
				"give tax advice without a registered accountant",
			},
			ComplianceFlags: []string{"financial_no_advice"},
			CommonScenarios: []string{
				"Vocês fazem declaração de imposto de renda?",
				"Como funciona a abertura de empresa MEI?",
				"Quanto custa a contabilidade mensal?",
				"Vocês atendem prestador de serviço autônomo?",
				"Posso agendar uma reunião?",
			},
			DataCollection: []string{"name", "phone", "email"},
			EmojiUsage:     "none",
			PrimaryGoal:    "qualify_lead",
		},
		"tech": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "18:00",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Como funciona o trial?",
				"Quanto custa o plano X?",
				"Vocês integram com Y?",
				"Tem suporte 24/7?",
				"Posso falar com um atendente humano?",
			},
			DataCollection: []string{"name", "email"},
			EmojiUsage:     "sparse",
			PrimaryGoal:    "qualify_lead",
		},
		"logistics": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "08:00", WeekdaysTo: "18:00",
				SaturdayFrom: "08:00", SaturdayTo: "12:00",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Quanto custa um frete pra cidade X?",
				"Em quanto tempo entrega?",
				"Vocês fazem coleta?",
				"Como rastreio meu envio?",
				"Aceitam carga frágil?",
			},
			DataCollection: []string{"name", "phone", "address"},
			EmojiUsage:     "none",
			PrimaryGoal:    "qualify_lead",
		},
		"construction": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "08:00", WeekdaysTo: "17:00",
				SaturdayFrom: "08:00", SaturdayTo: "12:00",
				Timezone: "America/Sao_Paulo",
			},
			ScopeOutBullets: []string{
				"give a final price without site inspection",
			},
			CommonScenarios: []string{
				"Quanto custa pintar uma casa de 100m²?",
				"Em quanto tempo fica pronto?",
				"Vocês fornecem o material?",
				"Atendem aqui na cidade X?",
				"Como funciona o orçamento?",
			},
			DataCollection: []string{"name", "phone", "address"},
			EmojiUsage:     "none",
			PrimaryGoal:    "qualify_lead",
		},
		"services": {
			BusinessHours: BusinessHours{
				WeekdaysFrom: "09:00", WeekdaysTo: "18:00",
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Quais serviços vocês oferecem?",
				"Quanto custa?",
				"Tem horário disponível?",
				"Como funciona o pagamento?",
				"Vocês atendem aqui na minha região?",
			},
			DataCollection: []string{"name", "phone"},
			EmojiUsage:     "sparse",
			PrimaryGoal:    "qualify_lead",
		},
		"other": {
			BusinessHours: BusinessHours{
				Timezone: "America/Sao_Paulo",
			},
			CommonScenarios: []string{
				"Quais serviços vocês oferecem?",
				"Quanto custa?",
				"Como faço pra contratar?",
				"Aceitam pix / cartão?",
				"Tem atendente humano?",
			},
			EmojiUsage:  "sparse",
			PrimaryGoal: "support",
		},
	}
}

// DefaultsFor returns the seed values for `verticalID` or a zero-value
// (safe to read) when the id isn't recognized — keeps the synthesizer
// from crashing on a typo.
func DefaultsFor(verticalID string) VerticalDefaults {
	if d, ok := VerticalDefaultsByID()[verticalID]; ok {
		return d
	}
	return VerticalDefaults{}
}
