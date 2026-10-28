package domain

import (
	"context"
	"strings"
	"time"
)

// Company data providers (US-M002).
//
// Prospecting today searches Google Places: good for "clinics near me", blind
// to everything a B2B seller actually filters on (CNAE, company size, opening
// date, whether they are even active). Brazilian B2B data lives in Receita
// Federal derivatives — Speedio being the one this product standardizes on.
//
// The interface exists so the waterfall the strategy doc calls for (single
// provider = 73% coverage, waterfall = 95%+) can be added without touching
// the callers: a second provider becomes another implementation, and the
// resolver tries them in order.

// CompanySearchQuery is a prospecting filter. Every field is optional; an
// empty query is rejected by the provider rather than returning the whole
// country.
type CompanySearchQuery struct {
	// CNAE codes (Brazilian industry classification), e.g. "8610101" for
	// general hospitals. The single most useful filter for a vertical.
	CNAEs []string
	// UF (state) and city narrow the territory a salesperson can serve.
	UF   string
	City string
	// MinEmployees/MaxEmployees bound company size — a 3-person clinic and a
	// 400-bed hospital are different products.
	MinEmployees int
	MaxEmployees int
	// OnlyActive filters out companies with a closed Receita Federal
	// registration. Defaults to true at the adapter; a dead CNPJ is not a
	// lead.
	OnlyActive bool
	Limit      int
	Offset     int
}

// Valid reports whether the query is narrow enough to run.
func (q CompanySearchQuery) Valid() bool {
	return len(q.CNAEs) > 0 || strings.TrimSpace(q.City) != "" || strings.TrimSpace(q.UF) != ""
}

// CompanyRecord is one company as a data provider describes it. Strictly
// company-level: contact PEOPLE come from enrichment, which is per-account
// private data (see SanitizeCompanyPayload).
type CompanyRecord struct {
	CNPJ         string    `json:"cnpj"`
	LegalName    string    `json:"legal_name,omitempty"`
	TradeName    string    `json:"trade_name,omitempty"`
	CNAE         string    `json:"cnae,omitempty"`
	CNAELabel    string    `json:"cnae_label,omitempty"`
	Website      string    `json:"website,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	Address      string    `json:"address,omitempty"`
	City         string    `json:"city,omitempty"`
	State        string    `json:"state,omitempty"`
	Zip          string    `json:"zip,omitempty"`
	Employees    int       `json:"employees,omitempty"`
	RevenueRange string    `json:"revenue_range,omitempty"`
	OpeningDate  string    `json:"opening_date,omitempty"`
	Active       bool      `json:"active"`
	FetchedAt    time.Time `json:"-"`
}

// DisplayName is the name to show a salesperson: the trade name when the
// company has one (that is what is on the door), else the legal name.
func (c CompanyRecord) DisplayName() string {
	if n := strings.TrimSpace(c.TradeName); n != "" {
		return n
	}
	return strings.TrimSpace(c.LegalName)
}

// CompanyDataProvider is a source of Brazilian company data.
type CompanyDataProvider interface {
	// Name identifies the provider in cache rows and logs.
	Name() string
	// Search returns companies matching the query.
	Search(ctx context.Context, q CompanySearchQuery) ([]CompanyRecord, error)
	// Lookup fetches a single company by CNPJ. Returns (nil, nil) when the
	// provider has no record — an unknown CNPJ is not an error.
	Lookup(ctx context.Context, cnpj string) (*CompanyRecord, error)
}

var (
	ErrDataProviderUnconfigured = &AppError{Code: "DATA_PROVIDER_UNCONFIGURED", Message: "no company data provider is configured"}
	ErrDataProviderQueryTooWide = &AppError{Code: "DATA_PROVIDER_QUERY_TOO_WIDE", Message: "narrow the search by CNAE, city or state"}
	ErrDataProviderFailed       = &AppError{Code: "DATA_PROVIDER_FAILED", Message: "the company data provider is unavailable"}
)
