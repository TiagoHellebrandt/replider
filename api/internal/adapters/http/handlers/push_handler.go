package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// PushHandler exposes the Web Push (and future FCM/APNs) subscription API.
// One device = one POST to /v1/push/subscriptions. The server keeps the
// endpoint so inbound-message pushes can fan out to all attendants.
type PushHandler struct {
	uc *usecases.PushNotificationUseCase
}

func NewPushHandler(uc *usecases.PushNotificationUseCase) *PushHandler {
	return &PushHandler{uc: uc}
}

// PushSubscribeReq mirrors the W3C PushSubscription JSON shape for the web
// path and carries a device_token field for future FCM/APNs clients.
type PushSubscribeReq struct {
	Platform string `json:"platform"` // web|ios|android, default web
	Endpoint string `json:"endpoint" binding:"required"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	DeviceToken string `json:"device_token"`
	UserAgent   string `json:"user_agent"`
}

// HandleSubscribe godoc
// @Summary      Register a push subscription for the calling user
// @Description  Upserts a web/mobile push endpoint owned by the authenticated user.
// @Tags         push
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body PushSubscribeReq true "Subscription payload"
// @Success      201 {object} domain.PushSubscription
// @Router       /push/subscriptions [post]
func (h *PushHandler) HandleSubscribe(c *gin.Context) {
	var req PushSubscribeReq
	if !bindJSON(c, &req) {
		return
	}
	sub, err := h.uc.Subscribe(c.Request.Context(), getAccountID(c), getUserID(c), usecases.SubscribeDTO{
		Platform:    req.Platform,
		Endpoint:    req.Endpoint,
		P256dh:      req.Keys.P256dh,
		Auth:        req.Keys.Auth,
		DeviceToken: req.DeviceToken,
		UserAgent:   req.UserAgent,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":         sub.ID,
		"endpoint":   sub.Endpoint,
		"platform":   sub.Platform,
		"enabled":    sub.Enabled,
		"created_at": sub.CreatedAt,
	})
}

type PushUnsubscribeReq struct {
	Endpoint string `json:"endpoint" binding:"required"`
}

// HandleUnsubscribe godoc
// @Summary      Remove a push subscription
// @Tags         push
// @Security     BearerAuth
// @Router       /push/subscriptions [delete]
func (h *PushHandler) HandleUnsubscribe(c *gin.Context) {
	var req PushUnsubscribeReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.Unsubscribe(c.Request.Context(), getUserID(c), req.Endpoint); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleList godoc
// @Summary      List the caller's push subscriptions
// @Tags         push
// @Security     BearerAuth
// @Router       /push/subscriptions [get]
func (h *PushHandler) HandleList(c *gin.Context) {
	subs, err := h.uc.List(c.Request.Context(), getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list push subscriptions"})
		return
	}
	// Strip secret key material — callers never need to read their own
	// p256dh / auth after upload.
	out := make([]gin.H, 0, len(subs))
	for _, s := range subs {
		out = append(out, gin.H{
			"id":         s.ID,
			"platform":   s.Platform,
			"endpoint":   s.Endpoint,
			"user_agent": s.UserAgent,
			"enabled":    s.Enabled,
			"created_at": s.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// HandleSendTest godoc
// @Summary      Send a diagnostic push to the caller's own subscriptions
// @Description  Fans a synthetic notification to every enabled subscription owned by the
//
//	authenticated user and returns per-endpoint outcomes. Lets the dashboard
//	surface why pushes aren't arriving (no rows, key rotated, gone, etc.).
//
// @Tags         push
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} usecases.PushTestResult
// @Router       /push/test [post]
func (h *PushHandler) HandleSendTest(c *gin.Context) {
	results, err := h.uc.SendTestToUser(c.Request.Context(), getUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, results)
}

// HandleVAPIDPublicKey godoc
// @Summary      Return the server's VAPID public key for web push
// @Description  Flutter web passes this value to PushManager.subscribe(). Empty string when push is disabled.
// @Tags         push
// @Security     BearerAuth
// @Router       /push/vapid-public-key [get]
func (h *PushHandler) HandleVAPIDPublicKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"public_key": h.uc.VAPIDPublicKey(),
	})
}
