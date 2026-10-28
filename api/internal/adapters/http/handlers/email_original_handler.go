package handlers

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// Serving the ORIGINAL markup of an inbound email.
//
// This is the one place the platform renders HTML written by a stranger it
// prospected itself, so the posture is: assume the document is hostile,
// remove what can execute, and let the browser enforce the rest.
//
// Three layers, none trusting the other:
//
//  1. The markup is stripped of scripts, event handlers and dangerous URL
//     schemes before it leaves the server.
//  2. A Content-Security-Policy of `default-src 'none'` blocks anything the
//     stripper missed — no script runs even if one survives.
//  3. Remote images are removed unless the reader explicitly asks for them,
//     because a remote image in an email is a read receipt for the sender.
//
// Served through the API under that policy rather than as a signed bucket
// URL: a signed URL renders the document straight from storage with no policy
// at all, every tracking pixel fires, and the reader is never asked.

// OriginalHTMLReader is the narrow slice of the attachment use case this
// needs.
type OriginalHTMLReader interface {
	ReadOriginalHTML(ctx context.Context, path string) (string, error)
}

type EmailOriginalHandler struct {
	reader  OriginalHTMLReader
	msgRepo domain.MessageRepository
	// secret signs the short-lived view token. Same secret as the session
	// JWT, but the signature is purpose-bound, so one can never stand in for
	// the other.
	secret string
	// inlineImages resolves the message's cid:-referenced attachments to
	// data: URIs. Optional (SetInlineImages) — unwired, embedded images
	// simply stay broken, the pre-2026-08-26 behaviour.
	inlineImages InlineImageReader
	// frameAncestor is the one origin allowed to iframe this document (the
	// dashboard). Empty falls back to 'self'.
	frameAncestor string

	// Rendered-document cache. The message is immutable and the render used
	// to cost ~9s (two GCS round-trips per open, no caching anywhere), so
	// the finished document is kept for a while. TTL stays far below the
	// inline-image URL TTL so a cached page never carries an expired URL.
	cacheMu sync.Mutex
	cache   map[string]originalCacheEntry
}

type originalCacheEntry struct {
	doc      string
	storedAt time.Time
}

const (
	originalCacheTTL = 30 * time.Minute
	// Entries are small on the URL path (KBs of HTML); the data-URI
	// fallback can reach MBs, so oversized documents skip the cache.
	originalCacheMaxEntryBytes = 2 << 20
	originalCacheMaxEntries    = 128
)

func (h *EmailOriginalHandler) cachedOriginal(key string) (string, bool) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	e, ok := h.cache[key]
	if !ok || time.Since(e.storedAt) > originalCacheTTL {
		return "", false
	}
	return e.doc, true
}

func (h *EmailOriginalHandler) storeOriginal(key, doc string) {
	if len(doc) > originalCacheMaxEntryBytes {
		return
	}
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if h.cache == nil {
		h.cache = map[string]originalCacheEntry{}
	}
	if len(h.cache) >= originalCacheMaxEntries {
		// Evict the oldest — a plain sweep is fine at this size.
		var oldestKey string
		var oldestAt time.Time
		for k, e := range h.cache {
			if oldestKey == "" || e.storedAt.Before(oldestAt) {
				oldestKey, oldestAt = k, e.storedAt
			}
		}
		delete(h.cache, oldestKey)
	}
	h.cache[key] = originalCacheEntry{doc: doc, storedAt: time.Now()}
}

// SetFrameAncestor names the origin allowed to embed the original view.
func (h *EmailOriginalHandler) SetFrameAncestor(origin string) {
	h.frameAncestor = strings.TrimSpace(origin)
}

// InlineImageReader is the slice of MessageAttachmentUseCase the cid:
// rewrite needs. URLs are the fast path (nothing downloaded server-side;
// nil when no signer is wired); data: URIs are the fallback.
type InlineImageReader interface {
	InlineImagesByCID(ctx context.Context, accountID, messageID string) map[string]string
	InlineImageURLsByCID(ctx context.Context, accountID, messageID string) map[string]string
}

// SetInlineImages wires cid: resolution.
func (h *EmailOriginalHandler) SetInlineImages(r InlineImageReader) { h.inlineImages = r }

