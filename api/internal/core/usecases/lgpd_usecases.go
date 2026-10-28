package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// recordAudit writes an audit log entry without failing the parent
// operation on DB errors. Audit logs are important but losing one
// must not block an LGPD response.
func recordAudit(
	ctx context.Context,
	repo domain.AuditLogRepository,
	accountID, actorID, action, entityType, entityID string,
	payload interface{},
) {
	if repo == nil {
		return
	}
	var payloadBytes []byte
	if payload != nil {
		payloadBytes, _ = json.Marshal(payload)
	}
	var actor *string
	if actorID != "" {
		a := actorID
		actor = &a
	}
	entry := &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ActorID:    actor,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Payload:    payloadBytes,
		CreatedAt:  time.Now().UTC(),
	}
	if err := repo.Create(ctx, entry); err != nil {
		slog.Warn("audit log create failed",
			"action", action, "entity_type", entityType, "error", err)
	}
}

// ExportContactDataUseCase gathers all data tied to a contact (conversations,
// messages, broadcast participations) into a single JSON bundle that an
// admin can download on behalf of the titular. Scoped by account.
type ExportContactDataUseCase struct {
	contactRepo  domain.ContactRepository
	lgpdRepo     domain.LGPDRepository
	auditLogRepo domain.AuditLogRepository // optional
}

func NewExportContactDataUseCase(c domain.ContactRepository, l domain.LGPDRepository) *ExportContactDataUseCase {
	return &ExportContactDataUseCase{contactRepo: c, lgpdRepo: l}
}

// WithAuditLog wires an audit log repo; every successful export records an
// entry so the regulator can trace who accessed whose data and when.
func (uc *ExportContactDataUseCase) WithAuditLog(a domain.AuditLogRepository) *ExportContactDataUseCase {
	uc.auditLogRepo = a
	return uc
}

// Execute resolves the contact by externalID (phone / chat_id), validates
// it belongs to the caller's account, and returns the full bundle. The
// caller is responsible for returning 404 when the bundle is empty AND
// the contact row does not exist — here we just report the facts.
func (uc *ExportContactDataUseCase) Execute(ctx context.Context, accountID, actorID, externalID string) (*domain.LGPDExportBundle, error) {
	contact, err := uc.contactRepo.GetByExternalID(ctx, accountID, externalID)
	if err != nil {
		// GetByExternalID returns ErrContactNotFound-shaped errors on no-row;
		// we want to still produce an export if there were messages but the
		// contact row was never materialized. So we swallow the "not found"
		// and let the downstream lookups run.
		contact = nil
	}

	convs, err := uc.lgpdRepo.ListConversationsByContact(ctx, accountID, externalID)
	if err != nil {
		return nil, err
	}
	msgs, err := uc.lgpdRepo.ListMessagesByContact(ctx, accountID, externalID)
	if err != nil {
		return nil, err
	}
	camps, err := uc.lgpdRepo.ListCampaignParticipations(ctx, accountID, externalID)
	if err != nil {
		return nil, err
	}

	if contact == nil && len(convs) == 0 && len(msgs) == 0 && len(camps) == 0 {
		return nil, &domain.AppError{
			Code:    "LGPD_NO_DATA",
			Message: "no data found for this contact",
		}
	}

	slog.Info("lgpd export generated",
		"account_id", accountID,
		"external_id", externalID,
		"conversations", len(convs),
		"messages", len(msgs),
		"campaigns", len(camps),
	)

	recordAudit(ctx, uc.auditLogRepo, accountID, actorID,
		"lgpd.export", "contact", externalID,
		map[string]int{
			"conversations": len(convs),
			"messages":      len(msgs),
			"campaigns":     len(camps),
		})

	return &domain.LGPDExportBundle{
		GeneratedAt:            time.Now().UTC(),
		AccountID:              accountID,
		Contact:                contact,
		Conversations:          convs,
		Messages:               msgs,
		CampaignParticipations: camps,
	}, nil
}

// AnonymizeContactUseCase is the LGPD "right to be forgotten" endpoint.
// It rewrites PII in-place across contact / conversation / message rows,
// replacing the externalID with a random anonymized token so future
// messages from the same phone start a fresh relationship. Aggregate
// counters (campaign counts, message counts) are preserved.
type AnonymizeContactUseCase struct {
	contactRepo  domain.ContactRepository
	lgpdRepo     domain.LGPDRepository
	txManager    domain.TransactionManager
	auditLogRepo domain.AuditLogRepository // optional
}

func NewAnonymizeContactUseCase(c domain.ContactRepository, l domain.LGPDRepository, tx domain.TransactionManager) *AnonymizeContactUseCase {
	return &AnonymizeContactUseCase{contactRepo: c, lgpdRepo: l, txManager: tx}
}

// WithAuditLog records each anonymization event in the audit log — this is
// the operation the regulator is most likely to check (the "right to be
// forgotten" must itself be auditable).
func (uc *AnonymizeContactUseCase) WithAuditLog(a domain.AuditLogRepository) *AnonymizeContactUseCase {
	uc.auditLogRepo = a
	return uc
}

func (uc *AnonymizeContactUseCase) Execute(ctx context.Context, accountID, actorID, externalID string) error {
	contact, _ := uc.contactRepo.GetByExternalID(ctx, accountID, externalID)
	if contact == nil {
		return &domain.AppError{
			Code:    "CONTACT_NOT_FOUND",
			Message: "no contact found for this external id",
		}
	}
	if contact.AccountID != accountID {
		// Defense in depth — repo already filters by account_id but double-check.
		return &domain.AppError{Code: "CONTACT_NOT_FOUND", Message: "no contact found"}
	}

	// Build a unique placeholder so future messages from the same phone
	// create a fresh contact row instead of colliding on the unique
	// (account_id, external_id) index.
	anonID := "anon-" + ulid.Make().String()

	runner := func(ctx context.Context) error {
		return uc.lgpdRepo.AnonymizeContact(ctx, accountID, externalID, anonID)
	}

	var err error
	if uc.txManager != nil {
		err = uc.txManager.WithinTransaction(ctx, runner)
	} else {
		err = runner(ctx)
	}
	if err != nil {
		return err
	}

	slog.Info("lgpd anonymized contact",
		"account_id", accountID,
		"external_id_was", externalID,
		"external_id_now", anonID,
	)

	recordAudit(ctx, uc.auditLogRepo, accountID, actorID,
		"lgpd.anonymize", "contact", externalID,
		map[string]string{"new_external_id": anonID})

	return nil
}
