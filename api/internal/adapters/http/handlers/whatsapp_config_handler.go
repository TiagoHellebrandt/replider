package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// templateNameRegex enforces Meta's allowed template name format and prevents
// injection attacks via crafted names (e.g., path traversal or query hijack).
var templateNameRegex = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)

// WhatsAppConfigHandler manages WhatsApp Business profile and templates per channel.
type WhatsAppConfigHandler struct {
	channelRepo domain.ChannelRepository
	gcsAdapter  *storage.GCSAdapter // optional — when nil the GCS-cached profile picture path is skipped
	appID       string
	appSecret   string // needed by Resumable Upload to mint the app access token

	// templateSyncUC backs POST /:id/templates/sync (US-018). Optional —
	// wired via SetTemplateSyncUseCase; when nil the route returns 503.
	templateSyncUC *usecases.SyncTemplatesUseCase
}

func NewWhatsAppConfigHandler(channelRepo domain.ChannelRepository, appID string) *WhatsAppConfigHandler {
	return &WhatsAppConfigHandler{channelRepo: channelRepo, appID: appID}
}

// SetTemplateSyncUseCase wires the Meta template sync use case (US-018)
// without changing the constructor signature (optional-setter pattern).
func (h *WhatsAppConfigHandler) SetTemplateSyncUseCase(uc *usecases.SyncTemplatesUseCase) {
	h.templateSyncUC = uc
}

// SetGCSAdapter wires the GCS adapter so HandleUploadProfilePicture can
// keep a server-side copy of the profile image. Meta's
// profile_picture_url field on whatsapp_business_profile GET is
// unreliable (often empty even when the picture is set on WhatsApp),
// so we fall back to a signed URL of our cached copy when serving the
// profile to the dashboard.
func (h *WhatsAppConfigHandler) SetGCSAdapter(g *storage.GCSAdapter) {
	h.gcsAdapter = g
}

// SetAppSecret wires the Meta app secret. Required by template HEADER
// media upload (Resumable Upload) — no other endpoint on this handler
// needs it. Optional setter so callers that don't ship the upload
// feature can keep the existing constructor signature.
func (h *WhatsAppConfigHandler) SetAppSecret(s string) {
	h.appSecret = s
}

// resolveWAChannel loads the channel, checks ownership and WhatsApp type,
// and returns (phoneNumberID, wabaID, accessToken) extracted from channel config.
// Writes the HTTP error response and returns ok=false on failure.
func (h *WhatsAppConfigHandler) resolveWAChannel(c *gin.Context) (phoneNumberID, wabaID, accessToken string, ok bool) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "only WhatsApp channels support this operation"})
		return
	}

	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)

	phoneNumberID = cfg["phone_number_id"]
	wabaID = cfg["waba_id"]
	accessToken = cfg["access_token"]
	if phoneNumberID == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "CHANNEL_MISSING_CONFIG", Message: "channel is missing phone_number_id or access_token"})
		return
	}
	ok = true
	return
}

