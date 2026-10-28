// Package signals pulls public buying signals from Brazilian government
// sources.
package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// PNCPFetcher reads the Portal Nacional de Contratações Públicas.
//
// Every Brazilian public tender must be published there, which makes it the
// one place where "this hospital intends to buy monitors, by the 14th, with
// a budget" is a fact rather than an inference. It is free and has no key.
//
// The parser is deliberately tolerant of field spellings, the same way the
// inbound email parser is: PNCP's payload has changed shape before, and a
// rename must degrade one field rather than stop the whole feed.
type PNCPFetcher struct {
	client  *http.Client
	baseURL string
	// modalities are the tender types worth reading. Not all of them are:
	// "dispensa de licitação" is a purchase already decided, so a supplier
	// reading it is reading history.
	modalities []int
	// pace is the wait between pages. A field so tests run instantly; see
	// pncpPagePace for why it exists at all.
	pace time.Duration
	// now is the clock, swappable so a window test pins exact dates instead of
	// asserting on "roughly a year ago".
	now func() time.Time
	// sleep is the pause primitive, swappable in tests. Returns false when
	// the context ended during the wait.
	sleep func(ctx context.Context, d time.Duration) bool
}

// pncpPageTimeout is per PAGE, not per walk. Generous because a page that
// takes 40s is still a page of real tenders, and the walk runs on a 6h tick
// that nothing waits for.
//
// (It was raised from 30s when live pages appeared to hang. The hanging
// turned out to be rate limiting — see pncpPagePace — so this is no longer
// the fix for that; it is simply headroom.)
const pncpPageTimeout = 60 * time.Second

// pncpPagePace is the wait BETWEEN pages of one walk.
//
// PNCP is free, unauthenticated and rate limited, and the walker had no
// pacing at all: it fired maxPNCPPages × modalities × UFs requests back to
// back. Measured on 2026-09-09, that earns `status 429` on every modality —
// while a single request from the same host answers 200 in 1.0s. The feed
// was not failing because the source was down; it was failing because we
// were hammering it.
const pncpPagePace = time.Second

// pncpMaxRetriesOnBackoff bounds the wait-and-retry of one page. Beyond this
// the walk gives up and keeps what it has (the caller logs it), rather than
// pushing a source that is already saying no.
const pncpMaxRetriesOnBackoff = 3

// retryablePNCPStatus reports a status worth waiting out on the SAME page.
//
// 429 is the source asking us to slow down. The 5xx family is the portal
// being briefly unwell, which for PNCP is a normal weekday: on 2026-09-09 a
// single host got 429 on one walk and then 500, 502 and 503 on the next
// four, while isolated requests from the same machine answered 200 in under
// a second. Treating those as fatal aborted every walk for MT and left the
// radar with no tenders at all — which is half of what the product promises.
//
// 4xx other than 429 stays fatal: a bad request does not get better by
// being sent again.
func retryablePNCPStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}

// pncpRetryAfterCap bounds an absurd or hostile Retry-After. Without it a
// header saying "3600" parks a worker goroutine for an hour.
const pncpRetryAfterCap = 60 * time.Second

func NewPNCPFetcher() *PNCPFetcher {
	return &PNCPFetcher{
		client:  &http.Client{Timeout: pncpPageTimeout},
		pace:    pncpPagePace,
		baseURL: "https://pncp.gov.br/api/consulta/v1",
		now:     time.Now,
		// 6 = pregão eletrônico (the overwhelming majority of health
		// purchases), 8 = dispensa, 4 = concorrência. Dispensa is included
		// because in health it is often used for urgent buys, where a
		// supplier who can deliver tomorrow genuinely wins the next one.
		modalities: []int{6, 8, 4},
		sleep:      sleepCtx,
	}
}

// sleepCtx waits, or gives up early when the context ends. Returns false
// when the wait was cut short — the caller must then stop, not carry on
// requesting against a cancelled context.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// WithPace sets the between-page wait. Tests pass 0.
func (f *PNCPFetcher) WithPace(d time.Duration) *PNCPFetcher {
	f.pace = d
	return f
}

