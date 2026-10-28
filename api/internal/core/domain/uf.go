package domain

import "strings"

// brazilianUFs is the closed set of federative units. A closed set rather
// than a two-letter shape test because "XX" passes the shape and then never
// matches a tender — a typo that turns into a feed that is quietly empty
// forever, which is the failure mode this codebase keeps finding.
var brazilianUFs = map[string]struct{}{
	"AC": {}, "AL": {}, "AM": {}, "AP": {}, "BA": {}, "CE": {}, "DF": {},
	"ES": {}, "GO": {}, "MA": {}, "MG": {}, "MS": {}, "MT": {}, "PA": {},
	"PB": {}, "PE": {}, "PI": {}, "PR": {}, "RJ": {}, "RN": {}, "RO": {},
	"RR": {}, "RS": {}, "SC": {}, "SE": {}, "SP": {}, "TO": {},
}

// NormalizeUF trims and upper-cases a state code, returning "" for anything
// that is not one of the 27.
func NormalizeUF(raw string) string {
	uf := strings.ToUpper(strings.TrimSpace(raw))
	if _, ok := brazilianUFs[uf]; !ok {
		return ""
	}
	return uf
}

// IsBrazilianUF reports whether raw names a federative unit.
func IsBrazilianUF(raw string) bool { return NormalizeUF(raw) != "" }

// NormalizeUFs cleans a list, dropping blanks, unknown codes and duplicates
// while keeping first-seen order so a request sequence built from it is
// predictable.
func NormalizeUFs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		uf := NormalizeUF(r)
		if uf == "" {
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

// CNPJRoot returns the head office's CNPJ for any establishment of the same
// company: the first eight digits, branch 0001, and freshly computed check
// digits.
//
// It exists because two public registers disagree about which CNPJ identifies
// a public body, and the disagreement made a whole feature return nothing.
// CNES records the HEALTH UNIT — a branch, "...0002-67" — while PNCP publishes
// that body's contracts under the head office, "...0001-86". Verified against
// the live portal on 2026-09-11: the branch answers 204 and the root answers
// 200 with a contract, for the same município.
//
// Returns "" for anything that is not fourteen digits. Callers treat that as
// "no root to try" rather than as an error: an establishment with a malformed
// CNPJ is a row to skip, not a request to make.
func CNPJRoot(cnpj string) string {
	digits := make([]byte, 0, 14)
	for i := 0; i < len(cnpj); i++ {
		if cnpj[i] >= '0' && cnpj[i] <= '9' {
			digits = append(digits, cnpj[i])
		}
	}
	if len(digits) != 14 {
		return ""
	}
	base := string(digits[:8]) + "0001"
	return base + cnpjCheckDigits(base)
}

// cnpjCheckDigits computes the two verifiers for a 12-digit CNPJ base.
//
// Recomputed rather than carried over from the branch: the verifiers are a
// function of all twelve preceding digits, so pasting the branch's digits onto
// a different branch number produces a CNPJ the portal rejects with a 400 —
// which is how this was first got wrong.
func cnpjCheckDigits(base string) string {
	weights1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	weights2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}

	digit := func(s string, weights []int) byte {
		sum := 0
		for i, w := range weights {
			sum += int(s[i]-'0') * w
		}
		r := sum % 11
		if r < 2 {
			return '0'
		}
		return byte('0' + 11 - r)
	}
	d1 := digit(base, weights1)
	d2 := digit(base+string(d1), weights2)
	return string([]byte{d1, d2})
}
