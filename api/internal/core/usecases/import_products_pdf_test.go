package usecases

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The brochure import's whole posture is "refuse instead of guess": every gate
// (antivirus, size, pages, provider capability) fails closed with a legible
// error, and the model's answer is distrusted field by field. These tests pin
// each refusal and each parser rule — a panic or a silently-guessed price here
// puts wrong numbers in front of a customer.

// ── fakes at the process boundaries ─────────────────────────────────────────

type pdfFakeProvider struct {
	resp *domain.AICallbackResponse
	err  error
}

func (p *pdfFakeProvider) GenerateResponse(context.Context, string, []domain.ChatMessage, []domain.AITool) (*domain.AICallbackResponse, error) {
	return nil, errors.New("not used")
}
func (p *pdfFakeProvider) GenerateStream(context.Context, string, []domain.ChatMessage, []domain.AITool) (domain.StreamResult, error) {
	return domain.StreamResult{}, errors.New("not used")
}
func (p *pdfFakeProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, errors.New("not used")
}
func (p *pdfFakeProvider) GenerateFromDocument(context.Context, string, string, domain.AIDocument) (*domain.AICallbackResponse, error) {
	return p.resp, p.err
}

// pdfBlindProvider satisfies AIProvider but NOT DocumentReader — the account
// whose model cannot read files.
type pdfBlindProvider struct{}

func (p *pdfBlindProvider) GenerateResponse(context.Context, string, []domain.ChatMessage, []domain.AITool) (*domain.AICallbackResponse, error) {
	return nil, errors.New("not used")
}
func (p *pdfBlindProvider) GenerateStream(context.Context, string, []domain.ChatMessage, []domain.AITool) (domain.StreamResult, error) {
	return domain.StreamResult{}, errors.New("not used")
}
func (p *pdfBlindProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, errors.New("not used")
}

type pdfFakeScanner struct {
	verdict domain.ScanVerdict
	err     error
}

func (s *pdfFakeScanner) Scan(context.Context, []byte) (domain.ScanVerdict, error) {
	return s.verdict, s.err
}

type pdfFailStore struct{ called bool }

func (s *pdfFailStore) UploadBytes(context.Context, string, []byte, string) error {
	s.called = true
	return errors.New("bucket down")
}

// ── fixture ─────────────────────────────────────────────────────────────────

func brochurePDF(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/folheto_produtos.pdf")
	require.NoError(t, err, "fixture PDF missing")
	return data
}

func newPDFImportFixture(t *testing.T, provider domain.AIProvider) (*ImportProductsPDFUseCase, *pdfFakeScanner) {
	t.Helper()
	accountRepo := new(MockAccountRepo)
	accountRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Currency: "brl"}, nil)
	productRepo := new(MockProductRepo)
	productRepo.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{{Name: "Ventilador Pulmonar V-200"}}, nil)
	aiFactory := new(MockAIFactory)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)

	csvUC := NewImportProductsUseCase(productRepo, accountRepo)
	uc := NewImportProductsPDFUseCase(csvUC, accountRepo, nil, aiFactory,
		func(data []byte) (int, error) { return 2, nil }, 1<<20, 10)
	scanner := &pdfFakeScanner{verdict: domain.ScanVerdict{Status: domain.ScanClean}}
	uc.SetScanner(scanner)
	return uc, scanner
}

const goodExtraction = `[
  {"name":"Tomógrafo Portátil TX-1","description":"16 cortes","sku":"TX1-2026","brand":"MedTech","model":"TX-1","price":"R$ 150.000,00","currency":"BRL","unit":"unidade","page":1},
  {"name":"Ventilador Pulmonar V-200","description":null,"sku":"V200","brand":"MedTech","model":null,"price":"1.234","currency":null,"unit":null,"page":2}
]`

// ── the ladder of refusals ──────────────────────────────────────────────────

