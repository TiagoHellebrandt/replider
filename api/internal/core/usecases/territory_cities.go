package usecases

import (
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Territory scoping for the CNES readers (radar, hunt, counts). The
// account's territory is a UF plus optional city NAMES; the CNES tables
// carry 6-digit IBGE city codes. When the territory names cities and at
// least one resolves, the park queries are filtered to those codes and
// the copy says "no seu território (Cuiabá, Várzea Grande)"; otherwise the
// whole UF is the scope and the copy says "em MT" — today's behaviour.

// territoryCityResolver is the one repo method the scoping needs.
type territoryCityResolver interface {
	CityCodesByName(uf string, names []string) ([]string, error)
}

// territoryScope is the resolved filter plus the label the cards print.
type territoryScope struct {
	UF string
	// CityCodes is empty for a UF-wide scope.
	CityCodes []string
	// Cities are the territory names that resolved (display order).
	Cities []string
}

// Label is the card context fragment: "no seu território (Cuiabá)" or
// "em MT".
func (s territoryScope) Label() string {
	if len(s.Cities) == 0 {
		return "em " + s.UF
	}
	return "no seu território (" + strings.Join(s.Cities, ", ") + ")"
}

// resolveTerritoryScope narrows to the territory's cities when they
// resolve. A lookup failure or an all-typos city list falls back to the
// UF (with no cities in the label) rather than filtering to nothing —
// "ninguém tem" is a claim an unresolved filter has not earned.
func resolveTerritoryScope(r territoryCityResolver, t *domain.Territory) territoryScope {
	scope := territoryScope{}
	if t == nil {
		return scope
	}
	scope.UF = strings.ToUpper(strings.TrimSpace(t.UF))
	if len(scope.UF) != 2 || r == nil {
		return scope
	}
	names := make([]string, 0, len(t.Cities))
	for _, c := range t.Cities {
		if c = strings.TrimSpace(c); c != "" {
			names = append(names, c)
		}
	}
	if len(names) == 0 {
		return scope
	}
	codes, err := r.CityCodesByName(scope.UF, names)
	if err != nil || len(codes) == 0 {
		return scope
	}
	scope.CityCodes = codes
	scope.Cities = names
	return scope
}
