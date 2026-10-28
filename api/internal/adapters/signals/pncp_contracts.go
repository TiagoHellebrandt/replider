package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Who a hospital buys from today, from the public record.
//
// PNCP's /contratos endpoint answers with the SUPPLIER on every signed
// contract — name, CNPJ, object, value, and when it runs out. Filtered by the
// buying agency's CNPJ, which cnes_establishments already stores, it turns
// "who is the incumbent here" from a guess into a citation.
//
// Verified against the live API before this was written: the endpoint exists,
// accepts cnpjOrgao, and the field names below are the ones it returns. The
// alternative — building against a remembered schema — is how a feature ships
// reading nulls forever.

const (
	// PNCP rejects an unbounded window; a contract older than this says
	// little about who serves the hospital now.
	// Three years, matching what the dossier considers a relevant incumbent
	// (contractRelevance). Each year is one request, so widening this is not
	// free: it multiplies across every buyer in the territory.
	contractLookback = 3 * 365 * 24 * time.Hour
	// Page size the endpoint accepts (10 is refused below, 500 above).
	contractPageSize = 50
	// One page is plenty: a hospital signs tens of contracts a year, and the
	// newest are the ones that say who is there now.
	maxContractPages = 2
)

// pncpContract is the answer's shape, trimmed to what a proposal can use.
type pncpContract struct {
	NumeroControlePNCP        string  `json:"numeroControlePNCP"`
	NiFornecedor              string  `json:"niFornecedor"`
	NomeRazaoSocialFornecedor string  `json:"nomeRazaoSocialFornecedor"`
	ObjetoContrato            string  `json:"objetoContrato"`
	ValorGlobal               float64 `json:"valorGlobal"`
	DataAssinatura            string  `json:"dataAssinatura"`
	DataVigenciaFim           string  `json:"dataVigenciaFim"`
	// The price registry this was bought under, and whether the buyer rode
	// somebody else's. Both arrive in every row and were parsed past.
	NumeroControlePncpAta string `json:"numeroControlePncpAta"`
	FrutoAdesao           bool   `json:"frutoAdesao"`

	OrgaoEntidade struct {
		CNPJ        string `json:"cnpj"`
		RazaoSocial string `json:"razaoSocial"`
	} `json:"orgaoEntidade"`

	UnidadeOrgao struct {
		MunicipioNome string `json:"municipioNome"`
		UfSigla       string `json:"ufSigla"`
	} `json:"unidadeOrgao"`
}

type pncpContractPage struct {
	Data []pncpContract `json:"data"`
}

// ContractsForBuyer returns the contracts an agency signed, newest first.
//
// Empty is a real answer: plenty of hospitals buy without a registered
// procurement, and reporting "no incumbent found" is honest where inventing
// one is not.
func (f *PNCPFetcher) ContractsForBuyer(ctx context.Context, cnpj string) ([]domain.PublicContract, error) {
	cnpj = onlyDigits(cnpj)
	if len(cnpj) != 14 {
		// Not an error: an establishment with no CNPJ on file simply cannot
		// be asked about.
		return nil, nil
	}

	out, err := f.contractsForCNPJ(ctx, cnpj)
	if err != nil || len(out) > 0 {
		return out, err
	}

	// THE TWO REGISTERS DISAGREE ABOUT WHICH CNPJ IS THE BUYER.
	//
	// CNES records the HEALTH UNIT, which is a branch — "...0002-67". PNCP
	// publishes that body's contracts under the HEAD OFFICE — "...0001-86".
	// Asking the number we have therefore returns 204 for a município that
	// has contracts, and the buying calendar stays empty while looking swept.
	// Verified against the live portal on 2026-09-11: for the same município,
	// the branch answered 204 and the root answered 200 with a contract.
	//
	// Asked SECOND, not instead: some bodies do publish under a branch (the
	// state of Mato Grosso publishes under "...0028-64"), so the number on the
	// establishment is still the most specific answer when it has one.
	root := domain.CNPJRoot(cnpj)
	if root == "" || root == cnpj {
		return out, nil
	}
	if !f.wait(ctx, f.pace) {
		return out, ctx.Err()
	}
	return f.contractsForCNPJ(ctx, root)
}