// HandleGetBusinessProfile godoc
// @Summary      Get WhatsApp business profile
// @Tags         channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  waadapter.BusinessProfile
// @Router       /channels/{id}/whatsapp/profile [get]
func (h *WhatsAppConfigHandler) HandleGetBusinessProfile(c *gin.Context) {
	channelID := c.Param("id")
	cacheKey := "wa_profile:" + channelID
	if cached, ok := metaCacheGet(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}
	accountID := c.GetString("accountID")
	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "only WhatsApp channels support this operation"})
		return
	}
	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
	phoneNumberID := cfg["phone_number_id"]
	accessToken := cfg["access_token"]
	if phoneNumberID == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "CHANNEL_MISSING_CONFIG", Message: "channel is missing phone_number_id or access_token"})
		return
	}
	profile, err := waadapter.GetBusinessProfile(c.Request.Context(), phoneNumberID, accessToken)
	if err != nil {
		slog.Error("failed to fetch WhatsApp profile", "error", err)
		c.JSON(http.StatusBadGateway, domain.AppError{Code: "WA_PROFILE_FETCH_FAILED", Message: err.Error()})
		return
	}
	// Meta's profile_picture_url is unreliable (frequently empty even
	// after a successful upload via profile_picture_handle). Strategy:
	//   - Meta has a URL → return it. Async-mirror to GCS when its
	//     fingerprint differs from the last cached copy so a future
	//     Meta-empty response can still serve the right picture.
	//   - Meta returns empty + GCS has a copy → serve a signed URL of
	//     the cached copy so the dashboard preview never goes blank
	//     between Meta hiccups.
	cachedPath := strings.TrimSpace(cfg["wa_profile_picture_path"])
	profile.HasCachedPicture = cachedPath != ""
	if h.gcsAdapter != nil {
		if profile.ProfilePictureURL != "" {
			fingerprint := metaPictureFingerprint(profile.ProfilePictureURL)
			if fingerprint != "" && cfg["wa_profile_picture_url_fp"] != fingerprint {
				// Snapshot what we need outside the request scope so
				// the goroutine survives the response returning.
				go h.recacheProfilePictureFromMeta(channelID, accountID, profile.ProfilePictureURL, fingerprint)
			}
		} else if cachedPath != "" {
			if signedURL, err := h.gcsAdapter.GenerateReadSignedURL(c.Request.Context(), cachedPath, 30*time.Minute); err == nil {
				profile.ProfilePictureURL = signedURL
			} else {
				slog.Warn("failed to sign cached WhatsApp profile picture",
					"channel_id", channelID, "path", cachedPath, "error", err)
			}
		}
	}
	metaCacheSet(cacheKey, profile)
	c.JSON(http.StatusOK, profile)
}

// metaPictureFingerprint returns a stable identifier for a Meta CDN
// picture URL. Meta rotates the signed query string (`?ccb=...&oh=...
// &oe=...`) on every fetch, so a raw URL comparison would re-fire the
// re-cache job on every GET. The path before the query is stable for a
// given asset (e.g. `/v/t61.24694-24/abc123_n.jpg`) so we use that as
// the change-detection key.
func metaPictureFingerprint(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		return rawURL[:i]
	}
	return rawURL
}

// recacheProfilePictureFromMeta mirrors the picture currently served
// by Meta into GCS. Runs in the background after a HandleGetBusiness
// Profile detected the Meta fingerprint changed (someone uploaded a
// new picture in Meta Business Manager) so the next time Meta returns
// empty we can still serve the right picture from our copy.
//
// All errors are best-effort and only logged — the live request has
// already been answered with Meta's URL.
func (h *WhatsAppConfigHandler) recacheProfilePictureFromMeta(channelID, accountID, metaURL, fingerprint string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		slog.Warn("recache: build request", "channel_id", channelID, "error", err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("recache: fetch Meta picture", "channel_id", channelID, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("recache: Meta picture non-200", "channel_id", channelID, "status", resp.StatusCode)
		return
	}
	// Cap the read at 5MB — same ceiling as the upload endpoint, so a
	// surprise oversized asset can't OOM the worker.
	bytesData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024+1))
	if err != nil {
		slog.Warn("recache: read Meta picture", "channel_id", channelID, "error", err)
		return
	}
	if len(bytesData) > 5*1024*1024 {
		slog.Warn("recache: Meta picture exceeds 5MB cap", "channel_id", channelID)
		return
	}
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = sniffMime(bytesData)
	}
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		slog.Warn("recache: unsupported Meta picture mime", "channel_id", channelID, "mime", mimeType)
		return
	}
	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	}
	objectPath := fmt.Sprintf("accounts/%s/whatsapp_profiles/%s.%s", accountID, channelID, ext)
	if err := h.gcsAdapter.UploadBytes(ctx, objectPath, bytesData, mimeType); err != nil {
		slog.Warn("recache: GCS upload", "channel_id", channelID, "error", err)
		return
	}
	if err := h.channelRepo.MergeCredentials(ctx, channelID, map[string]string{
		"wa_profile_picture_path":   objectPath,
		"wa_profile_picture_url_fp": fingerprint,
	}); err != nil {
		slog.Warn("recache: persist channel config", "channel_id", channelID, "error", err)
		return
	}
	slog.Info("recached WhatsApp profile picture from Meta",
		"channel_id", channelID, "object_path", objectPath, "bytes", len(bytesData))
}

