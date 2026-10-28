package e2e

import (
	"context"
	"strings"
	"sync"

	"github.com/replider/api/internal/core/domain"
)

// The journey test's contract with the outside world.
//
// journey_first_customer_test.go drives everything over HTTP, but three
// boundaries genuinely leave the process in production — the AI provider, the
// DNS a registrar serves, and the outbound mail hop — so the harness stands a
// fake at each one, wired at the SAME seam main.go wires the real adapter.
// Nothing here bypasses a use case and nothing writes state: every fake only
// answers a call the product itself made.

// e2eAIAccountMarker turns on the fake AI for one account. Accounts whose NAME
// carries this marker get a working provider from the factory; every other
// account keeps the real factory's behaviour, which with no key in the suite
// is a refusal. The marker is keyed on the name because the name is the one
// attribute a test can choose over the public signup route — and the refusal
// behaviour is itself load-bearing elsewhere (first_offer_test pins that a
// draft refusal is a domain error, not a 500), so the fake must never leak
// into accounts that did not ask for it.
const e2eAIAccountMarker = "E2E-AI"

// e2eFakeDraftText is what the fake model writes. Deliberately free of
// placeholders and promise language so the draft survives completeDraft's
// checks the way a good real draft would.
const e2eFakeDraftText = "Olá! Nossa clínica fornece equipamentos hospitalares com assistência local. Faz sentido conversarmos na próxima semana?"

func accountWantsFakeAI(account *domain.Account) bool {
	return account != nil && strings.Contains(account.Name, e2eAIAccountMarker)
}

// e2eAIFactory delegates to the real (key-less) factory except for marker
// accounts, which get the deterministic fake below.
type e2eAIFactory struct{ real domain.AIFactory }

func newE2EAIFactory(real domain.AIFactory) *e2eAIFactory { return &e2eAIFactory{real: real} }

func (f *e2eAIFactory) GetProvider(ctx context.Context, providerName string) (domain.AIProvider, error) {
	return f.real.GetProvider(ctx, providerName)
}

func (f *e2eAIFactory) GetProviderForAccount(ctx context.Context, providerName string, account *domain.Account, plan *domain.Plan) (domain.AIProvider, error) {
	if accountWantsFakeAI(account) {
		return &e2eFakeAIProvider{}, nil
	}
	return f.real.GetProviderForAccount(ctx, providerName, account, plan)
}

func (f *e2eAIFactory) GetCostConfig(provider, model string) (float64, float64) {
	return f.real.GetCostConfig(provider, model)
}

type e2eFakeAIProvider struct{}

func (p *e2eFakeAIProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	return &domain.AICallbackResponse{
		Content: e2eFakeDraftText,
		Usage:   domain.AIUsage{PromptTokens: 100, CompletionTokens: 50},
	}, nil
}

func (p *e2eFakeAIProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	ch := make(chan string, 1)
	ch <- e2eFakeDraftText
	close(ch)
	return domain.StreamResult{
		Chunks:   ch,
		GetUsage: func() (int64, int64) { return 100, 50 },
	}, nil
}

