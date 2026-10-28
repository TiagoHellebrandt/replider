package telegram

// Update is the root object of every Telegram webhook payload.
type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID int         `json:"message_id"`
	From      *User       `json:"from,omitempty"`
	Chat      Chat        `json:"chat"`
	Date      int64       `json:"date"`
	Text      string      `json:"text,omitempty"`
	Caption   string      `json:"caption,omitempty"` // caption for photo/video/document
	Photo     []PhotoSize `json:"photo,omitempty"`
	Audio     *Audio      `json:"audio,omitempty"`
	Voice     *Voice      `json:"voice,omitempty"`
	Video     *Video      `json:"video,omitempty"`
	Document  *Document   `json:"document,omitempty"`
	Location  *Location   `json:"location,omitempty"`
	Sticker   *Sticker    `json:"sticker,omitempty"`
}

// User represents a Telegram user or bot.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Chat represents a Telegram chat (private, group, etc.)
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", "supergroup", "channel"
}

// PhotoSize represents one resolution of a photo.
type PhotoSize struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size,omitempty"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// Audio represents an audio file.
type Audio struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Voice represents a voice message.
type Voice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type,omitempty"`
}

// Video represents a video file.
type Video struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Document represents any file type.
type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// Location represents geographic coordinates.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Sticker represents a sticker.
type Sticker struct {
	FileID string `json:"file_id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