// WithBaseURL points the fetcher at a test server.
func (f *PNCPFetcher) WithBaseURL(u string) *PNCPFetcher {
	f.baseURL = strings.TrimRight(u, "/")
	return f
}

func (f *PNCPFetcher) Source() domain.SignalSource { return domain.SignalPNCP }

// pncpPage is one page of the publication endpoint.
type pncpPage struct {
	Data []pncpItem `json:"data"`
	// TotalPaginas is how PNCP reports pagination. Absent on some
	// deployments, so the loop also stops on an empty page.
	TotalPaginas int `json:"totalPaginas"`
}

type pncpItem struct {
	NumeroControlePNCP     string `json:"numeroControlePNCP"`
	ObjetoCompra           string `json:"objetoCompra"`
	InformacaoComplementar string `json:"informacaoComplementar"`

	ValorTotalEstimado float64 `json:"valorTotalEstimado"`

	DataPublicacaoPncp       string `json:"dataPublicacaoPncp"`
	DataEncerramentoProposta string `json:"dataEncerramentoProposta"`

	OrgaoEntidade struct {
		RazaoSocial string `json:"razaoSocial"`
		CNPJ        string `json:"cnpj"`
	} `json:"orgaoEntidade"`

	UnidadeOrgao struct {
		MunicipioNome string `json:"municipioNome"`
		UfSigla       string `json:"ufSigla"`
	} `json:"unidadeOrgao"`

	LinkSistemaOrigem string `json:"linkSistemaOrigem"`
}

// maxPNCPPages bounds one (UF, modality) walk.
//
// It was 10 (500 rows) on the estimate that "MT publishes ~35 pregões in
// three days". Measured on 2026-09-09, MT/30 days in modality 6 alone
// answers totalRegistros=560 — an order of magnitude over the estimate, so
// the 30-day backfill was truncating before it finished a single modality,
// and the tail it dropped is exactly the older-but-still-open tenders the
// backfill exists to find.
//
// 40 pages is 2000 rows, which covers a month of the busiest modality in a
// mid-sized state with headroom. The cost is time, not burst: with
// pncpPagePace between pages a full 40-page walk takes ~40s plus request
// time, on a 6h tick that nothing waits for. Most walks stop far earlier —
// the loop ends as soon as a page comes back short.
const maxPNCPPages = 40

// pncpPageSize is asked for explicitly. The default is 10, which with the
// page cap reads a hundred rows per query — a few percent of the national
// feed and, with no ordering guarantee, none of a small state's.
const pncpPageSize = 50

