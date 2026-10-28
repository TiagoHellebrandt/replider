package usecases

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// What decides whether somebody may is the table, not the equals sign.
//
// A single `*caller.Role != "admin"` locked the account's OWNER out of the
// supervision panel — because "owner" is not literally "admin". Since the refusal
// came out as a 401, the app read "your credential died", cleared the session and
// sent them to login, from where the person returned to the same screen and was
// thrown out again. One equals sign, one login loop.
//
// The same comparison sat silent in edit account, delete account, per-attendant
// metrics, the self-commission warning and the last-administrator lock.
//
// This test reads the source and forbids it coming back.

var (
	// `*x.Role == "admin"` / `!=` — the authority of WHOEVER is acting.
	roleCompare = regexp.MustCompile(`\*\w+\.Role\s*[!=]=\s*"`)
	// A role that came from outside (a DTO, a form) is a VALUE to validate, not an
	// authority to check: `dto.Role != "admin"` is legitimate.
	dtoCompare = regexp.MustCompile(`\bdto\.Role\b|\breq\.Role\b|\binput\.Role\b`)
)

// Two comparisons do not decide AUTHORITY, and are therefore excluded: they
// define WHO JOINS THE ROTATION of conversations. Swapping in RoleSatisfies here
// fixes nothing — it starts putting owner and gestor into the service queue, which
// is a product decision, not a bug fix. If that is ever to happen, let it happen
// on purpose rather than in passing.
var roleCheckExceptions = map[string]string{
	"owner_sellers.go:250":   "pool of eligible attendants, not authority",
	"team_auto_assign.go:78": "the rotation pool, not authority",
}

func TestRoleChecksUseTheTable(t *testing.T) {
	var offenders []string

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ler o pacote: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ler %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !roleCompare.MatchString(line) || dtoCompare.MatchString(line) {
				continue
			}
			where := strings.TrimSpace(name) + ":" + itoa(i+1)
			if _, ok := roleCheckExceptions[where]; ok {
				continue
			}
			offenders = append(offenders, where+"  "+strings.TrimSpace(line))
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("Estes trechos decidem autoridade comparando o papel com uma "+
			"string, em vez de perguntar a domain.RoleSatisfies:\n  %s\n\n"+
			"Um owner não é literalmente \"admin\" — foi assim que o dono da "+
			"conta ficou de fora do próprio painel.",
			strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// A justification that outlived the line it justified is a lie the next reader
// will believe.
func TestRoleCheckExceptionsStillPointAtSomething(t *testing.T) {
	for where, why := range roleCheckExceptions {
		parts := strings.Split(where, ":")
		if len(parts) != 2 {
			t.Fatalf("exceção mal escrita: %q", where)
		}
		src, err := os.ReadFile(parts[0])
		if err != nil {
			t.Fatalf("%s (%s): o arquivo sumiu — apague a exceção", where, why)
		}
		lines := strings.Split(string(src), "\n")
		n := 0
		for _, c := range parts[1] {
			n = n*10 + int(c-'0')
		}
		if n < 1 || n > len(lines) || !roleCompare.MatchString(lines[n-1]) {
			t.Fatalf("%s (%s): não há mais comparação de papel nessa linha — "+
				"apague a exceção em vez de deixá-la apontando para o vazio",
				where, why)
		}
	}
}
