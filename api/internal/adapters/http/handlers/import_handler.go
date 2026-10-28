package handlers

import (
	"bufio"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ImportHandler handles conversation history imports.
type ImportHandler struct {
	convRepo    domain.ConversationRepository
	msgRepo     domain.MessageRepository
	channelRepo domain.ChannelRepository
	agentRepo   domain.AgentRepository
}

func NewImportHandler(
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	channelRepo domain.ChannelRepository,
	agentRepo domain.AgentRepository,
) *ImportHandler {
	return &ImportHandler{
		convRepo:    convRepo,
		msgRepo:     msgRepo,
		channelRepo: channelRepo,
		agentRepo:   agentRepo,
	}
}

// ImportRequest represents a simple conversation import.
type ImportRequest struct {
	ChannelID   string          `json:"channel_id" binding:"required"`
	ContactID   string          `json:"contact_id" binding:"required"`
	ContactName string          `json:"contact_name"`
	Messages    []ImportMessage `json:"messages"`
	// Alternative: raw WhatsApp export text
	WhatsAppExport string `json:"whatsapp_export"`
}

type ImportMessage struct {
	Direction string `json:"direction" binding:"required"` // "inbound" or "outbound"
	Content   string `json:"content" binding:"required"`
	Sender    string `json:"sender"`    // contact name, agent name, or "bot"
	Timestamp string `json:"timestamp"` // RFC3339 or "2006-01-02 15:04"
}

type ImportResult struct {
	ConversationID string `json:"conversation_id"`
	MessagesCount  int    `json:"messages_imported"`
}

// HandleImportConversation imports a conversation history.
// Accepts either structured messages or a raw WhatsApp export.
func (h *ImportHandler) HandleImportConversation(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: err.Error()})
		return
	}

	// Verify channel ownership (directly via account_id)
	channel, err := h.channelRepo.GetByID(c.Request.Context(), req.ChannelID)
	if err != nil || channel == nil || channel.AccountID != accountID {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "canal não encontrado"})
		return
	}

	// Parse WhatsApp export if provided
	messages := req.Messages
	if req.WhatsAppExport != "" {
		messages = parseWhatsAppExport(req.WhatsAppExport, req.ContactName)
	}

	if len(messages) == 0 {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "NO_MESSAGES", Message: "nenhuma mensagem para importar"})
		return
	}

	// Create conversation
	now := time.Now()
	convID := ulid.Make().String()
	contactName := req.ContactName
	if contactName == "" {
		contactName = req.ContactID
	}

	conv := &domain.Conversation{
		ID:          convID,
		AccountID:   accountID,
		ChannelID:   req.ChannelID,
		AgentID:     channel.AgentID,
		ContactID:   req.ContactID,
		ContactName: &contactName,
		Status:      domain.ConversationClosed, // imported conversations start as closed
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.convRepo.Create(c.Request.Context(), conv); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "falha ao criar conversa"})
		return
	}

	// Import messages
	imported := 0
	for _, msg := range messages {
		ts := now
		if msg.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
				ts = parsed
			} else if parsed, err := time.Parse("2006-01-02 15:04", msg.Timestamp); err == nil {
				ts = parsed
			} else if parsed, err := time.Parse("02/01/2006 15:04", msg.Timestamp); err == nil {
				ts = parsed
			}
		}

		sender := msg.Sender
		if sender == "" {
			if msg.Direction == "inbound" {
				sender = req.ContactID
			} else {
				sender = "agent"
			}
		}

		content := msg.Content
		m := &domain.Message{
			ID:             ulid.Make().String(),
			ChannelID:      req.ChannelID,
			ConversationID: &convID,
			From:           sender,
			Type:           "text",
			Content:        &content,
			Direction:      msg.Direction,
			Timestamp:      ts,
		}

		if _, err := h.msgRepo.Create(c.Request.Context(), m); err != nil {
			continue // skip individual message errors
		}
		imported++
	}

	c.JSON(http.StatusCreated, ImportResult{
		ConversationID: convID,
		MessagesCount:  imported,
	})
}

// parseWhatsAppExport parses a WhatsApp chat export text into messages.
// Format: "DD/MM/YYYY HH:MM - Sender: Message content"
var whatsappLineRegex = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4} \d{2}:\d{2}) - ([^:]+): (.+)$`)

func parseWhatsAppExport(text, contactName string) []ImportMessage {
	var messages []ImportMessage
	scanner := bufio.NewScanner(strings.NewReader(text))
	var currentMsg *ImportMessage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := whatsappLineRegex.FindStringSubmatch(line)
		if matches == nil {
			// Continuation of previous message
			if currentMsg != nil {
				currentMsg.Content += "\n" + line
			}
			continue
		}

		// Save previous message
		if currentMsg != nil {
			messages = append(messages, *currentMsg)
		}

		timestamp := matches[1]
		sender := strings.TrimSpace(matches[2])
		content := matches[3]

		// Determine direction: if sender matches contactName, it's inbound
		direction := "outbound"
		if contactName != "" && strings.EqualFold(sender, contactName) {
			direction = "inbound"
		}

		currentMsg = &ImportMessage{
			Direction: direction,
			Content:   content,
			Sender:    sender,
			Timestamp: timestamp,
		}
	}

	// Don't forget the last message
	if currentMsg != nil {
		messages = append(messages, *currentMsg)
	}

	return messages
}