// cidRefRes match src/href="cid:...", in both quoting styles. The content id
// is captured verbatim; matching is done against the trimmed, <>-stripped
// form the parser stored.
var cidRefRes = []*regexp.Regexp{
	regexp.MustCompile(`(?is)(src|href)\s*=\s*"cid:([^"]*)"`),
	regexp.MustCompile(`(?is)(src|href)\s*=\s*'cid:([^']*)'`),
}

// unresolvedImgRe matches an <img> whose src was neutralised to "#" —
// either a cid: we no longer hold (attachments stored before content_id
// existed) or a value the sanitiser refused.
var unresolvedImgRe = regexp.MustCompile(`(?is)<img[^>]*\ssrc\s*=\s*["']#["'][^>]*>`)

// rewriteCIDReferences swaps cid: references for the stored bytes. An id we
// do not hold becomes "#", and the <img> carrying it is then REMOVED
// outright: a broken-image icon with the filename as alt text reads as "the
// product failed to load this", when the truth is the bytes are gone.
func rewriteCIDReferences(in string, images map[string]string) string {
	out := in
	for _, re := range cidRefRes {
		out = re.ReplaceAllStringFunc(out, func(match string) string {
			groups := re.FindStringSubmatch(match)
			if len(groups) < 3 {
				return match
			}
			cid := strings.Trim(strings.TrimSpace(groups[2]), "<>")
			if uri, ok := images[cid]; ok {
				return groups[1] + `="` + uri + `"`
			}
			return groups[1] + `="#"`
		})
	}
	return unresolvedImgRe.ReplaceAllString(out, "")
}

func NewEmailOriginalHandler(
	reader OriginalHTMLReader,
	msgRepo domain.MessageRepository,
	secret string,
) *EmailOriginalHandler {
	return &EmailOriginalHandler{reader: reader, msgRepo: msgRepo, secret: secret}
}

// HandleMintToken serves POST /v1/messages/:id/original-token.
//
// Authenticated normally. Returns a two-minute token the browser can carry in
// a URL — the session JWT never goes in a query string, where it would land
// in history, proxy logs and the Referer of everything the page links to.
//
// @Summary      Mint a short-lived token to open an email's original markup
// @Tags         messages
// @Produce      json
// @Param        id path string true "Message ID"
// @Success      200 {object} map[string]string
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /messages/{id}/original-token [post]
func (h *EmailOriginalHandler) HandleMintToken(c *gin.Context) {
	if h.msgRepo == nil || h.secret == "" {
		respondError(c, &domain.AppError{
			Code: "EMAIL_ORIGINAL_UNAVAILABLE", Message: "original view is not configured"})
		return
	}
	accountID := getAccountID(c)
	// Authorise HERE, while the caller is still authenticated: the token is
	// minted only for a message this account can already read, so the render
	// endpoint never has to make an authorisation decision of its own.
	msg, err := h.msgRepo.GetByID(c.Request.Context(), c.Param("id"), accountID)
	if err != nil || msg == nil || metadataString(msg.Metadata, "email_html_path") == "" {
		respondError(c, domain.ErrMessageNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      MintEmailViewToken(h.secret, accountID, msg.ID, time.Now()),
		"expires_in": int(emailViewTokenTTL.Seconds()),
	})
}

