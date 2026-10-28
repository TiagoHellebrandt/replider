package usecases

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// The account's owner was left out of what is theirs.
//
// The check compared the role by string equality, and "owner" is not literally
// "admin" — so editing the account, deleting the account and opening the
// supervision panel all answered no to the person with the most right to them.
// Worse: the refusal came out as a 401, and a 401 means "your credential died". The
// app cleared the session, returned the person to login, and `?from=` brought them
// back to the same screen — thrown out again, in a loop.
//
// The table in domain/rbac.go is the only rule. This test exists so the next role
// check is not written by hand again.

func TestRoleOf_ReadsTheRoleForTheTable(t *testing.T) {
	owner := "owner"
	assert.Equal(t, domain.RoleOwner, roleOf(&domain.User{Role: &owner}))

	// An absent or nil role does not become admin by accident.
	assert.False(t,
		domain.RoleSatisfies(roleOf(&domain.User{}), domain.RoleAdminRole))
	assert.False(t, domain.RoleSatisfies(roleOf(nil), domain.RoleAdminRole))
}

func TestRoleOf_OwnerSatisfiesAdmin(t *testing.T) {
	for role, allowed := range map[string]bool{
		"owner":     true,
		"admin":     true,
		"gestor":    false,
		"vendedor":  false,
		"attendant": false,
		"":          false,
		"lixo":      false,
	} {
		r := role
		got := domain.RoleSatisfies(
			roleOf(&domain.User{Role: &r}), domain.RoleAdminRole)
		assert.Equal(t, allowed, got, "papel %q", role)
	}
}
