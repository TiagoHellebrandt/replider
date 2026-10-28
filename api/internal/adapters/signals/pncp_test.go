package signals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PNCP is a free government API with no key and a payload that has changed
// shape before. The parser must degrade one field rather than lose the feed.

func pncpServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func writePage(w http.ResponseWriter, items []map[string]any, totalPages int) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": items, "totalPaginas": totalPages,
	})
}

func tenderItem() map[string]any {
	return map[string]any{
		"numeroControlePNCP":       "00000000-1-000001/2026",
		"objetoCompra":             "Aquisição de monitores multiparâmetro",
		"informacaoComplementar":   "Registro de preços para 12 meses",
		"valorTotalEstimado":       450000.0,
		"dataPublicacaoPncp":       "2026-03-08T10:00:00",
		"dataEncerramentoProposta": "2026-03-20T18:00:00",
		"orgaoEntidade": map[string]any{
			"razaoSocial": "Hospital Municipal de Sorocaba",
			"cnpj":        "12.345.678/0001-95",
		},
		"unidadeOrgao": map[string]any{
			"municipioNome": "Sorocaba", "ufSigla": "sp",
		},
		"linkSistemaOrigem": "https://pncp.gov.br/app/editais/1",
	}
}

func TestPNCP_ParsesATender(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writePage(w, []map[string]any{tenderItem()}, 1)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now().Add(-24*time.Hour), nil)

	require.NoError(t, err)
	require.NotEmpty(t, got)
	s := got[0]
	assert.Equal(t, domain.SignalPNCP, s.Source)
	assert.Equal(t, "00000000-1-000001/2026", s.ExternalID)
	assert.Equal(t, "Hospital Municipal de Sorocaba", s.Organization)
	// Digits only: the CNPJ is a join key to everything else we know about
	// this company, and a punctuated one joins to nothing.
	assert.Equal(t, "12345678000195", s.CNPJ)
	assert.Equal(t, "SP", s.State)
	assert.Equal(t, int64(45000000), s.ValueCents)
	require.NotNil(t, s.ClosesAt)
}

// The same tender appears under more than one modality query when it is
// amended mid-flight, and a salesperson who sees it three times stops
// reading them.
func TestPNCP_DeduplicatesAcrossModalities(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writePage(w, []map[string]any{tenderItem()}, 1)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now().Add(-24*time.Hour), nil)

	require.NoError(t, err)
	assert.Len(t, got, 1, "three modality queries, one tender")
}

// 204 is PNCP's "nothing in this window" and the most common answer on a
// quiet day. Reading it as an error would fill the log with false alarms.
func TestPNCP_NoContentIsNotAnError(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now(), nil)

	require.NoError(t, err)
	assert.Empty(t, got)
}

// A source that refuses EVERY request is not a quiet week, and this used to
// report the two as the same thing: nothing, no error. That silence is what
// let the radar show eight-day-old tenders as today's while PNCP was down.
//
// The next run still retries and no other source is affected — what changed
// is that the caller can now tell the difference and say so on screen.
func TestPNCP_TotalRefusalIsReportedAsAnOutage(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unreachable")
	assert.Empty(t, got)
}

// Partial is still a success. A walk that died on page seven kept six pages
// of real tenders, and calling that an outage would blank a feed that has
// something in it.
// A partial walk keeps its rows AND says it was partial.
//
// It used to say only the first half. On 2026-09-17 two of three modality
// walks for MT returned nothing, modality 8 kept 50, and the poll recorded a
// clean read — the radar showed one tender and claimed the feed was current.
func TestPNCP_APartialWalkKeepsItsRowsAndReportsItself(t *testing.T) {
	var calls int
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			writePage(w, []map[string]any{tenderItem()}, 2)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now(), []string{"MT"})

	assert.Len(t, got, 1, "one page of real tenders is a result, not an outage")
	require.ErrorIs(t, err, domain.ErrSignalSourcePartial,
		"the caller has to be able to tell a full read from this one")
}

// An item with no id cannot be deduplicated, and one with no object has
// nothing to match a keyword against or to write about.
func TestPNCP_DropsUnusableItems(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		noID := tenderItem()
		noID["numeroControlePNCP"] = ""
		noObject := tenderItem()
		noObject["numeroControlePNCP"] = "outro"
		noObject["objetoCompra"] = ""
		writePage(w, []map[string]any{noID, noObject, tenderItem()}, 1)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now(), nil)

	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// A tender with no published value is common, and zero must not be read as
