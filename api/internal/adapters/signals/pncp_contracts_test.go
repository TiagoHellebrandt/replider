package signals

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The fixture is a REAL response from pncp.gov.br, captured before this was
// written. Building against a remembered schema is how a feature ships reading
// nulls forever — and this one exists to put a supplier's name in a message to
// the hospital that signed with them, where a wrong field is not a bug, it is
// an accusation.
const realPNCPContract = `{"data":[{
  "numeroControlePNCP":"00394460005887-2-007096/2024",
  "niFornecedor":"06121879000106",
  "nomeRazaoSocialFornecedor":"IMPERIO TRANSPORTES TURISTICOS LTDA",
  "objetoContrato":"Alienação de mercadorias apreendidas pela Receita Federal",
  "valorGlobal":7000.0,
  "dataAssinatura":"2024-12-31",
  "dataVigenciaFim":"2025-01-30",
  "orgaoEntidade":{"cnpj":"00394460005887","razaoSocial":"MINISTERIO DA ECONOMIA"},
  "unidadeOrgao":{"ufSigla":"PR","municipioNome":"Curitiba"}
}]}`

func TestContractsForBuyer_ReadsTheRealShape(t *testing.T) {
	var asked string
	var served bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("cnpjOrgao")
		// The lookback is walked one year-window at a time, so only the first
		// span carries this contract; the rest are empty, as they are in life.
		if served {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		served = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realPNCPContract))
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL

	got, err := f.ContractsForBuyer(context.Background(), "00.394.460/0058-87")
	if err != nil {
		t.Fatalf("ContractsForBuyer: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d contracts", len(got))
	}
	c := got[0]
	// The supplier is the whole point.
	if c.SupplierName != "IMPERIO TRANSPORTES TURISTICOS LTDA" || c.SupplierCNPJ != "06121879000106" {
		t.Fatalf("supplier lost: %+v", c)
	}
	// R$ 7.000,00 must not become R$ 6.999,99.
	if c.ValueCents != 700000 {
		t.Fatalf("value = %d cents", c.ValueCents)
	}
	if c.ExpiresAt == nil || c.ExpiresAt.Format("2006-01-02") != "2025-01-30" {
		t.Fatalf("expiry lost: %+v", c.ExpiresAt)
	}
	// Every claim built on this has to be checkable by whoever receives it.
	if c.URL() != "https://pncp.gov.br/app/contratos/00394460005887-2-007096/2024" {
		t.Fatalf("url = %q", c.URL())
	}
	// The CNPJ goes out as digits, whatever punctuation it arrived with.
	if asked != "00394460005887" {
		t.Fatalf("asked cnpjOrgao=%q", asked)
	}
}

// Plenty of hospitals buy without a registered procurement. "No incumbent
// found" is honest; inventing one is not.
func TestContractsForBuyer_EmptyIsAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL

	got, err := f.ContractsForBuyer(context.Background(), "00394460005887")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
}

// An establishment with no CNPJ on file cannot be asked about — and that is
// not a failure worth an error.
func TestContractsForBuyer_NoCNPJIsQuiet(t *testing.T) {
	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = "http://127.0.0.1:1" // must never be dialled
	got, err := f.ContractsForBuyer(context.Background(), "")
	if err != nil || got != nil {
		t.Fatalf("got %v err %v", got, err)
	}
}

// A date PNCP could not give must be absent, never today: a fabricated expiry
// is a fabricated buying window.
func TestParsePNCPDate_RefusesGarbage(t *testing.T) {
	for _, s := range []string{"", "ontem", "31/12/2024"} {
		if got := parsePNCPDate(s); got != nil {
			t.Fatalf("%q became %v", s, got)
		}
	}
	if got := parsePNCPDate("2025-01-30T00:00:00"); got == nil ||
		got.Format("2006-01-02") != "2025-01-30" {
		t.Fatalf("timestamp form rejected: %v", got)
	}
}

// Guard: the fixture above must stay valid JSON of the shape the decoder
// expects, so a careless edit fails here rather than in production.
func TestRealFixtureIsWellFormed(t *testing.T) {
	var page pncpContractPage
	if err := json.Unmarshal([]byte(realPNCPContract), &page); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].NomeRazaoSocialFornecedor == "" {
		t.Fatal("fixture no longer carries a supplier")
	}
}

// The portal asking us to slow down is not the portal being broken.
//
// The tender walk has waited out a 429 since September; this call did not, and
// the first live territory sweep proved the difference — every buyer in Cuiabá
// came back `status 429` in a row, because a refusal was treated as permanent
// and the sweep moved straight on to hammer the next one.
func TestContractsForBuyer_WaitsOutA429(t *testing.T) {
	var calls, served int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if served > 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		served++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realPNCPContract))
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL
	// The wait itself is the behaviour; the duration is not, so tests do not
	// pay four seconds to observe it.
	f.sleep = func(context.Context, time.Duration) bool { return true }

	got, err := f.ContractsForBuyer(context.Background(), "00394460005887")
	if err != nil {
		t.Fatalf("a refusal must be waited out, not surrendered to: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d contracts after the retry", len(got))
	}
	if calls < 2 {
		t.Fatalf("asked %d times, want at least one refusal and one answer", calls)
	}
}