// HandleOriginal serves GET /v1/messages/:id/original.
//
// @Summary      Render the original markup of an inbound email
// @Tags         messages
// @Produce      html
// @Param        token query string true "Short-lived token from /messages/{id}/original-token"
// @Param        load_images query bool false "Load remote images. Off by default: a remote image tells the sender you opened the message."
// @Success      200 {string} string "sanitised HTML"
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /messages/{id}/original [get]
func (h *EmailOriginalHandler) HandleOriginal(c *gin.Context) {
	if h.reader == nil || h.msgRepo == nil || h.secret == "" {
		respondError(c, &domain.AppError{
			Code: "EMAIL_ORIGINAL_UNAVAILABLE", Message: "original view is not configured"})
		return
	}

	// The token IS the authorisation here — this route carries no session,
	// because it is opened by a browser tab. It names one account and one
	// message, and expires in two minutes.
	accountID, messageID, err := ParseEmailViewToken(h.secret, c.Query("token"), time.Now())
	if err != nil {
		respondError(c, domain.ErrMessageNotFound)
		return
	}
	// Re-read scoped by the account the TOKEN names, never by anything the
	// request could otherwise claim.
	msg, msgErr := h.msgRepo.GetByID(c.Request.Context(), messageID, accountID)
	if msgErr != nil || msg == nil {
		respondError(c, domain.ErrMessageNotFound)
		return
	}

	path := metadataString(msg.Metadata, "email_html_path")
	if path == "" {
		respondError(c, domain.ErrMessageNotFound)
		return
	}

	loadImages := c.Query("load_images") == "true"
	cacheKey := messageID + "|" + strconv.FormatBool(loadImages)
	safe, fromCache := h.cachedOriginal(cacheKey)
	if !fromCache {
		raw, readErr := h.reader.ReadOriginalHTML(c.Request.Context(), path)
		if readErr != nil {
			respondError(c, readErr)
			return
		}
		safe = sanitizeOriginalHTML(raw, loadImages)
		// cid: references (a photo pasted into Gmail, a signature logo)
		// point at this message's own inline attachments. Fast path: rewrite
		// them to signed media-proxy URLs — same-origin, so nothing here
		// downloads a byte and the document stays small. Fallback (no signer
		// wired): base64 data: URIs from the stored bytes. Either way the
		// rewrite runs AFTER sanitising: the sanitiser strips remote <img>
		// when load_images is off, and a proxy URL injected before it would
		// be stripped with them; a cid: value passes through it untouched.
		if h.inlineImages != nil {
			ctx := c.Request.Context()
			urls := h.inlineImages.InlineImageURLsByCID(ctx, accountID, messageID)
			if urls == nil {
				urls = h.inlineImages.InlineImagesByCID(ctx, accountID, messageID)
			}
			safe = rewriteCIDReferences(safe, urls)
		}
		h.storeOriginal(cacheKey, safe)
	}

	// Layer 2. `default-src 'none'` denies everything not explicitly allowed:
	// no scripts, no frames, no fetch, no fonts. Inline styles stay because
	// without them the document is unreadable, and style cannot execute under
	// this policy.
	//
	// data: images always — those are embedded in the message and tell the
	// sender nothing. 'self' covers the media proxy the cid: rewrite points
	// at (the document is served from this same origin; the proxy demands a
	// valid HMAC, so an email fabricating a same-origin <img> gets nothing).
	// https: only when the reader explicitly asked for remote images.
	imgSrc := "'self' data:"
	if loadImages {
		imgSrc = "'self' https: data:"
	}
	// This document is MEANT to be framed — by the app's sandboxed iframe
	// and nothing else. The global middleware stamps X-Frame-Options: DENY
	// on every response, which made Chrome refuse the embed; here the CSP's
	// frame-ancestors (scoped to the app when configured) is the rule, so
	// the legacy header is dropped rather than contradicted.
	c.Writer.Header().Del("X-Frame-Options")
	frameAncestors := "'self'"
	if h.frameAncestor != "" {
		frameAncestors = h.frameAncestor
	}
	c.Header("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; img-src "+imgSrc+
			"; base-uri 'none'; form-action 'none'; frame-ancestors "+frameAncestors+"; sandbox")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	// Never cached: the document is served under a policy that depends on a
	// query param, and a cached copy could be replayed under the other one.
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(safe))
}

// ── Sanitisation (layer 1) ────────────────────────────────────────────────