type updateWAProfileRequest struct {
	About                string   `json:"about"`
	Address              string   `json:"address"`
	Description          string   `json:"description"`
	Email                string   `json:"email"`
	Websites             []string `json:"websites"`
	Vertical             string   `json:"vertical"`
	ProfilePictureHandle string   `json:"profile_picture_handle"`
}

// HandleUpdateBusinessProfile godoc
// @Summary      Update WhatsApp business profile
// @Tags         channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                 true  "Channel ID"
// @Param        body  body  updateWAProfileRequest   true  "Profile fields"
// @Success      204
// @Router       /channels/{id}/whatsapp/profile [put]
func (h *WhatsAppConfigHandler) HandleUpdateBusinessProfile(c *gin.Context) {
	phoneNumberID, _, accessToken, ok := h.resolveWAChannel(c)
	if !ok {
		return
	}

	var req updateWAProfileRequest
	if !bindJSON(c, &req) {
		return
	}

	profile := waadapter.BusinessProfile{
		About:       req.About,
		Address:     req.Address,
		Description: req.Description,
		Email:       req.Email,
		Websites:    req.Websites,
		Vertical:    req.Vertical,
	}
	if err := waadapter.UpdateBusinessProfile(c.Request.Context(), phoneNumberID, accessToken, profile, req.ProfilePictureHandle); err != nil {
		slog.Error("failed to update WhatsApp profile", "error", err)
		c.JSON(http.StatusBadGateway, domain.AppError{Code: "WA_PROFILE_UPDATE_FAILED", Message: err.Error()})
		return
	}
	metaCacheInvalidate(c.Param("id"))
	c.Status(http.StatusNoContent)
}

// HandleUploadProfilePicture godoc
// @Summary      Upload WhatsApp business profile picture
// @Description  Accepts a multipart "file" field, proxies the bytes to Meta's Resumable Upload API, and returns the handle to be used in update profile.
// @Tags         channels
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Channel ID"
// @Param        file  formData  file  true  "Image file (JPEG/PNG)"
// @Success      200  {object}  map[string]string  "{handle: ...}"
// @Router       /channels/{id}/whatsapp/profile/picture [post]
func (h *WhatsAppConfigHandler) HandleUploadProfilePicture(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")
	_, _, accessToken, ok := h.resolveWAChannel(c)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "FILE_REQUIRED", Message: "multipart 'file' field is required"})
		return
	}
	if fileHeader.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "FILE_TOO_LARGE", Message: "file must be <= 5MB"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "FILE_OPEN_FAILED", Message: err.Error()})
		return
	}
	defer f.Close()
	bytesData, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "FILE_READ_FAILED", Message: err.Error()})
		return
	}

	// Validate content is actually JPEG or PNG via magic bytes (don't trust client headers).
	// JPEG: FF D8 FF ; PNG: 89 50 4E 47
	mimeType := ""
	if len(bytesData) >= 3 && bytesData[0] == 0xFF && bytesData[1] == 0xD8 && bytesData[2] == 0xFF {
		mimeType = "image/jpeg"
	} else if len(bytesData) >= 4 && bytesData[0] == 0x89 && bytesData[1] == 0x50 && bytesData[2] == 0x4E && bytesData[3] == 0x47 {
		mimeType = "image/png"
	} else {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_IMAGE_FORMAT", Message: "only JPEG and PNG images are supported"})
		return
	}

	handle, err := waadapter.UploadProfilePictureToMeta(c.Request.Context(), h.appID, accessToken, bytesData, mimeType)
	if err != nil {
		slog.Error("failed to upload WhatsApp profile picture", "error", err)
		c.JSON(http.StatusBadGateway, domain.AppError{Code: "WA_PICTURE_UPLOAD_FAILED", Message: err.Error()})
		return
	}
	// Best-effort: keep our own copy in GCS so the dashboard can render
	// the picture even when Meta's profile_picture_url comes back empty
	// (a documented Cloud API quirk that breaks the profile screen
	// preview on reload). A failure here does not block the Meta-side
	// upload — the user's WhatsApp number still gets the new picture.
	h.persistProfilePictureToGCS(c.Request.Context(), accountID, channelID, bytesData, mimeType)
	// Drop any cached profile so the next GET re-runs the GCS-fallback
	// branch and surfaces the freshly uploaded picture to the dashboard
	// — without this the 30s cache window keeps showing the empty avatar.
	metaCacheInvalidate(channelID)
	c.JSON(http.StatusOK, gin.H{"handle": handle})
}