// Fetch pulls everything published since a date in the given UFs. An empty
// ufs reads the whole country (the old behaviour, kept for callers that
// have no territory).
func (f *PNCPFetcher) Fetch(ctx context.Context, since time.Time, ufs []string) ([]*domain.Signal, error) {
	var out []*domain.Signal
	seen := map[string]struct{}{}

	scopes := normalizeUFs(ufs)
	if len(scopes) == 0 {
		scopes = []string{""}
	}
	// walks/failed decide whether this pass reached the source at all.
	//
	// Every modality error used to be swallowed, so a walk where the portal
	// refused all 429 requests returned `(nothing, nil)` — indistinguishable
	// from a quiet week. That is how the radar could sit on eight-day-old
	// tenders while every probe said healthy. A pass that kept nothing AND
	// failed everywhere is the source being unreachable, and it says so.
	var walks, failed int
	var lastErr error
	first := true
	for _, uf := range scopes {
		for _, modality := range f.modalities {
			// Pacing pages is not enough on its own: the walks themselves
			// run back to back, so three modalities across N states is
			// still a burst at the boundaries. One pause before each walk
			// but the first.
			if !first && !f.wait(ctx, f.pace) {
				return out, nil
			}
			first = false

			walks++
			items, err := f.fetchModality(ctx, since, modality, uf)
			if err != nil {
				failed++
				lastErr = err
				// One modality failing must not lose the others: a partial
				// feed is worth more than an empty one, and the next run
				// retries.
				//
				// It must not lose ITS OWN pages either. fetchModality
				// returns what it read alongside the error, and this used
				// to discard that — a walk that died on page seven threw
				// away six pages of real tenders. `items` is now kept.
				//
				// And it must not be SILENT. A failed walk dropped a whole
				// modality with nothing in the log, so a poll that found
				// nothing looked identical to a poll that never asked —
				// which is what made "o radar não acha licitação"
				// impossible to diagnose from outside. The first thing this
				// warning said, once it existed, was `status 429`.
				slog.WarnContext(ctx, "pncp: modality walk incomplete",
					"uf", uf, "modality", modality,
					"kept", len(items), "error", err)
			}
			for _, s := range items {
				// The same tender can appear under more than one modality
				// query when it is amended mid-flight.
				if _, dup := seen[s.ExternalID]; dup {
					continue
				}
				seen[s.ExternalID] = struct{}{}
				out = append(out, s)
			}
		}
	}
	// Partial keeps its rows and still says it was partial.
	//
	// "A walk that died on page seven kept six pages of real tenders" is why
	// the rows are returned; it was NOT a reason to call the pass a success,
	// and calling it one is what let the radar sit on a feed that was mostly
	// missing while every stamp read healthy. On 2026-09-17 modalities 6 and
	// 4 for MT returned nothing at all, modality 8 kept 50, and the poll
	// recorded a clean read. Two thirds of the source went unread and the
	// screen had no way to know.
	//
	// Only "asked everywhere, got nothing, everywhere refused" is the source
	// being down; anything in between is incomplete, and says so.
	switch {
	case walks > 0 && failed == walks && len(out) == 0:
		return nil, fmt.Errorf("pncp: source unreachable (%d/%d walks failed): %w",
			failed, walks, lastErr)
	case failed > 0:
		return out, fmt.Errorf("pncp: %d of %d walks incomplete (%v): %w",
			failed, walks, lastErr, domain.ErrSignalSourcePartial)
	}
	return out, nil
}

// normalizeUFs upper-cases, trims, drops anything that is not a two-letter
// code and dedupes, keeping first-seen order so the request sequence is
// predictable.
func normalizeUFs(ufs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range ufs {
		uf := strings.ToUpper(strings.TrimSpace(raw))
		if len(uf) != 2 {
			continue
		}
		if _, dup := seen[uf]; dup {
			continue
		}
		seen[uf] = struct{}{}
		out = append(out, uf)
	}
	return out
}

// fetchModality walks one (UF, modality). An empty uf means national.
//
// Paced, and patient with a 429: the source telling us to slow down is not
// the source being broken, and the walk that gives up on it is the walk that
// leaves the feed empty.
func (f *PNCPFetcher) fetchModality(ctx context.Context, since time.Time, modality int, uf string) ([]*domain.Signal, error) {
	var out []*domain.Signal

	for page := 1; page <= maxPNCPPages; page++ {
		// Between pages, never before the first: a single-page walk (the
		// common case on the 48h window) must stay as fast as it was.
		if page > 1 && !f.wait(ctx, f.pace) {
			return out, ctx.Err()
		}

		body, err := f.fetchPage(ctx, since, modality, uf, page)
		if err != nil {
			return out, err
		}
		if body == nil {
			// 204: PNCP saying "nothing in this window". A normal answer,
			// and the most common one on a quiet day.
			return out, nil
		}

		for i := range body.Data {
			if s := normalizePNCP(body.Data[i]); s != nil {
				out = append(out, s)
			}
		}
		if len(body.Data) == 0 || (body.TotalPaginas > 0 && page >= body.TotalPaginas) {
			return out, nil
		}
	}
	return out, nil
}