var (
	// Elements whose CONTENT must go with them, not just their tags.
	killElementRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<script[^>]*>.*?</\s*script\s*>`),
		regexp.MustCompile(`(?is)<script[^>]*>.*`), // unclosed
		regexp.MustCompile(`(?is)<iframe[^>]*>.*?</\s*iframe\s*>`),
		regexp.MustCompile(`(?is)<object[^>]*>.*?</\s*object\s*>`),
		regexp.MustCompile(`(?is)<embed[^>]*>`),
		regexp.MustCompile(`(?is)<link[^>]*>`),
		regexp.MustCompile(`(?is)<meta[^>]*>`),
		regexp.MustCompile(`(?is)<base[^>]*>`),
		regexp.MustCompile(`(?is)<form[^>]*>`),
	}
	// on* handlers, in any quoting style. RE2 has no backreferences, so each
	// quoting form gets its own pattern.
	eventAttrRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)\son[a-z-]+\s*=\s*"[^"]*"`),
		regexp.MustCompile(`(?is)\son[a-z-]+\s*=\s*'[^']*'`),
		regexp.MustCompile(`(?is)\son[a-z-]+\s*=\s*[^\s>]+`),
	}
	// URL-bearing attributes, captured so their VALUE can be judged in Go.
	// A pure regex cannot express "data: unless it is an image" — RE2 has no
	// negative lookahead — and blanket-blocking data: destroys every
	// embedded image, which is most of what makes this view worth opening.
	urlAttrRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)(href|src|action|formaction)\s*=\s*"([^"]*)"`),
		regexp.MustCompile(`(?is)(href|src|action|formaction)\s*=\s*'([^']*)'`),
	}
	remoteImgRe = regexp.MustCompile(`(?is)<img[^>]*\ssrc\s*=\s*["']?https?://[^>]*>`)
)

// sanitizeOriginalHTML removes what can execute, and — unless the reader
// asked — what can phone home.
//
// This is NOT a general-purpose sanitiser, and it does not need to be: it is
// the first of three layers, and the browser is denied script execution
// outright by the CSP above. Its job is to make the common case clean rather
// than to be the last line of defence, because a sanitiser that IS the last
// line of defence has to be right forever.
func sanitizeOriginalHTML(in string, loadImages bool) string {
	out := in
	for _, re := range killElementRes {
		out = re.ReplaceAllString(out, "")
	}
	for _, re := range eventAttrRes {
		out = re.ReplaceAllString(out, "")
	}
	for _, re := range urlAttrRes {
		out = re.ReplaceAllStringFunc(out, func(match string) string {
			groups := re.FindStringSubmatch(match)
			if len(groups) < 3 || isSafeURLValue(groups[2]) {
				return match
			}
			// Keep the attribute name so the tag stays well-formed, drop the
			// value: a half-removed attribute can re-open the tag it lived in.
			return groups[1] + `="#"`
		})
	}
	if !loadImages {
		// A remote image in an email is a read receipt. Removing the whole
		// tag rather than blanking the src also removes width/height
		// beacons sized to look like layout.
		out = remoteImgRe.ReplaceAllString(out, "")
	}
	return out
}

// isSafeURLValue judges one attribute value.
//
// Deny-listed rather than allow-listed on purpose: an allow-list of schemes
// would break every relative URL and every mailto:, which are the normal
// contents of a real email. What must never survive is a scheme the browser
// treats as executable code, or a data: URI that is a whole document rather
// than an image.
func isSafeURLValue(raw string) bool {
	// HTML entities can hide a scheme ("&#106;avascript:"). Decode before
	// judging, and judge the decoded form.
	v := strings.ToLower(strings.TrimSpace(html.UnescapeString(raw)))
	// Whitespace and control characters inside a scheme are ignored by
	// browsers ("java\tscript:"), so they must be ignored here too.
	v = schemeNoiseRe.ReplaceAllString(v, "")

	switch {
	case strings.HasPrefix(v, "javascript:"), strings.HasPrefix(v, "vbscript:"):
		return false
	case strings.HasPrefix(v, "data:"):
		// Only images. data:text/html is a document with script in it.
		return strings.HasPrefix(v, "data:image/")
	}
	return true
}

var schemeNoiseRe = regexp.MustCompile(`[\x00-\x20]`)

// metadataString reads one key out of a message's metadata.
//
// The shape depends on where the struct came from: a freshly built message
// carries a map, while one read back from Postgres carries the raw JSONB
// bytes. Handling only the map is how this silently returned nothing for
// every message that had actually been persisted.
func metadataString(meta any, key string) string {
	var m map[string]any
	switch v := meta.(type) {
	case map[string]any:
		m = v
	case []byte:
		if json.Unmarshal(v, &m) != nil {
			return ""
		}
	case json.RawMessage:
		if json.Unmarshal(v, &m) != nil {
			return ""
		}
	case string:
		if json.Unmarshal([]byte(v), &m) != nil {
			return ""
		}
	default:
		return ""
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}
