package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024 // 64KB — AI responses can be large
)

type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	Send      chan []byte
	AccountID string
	UserID    string
	cancel    context.CancelFunc
}

type WSMessage struct {
	AgentID string `json:"agent_id"`
	Content string `json:"content"`
}

type WSHandler struct {
	Hub              *Hub
	processMessageUC *usecases.ProcessMessageUseCase
	jwtService       domain.JWTService
	upgrader         websocket.Upgrader
	// blacklist and sessions make first-frame auth honour logout and
	// member removal, exactly as AuthMiddleware does for HTTP. Either may
	// be nil (tests); nil skips that check.
	blacklist domain.TokenBlacklist
	sessions  domain.SessionInvalidationReader
}

// SetTokenBlacklist wires logout revocation into first-frame auth.
func (h *WSHandler) SetTokenBlacklist(bl domain.TokenBlacklist) { h.blacklist = bl }

// SetSessionInvalidationReader wires member-removal revocation into
// first-frame auth.
func (h *WSHandler) SetSessionInvalidationReader(r domain.SessionInvalidationReader) {
	h.sessions = r
}

// NewWSHandler wires the WebSocket handler. `environment` follows APP_ENV
// ("production" | anything else). When `allowedOrigins` is empty:
//   - production: every WS upgrade is rejected (fail-closed).
//   - non-production: allow-all stays on so local dev / e2e work without
//     having to set CORS_ALLOWED_ORIGINS.
//
// A misconfigured prod (env var unset) used to silently allow any origin
// to upgrade — combined with first-frame JWT auth, a leaked token meant
// cross-origin live-stream of the entire dashboard.
func NewWSHandler(hub *Hub, uc *usecases.ProcessMessageUseCase, jwtService domain.JWTService, allowedOrigins, environment string) *WSHandler {
	origins := parseOrigins(allowedOrigins)
	isProduction := strings.EqualFold(environment, "production")
	if len(origins) == 0 && isProduction {
		slog.Error("websocket: CORS_ALLOWED_ORIGINS is empty in production — all WS upgrades will be rejected. Set CORS_ALLOWED_ORIGINS to enable.")
	}
	return &WSHandler{
		Hub:              hub,
		processMessageUC: uc,
		jwtService:       jwtService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				if len(origins) == 0 {
					return !isProduction
				}
				origin := r.Header.Get("Origin")
				for _, o := range origins {
					if strings.EqualFold(origin, o) {
						return true
					}
				}
				return false
			},
		},
	}
}

// parseOrigins splits a comma-separated string into trimmed, non-empty origins.
func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// authenticateViaFrame reads the first WebSocket message expecting {"type":"auth","token":"..."}.
// Returns accountID, userID on success or an error.
func (h *WSHandler) authenticateViaFrame(conn *websocket.Conn) (string, string, error) {
	// Give the client 10 seconds to send the auth frame
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return "", "", fmt.Errorf("failed to read auth frame: %w", err)
	}

	var authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(msg, &authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
		return "", "", fmt.Errorf("invalid auth frame")
	}

	claims, err := h.jwtService.ValidateToken(authMsg.Token)
	if err != nil {
		return "", "", fmt.Errorf("invalid token: %w", err)
	}
	if err := h.checkRevocation(authMsg.Token, claims); err != nil {
		return "", "", err
	}

	// Role satisfaction, not string equality — the same rule the dashboard
	// routes go through (middleware.RequireRole). This check was left as a
	// literal comparison when US-F008 introduced owner/gestor/vendedor, so
	// an OWNER got a 403 on the socket and the client reconnected every 3s
	// forever: the account's highest-privileged user was the only one with
	// no realtime.
	//
	// The gate mirrors the dashboard's ("admin", "attendant"), so whoever
	// may open the inbox may receive its events — and a vendedor, who
	// deliberately satisfies neither, still cannot.
	role := domain.NormalizeRole(claims.Role)
	if !domain.RoleSatisfies(role, domain.RoleAdminRole) &&
		!domain.RoleSatisfies(role, domain.RoleAtendenteRole) {
		return "", "", fmt.Errorf("insufficient role: %s", claims.Role)
	}

	// Reset read deadline for normal operation
	_ = conn.SetReadDeadline(time.Time{})

	return claims.AccountID, claims.UserID, nil
}