// The portal caps a date range at a year, and says so in Portuguese:
// "Período maior que 365 dias." This asked for the whole lookback in one call
// and took a 422 on every request for every buyer since the day it was
// written — so the dossier never named an incumbent supplier, and the contracts
// table sat at zero rows looking like an empty territory.
func TestContractsForBuyer_NeverAsksForMoreThanAYear(t *testing.T) {
	type span struct{ from, to string }
	var spans []span
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		spans = append(spans, span{q.Get("dataInicial"), q.Get("dataFinal")})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL
	f.now = func() time.Time { return time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC) }

	if _, err := f.ContractsForBuyer(context.Background(), "00394460005887"); err != nil {
		t.Fatalf("ContractsForBuyer: %v", err)
	}
	if len(spans) == 0 {
		t.Fatal("no request was made")
	}

	const layout = "20060102"
	for i, s := range spans {
		from, err := time.Parse(layout, s.from)
		if err != nil {
			t.Fatalf("window %d: unparsable dataInicial %q", i, s.from)
		}
		to, err := time.Parse(layout, s.to)
		if err != nil {
			t.Fatalf("window %d: unparsable dataFinal %q", i, s.to)
		}
		if days := to.Sub(from).Hours() / 24; days > 365 {
			t.Fatalf("window %d spans %.0f days; the portal refuses anything over 365", i, days)
		}
	}

	// Newest first: a walk that gives up halfway should have collected the
	// contracts most likely to still be running.
	if spans[0].to != "20260911" {
		t.Fatalf("first window ends %q, want today", spans[0].to)
	}
}

// And it gives up eventually: a source that refuses forever must not hold a
// background sweep open forever.
func TestContractsForBuyer_GivesUpOnAPermanentRefusal(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL
	f.sleep = func(context.Context, time.Duration) bool { return true }

	if _, err := f.ContractsForBuyer(context.Background(), "00394460005887"); err == nil {
		t.Fatal("a source refusing every attempt must be reported, not hidden")
	}
	if calls > pncpMaxRetriesOnBackoff+1 {
		t.Fatalf("asked %d times; the retry ladder is meant to be bounded", calls)
	}
}

// A 400 is OUR mistake and will be our mistake again. Retrying it spends the
// same budget the 429 ladder exists to protect.
func TestContractsForBuyer_DoesNotRetryAClientError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL
	f.sleep = func(context.Context, time.Duration) bool { return true }

	if _, err := f.ContractsForBuyer(context.Background(), "00394460005887"); err == nil {
		t.Fatal("a 400 must be reported")
	}
	if calls != 1 {
		t.Fatalf("asked %d times; a client error costs exactly one request", calls)
	}
}

// The first year is the qualifying question.
//
// Most of a state's CNES register is private business that never buys
// publicly: on the first live sweep of Mato Grosso, twenty-seven
// establishments in a row answered "nothing" for the current year — drugstores,
// private clinics, a diagnostics lab — and every one then cost two more
// requests, several of which timed out after sixty seconds each.
//
// The bodies this feature is for sign contracts continuously, so a silent year
// is not a thing that happens to them.
func TestContractsForBuyer_ASilentYearEndsTheWalk(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL

	got, err := f.ContractsForBuyer(context.Background(), "04204945000267")
	if err != nil {
		t.Fatalf("ContractsForBuyer: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d contracts from a silent buyer", len(got))
	}
	// Two, not one: the branch's silent year ends ITS walk, and then the head
	// office is asked once — see the root fallback below for why.
	if calls != 2 {
		t.Fatalf("asked %d times; want one silent year each for the branch and the root", calls)
	}
}

// The registry holds the branch; the portal holds the root.
//
// CNES records the health UNIT ("...0002-67") and PNCP publishes that body's
// contracts under the HEAD OFFICE ("...0001-86"). Asking only the number we
// have returns 204 for a município that has contracts, and the buying calendar
// stays empty while looking swept. Verified against the live portal on
// 2026-09-11 for exactly these two numbers.
func TestContractsForBuyer_FallsBackToTheHeadOffice(t *testing.T) {
	var asked []string
	var served bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cnpj := r.URL.Query().Get("cnpjOrgao")
		asked = append(asked, cnpj)
		// Only the head office publishes, and only in its most recent year —
		// the shape a real body has.
		if cnpj != "04204945000186" || served {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		served = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realPNCPContract))
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL

	got, err := f.ContractsForBuyer(context.Background(), "04.204.945/0002-67")
	if err != nil {
		t.Fatalf("ContractsForBuyer: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d contracts; the head office's contract must be found", len(got))
	}
	if len(asked) < 2 || asked[0] != "04204945000267" || asked[1] != "04204945000186" {
		t.Fatalf("asked %v; want the branch first, then its head office", asked)
	}
}

// A branch that answers is the most specific answer there is, so the root is
// never asked. Some bodies really do publish under a branch — the state of
// Mato Grosso publishes under "...0028-64".
func TestContractsForBuyer_APublishingBranchIsNotSecondGuessed(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("cnpjOrgao"))
		if len(asked) == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(realPNCPContract))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL

	if _, err := f.ContractsForBuyer(context.Background(), "03507415002864"); err != nil {
		t.Fatalf("ContractsForBuyer: %v", err)
	}
	for _, c := range asked {
		if c == "03507415000144" {
			t.Fatal("the head office was asked about a branch that already answered")
		}
	}
}

// A buyer that DID publish this year gets its whole history read: the older
// windows are where a contract expiring next quarter was signed.
func TestContractsForBuyer_APublishingBuyerGetsTheWholeHistory(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(realPNCPContract))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := NewPNCPFetcher().WithPace(0)
	f.baseURL = srv.URL

	if _, err := f.ContractsForBuyer(context.Background(), "00394460005887"); err != nil {
		t.Fatalf("ContractsForBuyer: %v", err)
	}
	if calls < 3 {
		t.Fatalf("asked %d times; a publishing buyer's older years must still be read", calls)
	}
}