// persistProfilePictureToGCS stores the uploaded bytes under
// accounts/{account}/whatsapp_profiles/{channel}.{ext} and writes the
// resulting object path to the channel's config JSONB so subsequent
// HandleGetBusinessProfile calls can serve a signed URL of it.
func (h *WhatsAppConfigHandler) persistProfilePictureToGCS(ctx context.Context, accountID, channelID string, data []byte, mimeType string) {
	if h.gcsAdapter == nil {
		return
	}
	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	}
	objectPath := fmt.Sprintf("accounts/%s/whatsapp_profiles/%s.%s", accountID, channelID, ext)
	if err := h.gcsAdapter.UploadBytes(ctx, objectPath, data, mimeType); err != nil {
		slog.Warn("failed to cache WhatsApp profile picture in GCS",
			"channel_id", channelID, "object_path", objectPath, "error", err)
		return
	}
	if err := h.channelRepo.MergeCredentials(ctx, channelID, map[string]string{
		"wa_profile_picture_path": objectPath,
		// Clear the Meta fingerprint so the next GET re-mirrors from
		// Meta once Meta starts returning the new URL. Without this
		// the recache job thinks the cache is up to date and never
		// catches the URL change Meta eventually publishes.
		"wa_profile_picture_url_fp": "",
	}); err != nil {
		slog.Warn("failed to persist wa_profile_picture_path on channel",
			"channel_id", channelID, "error", err)
	}
}

// HandleDeleteProfilePicture godoc
// @Summary      Remove cached WhatsApp profile picture
// @Description  Drops the server-side copy of the picture (and the
// @Description  fingerprint that pins it). The next GET re-mirrors
// @Description  whatever Meta returns. Use after removing the
// @Description  picture in Meta Business Manager so the dashboard
// @Description  stops serving the stale fallback.
// @Tags         channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      204
// @Router       /channels/{id}/whatsapp/profile/picture [delete]
func (h *WhatsAppConfigHandler) HandleDeleteProfilePicture(c *gin.Context) {
	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	ch, err := h.channelRepo.GetByID(c.Request.Context(), channelID)
	if err != nil || ch == nil || ch.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return
	}
	if ch.Type != "whatsapp" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "only WhatsApp channels support this operation"})
		return
	}
	var cfg map[string]string
	_ = json.Unmarshal(rawConfigBytes(ch.Config), &cfg)
	objectPath := strings.TrimSpace(cfg["wa_profile_picture_path"])
	if objectPath != "" && h.gcsAdapter != nil {
		if err := h.gcsAdapter.DeleteObject(c.Request.Context(), objectPath); err != nil {
			slog.Warn("failed to delete cached WhatsApp profile picture from GCS",
				"channel_id", channelID, "object_path", objectPath, "error", err)
		}
	}
	if err := h.channelRepo.MergeCredentials(c.Request.Context(), channelID, map[string]string{
		"wa_profile_picture_path":   "",
		"wa_profile_picture_url_fp": "",
	}); err != nil {
		slog.Error("failed to clear cached profile picture markers",
			"channel_id", channelID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to clear cached picture"})
		return
	}
	metaCacheInvalidate(channelID)
	c.Status(http.StatusNoContent)
}