func (p *e2eFakeAIProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

// e2eFakePDFExtraction is what the fake model reads out of the brochure
// fixture: one clean row, one ambiguous price (the refusal the preview must
// surface), and one product the CSV import already created (the dedup).
const e2eFakePDFExtraction = `[
  {"name":"Monitor Multiparâmetro M-10","description":"monitor de sinais vitais","sku":"M10","brand":"MedTech","model":"M-10","price":"R$ 32.500,00","currency":"BRL","unit":"unidade","page":1},
  {"name":"Bomba de Infusão B-3","description":null,"sku":"B3","brand":"MedTech","model":null,"price":"1.234","currency":null,"unit":null,"page":2},
  {"name":"Tomógrafo Portátil TX-1","description":"tomógrafo 16 cortes","sku":"TX1-2026","brand":"MedTech","model":"TX-1","price":"R$ 150.000,00","currency":"BRL","unit":"unidade","page":1}
]`

// GenerateFromDocument makes the fake provider a domain.DocumentReader — the
// journey's PDF import runs the REAL gate ladder (antivirus, pages, spend)
// and only the model call itself is canned.
func (p *e2eFakeAIProvider) GenerateFromDocument(ctx context.Context, systemPrompt, prompt string, doc domain.AIDocument) (*domain.AICallbackResponse, error) {
	return &domain.AICallbackResponse{
		Content: e2eFakePDFExtraction,
		Usage:   domain.AIUsage{PromptTokens: 500, CompletionTokens: 120},
	}, nil
}

// e2eCleanScanner is the antivirus boundary: the suite has no clamd, and the
// use case refuses with a nil scanner, so the harness stands a scanner that
// answers "clean" — the refusal paths are pinned by unit tests.
type e2eCleanScanner struct{}

func (s *e2eCleanScanner) Scan(ctx context.Context, data []byte) (domain.ScanVerdict, error) {
	return domain.ScanVerdict{Status: domain.ScanClean}, nil
}

// e2eEnrichmentFactory mirrors the BYOK gate the real grounding factory
// enforces: a marker account gets a deterministic enrichment, everyone else
// gets the same "configure your key" refusal an unconfigured production
// account would see.
type e2eEnrichmentFactory struct{}

func (f *e2eEnrichmentFactory) ProviderForAccount(ctx context.Context, account *domain.Account, plan *domain.Plan) (domain.LeadEnrichmentProvider, error) {
	if accountWantsFakeAI(account) {
		return &e2eFakeEnricher{}, nil
	}
	return nil, domain.ErrBYOKNotConfigured
}

type e2eFakeEnricher struct{}

func (p *e2eFakeEnricher) Enrich(ctx context.Context, lead *domain.Lead) (domain.EnrichmentData, error) {
	return domain.EnrichmentData{
		DecisionMakerName: "Dra. Ana Souza",
		DecisionMakerRole: "Diretora de Compras",
		Sources:           []string{"https://hospitalsaolucas.com.br/institucional"},
	}, nil
}

// e2eDNSResolver is the registrar. The journey reads the records the setup
// screen asks for over HTTP, "publishes" them here, and then lets the real
// verifier check them — the same round trip a customer makes against their
// real zone.
type e2eDNSResolverT struct {
	mu    sync.Mutex
	txt   map[string][]string
	cname map[string]string
}

var e2eDNSResolver = &e2eDNSResolverT{
	txt:   map[string][]string{},
	cname: map[string]string{},
}

func dnsKey(name string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
}

func (r *e2eDNSResolverT) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.txt = map[string][]string{}
	r.cname = map[string]string{}
}

// Publish records one DNS entry, exactly as the customer would in their
// registrar's panel.
func (r *e2eDNSResolverT) Publish(recordType, host, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch strings.ToUpper(recordType) {
	case "TXT":
		r.txt[dnsKey(host)] = append(r.txt[dnsKey(host)], value)
	case "CNAME":
		r.cname[dnsKey(host)] = value
	}
}

func (r *e2eDNSResolverT) LookupTXT(ctx context.Context, name string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.txt[dnsKey(name)], nil
}

func (r *e2eDNSResolverT) LookupCNAME(ctx context.Context, name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cname[dnsKey(name)], nil
}

func (r *e2eDNSResolverT) LookupNS(ctx context.Context, name string) ([]string, error) {
	return []string{"ns1.registro.br."}, nil
}

// e2eEmailOutbox is the mail hop: it satisfies the ApproachSender port the
// email adapter satisfies in main.go and records every send instead of
// dialing SMTP, so the journey can assert exactly what left and to whom.
type e2eEmailSend struct {
	ChannelID string
	To        string
	Body      string
}

type e2eEmailOutboxT struct {
	mu    sync.Mutex
	sends []e2eEmailSend
}

var e2eEmailOutbox = &e2eEmailOutboxT{}

func (o *e2eEmailOutboxT) SendMessageAndGetID(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	body := ""
	if msg != nil && msg.Content != nil {
		body = *msg.Content
	}
	o.sends = append(o.sends, e2eEmailSend{ChannelID: channel.ID, To: to, Body: body})
	return "e2e-mail-1", nil
}

func (o *e2eEmailOutboxT) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sends = nil
}

func (o *e2eEmailOutboxT) Sends() []e2eEmailSend {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]e2eEmailSend, len(o.sends))
	copy(out, o.sends)
	return out
}
