package usecases

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Storing, scanning and serving inbound attachments.
//
// The product asks people to open files sent by strangers it prospected
// itself. Everything here follows from that: the file is stored, scanned,
// and only handed back once a scanner has actually said it is clean.

// AttachmentStore is the narrow slice of object storage this needs — a port
// rather than the whole GCS adapter so the use case can be tested without
// credentials.
type AttachmentStore interface {
	UploadBytes(ctx context.Context, objectPath string, data []byte, contentType string) error
	GenerateReadSignedURL(ctx context.Context, objectPath string, expiry time.Duration) (string, error)
	DownloadObject(ctx context.Context, objectPath string, maxBytes int64) ([]byte, error)
}

// MaxAttachmentBytes caps what is accepted per file.
//
// 25MB matches clamd's default stream limit, so anything storable is also
// scannable — a larger cap would mean routinely storing files that can only
// ever be "skipped", which is a download button that never works.
const MaxAttachmentBytes = 25 * 1024 * 1024

// attachmentURLTTL is short because the URL is minted per click. A long-lived
// signed URL for a customer's contract outlives the session that earned it.
const attachmentURLTTL = 15 * time.Minute

type MessageAttachmentUseCase struct {
	repo    domain.MessageAttachmentRepository
	store   AttachmentStore
	scanner domain.AttachmentScanner
	// mediaSigner mints media-proxy URLs for inline images. Optional
	// (SetMediaSigner) — unwired, the original view falls back to base64
	// data: URIs, which means the API re-downloads every embedded photo
	// from GCS on every open.
	mediaSigner InlineMediaSigner
	// mediaCache pre-warms the proxy's disk cache at ingest. Optional.
	mediaCache MediaByteCache
}

// InlineMediaSigner is the slice of storage.URLSigner the cid: URL
// rewrite needs.
type InlineMediaSigner interface {
	SignObjectURL(accountID, objectPath string, ttl time.Duration) string
}

// SetMediaSigner wires media-proxy URL minting for inline images.
func (uc *MessageAttachmentUseCase) SetMediaSigner(s InlineMediaSigner) { uc.mediaSigner = s }

// MediaByteCache pre-warms the media proxy's local disk cache. Narrow port —
// storage.DiskCache satisfies it; the core never imports the adapter.
type MediaByteCache interface {
	PutBytes(objectPath string, data []byte, contentType string)
}

// SetMediaCache wires ingest-time pre-warming: attachments land in the local
// cache at the moment they arrive, while the bytes are already in memory, so
// even the FIRST open of an email serves its images from local disk instead
// of a ~9s cross-continent GCS fetch.
func (uc *MessageAttachmentUseCase) SetMediaCache(c MediaByteCache) { uc.mediaCache = c }

func NewMessageAttachmentUseCase(
	repo domain.MessageAttachmentRepository,
	store AttachmentStore,
) *MessageAttachmentUseCase {
	return &MessageAttachmentUseCase{repo: repo, store: store}
}

// SetScanner wires the antivirus. Nil means none is configured, and files
// are recorded as skipped rather than clean — the platform never claims a
// verdict it did not compute.
func (uc *MessageAttachmentUseCase) SetScanner(s domain.AttachmentScanner) { uc.scanner = s }

// IncomingAttachment is one file to persist, decoupled from the email
// package so this use case does not depend on a channel adapter.
type IncomingAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
	IsInline    bool
	// ContentID is the cid: the message HTML references this file by. It
	// used to be parsed and dropped — which left every embedded image in
	// the "view original" permanently broken.
	ContentID string
}

