package domain

import "time"

// Objections v1 (tese-agente-autonomo §3): what the prospect will push back
// with, answered BEFORE they say it. The library is company-level knowledge
// (like the brand kit's forbidden claims); the anticipated set is per-lead,
// chosen by the same model call that picks the product.

// Objection is one library entry: the pushback and its counter.
type Objection struct {
	ID        string    `json:"id"`
	AccountID string    `json:"-"`
	Label     string    `json:"label"`
	Counter   string    `json:"counter"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// AnticipatedObjection is one per-lead prediction: a library objection (or
// the model's own phrasing of it) tuned to this customer.
type AnticipatedObjection struct {
	Label   string `json:"label"`
	Counter string `json:"counter"`
}

// ObjectionRepository stores the account's library. Raw-SQL repo.
type ObjectionRepository interface {
	List(accountID string) ([]Objection, error)
	// Replace swaps the whole library in one transaction — the edit surface
	// is a small ordered list, and per-row CRUD over ~10 rows is ceremony.
	Replace(accountID string, items []Objection) error
}

// DefaultHealthObjections is the vertical seed: the pushbacks a hospital-
// equipment seller hears every week, each with a one-line counter direction.
// Used whenever the account's own library is empty, so a fresh account is
// covered without a seeding step; the account's first save replaces it
// entirely. Generic by design — no prices, no brand names, no promises the
// brand kit might forbid.
func DefaultHealthObjections() []AnticipatedObjection {
	return []AnticipatedObjection{
		{Label: "Já temos fornecedor",
			Counter: "Não pede a conta inteira: pede a próxima compra pequena ou o item que o fornecedor atual atende mal (prazo, assistência)."},
		{Label: "Só compramos por licitação",
			Counter: "Ótimo — a proposta é entrar cadastrado e cotado no próximo pregão; compra direta abaixo do limite legal também existe."},
		{Label: "Sem verba este ano",
			Counter: "Locação ou comodato resolve sem CAPEX; e o orçamento do ano que vem se ganha entrando na especificação agora."},
		{Label: "Manutenção a gente faz interno",
			Counter: "O contrato cobre o que a equipe interna não cobre: peça original, calibração certificada e laudo para a vigilância."},
		{Label: "Preciso de mais de um orçamento",
			Counter: "Mande cotar — o objetivo é ser um dos orçamentos; oferece mandar a proposta formal hoje."},
		{Label: "Equipamento de vocês é caro",
			Counter: "Compara custo total: parada de máquina, prazo de peça e assistência local valem mais que a diferença de tabela."},
		{Label: "Manda por e-mail que a gente avalia",
			Counter: "Manda hoje mesmo E marca uma ligação curta de follow-up com data — e-mail sem dono morre na caixa."},
	}
}
