package domain

import (
	"context"
	"time"
)

// MessageAttachment is one file that arrived with a message.
//
// Its own table rather than more columns on messages: one email carries N
// files, and each needs its own scan verdict — a message with three
// attachments can legitimately have one infected and two clean.
type MessageAttachment struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id"`
	AccountID string `json:"-"`

	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`

	// ObjectPath is the private-bucket path. Never serialised: handing the
	// client a storage path invites it to build its own URL, and the whole
	// access model is that reads go through a signed URL the server mints
	// after checking ownership AND the scan verdict.
	ObjectPath string `json:"-"`

	// IsInline marks message furniture (a signature logo, a tracking pixel)
	// rather than a document someone meant to send.
	IsInline bool `json:"is_inline"`

	// ContentID is the cid: this file is referenced by from the message's
	// HTML (empty for regular attachments). Server-side only: it exists so
	// the "view original" can rewrite cid: into the stored bytes.
	ContentID string `json:"-"`

	ScanStatus    ScanStatus `json:"scan_status"`
	ScanSignature string     `json:"scan_signature,omitempty"`
	ScannedAt     *time.Time `json:"scanned_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// Downloadable is the single question the download endpoint asks.
func (a *MessageAttachment) Downloadable() bool { return a.ScanStatus.Downloadable() }

// ErrAttachmentNotDownloadable is returned instead of a signed URL when the
// file has not come back clean.
//
// One error for every non-clean state, with the state carried in the payload:
// separate codes would tempt the client into treating "skipped" as a softer
// "clean", and the distinction that matters to the reader is what the message
// says, not which branch produced it.
var ErrAttachmentNotDownloadable = &AppError{
	Code:    "ATTACHMENT_NOT_DOWNLOADABLE",
	Message: "este arquivo ainda não foi liberado para download",
}

// ErrAttachmentNotFound also covers "belongs to another account": one error
// for both so probing ids cannot distinguish a file that does not exist from
// one the caller may not have.
var ErrAttachmentNotFound = &AppError{
	Code:    "ATTACHMENT_NOT_FOUND",
	Message: "anexo não encontrado",
}

// ErrAttachmentInfected is its own code because the UI must not offer a
// retry: there is nothing to wait for.
var ErrAttachmentInfected = &AppError{
	Code:    "ATTACHMENT_INFECTED",
	Message: "o antivírus encontrou uma ameaça neste arquivo",
}

// MessageAttachmentRepository persists and reads attachments.
type MessageAttachmentRepository interface {
	Create(ctx context.Context, a *MessageAttachment) error
	// ListByMessage returns every attachment of one message, oldest first.
	ListByMessage(ctx context.Context, messageID string) ([]*MessageAttachment, error)
	// ListByMessages is the batched form for a transcript page — N+1 on a
	// conversation with fifty emails is fifty round-trips.
	ListByMessages(ctx context.Context, messageIDs []string) (map[string][]*MessageAttachment, error)
	// GetForAccount scopes by account: this is the query the download
	// endpoint runs, and it is the one place a missed tenant check hands a
	// customer's contract to another account.
	GetForAccount(ctx context.Context, id, accountID string) (*MessageAttachment, error)
	// SetScanResult records a verdict. Idempotent by id.
	SetScanResult(ctx context.Context, id string, status ScanStatus, signature string) error
	// ListUnscanned drives the rescan worker: files stored while the scanner
	// was down must not stay unopenable forever.
	ListUnscanned(ctx context.Context, limit int32) ([]*MessageAttachment, error)
}