// contractsForCNPJ reads one buyer's contracts, a year at a time.
func (f *PNCPFetcher) contractsForCNPJ(ctx context.Context, cnpj string) ([]domain.PublicContract, error) {
	out := make([]domain.PublicContract, 0, contractPageSize)
	// One request per YEAR, because the portal refuses anything wider.
	//
	// This asked for the whole lookback in a single call and PNCP answered, in
	// Portuguese, "Período maior que 365 dias." — a 422 on every request, for
	// every buyer, since the call was written. The dossier's incumbent-supplier
	// lookup therefore never worked: it logged a WARN nobody read and carried
	// on with no supplier, which reads exactly like "this hospital has no
	// registered contracts". The contracts table held zero rows and looked
	// like an empty territory rather than a broken query.
	for i, w := range yearWindows(f.now(), contractLookback) {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		// Between windows, never before the first. One buyer is now several
		// requests, and firing them back to back is how the sweep earned a
		// wall of 429s on its first live run.
		if i > 0 && !f.wait(ctx, f.pace) {
			return out, ctx.Err()
		}
		found, err := f.contractsInWindow(ctx, cnpj, w.from, w.to)
		if err != nil {
			return out, err
		}
		out = append(out, found...)

		// THE FIRST YEAR IS THE QUALIFYING QUESTION.
		//
		// Nothing signed in the last twelve months means this is not a public
		// buyer, and asking about the two years before that is spending the
		// portal's patience to confirm it. Measured on the first live sweep of
		// Mato Grosso: twenty-seven establishments in a row answered "nothing"
		// for the current year — drugstores, private clinics, a diagnostics
		// lab — and every one of them then cost two more requests, several of
		// which timed out after sixty seconds each.
		//
		// Most of a state's CNES register is private business that never buys
		// publicly. The bodies this feature is for — prefeituras, the state,
		// their autarquias — sign contracts continuously, so a silent year is
		// not a thing that happens to them.
		//
		// The trade is real and small: a three-year contract signed 25 months
		// ago, at a buyer who has signed nothing since, is missed. That buyer
		// is asked again a fortnight later, and if they publish anything at
		// all in the meantime the full history follows.
		if i == 0 && len(found) == 0 {
			return out, nil
		}
	}
	return out, nil
}

// contractWindow is one request's worth of time.
type contractWindow struct{ from, to time.Time }

// yearWindows splits a lookback into spans the portal will accept.
//
// Newest first: a sweep that gives up halfway (a refusal, a cancelled context)
// should have collected the contracts most likely to still be running, not the
// oldest ones.
//
// 364 days rather than 365 — the portal compares inclusively, and a window
// exactly at the limit is a window that fails on a leap year.
func yearWindows(now time.Time, lookback time.Duration) []contractWindow {
	const span = 364 * 24 * time.Hour
	oldest := now.Add(-lookback)
	out := make([]contractWindow, 0, 4)
	for to := now; to.After(oldest); to = to.Add(-span) {
		from := to.Add(-span)
		if from.Before(oldest) {
			from = oldest
		}
		out = append(out, contractWindow{from: from, to: to})
	}
	return out
}

// contractsInWindow pages one accepted span.
func (f *PNCPFetcher) contractsInWindow(
	ctx context.Context, cnpj string, from, to time.Time,
) ([]domain.PublicContract, error) {
	out := make([]domain.PublicContract, 0, contractPageSize)
	for page := 1; page <= maxContractPages; page++ {
		q := url.Values{}
		q.Set("dataInicial", from.Format("20060102"))
		q.Set("dataFinal", to.Format("20060102"))
		q.Set("cnpjOrgao", cnpj)
		q.Set("pagina", fmt.Sprint(page))
		q.Set("tamanhoPagina", fmt.Sprint(contractPageSize))

		body, err := f.contractPage(ctx, q)
		if err != nil {
			return out, err
		}
		if body == nil {
			// 204: nothing in this span. An answer, and the common one —
			// most establishments are private and buy nothing publicly.
			return out, nil
		}
		for _, c := range body.Data {
			out = append(out, toPublicContract(c))
		}
		if len(body.Data) < contractPageSize {
			break
		}
	}
	return out, nil
}

