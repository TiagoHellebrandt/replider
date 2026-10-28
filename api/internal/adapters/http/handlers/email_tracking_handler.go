package handlers

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// The two endpoints a mail client hits.
//
// Unauthenticated by nature: the caller is Gmail fetching an image, or a
// customer clicking a link in their inbox. The signed token IS the
// authorisation, and it can do exactly two things — increment a counter, and
// redirect to a destination that was signed at send time.

// transparentGIF is a 1x1 pixel. Inlined rather than served from disk so the
// endpoint cannot fail on a missing file: an open that 404s is a broken image
// in a stranger's inbox.
var transparentGIF, _ = base64.StdEncoding.DecodeString(
	"R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")

type EmailTrackingHandler struct {
	repo   domain.EmailTemplateRepository
	secret string
}

func NewEmailTrackingHandler(repo domain.EmailTemplateRepository, secret string) *EmailTrackingHandler {
	return &EmailTrackingHandler{repo: repo, secret: secret}
}

// HandleOpen serves GET /v1/t/o/:token.
//
// ALWAYS answers with the pixel, even for a token it cannot read. A broken
// image in a stranger's cold email is a worse outcome than an uncounted open,
// and telling a caller their token was invalid tells an attacker they are
// probing something real.
//
// @Summary      Open-tracking pixel
// @Tags         email
// @Produce      image/gif
// @Param        token path string true "Signed tracking token"
// @Success      200 {string} binary "1x1 GIF"
// @Router       /t/o/{token} [get]
func (h *EmailTrackingHandler) HandleOpen(c *gin.Context) {
	defer h.servePixel(c)

	if h.repo == nil || h.secret == "" {
		return
	}
	tok, err := domain.ParseTrackToken(h.secret, c.Param("token"))
	if err != nil || tok.Kind != domain.TrackOpen {
		return
	}
	// Best effort: the pixel is already promised by the defer, and a failed
	// counter must not turn into a broken image.
	_ = h.repo.RecordOpen(c.Request.Context(), tok.TemplateID, tok.Version)
}

func (h *EmailTrackingHandler) servePixel(c *gin.Context) {
	// Never cached: a cached pixel is an open counted once for a message
	// somebody opened five times, and the caches in question are Gmail's.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Data(http.StatusOK, "image/gif", transparentGIF)
}

// HandleClick serves GET /v1/t/c/:token.
//
// The destination comes from the SIGNED token and is re-checked against the
// http(s) allow-list before redirecting. Both halves matter: the signature
// stops a stranger pointing this at their own page, and the scheme check
// stops a bug of ours from doing the same.
//
// @Summary      Click-tracking redirect
// @Tags         email
// @Param        token path string true "Signed tracking token"
// @Success      302 "redirect to the signed destination"
// @Failure      404 {object} domain.AppError
// @Router       /t/c/{token} [get]
func (h *EmailTrackingHandler) HandleClick(c *gin.Context) {
	if h.secret == "" {
		c.Status(http.StatusNotFound)
		return
	}
	tok, err := domain.ParseTrackToken(h.secret, c.Param("token"))
	if err != nil || tok.Kind != domain.TrackClick ||
		!domain.IsSafeRedirectTarget(tok.URL) {
		// 404 rather than an error page: a probing caller learns nothing,
		// and a customer who somehow got a bad link sees their browser's
		// own message rather than ours claiming their email is broken.
		c.Status(http.StatusNotFound)
		return
	}

	if h.repo != nil {
		_ = h.repo.RecordClick(c.Request.Context(), tok.TemplateID, tok.Version)
	}
	// No referrer: the destination must not learn which tracking URL sent
	// the visitor, since that URL identifies the message and its recipient.
	c.Header("Referrer-Policy", "no-referrer")
	c.Redirect(http.StatusFound, tok.URL)
}
