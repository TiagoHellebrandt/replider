package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type MessageAttachmentRepository struct{ db DBTX }

func NewMessageAttachmentRepository(db DBTX) *MessageAttachmentRepository {
	return &MessageAttachmentRepository{db: db}
}

const attachmentCols = `id, message_id, account_id, filename, content_type,
	size_bytes, object_path, is_inline, COALESCE(content_id, ''), scan_status,
	scan_signature, scanned_at, created_at`

func scanAttachment(row pgx.Row) (*domain.MessageAttachment, error) {
	a := &domain.MessageAttachment{}
	err := row.Scan(&a.ID, &a.MessageID, &a.AccountID, &a.Filename, &a.ContentType,
		&a.SizeBytes, &a.ObjectPath, &a.IsInline, &a.ContentID, &a.ScanStatus,
		&a.ScanSignature, &a.ScannedAt, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (r *MessageAttachmentRepository) Create(ctx context.Context, a *domain.MessageAttachment) error {
	if a.ScanStatus == "" {
		// Never default to clean. A row written without a verdict is one
		// nobody has scanned yet, and the column's own default says the
		// same thing — this just keeps an in-memory struct honest too.
		a.ScanStatus = domain.ScanPending
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO message_attachments
		 (id, message_id, account_id, filename, content_type, size_bytes,
		  object_path, is_inline, content_id, scan_status, scan_signature)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		a.ID, a.MessageID, a.AccountID, a.Filename, a.ContentType, a.SizeBytes,
		a.ObjectPath, a.IsInline, nullIfEmpty(a.ContentID), string(a.ScanStatus),
		a.ScanSignature)
	return err
}

func (r *MessageAttachmentRepository) ListByMessage(ctx context.Context, messageID string) ([]*domain.MessageAttachment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+attachmentCols+` FROM message_attachments
		  WHERE message_id = $1 ORDER BY created_at, id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAttachments(rows)
}

// ListByMessages batches a transcript page: N+1 on a conversation with fifty
// emails is fifty round-trips.
func (r *MessageAttachmentRepository) ListByMessages(ctx context.Context, messageIDs []string) (map[string][]*domain.MessageAttachment, error) {
	out := map[string][]*domain.MessageAttachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+attachmentCols+` FROM message_attachments
		  WHERE message_id = ANY($1) ORDER BY created_at, id`, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all, err := collectAttachments(rows)
	if err != nil {
		return nil, err
	}
	for _, a := range all {
		out[a.MessageID] = append(out[a.MessageID], a)
	}
	return out, nil
}

// GetForAccount is the download endpoint's query. The account filter is in
// the WHERE clause rather than checked afterwards so there is no code path
// where a forgotten comparison serves another tenant's file.
func (r *MessageAttachmentRepository) GetForAccount(ctx context.Context, id, accountID string) (*domain.MessageAttachment, error) {
	return scanAttachment(r.db.QueryRow(ctx,
		`SELECT `+attachmentCols+` FROM message_attachments
		  WHERE id = $1 AND account_id = $2`, id, accountID))
}

func (r *MessageAttachmentRepository) SetScanResult(ctx context.Context, id string, status domain.ScanStatus, signature string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE message_attachments
		    SET scan_status = $2, scan_signature = $3, scanned_at = NOW()
		  WHERE id = $1`, id, string(status), signature)
	return err
}

// ListUnscanned feeds the rescan worker. Includes 'failed' as well as
// 'pending': a file stored while clamd was down must not stay unopenable
// forever because of a transient outage. 'skipped' is in the set too —
// those were stored while NO scanner was configured at all, and turning
// the scanner on should retroactively verify them rather than leave every
// pre-existing attachment "Não verificado" forever. The worker only runs
// when a scanner is wired, so including them costs nothing until then.
func (r *MessageAttachmentRepository) ListUnscanned(ctx context.Context, limit int32) ([]*domain.MessageAttachment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+attachmentCols+` FROM message_attachments
		  WHERE scan_status IN ('pending', 'failed', 'skipped')
		  ORDER BY created_at
		  LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAttachments(rows)
}

func collectAttachments(rows pgx.Rows) ([]*domain.MessageAttachment, error) {
	var out []*domain.MessageAttachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		if a != nil {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}