// contractPage reads one page, waiting out a 429 rather than surrendering.
//
// The tender walk has done this since September; this call did not, and the
// difference showed the first time the territory sweep ran: every buyer in
// Cuiabá came back `status 429` in a row, because a refusal was treated as a
// permanent failure and the loop moved straight on to hammer the next one.
//
// The portal telling us to slow down is not the portal being broken. A worker
// that keeps knocking after being asked to wait is how a free government API
// stops answering this IP at all.
func (f *PNCPFetcher) contractPage(ctx context.Context, q url.Values) (*pncpContractPage, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			f.baseURL+"/contratos?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := f.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("pncp contracts: %w", err)
		}

		if retryablePNCPStatus(resp.StatusCode) {
			status := resp.StatusCode
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			_ = resp.Body.Close()
			if attempt >= pncpMaxRetriesOnBackoff {
				return nil, fmt.Errorf("pncp contracts: status %d after %d retries", status, attempt)
			}
			wait := retryAfter
			if wait <= 0 {
				// Doubling, and from a base an order of magnitude above the
				// page pace: the same pause that already failed will fail
				// again, and this walk is a background sweep nobody is
				// waiting on.
				wait = contractRetryBase * time.Duration(1<<uint(attempt))
				if wait > pncpRetryAfterCap {
					wait = pncpRetryAfterCap
				}
			}
			slog.InfoContext(ctx, "pncp contracts: transient status, backing off",
				"status", status, "attempt", attempt+1, "wait", wait.String())
			if !f.wait(ctx, wait) {
				return nil, ctx.Err()
			}
			continue
		}
		if resp.StatusCode == http.StatusNoContent {
			_ = resp.Body.Close()
			return nil, nil
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("pncp contracts: status %d", resp.StatusCode)
		}
		var pageOut pncpContractPage
		decodeErr := json.NewDecoder(resp.Body).Decode(&pageOut)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("pncp contracts: decode: %w", decodeErr)
		}
		return &pageOut, nil
	}
}

// contractRetryBase is the first pause after a refusal.
//
// Seconds, not milliseconds. This is a background sweep of a territory that
// refreshes over days; nobody is waiting on the next hospital, and the only
// thing haste buys here is a longer ban.
const contractRetryBase = 4 * time.Second

func toPublicContract(c pncpContract) domain.PublicContract {
	out := domain.PublicContract{
		PNCPID:       strings.TrimSpace(c.NumeroControlePNCP),
		BuyerCNPJ:    onlyDigits(c.OrgaoEntidade.CNPJ),
		BuyerName:    strings.TrimSpace(c.OrgaoEntidade.RazaoSocial),
		SupplierCNPJ: onlyDigits(c.NiFornecedor),
		SupplierName: strings.TrimSpace(c.NomeRazaoSocialFornecedor),
		Object:       strings.TrimSpace(c.ObjetoContrato),
		// Reais to cents, rounded rather than truncated: a contract of
		// R$ 7.000,00 must not become R$ 6.999,99.
		ValueCents: int64(c.ValorGlobal*100 + 0.5),
		UF:         strings.ToUpper(strings.TrimSpace(c.UnidadeOrgao.UfSigla)),
		City:       strings.TrimSpace(c.UnidadeOrgao.MunicipioNome),
	}
	out.AtaPNCPID = strings.TrimSpace(c.NumeroControlePncpAta)
	out.FromAdhesion = c.FrutoAdesao
	out.SignedAt = sanePNCPDate(parsePNCPDate(c.DataAssinatura))
	out.ExpiresAt = sanePNCPDate(parsePNCPDate(c.DataVigenciaFim))
	return out
}

// sanePNCPDate drops a date the source clearly got wrong.
//
// Not defensive programming for its own sake: a live ata read on 2026-09-11
// published "vigenciaFim": "7203-07-27". A typo'd year is indistinguishable
// from a real one to a parser, and it survives into a calendar as a contract
// that never expires — sorting last forever, and telling a salesperson the
// window is five thousand years away. Better to admit the date is unknown.
func sanePNCPDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	if t.Year() < 1990 || t.Year() > 2100 {
		return nil
	}
	return t
}

// parsePNCPDate reads the "2024-12-31" the contracts endpoint returns. An
// unparseable date is nil, never today: a fabricated expiry is a fabricated
// buying window.
func parsePNCPDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