// "R$ 0,00" by anything downstream.
func TestPNCP_UndisclosedValueStaysZero(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		item := tenderItem()
		delete(item, "valorTotalEstimado")
		writePage(w, []map[string]any{item}, 1)
	})

	got, _ := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).Fetch(context.Background(), time.Now(), nil)

	require.NotEmpty(t, got)
	assert.Zero(t, got[0].ValueCents)
}

// A missing publication date must not produce a signal that cannot be
// ordered or aged.
func TestPNCP_MissingDateFallsBackToNow(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		item := tenderItem()
		item["dataPublicacaoPncp"] = ""
		writePage(w, []map[string]any{item}, 1)
	})

	got, _ := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).Fetch(context.Background(), time.Now(), nil)

	require.NotEmpty(t, got)
	assert.False(t, got[0].PublishedAt.IsZero())
}

// The page walk has to stop. An unbounded one spends an hour and a rate
// limit on a backlog nobody asked for.
func TestPNCP_PageWalkIsBounded(t *testing.T) {
	var requests int
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		// Always a full page and never a last-page marker: without the cap
		// this loops forever.
		writePage(w, []map[string]any{tenderItem()}, 0)
	})

	_, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).Fetch(context.Background(), time.Now(), nil)

	require.NoError(t, err)
	assert.LessOrEqual(t, requests, maxPNCPPages*3, "3 modalities x %d pages", maxPNCPPages)
}

// Garbage must not take the PROCESS down with it: no panic, nothing half
// parsed, the other sources untouched. But a maintenance page on every walk
// means no tender was read, and that is reported rather than served as
// silence — a 200 carrying HTML is a source you cannot read.
func TestPNCP_MalformedBodyIsSurvivableAndReported(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>manutenção</html>"))
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).Fetch(context.Background(), time.Now(), nil)

	require.Error(t, err)
	assert.Empty(t, got)
}

// The feed is per state. PNCP pages nationally with no ordering guarantee,
// so a bounded national walk reads a few percent of the register and, for
// a small state, none of it. Every request must carry the UF and ask for a
// full page.
func TestPNCP_FetchesEachUFWithAFullPage(t *testing.T) {
	var ufs []string
	var pageSizes []string
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		ufs = append(ufs, r.URL.Query().Get("uf"))
		pageSizes = append(pageSizes, r.URL.Query().Get("tamanhoPagina"))
		item := tenderItem()
		item["numeroControlePNCP"] = "tender-" + r.URL.Query().Get("uf")
		writePage(w, []map[string]any{item}, 1)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).
		Fetch(context.Background(), time.Now(), []string{"mt", " SP ", "mt", "x"})

	require.NoError(t, err)
	// 2 valid UFs (deduped, normalised) x 3 modalities; the junk one dropped.
	require.Len(t, ufs, 6)
	for _, uf := range ufs {
		assert.Contains(t, []string{"MT", "SP"}, uf)
	}
	assert.Equal(t, "MT", ufs[0])
	assert.Equal(t, "SP", ufs[3])
	for _, size := range pageSizes {
		assert.Equal(t, "50", size)
	}
	assert.Len(t, got, 2, "one tender per UF, deduped across modalities")
}

// No UFs means the whole country — the old behaviour, kept for callers with
// no territory — but still with a full page, not the 10-row default.
func TestPNCP_NoUFsIsNationalWithAFullPage(t *testing.T) {
	var requests int
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, hasUF := r.URL.Query()["uf"]
		assert.False(t, hasUF, "national read must not send an empty uf")
		assert.Equal(t, "50", r.URL.Query().Get("tamanhoPagina"))
		writePage(w, []map[string]any{tenderItem()}, 1)
	})

	got, err := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL).Fetch(context.Background(), time.Now(), nil)

	require.NoError(t, err)
	assert.Equal(t, 3, requests, "one query per modality")
	assert.Len(t, got, 1)
}

// A walk that dies mid-way used to lose everything it had already read, and
// said nothing about it. PNCP is free, unauthenticated and visibly bursty —
// on 2026-09-08 a live MT page did not answer inside 45s, having answered in
// seconds two hours before — so this is the ordinary case, not the exotic
// one. A poll that never asked and a poll that found nothing must not look
// identical in the log.

