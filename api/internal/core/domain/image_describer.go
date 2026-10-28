package domain

import "context"

// ImageDescriber returns a short natural-language description of an image.
// In Brazilian SMB WhatsApp, customers routinely send photos ("quanto é
// esse?", just a picture of a product). Without conversion to text, the
// bot replies with a canned "não entendi" and looks broken.
//
// Implementations should be best-effort: any failure returns an empty
// string + error, and the caller falls back to a polite "vi que você
// enviou uma imagem" message so the conversation keeps moving.
type ImageDescriber interface {
	// Describe fetches imageURL and returns a one-paragraph description
	// in the given language (ISO-639-1; empty = "pt"). The description
	// is used as the user-message content fed into the AI pipeline.
	Describe(ctx context.Context, imageURL string, language string) (string, error)
}
