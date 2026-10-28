package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
)

// safeFilenameRe matches characters that are NOT alphanumeric, dot, hyphen, or underscore.
var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9.\-_]`)

// blockedExtensions lists file types we refuse to mint upload URLs for.
// The bucket is private and read access is signed, so an attacker
// can't directly execute a stored binary — but we still block these
// to keep them off the platform: customers proxy these signed URLs to
// their own contacts in WhatsApp/Telegram outbound media, and we don't
// want to be the distribution channel for native installers, scripts,
// or shortcut files that could trigger anti-virus / Meta-policy
// rejections on the customer's side.
var blockedExtensions = map[string]struct{}{
	// Native executables
	".exe": {}, ".dll": {}, ".com": {}, ".scr": {}, ".pif": {}, ".cpl": {},
	".msi": {}, ".msp": {}, ".gadget": {}, ".app": {}, ".dmg": {}, ".pkg": {},
	".deb": {}, ".rpm": {}, ".bin": {},
	// Shell / batch / scripting (everything below can spawn a shell on a
	// recipient that double-clicks)
	".bat": {}, ".cmd": {}, ".ps1": {}, ".sh": {}, ".vbs": {}, ".vbe": {},
	".js": {}, ".jse": {}, ".wsf": {}, ".wsh": {}, ".hta": {}, ".lnk": {},
	// JVM / macro-laden archives (LNK in particular is a known phishing vector)
	".jar": {}, ".class": {},
}

// sanitizeFilename removes path separators, restricts characters, and limits length.
func sanitizeFilename(name string) string {
	// Strip path separators and get just the filename
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)

	// Replace any non-alphanumeric characters (except . - _) with underscore
	name = safeFilenameRe.ReplaceAllString(name, "_")

	// Limit length to 255 characters
	if len(name) > 255 {
		name = name[:255]
	}

	// If filename is empty or just ".", generate a UUID-based name
	if name == "" || name == "." {
		name = ulid.Make().String()
	}

	return name
}

// MediaHandler handles media upload requests
type MediaHandler struct {
	gcsAdapter *storage.GCSAdapter
}

// NewMediaHandler creates a new MediaHandler
func NewMediaHandler(gcsAdapter *storage.GCSAdapter) *MediaHandler {
	return &MediaHandler{gcsAdapter: gcsAdapter}
}

type UploadRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
}

type UploadResponse struct {
	SignedURL  string `json:"signed_url"`  // V4 signed PUT URL; expires in 15 min
	ObjectPath string `json:"object_path"` // store this as media_url when calling /reply
}

// HandleUpload godoc
// @Summary      Get a signed upload URL
// @Description  Returns a GCS V4 signed URL that the client can PUT a file to directly. URL expires in 15 minutes.
// @Tags         media
// @Accept       json
// @Produce      json
// @Param        request body UploadRequest true "Upload metadata"
// @Success      200  {object}  UploadResponse  "Signed URL and object path"
// @Failure      400  {object}  domain.AppError  "Invalid request"
// @Failure      500  {object}  domain.AppError  "Failed to generate signed URL"
// @Security     BearerAuth
// @Router       /media/upload [post]
func (h *MediaHandler) HandleUpload(c *gin.Context) {
	accountID := getAccountID(c)
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	var req UploadRequest
	if !bindJSON(c, &req) {
		return
	}

	safeName := sanitizeFilename(req.Filename)
	slog.Debug("media upload filename sanitized", "original", req.Filename, "sanitized", safeName)

	// Reject blocked extensions before minting a signed URL. The PUT
	// itself runs on GCS infrastructure, so once we hand the URL out we
	// can't take it back — gate here at the only choke point.
	if ext := strings.ToLower(filepath.Ext(safeName)); ext != "" {
		if _, blocked := blockedExtensions[ext]; blocked {
			c.JSON(http.StatusBadRequest, domain.AppError{
				Code:    "MEDIA_TYPE_BLOCKED",
				Message: "file type not allowed",
			})
			return
		}
	}

	objectPath := fmt.Sprintf("accounts/%s/%d/%s", accountID, time.Now().UnixMilli(), safeName)

	signedURL, err := h.gcsAdapter.GenerateSignedURL(c.Request.Context(), objectPath, req.ContentType, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "STORAGE_ERROR", Message: "failed to generate upload URL"})
		return
	}

	c.JSON(http.StatusOK, UploadResponse{
		SignedURL:  signedURL,
		ObjectPath: objectPath,
	})
}
