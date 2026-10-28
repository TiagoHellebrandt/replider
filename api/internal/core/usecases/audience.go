package usecases

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Audience suggestions (§2, D3).
//
// The value is not the first audience — a customer knows who they sell to.
// It is the second and third. Somebody who sells patient monitors to
// hospitals usually also sells them to dialysis clinics and to home-care
// operators, and the reason they do not prospect those is that nobody ever
// wrote them down.
//
// So exactly one suggestion per round is deliberately ADJACENT rather than
// obvious. Two would be a list, and a list of things you have not considered
// is just noise.
type AudienceUseCase struct {
	repo      domain.AudienceRepository
	brandRepo domain.BrandKitRepository
	// aiFactory resolves the account's own provider, so a suggestion is
	// billed and keyed the same way every other generation is.
	aiFactory   domain.AIFactory
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	model       string
	meter       *AIMeter
}

func NewAudienceUseCase(repo domain.AudienceRepository) *AudienceUseCase {
	return &AudienceUseCase{repo: repo}
}

func (uc *AudienceUseCase) SetBrandRepo(r domain.BrandKitRepository) { uc.brandRepo = r }

// SetAI wires generation. All three are needed together: the provider is
// resolved from the ACCOUNT's plan, the same way every other generation is,
// so a suggestion is billed and keyed like the rest.
func (uc *AudienceUseCase) SetAI(f domain.AIFactory, accounts domain.AccountRepository, plans domain.PlanRepository) {
	uc.aiFactory, uc.accountRepo, uc.planRepo = f, accounts, plans
}

func (uc *AudienceUseCase) SetModel(m string) { uc.model = strings.TrimSpace(m) }

func (uc *AudienceUseCase) SetAIMeter(m *AIMeter) { uc.meter = m }

func (uc *AudienceUseCase) List(ctx context.Context, accountID string) ([]*domain.Audience, error) {
	list, err := uc.repo.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Audience{}
	}
	return list, nil
}

