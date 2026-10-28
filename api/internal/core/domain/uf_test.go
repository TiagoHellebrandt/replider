package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// "XX" passes any two-letter shape test and then never matches a tender —
// a typo that becomes a feed that is quietly empty forever. The set is closed
// for exactly that reason.
func TestNormalizeUF(t *testing.T) {
	assert.Equal(t, "MT", NormalizeUF(" mt "))
	assert.Equal(t, "SP", NormalizeUF("Sp"))
	assert.Equal(t, "", NormalizeUF("XX"))
	assert.Equal(t, "", NormalizeUF("Mato Grosso"))
	assert.Equal(t, "", NormalizeUF(""))
}

// The list feeds a request sequence, so order is kept and duplicates cannot
// turn one state into two walks of the same feed.
func TestNormalizeUFs_DropsNoiseAndKeepsOrder(t *testing.T) {
	assert.Equal(t, []string{"MT", "SP"},
		NormalizeUFs([]string{" mt ", "XX", "MT", "", "sp"}))
	assert.Empty(t, NormalizeUFs([]string{"nowhere"}))
}

// Two public registers disagree about which CNPJ identifies a public body, and
// the disagreement made the buying calendar return nothing: CNES records the
// health UNIT (a branch) and PNCP publishes that body's contracts under the
// head office.
//
// The numbers here are real and were checked against the live portal on
// 2026-09-11 — the branch answered 204, the root answered 200 with a contract.
func TestCNPJRoot(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"a branch resolves to its head office", "04204945000267", "04204945000186"},
		{"punctuation is not part of the number", "04.204.945/0002-67", "04204945000186"},
		{"a head office resolves to itself", "04204945000186", "04204945000186"},
		{"another município, verified the same way", "03507514000207", "03507514000126"},
		// The verifiers are a function of all twelve preceding digits, so
		// carrying the branch's own digits over produces a number the portal
		// rejects with a 400. That is how this was first got wrong.
		{"the check digits are recomputed, not copied", "03507415002864", "03507415000144"},
		{"too short is no root at all", "0420494500026", ""},
		{"empty is no root at all", "", ""},
		{"letters are not a CNPJ", "abcdefghijklmn", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CNPJRoot(tc.in); got != tc.want {
				t.Fatalf("CNPJRoot(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