// fetchPage reads one page, waiting out a 429 rather than surrendering the
// walk. Returns (nil, nil) for 204.
func (f *PNCPFetcher) fetchPage(
	ctx context.Context, since time.Time, modality int, uf string, page int,
) (*pncpPage, error) {
	q := url.Values{}
	// PNCP takes dates as yyyyMMdd, not RFC 3339.
	q.Set("dataInicial", since.Format("20060102"))
	q.Set("dataFinal", time.Now().Format("20060102"))
	q.Set("codigoModalidadeContratacao", fmt.Sprint(modality))
	q.Set("pagina", fmt.Sprint(page))
	q.Set("tamanhoPagina", fmt.Sprint(pncpPageSize))
	if uf != "" {
		q.Set("uf", uf)
	}
	reqURL := f.baseURL + "/contratacoes/publicacao?" + q.Encode()

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := f.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("pncp: %w", err)
		}

		if retryablePNCPStatus(resp.StatusCode) {
			status := resp.StatusCode
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			_ = resp.Body.Close()
			if attempt >= pncpMaxRetriesOnBackoff {
				return nil, fmt.Errorf("pncp: status %d after %d retries", status, attempt)
			}
			// Back off further each time even when the source names no
			// delay: the same pause that already failed will fail again.
			wait := retryAfter
			if wait <= 0 {
				wait = f.pace * time.Duration(1<<uint(attempt+1))
				if wait > pncpRetryAfterCap {
					wait = pncpRetryAfterCap
				}
			}
			slog.InfoContext(ctx, "pncp: transient status, backing off",
				"uf", uf, "modality", modality, "page", page, "status", status,
				"attempt", attempt+1, "wait", wait.String())
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
			return nil, fmt.Errorf("pncp: status %d", resp.StatusCode)
		}

		var body pncpPage
		decodeErr := json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("pncp: %w", decodeErr)
		}
		return &body, nil
	}
}

// wait pauses, honouring the fetcher's swappable sleep.
func (f *PNCPFetcher) wait(ctx context.Context, d time.Duration) bool {
	if f.sleep == nil {
		return sleepCtx(ctx, d)
	}
	return f.sleep(ctx, d)
}

// parseRetryAfter reads the header in its seconds form, capped. PNCP sends
// the HTTP-date form rarely enough that a failed parse simply falls back to
// the caller's own backoff.
func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	d := time.Duration(secs) * time.Second
	if d > pncpRetryAfterCap {
		d = pncpRetryAfterCap
	}
	return d
}

// normalizePNCP turns one item into a signal, or nil when it is unusable.
func normalizePNCP(item pncpItem) *domain.Signal {
	id := strings.TrimSpace(item.NumeroControlePNCP)
	title := strings.TrimSpace(item.ObjetoCompra)
	if id == "" || title == "" {
		// Without an id it cannot be deduplicated, and without an object
		// there is nothing to match a keyword against or to write about.
		return nil
	}

	s := &domain.Signal{
		Source:       domain.SignalPNCP,
		ExternalID:   id,
		Title:        title,
		Description:  strings.TrimSpace(item.InformacaoComplementar),
		Organization: strings.TrimSpace(item.OrgaoEntidade.RazaoSocial),
		CNPJ:         onlyDigits(item.OrgaoEntidade.CNPJ),
		City:         strings.TrimSpace(item.UnidadeOrgao.MunicipioNome),
		State:        strings.ToUpper(strings.TrimSpace(item.UnidadeOrgao.UfSigla)),
		URL:          strings.TrimSpace(item.LinkSistemaOrigem),
		PublishedAt:  parsePNCPTime(item.DataPublicacaoPncp),
	}
	// Reais to cents, rounded rather than truncated — a value of 1234.995
	// is R$ 1.235,00 to everyone who reads it.
	if item.ValorTotalEstimado > 0 {
		s.ValueCents = int64(item.ValorTotalEstimado*100 + 0.5)
	}
	if closes := parsePNCPTime(item.DataEncerramentoProposta); !closes.IsZero() {
		s.ClosesAt = &closes
	}
	if s.PublishedAt.IsZero() {
		// A signal with no date cannot be aged or ordered. Treating "now" as
		// the publication date is a small lie that keeps it usable, and it
		// is only ever wrong in the direction of showing it too long.
		s.PublishedAt = time.Now()
	}
	return s
}

// pncpTimeLayouts are the shapes the API has used. Tried in order.
var pncpTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func parsePNCPTime(raw string) time.Time {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}
	}
	for _, layout := range pncpTimeLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var _ domain.SignalFetcher = (*PNCPFetcher)(nil)