// Save creates or updates one audience.
func (uc *AudienceUseCase) Save(ctx context.Context, accountID string, a *domain.Audience) (*domain.Audience, error) {
	if strings.TrimSpace(a.Name) == "" && strings.TrimSpace(a.Segment) == "" {
		return nil, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "informe ao menos um nome ou segmento"}
	}
	if strings.TrimSpace(a.Name) == "" {
		// A nameless audience is unpickable in a dropdown. The segment is
		// what the person would have called it anyway.
		a.Name = a.Segment
	}
	if a.ID == "" {
		a.ID = ulid.Make().String()
	}
	a.AccountID = accountID
	a.IsActive = true
	if len(a.Keywords) == 0 {
		a.Keywords = deriveKeywords(a)
	}
	if err := uc.repo.Upsert(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (uc *AudienceUseCase) Delete(ctx context.Context, id, accountID string) error {
	return uc.repo.Delete(ctx, id, accountID)
}

// deriveKeywords turns the human description into something a prospecting
// source will actually match.
//
// The segment plus the size, never the decision-maker role: Google Places
// indexes businesses, not job titles, and "gerente de compras" in the query
// finds recruitment agencies.
func deriveKeywords(a *domain.Audience) []string {
	var out []string
	if s := strings.TrimSpace(a.Segment); s != "" {
		out = append(out, s)
	}
	if s := strings.TrimSpace(a.Size); s != "" {
		out = append(out, s)
	}
	return out
}

var errAudienceAIUnavailable = &domain.AppError{
	Code:    "AUDIENCE_AI_UNAVAILABLE",
	Message: "sugestão de públicos indisponível: nenhum provedor de IA configurado",
}

// Suggest proposes audiences from what the company already is and sells.
//
// Nothing is saved. The suggestions come back for a person to pick from, and
// a proposal written straight into their list would be the product deciding
// who they sell to.
func (uc *AudienceUseCase) Suggest(ctx context.Context, accountID string) ([]*domain.Audience, error) {
	provider, err := uc.provider(ctx, accountID)
	if err != nil {
		return nil, err
	}

	actx := uc.gatherContext(ctx, accountID)
	if strings.TrimSpace(actx.About) == "" && len(actx.ExistingAudiences) == 0 {
		// Nothing to reason from. A suggestion invented out of an empty
		// brand kit is a guess wearing a recommendation's clothes.
		return nil, &domain.AppError{
			Code:    "AUDIENCE_NO_CONTEXT",
			Message: "descreva a empresa em Identidade da marca antes de pedir sugestões",
		}
	}

	resp, err := provider.GenerateResponse(ctx, audienceSystemPrompt,
		[]domain.ChatMessage{{Role: "user", Content: audiencePrompt(actx)}}, nil)
	if err != nil || resp == nil {
		return nil, errAudienceAIUnavailable
	}
	return parseAudienceSuggestions(resp.Content), nil
}

// provider resolves the account's own AI, respecting its plan and BYOK key.
func (uc *AudienceUseCase) provider(ctx context.Context, accountID string) (domain.AIProvider, error) {
	if uc.aiFactory == nil || uc.accountRepo == nil {
		return nil, errAudienceAIUnavailable
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && uc.planRepo != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	// A cheap model is right here: this is a brainstorm over a paragraph of
	// context, not a message a customer will read.
	if uc.model != "" && uc.model != plan.AIModel {
		clone := *plan
		clone.AIModel = uc.model
		plan = &clone
	}
	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil || provider == nil {
		return nil, errAudienceAIUnavailable
	}
	return provider, nil
}

const audienceSystemPrompt = "Você é um consultor de vendas B2B brasileiro. " +
	"Responde só com JSON válido, sem comentários e sem cercas de código."

func (uc *AudienceUseCase) gatherContext(ctx context.Context, accountID string) domain.AudienceContext {
	out := domain.AudienceContext{}
	if uc.brandRepo != nil {
		if kit, err := uc.brandRepo.Get(ctx, accountID); err == nil && kit != nil {
			out.CompanyName, out.About = kit.CompanyName, kit.About
		}
	}
	if existing, err := uc.repo.List(ctx, accountID); err == nil {
		for _, a := range existing {
			out.ExistingAudiences = append(out.ExistingAudiences, a.Segment)
		}
	}
	return out
}

func audiencePrompt(c domain.AudienceContext) string {
	var b strings.Builder
	b.WriteString("Você ajuda uma empresa B2B a descobrir para QUEM MAIS pode vender.\n\n")
	if c.CompanyName != "" {
		b.WriteString("Empresa: " + c.CompanyName + "\n")
	}
	if c.About != "" {
		b.WriteString("O que faz: " + c.About + "\n")
	}
	if len(c.Products) > 0 {
		b.WriteString("Vende: " + strings.Join(c.Products, ", ") + "\n")
	}
	if len(c.WonSegments) > 0 {
		b.WriteString("Já vendeu para: " + strings.Join(c.WonSegments, ", ") + "\n")
	}
	if len(c.ExistingAudiences) > 0 {
		// Listed so the round does not propose what is already saved. A
		// suggestion the customer already had is worse than no suggestion:
		// it says nobody looked.
		b.WriteString("JÁ prospecta (não repita): " +
			strings.Join(c.ExistingAudiences, "; ") + "\n")
	}
	b.WriteString(`
Proponha exatamente 3 públicos-alvo. Dois óbvios para quem conhece o negócio.
UM deve ser adjacente — um segmento que compra o mesmo tipo de solução por um
motivo diferente, e que essa empresa provavelmente não considerou. Marque só
esse com "unexplored": true.

Responda APENAS com JSON:
[{"name":"","segment":"","size":"","decision_maker":"","regions":[],"keywords":[],"unexplored":false}]

Regras:
- segment nas palavras de quem vende, não código CNAE
- decision_maker é o CARGO a alcançar
- keywords são o que se digitaria numa busca de empresas`)
	return b.String()
}

// parseAudienceSuggestions reads the model's answer, tolerating the fences it
// insists on adding.
func parseAudienceSuggestions(raw string) []*domain.Audience {
	body := extractJSONArray(raw)
	if body == "" {
		return nil
	}
	var parsed []struct {
		Name          string   `json:"name"`
		Segment       string   `json:"segment"`
		Size          string   `json:"size"`
		DecisionMaker string   `json:"decision_maker"`
		Regions       []string `json:"regions"`
		Keywords      []string `json:"keywords"`
		Unexplored    bool     `json:"unexplored"`
	}
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return nil
	}

	out := make([]*domain.Audience, 0, len(parsed))
	unexploredSeen := false
	for _, p := range parsed {
		if strings.TrimSpace(p.Segment) == "" {
			continue
		}
		a := &domain.Audience{
			ID: ulid.Make().String(), Name: p.Name, Segment: p.Segment,
			Size: p.Size, DecisionMaker: p.DecisionMaker,
			Regions: p.Regions, Keywords: p.Keywords,
			Suggested: true, IsActive: true,
		}
		if a.Name == "" {
			a.Name = a.Segment
		}
		if len(a.Keywords) == 0 {
			a.Keywords = deriveKeywords(a)
		}
		// Exactly one adjacent suggestion, whatever the model returns. Two
		// "you may not have seen this" cards is a list, and a list of things
		// you have not considered is noise.
		if p.Unexplored && !unexploredSeen {
			a.Unexplored = true
			unexploredSeen = true
		}
		out = append(out, a)
		if len(out) == domain.AudienceSuggestionCount {
			break
		}
	}
	return out
}

// extractJSONArray finds the array inside whatever the model wrapped it in.
func extractJSONArray(raw string) string {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}
