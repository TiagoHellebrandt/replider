package postgres

import (
	"context"
	"encoding/json"

	"github.com/replider/api/internal/core/domain"
)

// WhatsAppTemplateRepository implements domain.WhatsAppTemplateRepository
// over the sqlc-generated Queries (US-018). Wraps whatsapp_templates.sql.
type WhatsAppTemplateRepository struct {
	queries *Queries
}

func NewWhatsAppTemplateRepository(db DBTX) *WhatsAppTemplateRepository {
	return &WhatsAppTemplateRepository{queries: New(db)}
}

func toDomainWhatsAppTemplate(row WhatsappTemplate) *domain.WhatsAppTemplate {
	components := json.RawMessage(row.Components)
	if len(components) == 0 {
		components = json.RawMessage("[]")
	}
	variables := json.RawMessage(row.Variables)
	if len(variables) == 0 {
		variables = json.RawMessage("[]")
	}
	return &domain.WhatsAppTemplate{
		ID:             row.ID,
		AccountID:      row.AccountID,
		ChannelID:      row.ChannelID,
		MetaTemplateID: row.MetaTemplateID,
		Name:           row.Name,
		Language:       row.Language,
		Category:       row.Category,
		MetaStatus:     row.MetaStatus,
		Body:           row.Body,
		Components:     components,
		Variables:      variables,
		RejectedReason: row.RejectedReason,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func (r *WhatsAppTemplateRepository) Upsert(ctx context.Context, t *domain.WhatsAppTemplate) error {
	q := getQueries(ctx, r.queries)
	_, err := q.UpsertWhatsAppTemplate(ctx, UpsertWhatsAppTemplateParams{
		ID:             t.ID,
		AccountID:      t.AccountID,
		ChannelID:      t.ChannelID,
		MetaTemplateID: t.MetaTemplateID,
		Name:           t.Name,
		Language:       t.Language,
		Category:       t.Category,
		MetaStatus:     t.MetaStatus,
		Body:           t.Body,
		Components:     toBytes(t.Components),
		Variables:      toBytes(t.Variables),
		RejectedReason: t.RejectedReason,
	})
	return err
}

func (r *WhatsAppTemplateRepository) ListByChannel(ctx context.Context, channelID string) ([]*domain.WhatsAppTemplate, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListWhatsAppTemplatesByChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	templates := make([]*domain.WhatsAppTemplate, 0, len(rows))
	for _, row := range rows {
		templates = append(templates, toDomainWhatsAppTemplate(row))
	}
	return templates, nil
}