func TestPNCPFetch_KeepsThePagesItReadBeforeFailing(t *testing.T) {
	var pages int
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		// Fail the third page of the FIRST modality only; the rest answer
		// an empty page so the walk ends.
		if pages == 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if pages > 2 {
			_ = json.NewEncoder(w).Encode(pncpPage{Data: nil})
			return
		}
		_ = json.NewEncoder(w).Encode(pncpPage{Data: []pncpItem{
			pncpItemFixture("page-" + r.URL.Query().Get("pagina")),
		}})
	})

	f := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL)
	out, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})

	// The 502 is retried and the retry answers an empty page, so this walk
	// RECOVERS — no partial flag. What it pins is the other half: the two
	// pages read before the stumble are not thrown away.
	require.NoError(t, err)
	require.Len(t, out, 2,
		"the two pages read before the failure are real tenders and must survive")
}

func TestPNCPFetch_SaysSoWhenAModalityWalkIsIncomplete(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	})

	f := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL)
	_, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})

	require.Error(t, err, "every walk refused: the caller must be able to say so")
	logged := buf.String()
	assert.Contains(t, logged, "pncp: modality walk incomplete")
	assert.Contains(t, logged, "uf=MT")
	assert.Contains(t, logged, "504",
		"the status has to reach the log or the warning cannot be acted on")
}

func TestPNCPFetch_QuietWhenNothingFailed(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	f := NewPNCPFetcher().WithPace(0).WithBaseURL(srv.URL)
	_, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})

	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "walk incomplete",
		"204 is PNCP saying 'nothing this window', not a failure")
}

// pncpItemFixture is the minimum PNCP accepts as a usable tender: an id and
// an object. Anything less is dropped by normalizePNCP.
func pncpItemFixture(id string) pncpItem {
	var it pncpItem
	it.NumeroControlePNCP = id
	it.ObjetoCompra = "Aquisição de monitores multiparâmetro"
	it.OrgaoEntidade.RazaoSocial = "Hospital Municipal"
	it.UnidadeOrgao.UfSigla = "MT"
	return it
}

// Measured on 2026-09-09: the unpaced walker earned `status 429` on every
// modality, while a single request from the same host answered 200 in 1.0s.
// The feed was not failing because PNCP was down — it was failing because we
// were hammering a free, unauthenticated, rate-limited API.

// paced builds a fetcher whose waits are recorded instead of slept.
func paced(t *testing.T, url string) (*PNCPFetcher, *[]time.Duration) {
	t.Helper()
	var waits []time.Duration
	f := NewPNCPFetcher().WithBaseURL(url)
	f.pace = time.Second
	f.sleep = func(ctx context.Context, d time.Duration) bool {
		waits = append(waits, d)
		return ctx.Err() == nil
	}
	return f, &waits
}

func TestPNCPFetch_PausesBetweenPagesButNotBeforeTheFirst(t *testing.T) {
	var hits int
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		// Two full pages then a short one, for the FIRST modality; the
		// other two answer 204 immediately.
		if hits <= 2 {
			data := make([]pncpItem, pncpPageSize)
			for i := range data {
				data[i] = pncpItemFixture(fmt.Sprintf("h%d-i%d", hits, i))
			}
			_ = json.NewEncoder(w).Encode(pncpPage{Data: data})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	f, waits := paced(t, srv.URL)
	_, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})
	require.NoError(t, err)

	// One walk of 3 pages = 2 inter-page pauses; 3 modalities = 2 more
	// pauses at the walk boundaries. The very first request waits for
	// nothing, so a one-page poll is as fast as it ever was.
	require.Len(t, *waits, 4)
	for _, d := range *waits {
		assert.Equal(t, time.Second, d)
	}
}

func TestPNCPFetch_WaitsOutA429AndFinishesTheWalk(t *testing.T) {
	var hits int
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if hits == 2 {
			_ = json.NewEncoder(w).Encode(pncpPage{Data: []pncpItem{
				pncpItemFixture("survived-the-429"),
			}})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	f, waits := paced(t, srv.URL)
	out, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})

	require.NoError(t, err)
	require.Len(t, out, 1, "a 429 is 'slow down', not 'give up' — the page must be re-read")
	assert.Equal(t, "survived-the-429", out[0].ExternalID)
	assert.Contains(t, *waits, 2*time.Second, "Retry-After is obeyed, not ignored")
}

