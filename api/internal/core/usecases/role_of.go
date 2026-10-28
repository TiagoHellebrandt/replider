package usecases

import "github.com/replider/api/internal/core/domain"

// roleOf reads a user's role for the RBAC table.
//
// It exists because the role is a *string on the model and the correct
// comparison is domain.RoleSatisfies, not equality — comparing against "admin"
// by hand is what left the account's owner out of the supervision panel, and the
// refusal came out as a 401, tearing down the whole session in a loop.
func roleOf(u *domain.User) domain.Role {
	if u == nil || u.Role == nil {
		return domain.NormalizeRole("")
	}
	return domain.NormalizeRole(*u.Role)
}