// HandleUploadTemplateMedia godoc
// @Summary      Upload media for use as a WhatsApp template HEADER
// @Description  Two-step Resumable Upload to Meta. The returned `handle`
//
//	is the value to plug into a template's `header` component
//	`example.header_handle` field (image / video / document templates).
//	Authenticates with the app token (id|secret); the channel's user
//	token is irrelevant here because the upload bucket is per-app.
//
// @Tags         channels
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Channel ID — used for authz scope, not the upload itself"
// @Param        file  formData  file  true  "Image / Video / Document"
// @Success      200  {object}  map[string]string  "{handle: ...}"
// @Router       /channels/{id}/whatsapp/templates/upload-media [post]
func (h *WhatsAppConfigHandler) HandleUploadTemplateMedia(c *gin.Context) {
	if h.appID == "" || h.appSecret == "" {
		c.JSON(http.StatusInternalServerError, domain.AppError{
			Code:    "WA_APP_NOT_CONFIGURED",
			Message: "WhatsApp app credentials missing — cannot reach Meta Resumable Upload",
		})
		return
	}
	// Authz scope: must own the channel even though the upload itself is
	// app-scoped. Without this any tenant could upload via any channel.
	if _, _, _, ok := h.resolveWAChannel(c); !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "FILE_REQUIRED", Message: "multipart 'file' field is required",
		})
		return
	}
	// Largest WhatsApp media type is document at 100MB. Reject before
	// reading bytes so we don't fill memory on malicious uploads.
	if fileHeader.Size > 100*1024*1024 {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "FILE_TOO_LARGE", Message: "file must be <= 100MB",
		})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "FILE_OPEN_FAILED", Message: err.Error()})
		return
	}
	defer f.Close()
	bytesData, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "FILE_READ_FAILED", Message: err.Error()})
		return
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = sniffMime(bytesData)
	}
	if mimeType == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "INVALID_MIME", Message: "could not determine file MIME type",
		})
		return
	}
	// Per-type size enforcement matches Meta's Cloud API limits so we
	// fail fast instead of after the upload begins.
	if max, ok := waMediaMaxBytes(mimeType); ok && fileHeader.Size > max {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "FILE_TOO_LARGE",
			Message: fmt.Sprintf("file exceeds Meta limit for %s", mimeType),
		})
		return
	}

	handle, err := waadapter.ResumableUploadMedia(c.Request.Context(), h.appID, h.appSecret, bytesData, mimeType)
	if err != nil {
		slog.Error("failed to upload WhatsApp template media", "error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionUploadMedia, "WA_TEMPLATE_MEDIA_UPLOAD_FAILED",
			"failed to upload media to Meta")
		c.JSON(status, appErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"handle": handle, "mime_type": mimeType})
}

// sniffMime classifies the first few bytes of an upload. Mirrors the
// magic-byte check used by the profile picture handler so we don't
// trust the client-supplied Content-Type. Covers the formats Meta
// accepts as template headers (image, video, pdf).
func sniffMime(b []byte) string {
	if len(b) >= 4 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return "image/jpeg"
	}
	if len(b) >= 8 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 {
		return "image/png"
	}
	if len(b) >= 4 && b[0] == '%' && b[1] == 'P' && b[2] == 'D' && b[3] == 'F' {
		return "application/pdf"
	}
	// MP4: 'ftyp' box at offset 4
	if len(b) >= 12 && b[4] == 'f' && b[5] == 't' && b[6] == 'y' && b[7] == 'p' {
		return "video/mp4"
	}
	return ""
}

// waMediaMaxBytes returns Meta's max file size for a given mime so the
// handler rejects out-of-spec uploads before burning network bandwidth.
// (image 5MB, video 16MB, audio 16MB, document 100MB.)
func waMediaMaxBytes(mime string) (int64, bool) {
	switch {
	case mime == "image/jpeg" || mime == "image/png":
		return 5 * 1024 * 1024, true
	case mime == "video/mp4" || mime == "video/3gpp":
		return 16 * 1024 * 1024, true
	case mime == "audio/mpeg" || mime == "audio/aac" || mime == "audio/ogg":
		return 16 * 1024 * 1024, true
	case mime == "application/pdf":
		return 100 * 1024 * 1024, true
	}
	return 0, false
}

