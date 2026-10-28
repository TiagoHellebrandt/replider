package domain

// Localised labels rendered to the dashboard user when an agent
// template seeds its initial "Perguntas frequentes" knowledge source.
//
// The agent persona itself stays Portuguese — every system_prompt in
// agent_templates.go ends with "Responda sempre em português do Brasil."
// because the agent talks to Brazilian customers. What gets translated
// here is the *admin-facing* metadata: the source name in the dashboard
// and the per-chunk Title that shows up in the chunk list. Chunk
// Content remains PT.
//
// Locale codes are the same coarse pt/en split the HTTP layer uses.

// SeedSourceName returns the localised name for the knowledge source
// created by SeedFromTemplate. Defaults to PT when locale is unknown.
func SeedSourceName(locale string) string {
	switch locale {
	case "en":
		return "FAQ"
	default:
		return "Perguntas frequentes"
	}
}

// LocalizeSeedTitle returns the localised label for a seed-chunk title.
// Falls back to the original PT title when the locale is unknown or the
// title isn't in the translation table — keeps unknown templates working
// instead of crashing.
func LocalizeSeedTitle(ptTitle, locale string) string {
	if locale != "en" {
		return ptTitle
	}
	if t, ok := seedTitleEN[ptTitle]; ok {
		return t
	}
	return ptTitle
}

// seedTitleEN maps every Title used across agent_templates.go to its
// English equivalent. Keep this in sync whenever a template gains a
// new seed chunk — there's a unit test that flags missing keys.
var seedTitleEN = map[string]string{
	"Aula experimental":                 "Trial class",
	"Cancelamento de consulta":          "Appointment cancellation",
	"Cancelamento de matrícula":         "Enrolment cancellation",
	"Caução e garantias":                "Deposit and guarantees",
	"Como agendar":                      "How to book",
	"Como agendar visita":               "How to schedule a visit",
	"Como fazer a matrícula":            "How to enrol",
	"Convênios":                         "Insurance plans",
	"Descontos e bolsas":                "Discounts and scholarships",
	"Documentos para aluguel":           "Rental documents",
	"Documentos para compra":            "Purchase documents",
	"Formas de pagamento":               "Payment methods",
	"Horário de atendimento":            "Business hours",
	"Material didático":                 "Course materials",
	"Mensalidade e formas de pagamento": "Tuition and payment methods",
	"Política de reembolso":             "Refund policy",
	"Prazo de entrega":                  "Delivery time",
	"Primeira consulta":                 "First appointment",
	"Produto com defeito":               "Defective product",
	"Rastreamento do pedido":            "Order tracking",
	"Reembolso":                         "Refunds",
	"Taxa de intermediação":             "Brokerage fee",
	"Trocas e devoluções":               "Exchanges and returns",
}