// HandleConnections upgrades the HTTP request to a WebSocket
// @Summary      Connect to Chat WebSocket
// @Description  Establishes a WebSocket connection for real-time chat with AI Agents. Sends JSON frames: {"chunk":"..."} for streaming tokens, {"done":"true"} for end-of-message, {"error":"..."} for errors, and {"status":"credit_exhausted"} when AI limits are reached.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        agent_id query string true "Agent ID to chat with"
// @Success      101  {string}  string  "Switching Protocols – WebSocket connection established"
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Router       /ws/chat [get]
func (h *WSHandler) HandleConnections(c *gin.Context) {
	// Support two auth methods:
	// 1. Query param ?token= (legacy, authenticated by middleware before upgrade)
	// 2. First-frame auth (new: upgrade without auth, authenticate via first message)
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	needsFrameAuth := accountID == "" || userID == ""

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("websocket upgrade error", "error", err)
		return
	}

	// If not already authenticated via middleware, wait for auth frame
	if needsFrameAuth {
		accountID, userID, err = h.authenticateViaFrame(conn)
		if err != nil {
			slog.Warn("websocket frame auth failed", "error", err)
			_ = conn.WriteJSON(map[string]string{"error": "unauthorized"})
			conn.Close()
			return
		}
	}

	// Use the HTTP request context as parent so the WebSocket lifecycle
	// is tied to the server's graceful shutdown. Previously used
	// context.Background() which could leak goroutines on server stop.
	ctx, cancel := context.WithCancel(c.Request.Context())
	client := &Client{
		Hub:       h.Hub,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		AccountID: accountID,
		UserID:    userID,
		cancel:    cancel,
	}

	client.Hub.Register <- client

	go client.writePump()
	go client.readPump(ctx, h.processMessageUC)
}

func (c *Client) readPump(ctx context.Context, uc *usecases.ProcessMessageUseCase) {
	defer func() {
		c.cancel()
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var payload WSMessage
		if err := json.Unmarshal(message, &payload); err != nil {
			c.Send <- []byte("{\"error\":\"invalid message format\"}")
			continue
		}

		// Process message via Use Case
		dto := usecases.ProcessMessageDTO{
			AccountID: c.AccountID,
			AgentID:   payload.AgentID,
			Message:   payload.Content,
		}

		msgCtx, msgCancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := uc.Execute(msgCtx, dto)
		if err != nil {
			msgCancel()
			// Hybrid Fallback check
			if errors.Is(err, domain.ErrNoCredits) || err.Error() == domain.ErrNoCredits.Error() ||
				errors.Is(err, domain.ErrUsageLimitExceeded) || err.Error() == domain.ErrUsageLimitExceeded.Error() ||
				errors.Is(err, domain.ErrAccountSuspended) || err.Error() == domain.ErrAccountSuspended.Error() {
				errMsg, _ := json.Marshal(map[string]string{
					"status": "credit_exhausted",
					"error":  "AI limits reached. Please reply manually.",
				})
				c.Send <- errMsg
				continue
			}

			// Generic error
			slog.Error("websocket message processing failed", "error", err)
			errMsg, _ := json.Marshal(map[string]string{"error": "internal server error"})
			c.Send <- errMsg
			continue
		}

		for chunk := range result.Chunks {
			chunkMsg, _ := json.Marshal(map[string]string{"chunk": chunk})
			c.Send <- chunkMsg
		}
		msgCancel()

		// Signal end of message
		doneMsg, _ := json.Marshal(map[string]string{"done": "true"})
		c.Send <- doneMsg

		// Check for usage alert and broadcast to account
		select {
		case alert := <-result.UsageAlert:
			if alert != nil {
				alertMsg, _ := json.Marshal(map[string]interface{}{
					"event":           "usage_alert",
					"percent_used":    alert.PercentUsed,
					"tokens_used":     alert.TokensUsed,
					"effective_limit": alert.EffectiveLimit,
				})
				c.Send <- alertMsg
			}
		default:
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// checkRevocation is the part of AuthMiddleware the handshake skipped when
// the token arrives in the first frame: the logout blacklist and the
// member-removal stamp. Bounded by a short timeout so a slow store cannot
// hold the socket open unauthenticated.
func (h *WSHandler) checkRevocation(token string, claims *domain.UserClaims) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if h.blacklist != nil && h.blacklist.IsRevoked(ctx, token) {
		return errors.New("token has been revoked")
	}
	if h.sessions != nil && claims.IssuedAt != nil {
		at, err := h.sessions.SessionInvalidatedAt(ctx, claims.UserID)
		if err == nil && domain.SessionInvalidatedBefore(claims.IssuedAt.Time, at) {
			return errors.New("session has been revoked")
		}
	}
	return nil
}