func TestProductPDF_NoScannerRefuses(t *testing.T) {
	uc, _ := newPDFImportFixture(t, &pdfFakeProvider{})
	uc.SetScanner(nil)
	_, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_SCAN_UNAVAILABLE", appErr.Code,
		"sem antivírus configurado o upload recusa, nunca passa direto")
}

func TestProductPDF_ScanErrorFailsClosed(t *testing.T) {
	uc, scanner := newPDFImportFixture(t, &pdfFakeProvider{})
	scanner.err = errors.New("clamd down")
	_, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_SCAN_UNAVAILABLE", appErr.Code,
		"um scan que não completou não é um scan limpo")
}

func TestProductPDF_InfectedRefusesWithTheSignature(t *testing.T) {
	uc, scanner := newPDFImportFixture(t, &pdfFakeProvider{})
	scanner.verdict = domain.ScanVerdict{Status: domain.ScanInfected, Signature: "Eicar-Test-Signature"}
	_, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_INFECTED", appErr.Code)
	assert.Contains(t, appErr.Message, "Eicar", "o nome da assinatura viaja — 'infectado' sem nome é indiscutível")
}

func TestProductPDF_SizeAndPageCeilingsRefuse(t *testing.T) {
	uc, _ := newPDFImportFixture(t, &pdfFakeProvider{})
	uc.maxBytes = 10
	_, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_TOO_LARGE", appErr.Code)

	uc2, _ := newPDFImportFixture(t, &pdfFakeProvider{})
	uc2.maxPages = 1
	_, err = uc2.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_TOO_MANY_PAGES", appErr.Code)
	assert.Contains(t, appErr.Message, "2 páginas", "o erro diz quantas páginas o arquivo tem e qual é o limite")
}

func TestProductPDF_NotAPDFRefuses(t *testing.T) {
	uc, _ := newPDFImportFixture(t, &pdfFakeProvider{})
	_, err := uc.Extract(context.Background(), "acc-1", []byte("planilha.xlsx em disfarce"), "x.pdf")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_UNREADABLE", appErr.Code)
}

func TestProductPDF_ProviderThatCannotReadIsAnHonestRefusal(t *testing.T) {
	uc, _ := newPDFImportFixture(t, &pdfBlindProvider{})
	_, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "PRODUCT_PDF_PROVIDER_CANT_READ", appErr.Code)
}

// ── the happy path and its best-effort edge ─────────────────────────────────

func TestProductPDF_ExtractsPreviewWithDedupAndAmbiguousPrice(t *testing.T) {
	uc, _ := newPDFImportFixture(t, &pdfFakeProvider{
		resp: &domain.AICallbackResponse{Content: "```json\n" + goodExtraction + "\n```"},
	})
	store := &pdfFailStore{}
	uc.SetStore(store)

	preview, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")
	require.NoError(t, err, "a cópia de auditoria é best-effort: bucket fora não mata o import")
	assert.True(t, store.called, "o upload de auditoria foi tentado")

	require.Len(t, preview.Rows, 2)
	tx := preview.Rows[0]
	assert.Equal(t, "Tomógrafo Portátil TX-1", tx.Name)
	assert.Equal(t, int64(15_000_000), tx.PriceCents)
	assert.Equal(t, "TX1-2026", tx.SKU)
	assert.Equal(t, 1, tx.Page, "cada linha carrega a página de origem para o humano conferir")
	assert.Empty(t, tx.Problem)

	v200 := preview.Rows[1]
	// "1.234" é o caso que este produto se recusa a adivinhar — igual ao CSV.
	assert.Equal(t, domain.ProblemAmbiguousPrice, v200.Problem)
	assert.Equal(t, "BRL", v200.Currency, "moeda ausente cai no fallback da conta, nunca é inventada")

	// O dedup contra o catálogo roda no MESMO preview do CSV — mas só marca
	// linhas que ainda são importáveis; o preço ambíguo já é problema maior.
	assert.Equal(t, 1, preview.Importable)
	assert.Equal(t, 1, preview.Problems)
}

