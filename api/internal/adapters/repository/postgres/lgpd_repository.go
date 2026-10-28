package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

func ptrTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// LGPDRepository implements domain.LGPDRepository against sqlc-generated
// queries. Reads are plain SELECTs; anonymization runs three UPDATEs
// inside a transaction when the tx-scoped Queries is available.
type LGPDRepository struct {
	queries *Queries
}

func NewLGPDRepository(db DBTX) *LGPDRepository {
	return &LGPDRepository{queries: New(db)}
}

func (r *LGPDRepository) ListConversationsByContact(ctx context.Context, accountID, externalID string) ([]*domain.LGPDExportConversation, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.LgpdListConversationsByContact(ctx, LgpdListConversationsByContactParams{
		AccountID: accountID, ContactID: externalID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.LGPDExportConversation, len(rows))
	for i, row := range rows {
		out[i] = &domain.LGPDExportConversation{
			ID:          row.ID,
			ChannelID:   row.ChannelID,
			AgentID:     row.AgentID,
			ContactID:   row.ContactID,
			ContactName: row.ContactName,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt.Time,
		}
	}
	return out, nil
}

func (r *LGPDRepository) ListMessagesByContact(ctx context.Context, accountID, externalID string) ([]*domain.LGPDExportMessage, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.LgpdListMessagesByContact(ctx, LgpdListMessagesByContactParams{
		AccountID: accountID, ContactID: externalID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.LGPDExportMessage, len(rows))
	for i, row := range rows {
		out[i] = &domain.LGPDExportMessage{
			ID:             row.ID,
			ConversationID: row.ConversationID,
			ChannelID:      &row.ChannelID,
			Sender:         row.Sender,
			SenderName:     row.SenderName,
			Type:           row.Type,
			Content:        row.Content,
			Direction:      row.Direction,
			CreatedAt:      row.CreatedAt.Time,
		}
	}
	return out, nil
}

func (r *LGPDRepository) ListCampaignParticipations(ctx context.Context, accountID, externalID string) ([]*domain.LGPDCampaignParticipation, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.LgpdListCampaignRecipientsByPhone(ctx, LgpdListCampaignRecipientsByPhoneParams{
		AccountID: accountID, Recipient: externalID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.LGPDCampaignParticipation, len(rows))
	for i, row := range rows {
		out[i] = &domain.LGPDCampaignParticipation{
			RecipientID:  row.ID,
			CampaignID:   row.CampaignID,
			CampaignName: row.CampaignName,
			Recipient:    row.Recipient,
			Status:       row.Status,
			SentAt:       ptrTime(row.SentAt),
			DeliveredAt:  ptrTime(row.DeliveredAt),
			ReadAt:       ptrTime(row.ReadAt),
			FailedAt:     ptrTime(row.FailedAt),
		}
	}
	return out, nil
}

// AnonymizeContact applies the 3 UPDATEs as a unit. When this repo is
// used under a transaction-scoped context (via WithinTransaction), all
// three run atomically — avoiding a half-anonymized state if one step
// fails.
func (r *LGPDRepository) AnonymizeContact(ctx context.Context, accountID, externalID, anonymousExternalID string) error {
	q := getQueries(ctx, r.queries)
	if err := q.LgpdAnonymizeMessages(ctx, LgpdAnonymizeMessagesParams{
		AccountID: accountID, ContactID: externalID,
	}); err != nil {
		return err
	}
	if err := q.LgpdAnonymizeConversations(ctx, LgpdAnonymizeConversationsParams{
		AccountID: accountID, ContactID: externalID,
	}); err != nil {
		return err
	}
	if err := q.LgpdAnonymizeContact(ctx, LgpdAnonymizeContactParams{
		AccountID:    accountID,
		ExternalID:   externalID,
		ExternalID_2: anonymousExternalID,
	}); err != nil {
		return err
	}
	return nil
}