func TestPNCPFetch_BacksOffFurtherWhenNoRetryAfterIsGiven(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests) // no header, always
	})

	f, waits := paced(t, srv.URL)
	_, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})
	require.Error(t, err, "rate limited everywhere means nothing was read, and that is reported")

	// Repeating the pause that just failed is not a backoff. Per walk:
	// 2s, 4s, 8s, then give up (pncpMaxRetriesOnBackoff).
	require.GreaterOrEqual(t, len(*waits), 3)
	assert.Equal(t, 2*time.Second, (*waits)[0])
	assert.Equal(t, 4*time.Second, (*waits)[1])
	assert.Equal(t, 8*time.Second, (*waits)[2])
}

func TestPNCPFetch_CapsAnAbsurdRetryAfter(t *testing.T) {
	assert.Equal(t, pncpRetryAfterCap, parseRetryAfter("3600"),
		"a header must not park a worker goroutine for an hour")
	assert.Equal(t, 5*time.Second, parseRetryAfter(" 5 "))
	assert.Equal(t, time.Duration(0), parseRetryAfter("Wed, 21 Oct 2026 07:28:00 GMT"),
		"the HTTP-date form falls back to our own backoff rather than guessing")
	assert.Equal(t, time.Duration(0), parseRetryAfter(""))
}

func TestPNCPFetch_StopsWhenTheContextEndsMidWait(t *testing.T) {
	srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	ctx, cancel := context.WithCancel(context.Background())
	f := NewPNCPFetcher().WithBaseURL(srv.URL)
	f.pace = time.Second
	f.sleep = func(context.Context, time.Duration) bool {
		cancel()
		return false // the wait was cut short
	}

	_, err := f.Fetch(ctx, time.Now().Add(-48*time.Hour), []string{"MT"})
	assert.NoError(t, err, "a cancelled fetch returns what it has, it does not error")
}

// The 2026-09-09 production shape: the errors moved off 429 and onto the 5xx
// family, and only 429 was retried — so every MT walk aborted at page one and
// the radar showed no tenders at all. A 503 from a portal that answers 200 a
// second later is the same "wait and try again" a 429 is.
func TestPNCPFetch_WaitsOutATransient5xx(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls int
			srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					w.WriteHeader(status)
					return
				}
				if calls == 2 {
					_ = json.NewEncoder(w).Encode(pncpPage{Data: []pncpItem{
						pncpItemFixture("survived-the-5xx"),
					}})
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})

			f, waits := paced(t, srv.URL)
			out, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})

			require.NoError(t, err)
			require.Len(t, out, 1, "a transient 5xx must be waited out, not treated as an outage")
			assert.Equal(t, "survived-the-5xx", out[0].ExternalID)
			assert.NotEmpty(t, *waits)
		})
	}
}

// A 4xx that is not 429 is a request that will never work. Retrying it three
// times just spends three times as long being wrong. Measured against a
// walk that answers 204, so the assertion is "no extra calls", not a
// hardcoded count that moves when the modality list changes.
func TestPNCPFetch_DoesNotRetryAClientError(t *testing.T) {
	// Returns how many requests the walk cost, and whether the pass reported
	// the source as unreadable. The two are separate claims: a 400 is not
	// retried (cost) AND leaves nothing to show (outcome), while a 204 is not
	// retried either but is a perfectly good empty week.
	probe := func(status int) (calls int, failed bool) {
		srv := pncpServer(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(status)
		})
		f, _ := paced(t, srv.URL)
		_, err := f.Fetch(context.Background(), time.Now().Add(-48*time.Hour), []string{"MT"})
		return calls, err != nil
	}

	empty, emptyFailed := probe(http.StatusNoContent)
	bad, badFailed := probe(http.StatusBadRequest)

	assert.Equal(t, empty, bad,
		"a 400 must cost exactly one request per walk, same as an empty page")
	assert.False(t, emptyFailed, "an empty window is a quiet week, not an outage")
	assert.True(t, badFailed, "a refused request read no tenders and must say so")
}

func TestRetryablePNCPStatus(t *testing.T) {
	for _, code := range []int{429, 500, 502, 503, 504} {
		assert.True(t, retryablePNCPStatus(code), "status %d should be waited out", code)
	}
	for _, code := range []int{200, 204, 400, 401, 403, 404} {
		assert.False(t, retryablePNCPStatus(code), "status %d should be fatal", code)
	}
}
