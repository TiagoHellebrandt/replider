package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/adapters/storage"
)

// MediaFileHandler streams conversation media out of GCS through the API.
//
// Why not hand the browser a GCS signed URL, which is what this used to do:
// Flutter Web decodes an image by FETCHING it, and a GCS bucket answers with no
// CORS headers, so every attachment failed in the dashboard while the very same
// URL opened fine in a browser tab. Serving the bytes from here puts them on an
// origin whose CORS policy the product already owns.
//
// It also removes a second failure mode nobody would have connected to the
// first: a GCS signed URL carries its own expiry, so any place that persisted
// one handed out a link that died on a clock we did not control.
//
// Deliberately UNAUTHENTICATED, exactly like the Telegram file proxy it mirrors:
// an <img> cannot attach an Authorization header. The HMAC signature is the
// auth, and it pins the URL to (account, object, expiry).
type MediaFileHandler struct {
	gcs    *storage.GCSAdapter
	signer *storage.URLSigner
	// cache is the local disk cache (optional, SetDiskCache). The VPS reaches
	// the bucket at ~4Mbps, so without it every view of a 4MB photo re-pays a
	// ~9s GCS fetch. Objects are immutable, so a local copy is never stale.
	cache *storage.DiskCache
}

func NewMediaFileHandler(gcs *storage.GCSAdapter, signer *storage.URLSigner) *MediaFileHandler {
	return &MediaFileHandler{gcs: gcs, signer: signer}
}

// SetDiskCache wires the local byte cache.
func (h *MediaFileHandler) SetDiskCache(c *storage.DiskCache) { h.cache = c }

// mediaProxyTimeout bounds one object fetch. Generous next to the 15s the
// Telegram proxy allows because this can be a video, and mean next to the
// server's 60s write timeout so a stalled bucket cannot pin a connection.
const mediaProxyTimeout = 30 * time.Second

// Proxy validates the signature and streams the object back.
func (h *MediaFileHandler) Proxy(c *gin.Context) {
	accountID := c.Query("a")
	objectPath := c.Query("p")
	expStr := c.Query("e")
	sig := c.Query("s")

	if accountID == "" || objectPath == "" || expStr == "" || sig == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// Shape first, signature second: a traversal attempt is a bad request
	// whether or not somebody managed to sign it.
	if !storage.ObjectBelongsToAccount(accountID, objectPath) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if h.signer == nil || !h.signer.Verify(accountID, objectPath, expStr, sig) {
		// One answer for a bad signature and for an expired one. Telling them
		// apart tells a probe which half of the URL to keep working on.
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	// Local disk first: signature already validated above, and the cached
	// bytes are the same immutable object GCS would stream — just without
	// the cross-continent round trip.
	if h.cache != nil {
		if f, size, ct, ok := h.cache.Get(objectPath); ok {
			defer func() { _ = f.Close() }()
			serveMediaHeaders(c, ct, size)
			c.Status(http.StatusOK)
			if _, err := io.Copy(c.Writer, f); err != nil {
				slog.DebugContext(c.Request.Context(),
					"media proxy: cached stream aborted",
					"account_id", accountID, "error", err)
			}
			return
		}
	}

	if h.gcs == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), mediaProxyTimeout)
	defer cancel()

	body, contentType, size, err := h.gcs.OpenObject(ctx, objectPath)
	if err != nil {
		slog.WarnContext(ctx, "media proxy: open object failed",
			"account_id", accountID, "object", objectPath, "error", err)
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	// Closing the stream also closes the GCS client behind it, so a failure
	// here is a leaked connection rather than a cosmetic error — logged, not
	// swallowed, but never surfaced: the reader already has their bytes.
	defer func() {
		if cerr := body.Close(); cerr != nil {
			slog.WarnContext(ctx, "media proxy: closing object stream failed",
				"account_id", accountID, "object", objectPath, "error", cerr)
		}
	}()

	serveMediaHeaders(c, contentType, size)

	c.Status(http.StatusOK)
	// Tee the stream into the disk cache while serving it: the first reader
	// pays the GCS fetch once, everyone after reads local disk. An aborted
	// copy discards the temp file — a half-written blob must never become a
	// cache entry.
	var reader io.Reader = body
	var cw *storage.CacheWriter
	if h.cache != nil {
		if w, werr := h.cache.NewWriter(); werr == nil {
			cw = w
			reader = io.TeeReader(body, w)
		}
	}
	if _, err := io.Copy(c.Writer, reader); err != nil {
		// The reader closing mid-stream is the normal case (scrolled away,
		// tab closed), not an error worth waking anyone for.
		slog.DebugContext(ctx, "media proxy: stream aborted",
			"account_id", accountID, "error", err)
		if cw != nil {
			cw.Discard()
		}
		return
	}
	if cw != nil {
		cw.Commit(objectPath, contentType)
	}
}

// serveMediaHeaders stamps the response headers both the cached and the GCS
// path share.
func serveMediaHeaders(c *gin.Context, contentType string, size int64) {
	// The stored type is chosen by whoever sent the file. nosniff below stops
	// the browser GUESSING a type; it does not stop it OBEYING a declared
	// one — so an attachment declared text/html used to render as a page on
	// this API's own domain. mediaDisposition answers both questions at once:
	// what type to admit, and whether it may render at all.
	served, inline := mediaDisposition(contentType)
	c.Header("Content-Type", served)
	if !inline {
		// The second half, and the one that matters for a type we do not
		// recognise: the browser is told to save it, not open it.
		c.Header("Content-Disposition", "attachment")
	}
	if size > 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	// The object at a given path never changes — the writers all mint a fresh
	// ULID per upload — so this is cacheable for as long as the URL lives.
	// `private` because the signature makes the URL user-specific and a shared
	// cache holding it would be handing it to the next reader.
	c.Header("Cache-Control", "private, max-age=86400")
	// The browser must not sniff a stored content type into something
	// executable: these bytes came from a stranger on WhatsApp.
	c.Header("X-Content-Type-Options", "nosniff")
}
