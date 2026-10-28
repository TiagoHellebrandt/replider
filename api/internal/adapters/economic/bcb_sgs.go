// Package economic reads the published price indices a Brazilian contract is
// adjusted by.
package economic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// SGSFetcher reads the Banco Central's Sistema Gerenciador de Séries Temporais.
//
// One source for all four indices, free, no key, and authoritative enough to
// quote in a renegotiation: the customer's counterpart can look up the same
// series. IBGE's SIDRA carries IPCA and INPC but not the FGV indices, and the
// FGV portal is paywalled — SGS republishes all of them.
type SGSFetcher struct {
	client  *http.Client
	baseURL string
}

func NewSGSFetcher() *SGSFetcher {
	return &SGSFetcher{
		// A public API on a government host: slow sometimes, never urgent. The
		// worker that calls this runs on a schedule and nothing waits on it.
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.bcb.gov.br/dados/serie",
	}
}

// WithBaseURL points the fetcher at a test server.
func (f *SGSFetcher) WithBaseURL(u string) *SGSFetcher {
	f.baseURL = strings.TrimRight(u, "/")
	return f
}

// sgsSeries maps each index to its SGS series number.
//
// These numbers are the contract with BCB and are stable — but a wrong one here
// would silently adjust prices by the wrong index, so each is named.
var sgsSeries = map[domain.IndexCode]int{
	domain.IndexIPCA:  433, // IPCA — variação mensal (IBGE)
	domain.IndexIGPM:  189, // IGP-M — variação mensal (FGV)
	domain.IndexINPC:  188, // INPC — variação mensal (IBGE)
	domain.IndexIGPDI: 190, // IGP-DI — variação mensal (FGV)
}

// SupportedIndices is what this fetcher can read.
func SupportedIndices() []domain.IndexCode {
	return []domain.IndexCode{
		domain.IndexIPCA, domain.IndexIGPM, domain.IndexINPC, domain.IndexIGPDI,
	}
}

type sgsRow struct {
	// Data is dd/MM/yyyy, always the first of the reference month.
	Data string `json:"data"`
	// Valor is the monthly variation in percent, as a STRING with a decimal
	// point ("0.62"). Parsed rather than typed as float64 because SGS has
	// shipped both forms and a type error would drop the whole series.
	Valor string `json:"valor"`
}

// Fetch reads one index from `from` (inclusive) to today.
//
// Returns whatever it could parse: a single unreadable row must not discard the
// months around it, because the accumulation reports its own gaps.
func (f *SGSFetcher) Fetch(ctx context.Context, code domain.IndexCode, from time.Time) ([]domain.IndexValue, error) {
	series, ok := sgsSeries[code]
	if !ok {
		return nil, fmt.Errorf("economic: unknown index %q", code)
	}

	// SGS wants dd/MM/yyyy and rejects a range that ends in the future.
	url := fmt.Sprintf("%s/bcdata.sgs.%d/dados?formato=json&dataInicial=%s&dataFinal=%s",
		f.baseURL, series,
		domain.FirstOfMonth(from).Format("02/01/2006"),
		time.Now().UTC().Format("02/01/2006"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("economic: sgs series %d returned %d", series, resp.StatusCode)
	}
	// Bounded read: a public endpoint returning something enormous must not be
	// able to pull it all into memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	var rows []sgsRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("economic: sgs series %d payload: %w", series, err)
	}

	out := make([]domain.IndexValue, 0, len(rows))
	for _, row := range rows {
		month, err := time.Parse("02/01/2006", strings.TrimSpace(row.Data))
		if err != nil {
			continue
		}
		// Both separators accepted: the API documents a point and has been seen
		// with a comma on locale-sensitive deployments.
		rate, err := strconv.ParseFloat(
			strings.ReplaceAll(strings.TrimSpace(row.Valor), ",", "."), 64)
		if err != nil {
			continue
		}
		out = append(out, domain.IndexValue{
			Code:           code,
			ReferenceMonth: domain.FirstOfMonth(month),
			MonthlyRate:    rate,
		})
	}
	return out, nil
}
