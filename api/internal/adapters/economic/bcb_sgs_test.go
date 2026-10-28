package economic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The BCB series reader.
//
// A wrong series number here would adjust somebody's price by the wrong index,
// and a brittle parser would drop the whole series over one malformed row. Both
// are tested against a fake BCB.

func TestSGSFetcher_ParsesTheMonthlySeries(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
          {"data":"01/01/2026","valor":"0.42"},
          {"data":"01/02/2026","valor":"0.83"},
          {"data":"01/03/2026","valor":"-0.11"}
        ]`))
	}))
	defer srv.Close()

	values, err := NewSGSFetcher().WithBaseURL(srv.URL).
		Fetch(context.Background(), domain.IndexIPCA, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// IPCA is series 433. A wrong number here adjusts prices by the wrong index —
	// the single most expensive typo available in this file.
	if !strings.Contains(gotPath, "bcdata.sgs.433/dados") {
		t.Fatalf("requested %q, want the IPCA series 433", gotPath)
	}
	// The range starts at the FIRST of the month, not the day passed in: the
	// series is monthly and a mid-month start would skip January.
	if !strings.Contains(gotPath, "dataInicial=01/01/2026") {
		t.Fatalf("requested %q, want dataInicial=01/01/2026", gotPath)
	}

	if len(values) != 3 {
		t.Fatalf("got %d months, want 3", len(values))
	}
	if values[0].Code != domain.IndexIPCA {
		t.Fatalf("code = %q, want ipca", values[0].Code)
	}
	if values[0].ReferenceMonth.Format("2006-01-02") != "2026-01-01" {
		t.Fatalf("month = %v, want 2026-01-01", values[0].ReferenceMonth)
	}
	if values[0].MonthlyRate != 0.42 {
		t.Fatalf("rate = %v, want 0.42", values[0].MonthlyRate)
	}
	// Deflation is a real month, not an error to drop.
	if values[2].MonthlyRate != -0.11 {
		t.Fatalf("negative rate = %v, want -0.11", values[2].MonthlyRate)
	}
}

func TestSGSFetcher_KeepsTheMonthsAroundABadRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A comma decimal (seen on locale-sensitive deployments), an unparseable
		// date and an unparseable value, between two good months.
		_, _ = w.Write([]byte(`[
          {"data":"01/01/2026","valor":"0,42"},
          {"data":"janeiro/2026","valor":"0.50"},
          {"data":"01/03/2026","valor":"n/d"},
          {"data":"01/04/2026","valor":"0.31"}
        ]`))
	}))
	defer srv.Close()

	values, err := NewSGSFetcher().WithBaseURL(srv.URL).
		Fetch(context.Background(), domain.IndexIGPM, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Two survivors: one bad row must not discard the series around it — the
	// accumulation reports its own gaps, so a partial fetch degrades to
	// "incomplete" rather than to nothing.
	if len(values) != 2 {
		t.Fatalf("got %d months, want 2 (the two parseable ones)", len(values))
	}
	if values[0].MonthlyRate != 0.42 {
		t.Fatalf("comma decimal not parsed: %v", values[0].MonthlyRate)
	}
	if values[1].ReferenceMonth.Format("2006-01") != "2026-04" {
		t.Fatalf("second month = %v, want 2026-04", values[1].ReferenceMonth)
	}
}

func TestSGSFetcher_ReportsUpstreamFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	if _, err := NewSGSFetcher().WithBaseURL(srv.URL).
		Fetch(context.Background(), domain.IndexIPCA, time.Now()); err == nil {
		t.Fatal("a 502 from BCB must surface, not read as an empty series — " +
			"empty would look like 'no adjustment owed'")
	}

	// An index this fetcher does not read is refused up front rather than
	// silently mapped to a default series.
	if _, err := NewSGSFetcher().Fetch(context.Background(), domain.IndexCode("selic"), time.Now()); err == nil {
		t.Fatal("an unknown index must be refused")
	}
}

func TestSupportedIndices_CoversTheFourAContractNames(t *testing.T) {
	want := map[domain.IndexCode]bool{
		domain.IndexIPCA: true, domain.IndexIGPM: true,
		domain.IndexINPC: true, domain.IndexIGPDI: true,
	}
	got := SupportedIndices()
	if len(got) != len(want) {
		t.Fatalf("got %d indices, want %d", len(got), len(want))
	}
	for _, code := range got {
		if !want[code] {
			t.Fatalf("unexpected index %q", code)
		}
		// Every supported index needs a series number, or the worker fetches
		// nothing for it and every contract on that index silently loses its
		// percentage.
		if _, ok := sgsSeries[code]; !ok {
			t.Fatalf("index %q has no SGS series mapped", code)
		}
	}
}
