package middleware

import (
	"github.com/replider/api/internal/core/domain"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole returns middleware that checks if the authenticated user has one of the allowed roles.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := domain.NormalizeRole(c.GetString("role"))
		for _, allowed := range allowedRoles {
			// US-F008: role satisfaction, not string equality — an owner
			// must pass every RequireRole("admin") already in the router,
			// and a gestor must pass the dashboard's attendant gate. The
			// table in domain/rbac.go is the single source of truth; a
			// vendedor deliberately satisfies neither.
			if domain.RoleSatisfies(role, domain.Role(allowed)) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "insufficient permissions"})
	}
}