// ── the parser, fed hostile answers ─────────────────────────────────────────

func TestParseProductPDFExtraction_FailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"prosa sem JSON", "Claro! Aqui estão os produtos que encontrei no folheto.", "PRODUCT_PDF_EXTRACT_FAILED"},
		{"JSON malformado", `[{"name": "Tomógrafo", "price": }]`, "PRODUCT_PDF_EXTRACT_FAILED"},
		{"resposta vazia", "", "PRODUCT_PDF_EXTRACT_FAILED"},
		{"array vazio", "[]", "PRODUCT_PDF_NO_PRODUCTS"},
		{"objeto em vez de array", `{"products": "muitos"}`, "PRODUCT_PDF_EXTRACT_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseProductPDFExtraction(tc.content, "BRL")
			var appErr *domain.AppError
			require.ErrorAs(t, err, &appErr, "falha fechada com erro legível, nunca panic")
			assert.Equal(t, tc.wantErr, appErr.Code)
		})
	}
}

func TestParseProductPDFExtraction_FieldRules(t *testing.T) {
	rows, err := parseProductPDFExtraction(`[
	  {"name":null,"price":"10,00","page":1},
	  {"name":"Gaze Estéril","price":"abc","currency":"USD","page":3},
	  {"name":"Gaze Estéril","price":"5,00","page":4},
	  {"name":"Luva Nitrílica","description":null,"sku":null,"price":null,"page":2}
	]`, "BRL")
	require.NoError(t, err)
	require.Len(t, rows, 4)

	assert.Equal(t, domain.ProblemNoName, rows[0].Problem, "campo ausente é null; sem nome não importa")
	assert.Equal(t, domain.ProblemBadPrice, rows[1].Problem, "preço ilegível é marcado, não zerado em silêncio")
	assert.Equal(t, "USD", rows[1].Currency)
	assert.Equal(t, domain.ProblemDuplicateInFile, rows[2].Problem, "mesmo nome duas vezes no folheto")
	assert.Empty(t, rows[3].Problem, "preço null é um produto sem preço impresso — importável, editável depois")
	assert.Equal(t, int64(0), rows[3].PriceCents)
	assert.Equal(t, 2, rows[3].Page)
}

// A contract account with no BYOK key must hear that IT needs a key — not that
// the deployment is broken. The onboarding's product step is where this lands
// first, before anybody has been asked for a key, and flattening the cause into
// PRODUCT_PDF_IMPORT_UNAVAILABLE sent the customer hunting for our bug.
func TestProductPDF_MissingBYOKKeySaysWhoseProblemItIs(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	accountRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Currency: "brl"}, nil)
	productRepo := new(MockProductRepo)
	productRepo.On("ListByAccountID", mock.Anything, "acc-1").
		Return([]*domain.Product{}, nil)

	// MockAIFactory.GetProviderForAccount delegates to GetProvider, so the
	// BYOK refusal is stubbed there.
	aiFactory := new(MockAIFactory)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).
		Return(nil, domain.ErrBYOKKeyMissing)

	csvUC := NewImportProductsUseCase(productRepo, accountRepo)
	uc := NewImportProductsPDFUseCase(csvUC, accountRepo, nil, aiFactory,
		func(data []byte) (int, error) { return 2, nil }, 1<<20, 10)
	uc.SetScanner(&pdfFakeScanner{verdict: domain.ScanVerdict{Status: domain.ScanClean}})

	_, err := uc.Extract(context.Background(), "acc-1", brochurePDF(t), "folheto.pdf")

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "BYOK_KEY_MISSING", appErr.Code,
		"the account's missing key must reach the screen as its own cause")
	assert.NotEqual(t, "PRODUCT_PDF_IMPORT_UNAVAILABLE", appErr.Code)
}
