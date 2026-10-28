package handlers

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// GoogleMailboxHandler exposes "conecte o e-mail que você já tem".
//
// Two endpoints with deliberately different auth: starting the flow is an
// admin action on a logged-in account, while the callback arrives as an
// unauthenticated browser redirect from Google and authenticates on the
// HMAC-signed state instead — there is no session on that request to read.
type GoogleMailboxHandler struct {
	uc           *usecases.ConnectGoogleMailboxUseCase
	dashboardURL string
}

func NewGoogleMailboxHandler(
	uc *usecases.ConnectGoogleMailboxUseCase, dashboardURL string,
) *GoogleMailboxHandler {
	return &GoogleMailboxHandler{uc: uc, dashboardURL: dashboardURL}
}

// HandleConnect starts the consent flow.
//
//	@Summary		Connect a Google mailbox
//	@Description	Returns the Google consent URL. The account sends and receives through its own mailbox, so there is no SMTP host, port or app password to type — and no free-provider refusal, because the mail is sent THROUGH the account rather than on its behalf.
//	@Tags			channels
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		401	{object}	domain.AppError
//	@Failure		503	{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/channels/email/google/connect [post]
func (h *GoogleMailboxHandler) HandleConnect(c *gin.Context) {
	authURL, err := h.uc.AuthURL(getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleCallback receives Google's redirect.
//
// It never renders JSON: a person is looking at this in a browser tab, so
// success and failure both land back in the app with a flag it turns into a
// message. The flag distinguishes "connected" from "reconnected" because to the
// person one is setup and the other is a repair.
func (h *GoogleMailboxHandler) HandleCallback(c *gin.Context) {
	redirect := func(result string) {
		c.Redirect(http.StatusFound,
			h.dashboardURL+"/settings/channels?gmail="+url.QueryEscape(result))
	}
	// The consent screen's cancel button. Not an error — somebody changed their
	// mind, and telling them something failed would be a lie.
	if c.Query("error") != "" {
		redirect("denied")
		return
	}

	res, err := h.uc.HandleCallback(c.Request.Context(), c.Query("state"), c.Query("code"))
	if err != nil {
		accountID := ""
		if res != nil {
			accountID = res.AccountID
		}
		slog.ErrorContext(c.Request.Context(), "google mailbox connect failed",
			"account_id", accountID, "error", err)
		redirect("error")
		return
	}
	if res.Reconnected {
		redirect("reconnected")
		return
	}
	redirect("connected")
}

// Referenced so swag resolves domain.AppError in the annotations above.
var _ = domain.AppError{}
