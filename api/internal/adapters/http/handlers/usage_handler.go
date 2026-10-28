package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type UsageHandler struct {
	getUsageUC *usecases.GetUsageUseCase
}

func NewUsageHandler(getUsageUC *usecases.GetUsageUseCase) *UsageHandler {
	return &UsageHandler{
		getUsageUC: getUsageUC,
	}
}

// GetUsage godoc
// @Summary      Get Account Usage
// @Description  Returns current billing cycle usage vs plan limits.
// @Tags         accounts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.GetUsageResponse
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /accounts/usage [get]
func (h *UsageHandler) HandleGetUsage(c *gin.Context) {
	accountID := c.GetString("accountID")

	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	response, err := h.getUsageUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}
