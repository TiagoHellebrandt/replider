package telegram

import (
	"testing"
)

func TestParseUpdate_TextMessages(t *testing.T) {
	tests := []struct {
		name         string
		payload      string
		wantChatID   string
		wantText     string
		wantName     string
		wantUsername string
		wantType     string
		wantOK       bool
	}{
		{
			name: "valid text message with full name",
			payload: `{
				"update_id": 100,
				"message": {
					"message_id": 1,
					"from": {
						"id": 42,
						"is_bot": false,
						"first_name": "John",
						"last_name": "Doe",
						"username": "johndoe"
					},
					"chat": {"id": 99, "type": "private"},
					"date": 1700000000,
					"text": "Hello bot"
				}
			}`,
			wantChatID:   "99",
			wantText:     "Hello bot",
			wantName:     "John Doe",
			wantUsername: "johndoe",
			wantType:     "text",
			wantOK:       true,
		},
		{
			name: "valid text message without last name",
			payload: `{
				"update_id": 102,
				"message": {
					"message_id": 3,
					"from": {"id": 77, "is_bot": false, "first_name": "Bob"},
					"chat": {"id": 300, "type": "private"},
					"date": 1700000002,
					"text": "Hey"
				}
			}`,
			wantChatID:   "300",
			wantText:     "Hey",
			wantName:     "Bob",
			wantUsername: "",
			wantType:     "text",
			wantOK:       true,
		},
		{
			name: "message without From field",
			payload: `{
				"update_id": 105,
				"message": {
					"message_id": 5,
					"chat": {"id": 500, "type": "group"},
					"date": 1700000004,
					"text": "anonymous message"
				}
			}`,
			wantChatID:   "500",
			wantText:     "anonymous message",
			wantName:     "",
			wantUsername: "",
			wantType:     "text",
			wantOK:       true,
		},
		{
			name:    "non-message update (nil message)",
			payload: `{"update_id": 103}`,
			wantOK:  false,
		},
		{
			name: "empty message with no content",
			payload: `{
				"update_id": 104,
				"message": {
					"message_id": 4,
					"from": {"id": 88, "is_bot": false, "first_name": "Eve"},
					"chat": {"id": 400, "type": "private"},
					"date": 1700000003
				}
			}`,
			wantOK: false,
		},
		{
			name:    "invalid JSON",
			payload: `{not valid json`,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := ParseUpdate([]byte(tt.payload))

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if parsed.ChatID != tt.wantChatID {
				t.Errorf("ChatID = %q, want %q", parsed.ChatID, tt.wantChatID)
			}
			if parsed.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", parsed.Text, tt.wantText)
			}
			if parsed.SenderName != tt.wantName {
				t.Errorf("SenderName = %q, want %q", parsed.SenderName, tt.wantName)
			}
			if parsed.Username != tt.wantUsername {
				t.Errorf("Username = %q, want %q", parsed.Username, tt.wantUsername)
			}
			if parsed.MediaType != tt.wantType {
				t.Errorf("MediaType = %q, want %q", parsed.MediaType, tt.wantType)
			}
		})
	}
}

func TestParseUpdate_MediaMessages(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantType   string
		wantFileID string
		wantLatLon string
		wantText   string // caption
	}{
		{
			name: "photo with caption",
			payload: `{
				"update_id": 200,
				"message": {
					"message_id": 10,
					"from": {"id": 1, "first_name": "Ana"},
					"chat": {"id": 10, "type": "private"},
					"date": 1700000010,
					"caption": "Look at this",
					"photo": [
						{"file_id": "small_id", "width": 100, "height": 100},
						{"file_id": "large_id", "width": 1280, "height": 720}
					]
				}
			}`,
			wantType:   "image",
			wantFileID: "large_id",
			wantText:   "Look at this",
		},
		{
			name: "audio message",
			payload: `{
				"update_id": 201,
				"message": {
					"message_id": 11,
					"from": {"id": 2, "first_name": "Beto"},
					"chat": {"id": 11, "type": "private"},
					"date": 1700000011,
					"audio": {"file_id": "audio_file_id", "duration": 30, "file_name": "song.mp3"}
				}
			}`,
			wantType:   "audio",
			wantFileID: "audio_file_id",
		},
		{
			name: "voice message",
			payload: `{
				"update_id": 202,
				"message": {
					"message_id": 12,
					"from": {"id": 3, "first_name": "Carla"},
					"chat": {"id": 12, "type": "private"},
					"date": 1700000012,
					"voice": {"file_id": "voice_file_id", "duration": 5}
				}
			}`,
			wantType:   "voice",
			wantFileID: "voice_file_id",
		},
		{
			name: "video message",
			payload: `{
				"update_id": 203,
				"message": {
					"message_id": 13,
					"from": {"id": 4, "first_name": "Diego"},
					"chat": {"id": 13, "type": "private"},
					"date": 1700000013,
					"video": {"file_id": "video_file_id", "duration": 60}
				}
			}`,
			wantType:   "video",
			wantFileID: "video_file_id",
		},
		{
			name: "document message",
			payload: `{
				"update_id": 204,
				"message": {
					"message_id": 14,
					"from": {"id": 5, "first_name": "Elisa"},
					"chat": {"id": 14, "type": "private"},
					"date": 1700000014,
					"document": {"file_id": "doc_file_id", "file_name": "report.pdf"}
				}
			}`,
			wantType:   "document",
			wantFileID: "doc_file_id",
			wantText:   "report.pdf",
		},
		{
			name: "sticker message",
			payload: `{
				"update_id": 205,
				"message": {
					"message_id": 15,
					"from": {"id": 6, "first_name": "Fabio"},
					"chat": {"id": 15, "type": "private"},
					"date": 1700000015,
					"sticker": {"file_id": "sticker_file_id", "width": 512, "height": 512}
				}
			}`,
			wantType:   "sticker",
			wantFileID: "sticker_file_id",
		},
		{
			name: "location message",
			payload: `{
				"update_id": 206,
				"message": {
					"message_id": 16,
					"from": {"id": 7, "first_name": "Gabi"},
					"chat": {"id": 16, "type": "private"},
					"date": 1700000016,
					"location": {"latitude": -23.5505, "longitude": -46.6333}
				}
			}`,
			wantType:   "location",
			wantLatLon: "-23.550500,-46.633300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := ParseUpdate([]byte(tt.payload))
			if !ok {
				t.Fatalf("ParseUpdate() returned ok=false, want true")
			}
			if parsed.MediaType != tt.wantType {
				t.Errorf("MediaType = %q, want %q", parsed.MediaType, tt.wantType)
			}
			if tt.wantFileID != "" && parsed.MediaFileID != tt.wantFileID {
				t.Errorf("MediaFileID = %q, want %q", parsed.MediaFileID, tt.wantFileID)
			}
			if tt.wantLatLon != "" && parsed.LatLon != tt.wantLatLon {
				t.Errorf("LatLon = %q, want %q", parsed.LatLon, tt.wantLatLon)
			}
			if tt.wantText != "" && parsed.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", parsed.Text, tt.wantText)
			}
		})
	}
}
