package dataprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M002. The provider itself needs a paid key, so these tests pin the parts
// that break WITHOUT one: the query we send, the mapping we apply, and the
// difference between "no such company" and "the vendor is down" — a
// distinction the caller's stale-cache fallback depends on.

const searchBody = `{"data":[
  {"cnpj":"12.345.678/0001-99","razao_social":"HOSPITAL SAO LUCAS LTDA","nome_fantasia":"Hospital São Lucas",
   "cnae_principal":"8610101","cnae_descricao":"Atividades de atendimento hospitalar","site":"saolucas.com.br",
   "telefone":"+551133334444","logradouro":"Av. Paulista, 1000","cidade":"São Paulo","uf":"sp","cep":"01310-100",
   "funcionarios":240,"faixa_faturamento":"10M-50M","data_abertura":"1998-03-12","situacao_cadastral":"ATIVA"},
  {"cnpj":"","razao_social":"SEM CNPJ LTDA","situacao_cadastral":"ATIVA"},
  {"cnpj":"98765432000188","razao_social":"CLINICA FECHADA LTDA","situacao_cadastral":"BAIXADA"}
]}`

func TestSpeedio_SearchMapsAndFiltersRecords(t *testing.T) {
	var gotPath string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchBody))
	}))
	defer srv.Close()

	adapter := NewSpeedioAdapter("test-key").WithBaseURL(srv.URL)
	got, err := adapter.Search(context.Background(), domain.CompanySearchQuery{
		CNAEs:        []string{"8610101", "8630503"},
		UF:           "sp",
		City:         "São Paulo",
		MinEmployees: 50,
		Limit:        25,
	})
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.Contains(t, gotPath, "cnae=8610101%2C8630503")
	assert.Contains(t, gotPath, "uf=SP", "UF is normalized to upper case")
	assert.Contains(t, gotPath, "funcionarios_min=50")
	assert.Contains(t, gotPath, "limit=25")

	// The CNPJ-less row is dropped: without it the record cannot be cached
	// or deduped, so it is noise rather than a lead.
	require.Len(t, got, 2)

	first := got[0]
	assert.Equal(t, "12345678000199", first.CNPJ, "the mask is stripped")
	assert.Equal(t, "Hospital São Lucas", first.DisplayName(), "trade name wins — that is what is on the door")
	assert.Equal(t, "https://saolucas.com.br", first.Website, "a bare domain becomes a usable URL")
	assert.Equal(t, "SP", first.State)
	assert.Equal(t, 240, first.Employees)
	assert.True(t, first.Active)

	// Status is mapped, not guessed: only ATIVA counts as a live company.
	assert.False(t, got[1].Active, "a BAIXADA registration is not a lead")
}

func TestSpeedio_DisplayNameFallsBackToLegalName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"cnpj":"12345678000199","razao_social":"CLINICA XYZ LTDA","situacao_cadastral":"ATIVA"}}`))
	}))
	defer srv.Close()

	rec, err := NewSpeedioAdapter("k").WithBaseURL(srv.URL).Lookup(context.Background(), "12345678000199")
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "CLINICA XYZ LTDA", rec.DisplayName())
}

// A 404 is a MISS (nil, nil) — the caller keeps whatever it already knew.
func TestSpeedio_UnknownCNPJIsAMissNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rec, err := NewSpeedioAdapter("k").WithBaseURL(srv.URL).Lookup(context.Background(), "12345678000199")
	assert.NoError(t, err)
	assert.Nil(t, rec)
}

// ...while a 5xx IS an error, because that is what triggers the stale-cache
// fallback instead of silently reporting "company not found".
func TestSpeedio_ProviderOutageSurfacesAsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream down"}`))
	}))
	defer srv.Close()

	_, err := NewSpeedioAdapter("k").WithBaseURL(srv.URL).Lookup(context.Background(), "12345678000199")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrDataProviderFailed)
}

func TestSpeedio_MalformedResponseIsAProviderFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	_, err := NewSpeedioAdapter("k").WithBaseURL(srv.URL).Search(context.Background(),
		domain.CompanySearchQuery{CNAEs: []string{"8610101"}})
	assert.ErrorIs(t, err, domain.ErrDataProviderFailed)
}

// No key configured must degrade to a clear error, never a nil-pointer panic
// in a background prospecting job.
func TestSpeedio_UnconfiguredIsExplicit(t *testing.T) {
	adapter := NewSpeedioAdapter("   ")
	_, err := adapter.Search(context.Background(), domain.CompanySearchQuery{UF: "SP"})
	assert.ErrorIs(t, err, domain.ErrDataProviderUnconfigured)

	_, err = adapter.Lookup(context.Background(), "12345678000199")
	assert.ErrorIs(t, err, domain.ErrDataProviderUnconfigured)
}

func TestSpeedio_RejectsWideSearchBeforeSpendingACall(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	_, err := NewSpeedioAdapter("k").WithBaseURL(srv.URL).Search(context.Background(), domain.CompanySearchQuery{})
	assert.ErrorIs(t, err, domain.ErrDataProviderQueryTooWide)
	assert.False(t, called, "a query with no filters must not reach the (paid) provider")
}

func TestSpeedio_LookupRejectsMalformedCNPJ(t *testing.T) {
	_, err := NewSpeedioAdapter("k").Lookup(context.Background(), "123")
	assert.ErrorIs(t, err, domain.ErrInvalidCNPJ)
}
