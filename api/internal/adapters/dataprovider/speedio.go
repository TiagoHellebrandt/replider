// Package dataprovider holds adapters for Brazilian company-data sources
// (US-M002). Speedio is the first: ~60M CNPJs derived from Receita Federal,
// filterable by CNAE/UF/city/size — the filters a B2B salesperson actually
// works with, and the ones Google Places (today's prospecting source) cannot
// answer at all.
package dataprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const (
	defaultSpeedioBaseURL = "https://api.speedio.com.br/v1"
	speedioTimeout        = 20 * time.Second
	// A provider page bigger than this is a paging bug, not a real request.
	speedioMaxLimit = 100
	// Response bodies are bounded: a provider that starts streaming must not
	// be able to exhaust our memory.
	speedioMaxBody = 8 << 20
)

// SpeedioAdapter implements domain.CompanyDataProvider.
type SpeedioAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ domain.CompanyDataProvider = (*SpeedioAdapter)(nil)

// NewSpeedioAdapter builds the adapter. An empty key yields an adapter that
// reports itself unconfigured on every call rather than a nil interface —
// callers then surface DATA_PROVIDER_UNCONFIGURED instead of panicking.
func NewSpeedioAdapter(apiKey string) *SpeedioAdapter {
	return &SpeedioAdapter{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: defaultSpeedioBaseURL,
		client:  &http.Client{Timeout: speedioTimeout},
	}
}

// WithBaseURL points the adapter at a test server. Unexported field, same
// pattern as the Gemini grounding adapter — the SDK-less REST call is what
// makes the adapter testable at all.
func (a *SpeedioAdapter) WithBaseURL(u string) *SpeedioAdapter {
	a.baseURL = strings.TrimRight(u, "/")
	return a
}

func (a *SpeedioAdapter) Name() string { return "speedio" }

func (a *SpeedioAdapter) configured() bool { return a.apiKey != "" }

// speedioCompany is the provider's wire shape. Field names follow their API;
// the mapping to domain.CompanyRecord is the only place that knows them.
type speedioCompany struct {
	CNPJ         string `json:"cnpj"`
	RazaoSocial  string `json:"razao_social"`
	NomeFantasia string `json:"nome_fantasia"`
	CNAE         string `json:"cnae_principal"`
	CNAEDesc     string `json:"cnae_descricao"`
	Site         string `json:"site"`
	Telefone     string `json:"telefone"`
	Logradouro   string `json:"logradouro"`
	Cidade       string `json:"cidade"`
	UF           string `json:"uf"`
	CEP          string `json:"cep"`
	Funcionarios int    `json:"funcionarios"`
	FaixaFat     string `json:"faixa_faturamento"`
	DataAbertura string `json:"data_abertura"`
	Situacao     string `json:"situacao_cadastral"`
}

func (c speedioCompany) toDomain() domain.CompanyRecord {
	return domain.CompanyRecord{
		CNPJ:         domain.NormalizeCNPJ(c.CNPJ),
		LegalName:    strings.TrimSpace(c.RazaoSocial),
		TradeName:    strings.TrimSpace(c.NomeFantasia),
		CNAE:         strings.TrimSpace(c.CNAE),
		CNAELabel:    strings.TrimSpace(c.CNAEDesc),
		Website:      normalizeWebsite(c.Site),
		Phone:        strings.TrimSpace(c.Telefone),
		Address:      strings.TrimSpace(c.Logradouro),
		City:         strings.TrimSpace(c.Cidade),
		State:        strings.ToUpper(strings.TrimSpace(c.UF)),
		Zip:          strings.TrimSpace(c.CEP),
		Employees:    c.Funcionarios,
		RevenueRange: strings.TrimSpace(c.FaixaFat),
		OpeningDate:  strings.TrimSpace(c.DataAbertura),
		// Receita Federal statuses: only ATIVA means the company legally
		// exists today. A BAIXADA/SUSPENSA CNPJ is not a lead.
		Active:    strings.EqualFold(strings.TrimSpace(c.Situacao), "ATIVA"),
		FetchedAt: time.Now(),
	}
}