type createTemplateRequest struct {
	Name       string                   `json:"name" binding:"required"`
	Category   string                   `json:"category" binding:"required"`
	Language   string                   `json:"language" binding:"required"`
	Components []map[string]interface{} `json:"components" binding:"required"`
}

// HandleCreateTemplate godoc
// @Summary      Create a WhatsApp message template
// @Description  Submits a new message template to Meta for approval.
// @Tags         channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                  true  "Channel ID"
// @Param        body  body  createTemplateRequest   true  "Template payload"
// @Success      201
// @Router       /channels/{id}/templates [post]
func (h *WhatsAppConfigHandler) HandleCreateTemplate(c *gin.Context) {
	_, wabaID, accessToken, ok := h.resolveWAChannel(c)
	if !ok {
		return
	}
	if wabaID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_WABA_ID", Message: "channel has no WABA ID configured"})
		return
	}

	var req createTemplateRequest
	if !bindJSON(c, &req) {
		return
	}

	if !templateNameRegex.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_TEMPLATE_NAME", Message: "name must match ^[a-z0-9_]{1,512}$"})
		return
	}

	// Pre-validate components for known Meta footguns. The validator on
	// Meta's side returns a generic 2388299 without telling the user
	// which field broke; catching it here gives a straight answer.
	if reason := waadapter.PrevalidateComponents(req.Components); reason != "" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "INVALID_TEMPLATE_CONTENT",
			Message: reason,
		})
		return
	}

	payload := waadapter.TemplatePayload{
		Name:                req.Name,
		Category:            req.Category,
		Language:            req.Language,
		Components:          req.Components,
		AllowCategoryChange: true,
	}
	if err := waadapter.CreateTemplate(c.Request.Context(), wabaID, accessToken, payload); err != nil {
		// Log the outbound payload (the one we actually sent to Meta)
		// so we can correlate 2388299 with the exact body/category mix,
		// plus Meta's localized user_title/user_msg that explain *why*
		// — they carry the real reason while top-level "message" is
		// always the generic "Invalid parameter".
		outboundBytes, _ := json.Marshal(payload)
		fields := []any{
			"waba_id", wabaID,
			"template", req.Name,
			"outbound_payload", string(outboundBytes),
			"error", err,
		}
		var meta *waadapter.MetaError
		if errors.As(err, &meta) {
			fields = append(fields,
				"meta_code", meta.Code,
				"meta_subcode", meta.Subcode,
				"meta_user_title", meta.UserTitle,
				"meta_user_msg", meta.UserMsg,
				"meta_raw", meta.Raw,
			)
		}
		slog.Error("failed to create WhatsApp template", fields...)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionCreateTemplate,
			"WA_TEMPLATE_CREATE_FAILED", "failed to create template")
		c.JSON(status, appErr)
		return
	}
	c.Status(http.StatusCreated)
}

// HandleDeleteTemplate godoc
// @Summary      Delete a WhatsApp message template
// @Tags         channels
// @Produce      json
// @Security     BearerAuth
// @Param        id     path  string  true  "Channel ID"
// @Param        name   path  string  true  "Template name"
// @Success      204
// @Router       /channels/{id}/templates/{name} [delete]
func (h *WhatsAppConfigHandler) HandleDeleteTemplate(c *gin.Context) {
	_, wabaID, accessToken, ok := h.resolveWAChannel(c)
	if !ok {
		return
	}
	if wabaID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MISSING_WABA_ID", Message: "channel has no WABA ID configured"})
		return
	}

	name := c.Param("name")
	if !templateNameRegex.MatchString(name) {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_TEMPLATE_NAME", Message: "invalid template name"})
		return
	}

	if err := waadapter.DeleteTemplate(c.Request.Context(), wabaID, accessToken, name); err != nil {
		slog.Error("failed to delete WhatsApp template", "waba_id", wabaID, "template", name, "error", err)
		status, appErr := mapMetaErrorAction(err, waadapter.ActionDeleteTemplate,
			"WA_TEMPLATE_DELETE_FAILED", "failed to delete template")
		c.JSON(status, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}
