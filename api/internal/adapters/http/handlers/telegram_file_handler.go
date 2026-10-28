package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/adapters/channels/telegram"
	"github.com/replider/api/internal/core/domain"
)

// TelegramFileHandler proxies Telegram CDN files (profile photos today;
// media messages tomorrow) through the API so the bot token stays
// server-side. URLs are minted by telegram.URLSigner and carry an
// HMAC signature that pins them to (channel_id, file_path, exp).
//
// Endpoint is intentionally unauthenticated: the HMAC signature is the
// auth. This keeps it loadable from <img>/NetworkImage which can't
// attach Authorization headers.
type TelegramFileHandler struct {
	channelRepo domain.ChannelRepository
	tgAdapter   *telegram.TelegramAdapter
}

func NewTelegramFileHandler(
	channelRepo domain.ChannelRepository,
	tgAdapter *telegram.TelegramAdapter,
) *TelegramFileHandler {
	return &TelegramFileHandler{channelRepo: channelRepo, tgAdapter: tgAdapter}
}

// Proxy validates the signed URL, looks up the channel's bot token,
// fetches the file from Telegram and streams the body back to the
// caller. Only safe sub-paths are allowed (no "..", no absolute).
func (h *TelegramFileHandler) Proxy(c *gin.Context) {
	channelID := c.Query("c")
	filePath := c.Query("p")
	expStr := c.Query("e")
	sig := c.Query("s")

	if channelID == "" || filePath == "" || expStr == "" || sig == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if strings.Contains(filePath, "..") || strings.HasPrefix(filePath, "/") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !h.tgAdapter.VerifyFileURLSignature(channelID, filePath, expStr, sig) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	channel, err := h.channelRepo.GetByID(ctx, channelID)
	if err != nil || channel == nil || channel.Type != "telegram" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	resp, err := h.tgAdapter.FetchFile(ctx, channel, filePath)
	if err != nil {
		slog.Warn("telegram file proxy fetch failed",
			"channel_id", channelID, "path", filePath, "error", err)
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.AbortWithStatus(resp.StatusCode)
		return
	}

	// Pass through Telegram's content headers so browsers render the
	// correct MIME (jpeg/png/ogg/mp4/...). Aggressive caching is safe:
	// the signed URL itself rotates via exp, so a stale cache just
	// returns until the URL is reminted upstream.
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		slog.Debug("telegram file proxy stream aborted",
			"channel_id", channelID, "error", err)
	}
}