// normalizeWebsite makes a bare domain usable as a link. Providers return
// both "acme.com.br" and "https://acme.com.br"; the enrichment prompt and the
// UI both want a real URL.
func normalizeWebsite(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	return "https://" + s
}

func (a *SpeedioAdapter) Search(ctx context.Context, q domain.CompanySearchQuery) ([]domain.CompanyRecord, error) {
	if !a.configured() {
		return nil, domain.ErrDataProviderUnconfigured
	}
	if !q.Valid() {
		return nil, domain.ErrDataProviderQueryTooWide
	}

	params := url.Values{}
	if len(q.CNAEs) > 0 {
		params.Set("cnae", strings.Join(q.CNAEs, ","))
	}
	if uf := strings.TrimSpace(q.UF); uf != "" {
		params.Set("uf", strings.ToUpper(uf))
	}
	if city := strings.TrimSpace(q.City); city != "" {
		params.Set("cidade", city)
	}
	if q.MinEmployees > 0 {
		params.Set("funcionarios_min", strconv.Itoa(q.MinEmployees))
	}
	if q.MaxEmployees > 0 {
		params.Set("funcionarios_max", strconv.Itoa(q.MaxEmployees))
	}
	// Default to active-only: prospecting a closed company wastes a send and
	// burns the sender's reputation for nothing.
	if q.OnlyActive || q.MinEmployees == 0 {
		params.Set("situacao", "ATIVA")
	}
	limit := q.Limit
	if limit <= 0 || limit > speedioMaxLimit {
		limit = speedioMaxLimit
	}
	params.Set("limit", strconv.Itoa(limit))
	if q.Offset > 0 {
		params.Set("offset", strconv.Itoa(q.Offset))
	}

	var payload struct {
		Data []speedioCompany `json:"data"`
	}
	if err := a.get(ctx, "/companies?"+params.Encode(), &payload); err != nil {
		return nil, err
	}

	out := make([]domain.CompanyRecord, 0, len(payload.Data))
	for _, c := range payload.Data {
		rec := c.toDomain()
		// A record with no CNPJ cannot be cached or deduped — it is noise.
		if len(rec.CNPJ) != 14 {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (a *SpeedioAdapter) Lookup(ctx context.Context, cnpj string) (*domain.CompanyRecord, error) {
	if !a.configured() {
		return nil, domain.ErrDataProviderUnconfigured
	}
	digits := domain.NormalizeCNPJ(cnpj)
	if len(digits) != 14 {
		return nil, domain.ErrInvalidCNPJ
	}

	var payload struct {
		Data *speedioCompany `json:"data"`
	}
	err := a.get(ctx, "/companies/"+digits, &payload)
	if err != nil {
		// An unknown CNPJ is a miss, not a failure: the caller falls back to
		// whatever it already knows.
		if errIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if payload.Data == nil {
		return nil, nil
	}
	rec := payload.Data.toDomain()
	if len(rec.CNPJ) != 14 {
		rec.CNPJ = digits
	}
	return &rec, nil
}

// notFoundError distinguishes "this CNPJ is not in the provider's database"
// from "the provider is broken". The first is a normal miss; the second must
// surface so the caller can fall back to a stale cache row.
type notFoundError struct{ status int }

func (e notFoundError) Error() string { return fmt.Sprintf("speedio: not found (%d)", e.status) }

func errIsNotFound(err error) bool {
	var nf notFoundError
	return errors.As(err, &nf)
}

func (a *SpeedioAdapter) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrDataProviderFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return notFoundError{status: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body may carry the provider's reason, but it may also carry
		// data we should not log — read a bounded prefix for the message and
		// nothing more.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: status %d: %s", domain.ErrDataProviderFailed, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, speedioMaxBody))
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrDataProviderFailed, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: malformed response: %v", domain.ErrDataProviderFailed, err)
	}
	return nil
}
