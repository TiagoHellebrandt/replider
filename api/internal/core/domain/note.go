package domain

import "time"

type ConversationNote struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	UserName       string    `json:"user_name"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

var ErrNoteNotFound = &AppError{Code: "NOTE_NOT_FOUND", Message: "note not found"}