// Store persists the files of one message.
//
// Best-effort per file: one attachment failing to upload must not lose the
// message or its siblings. A dropped file is logged loudly, because silence
// is exactly the bug this whole feature replaces.
func (uc *MessageAttachmentUseCase) Store(ctx context.Context, accountID, messageID string, files []IncomingAttachment) []*domain.MessageAttachment {
	stored := make([]*domain.MessageAttachment, 0, len(files))
	for _, f := range files {
		if len(f.Content) == 0 {
			continue
		}
		if len(f.Content) > MaxAttachmentBytes {
			slog.WarnContext(ctx, "attachment too large, not stored",
				"message_id", messageID, "filename", f.Filename, "bytes", len(f.Content))
			continue
		}

		id := ulid.Make().String()
		// Account-first path so a bucket listing is already tenant-scoped,
		// and the attachment id (not the sender's filename) is the key —
		// two clinics both sending "orcamento.pdf" must not collide, and the
		// name is hostile input besides.
		objectPath := fmt.Sprintf("accounts/%s/attachments/%s/%s", accountID, messageID, id)

		contentType := strings.TrimSpace(f.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if err := uc.store.UploadBytes(ctx, objectPath, f.Content, contentType); err != nil {
			slog.ErrorContext(ctx, "attachment upload failed, file dropped",
				"message_id", messageID, "filename", f.Filename, "error", err)
			continue
		}
		// The bytes are in hand RIGHT NOW — warming the proxy's local cache
		// here is what makes the first open of this email instant.
		if uc.mediaCache != nil {
			uc.mediaCache.PutBytes(objectPath, f.Content, contentType)
		}

		att := &domain.MessageAttachment{
			ID:          id,
			MessageID:   messageID,
			AccountID:   accountID,
			Filename:    f.Filename,
			ContentType: contentType,
			SizeBytes:   int64(len(f.Content)),
			ObjectPath:  objectPath,
			IsInline:    f.IsInline,
			ContentID:   f.ContentID,
			ScanStatus:  domain.ScanPending,
		}
		if err := uc.repo.Create(ctx, att); err != nil {
			slog.ErrorContext(ctx, "attachment row failed, file orphaned in storage",
				"message_id", messageID, "object_path", objectPath, "error", err)
			continue
		}

		// Scan with the bytes already in hand rather than re-reading them
		// from storage: it is one fewer round-trip and one fewer way for the
		// scanned bytes to differ from the stored ones.
		uc.scanAndRecord(ctx, att, f.Content)
		stored = append(stored, att)
	}
	return stored
}

// scanAndRecord updates the row in place so the caller's copy matches what a
// later read will return.
func (uc *MessageAttachmentUseCase) scanAndRecord(ctx context.Context, att *domain.MessageAttachment, data []byte) {
	status, signature := domain.ScanSkipped, ""
	if uc.scanner != nil {
		verdict, err := uc.scanner.Scan(ctx, data)
		status, signature = verdict.Status, verdict.Signature
		if err != nil {
			// The adapter already returns ScanFailed alongside its error;
			// the log is what tells an operator the daemon is down, since
			// the user just sees a file that will not open.
			slog.WarnContext(ctx, "attachment scan failed",
				"attachment_id", att.ID, "error", err)
		}
		if status == domain.ScanInfected {
			slog.WarnContext(ctx, "infected attachment stored and blocked",
				"attachment_id", att.ID, "account_id", att.AccountID, "signature", signature)
		}
	}
	att.ScanStatus, att.ScanSignature = status, signature
	if err := uc.repo.SetScanResult(ctx, att.ID, status, signature); err != nil {
		slog.ErrorContext(ctx, "could not persist scan verdict",
			"attachment_id", att.ID, "error", err)
	}
}

// MaxOriginalHTMLBytes caps what is kept for "view original". A marketing
// email with everything inlined can run to megabytes; past this the plain
// text is what the reader gets, which is what they read anyway.
const MaxOriginalHTMLBytes = 2 << 20

// StoreOriginalHTML keeps the message's markup so it can be shown later.
//
// Stored, never rendered from here: the path is handed to an endpoint that
// sanitises and serves it under a locked-down CSP. Keeping it separate from
// the message body is the point — nothing can render it by accident.
//
// Returns the object path, or empty when there was nothing to keep or the
// upload failed. Failing is not an error the caller should care about: the
// message and its text are already saved, and "view original" simply will not
// appear.
func (uc *MessageAttachmentUseCase) StoreOriginalHTML(ctx context.Context, accountID, messageID, html string) string {
	if strings.TrimSpace(html) == "" || len(html) > MaxOriginalHTMLBytes {
		return ""
	}
	path := fmt.Sprintf("accounts/%s/email-html/%s", accountID, messageID)
	if err := uc.store.UploadBytes(ctx, path, []byte(html), "text/html; charset=utf-8"); err != nil {
		slog.WarnContext(ctx, "could not store the original email markup",
			"message_id", messageID, "error", err)
		return ""
	}
	return path
}

// ReadOriginalHTML returns the stored markup for rendering.
//
// Bounded by the same cap it was written under: a path that somehow points at
// something enormous must not become a memory spike.
func (uc *MessageAttachmentUseCase) ReadOriginalHTML(ctx context.Context, path string) (string, error) {
	data, err := uc.store.DownloadObject(ctx, path, MaxOriginalHTMLBytes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Caps for cid: inlining. Data URIs cost ~4/3 of the raw bytes and the
// whole document travels in one response, so both a per-file and a total
// ceiling — past them the image is skipped (renders broken) rather than
// the document failing.
// 8MB per file: the first real-world test was a 4.3MB phone photo, which
// the previous 3MB ceiling silently skipped — the exact class of image
// this feature exists for. 20MB total keeps a many-photo message bounded.
const (
	maxInlineImageBytes      = 8 << 20
	maxInlineImageTotalBytes = 20 << 20
)

// inlineImageURLTTL is how long a minted inline-image URL stays valid.
// Long enough for any reading session; the handler's document cache must
// stay well under it so a cached page never carries an expired URL.
const inlineImageURLTTL = 24 * time.Hour

// InlineImageURLsByCID resolves a message's cid:-referenced images to
// media-proxy URLs, keyed by content id. Nil when no signer is wired —
// the caller falls back to InlineImagesByCID (data: URIs).
//
// This is the fast path: nothing is downloaded here (the proxy streams the
// bytes when the browser asks), so the original view's HTML stays small and
// paints immediately instead of waiting for the API to fetch and base64
// every embedded photo from GCS — a 4.3MB phone photo held the whole
// document hostage for ~9s per open. The proxy response is browser-cached
// (max-age=86400), so repeat opens don't even refetch the image.
//
// Account-scoped exactly like the data-URI variant: rows are filtered by
// owner, and the account id is inside the signed URL. No size caps — the
// proxy streams, so a big photo costs bandwidth, never memory.
func (uc *MessageAttachmentUseCase) InlineImageURLsByCID(ctx context.Context, accountID, messageID string) map[string]string {
	if uc.mediaSigner == nil {
		return nil
	}
	atts, err := uc.repo.ListByMessage(ctx, messageID)
	if err != nil {
		slog.WarnContext(ctx, "cid inline: attachment list failed",
			"message_id", messageID, "error", err)
		return nil
	}
	out := map[string]string{}
	for _, a := range atts {
		if a == nil || a.AccountID != accountID || strings.TrimSpace(a.ContentID) == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(a.ContentType), "image/") {
			continue
		}
		out[a.ContentID] = uc.mediaSigner.SignObjectURL(
			accountID, a.ObjectPath, inlineImageURLTTL)
	}
	return out
}

// InlineImagesByCID resolves a message's cid:-referenced images to data:
// URIs, keyed by content id.
//
// This is what makes the "view original" actually show the message: a photo
// pasted into Gmail and a signature logo both arrive as inline attachments
// referenced by `cid:` — a scheme only a mail client resolves. The stored
// bytes ARE the image, so embedding them as data: renders inside the strict
// CSP (img-src data:) without one byte fetched from the network, which also
// means no read receipt.
//
// Account-scoped: the rows are filtered by owner so a forged message id can
// not pull another tenant's files. Only image/* content types — a cid: that
// points at anything else stays unresolved.
func (uc *MessageAttachmentUseCase) InlineImagesByCID(ctx context.Context, accountID, messageID string) map[string]string {
	atts, err := uc.repo.ListByMessage(ctx, messageID)
	if err != nil {
		slog.WarnContext(ctx, "cid inline: attachment list failed",
			"message_id", messageID, "error", err)
		return nil
	}
	out := map[string]string{}
	total := 0
	for _, a := range atts {
		if a == nil || a.AccountID != accountID || strings.TrimSpace(a.ContentID) == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(a.ContentType), "image/") {
			continue
		}
		if a.SizeBytes > maxInlineImageBytes ||
			total+int(a.SizeBytes) > maxInlineImageTotalBytes {
			continue
		}
		data, dlErr := uc.store.DownloadObject(ctx, a.ObjectPath, maxInlineImageBytes)
		if dlErr != nil {
			slog.WarnContext(ctx, "cid inline: download failed",
				"attachment_id", a.ID, "error", dlErr)
			continue
		}
		total += len(data)
		out[a.ContentID] = "data:" + a.ContentType + ";base64," +
			base64.StdEncoding.EncodeToString(data)
	}
	return out
}

// DownloadURL mints a short-lived signed URL, but only for a file that came
// back clean.
//
// The ownership check is the repository query itself (id AND account), so
// there is no path where a forgotten comparison serves another tenant's
// contract.
func (uc *MessageAttachmentUseCase) DownloadURL(
	ctx context.Context,
	actor domain.Actor,
	attachmentID string,
	acknowledgeUnscanned bool,
) (string, error) {
	att, err := uc.repo.GetForAccount(ctx, attachmentID, actor.AccountID)
	if err != nil {
		return "", err
	}
	if att == nil {
		return "", domain.ErrAttachmentNotFound
	}
	if att.ScanStatus == domain.ScanInfected {
		// Hard stop, and its own error. There is nothing to wait for and no
		// acknowledgement that unlocks it: a "download anyway" on a file a
		// scanner flagged would turn the whole scan into decoration.
		return "", domain.ErrAttachmentInfected
	}
	// An UNSCANNED file is a different decision from an infected one. Nobody
	// looked, and the person reading may still have a legitimate reason to
	// open a document they were expecting. That stays possible — but only
	// through an explicit acknowledgement, never as the default click, and
	// it is recorded.
	if !att.Downloadable() {
		if !acknowledgeUnscanned {
			return "", domain.ErrAttachmentNotDownloadable
		}
		slog.WarnContext(ctx, "unscanned attachment downloaded on explicit acknowledgement",
			"attachment_id", att.ID, "account_id", att.AccountID,
			"user_id", actor.UserID, "scan_status", string(att.ScanStatus))
	}
	return uc.store.GenerateReadSignedURL(ctx, att.ObjectPath, attachmentURLTTL)
}

// Rescan re-checks files stored while the scanner was unavailable, so a
// transient outage does not leave a real document permanently unopenable.
//
// Returns how many rows it settled, for the worker's log.
func (uc *MessageAttachmentUseCase) Rescan(ctx context.Context, limit int32) (int, error) {
	if uc.scanner == nil {
		return 0, nil
	}
	pending, err := uc.repo.ListUnscanned(ctx, limit)
	if err != nil {
		return 0, err
	}
	settled := 0
	for _, att := range pending {
		data, err := uc.store.DownloadObject(ctx, att.ObjectPath, MaxAttachmentBytes)
		if err != nil {
			slog.WarnContext(ctx, "rescan could not read the stored file",
				"attachment_id", att.ID, "error", err)
			continue
		}
		before := att.ScanStatus
		uc.scanAndRecord(ctx, att, data)
		if att.ScanStatus != before {
			settled++
		}
	}
	return settled, nil
}
