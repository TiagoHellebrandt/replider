package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ──────────────────────────────── SellerSupportUseCase ────────────────────────────────
//
// Canal interno US-007: the seller panel's grouped view of internal CS
// threads — carteira account → its users' threads — plus the shared
// account-level internal note visible from any thread of that account.

// ErrSupportNoteForbidden gates the note to platform sellers/admins —
// a regular platform attendant can read the inbox but not the carteira
// scratchpad.
var ErrSupportNoteForbidden = &domain.AppError{Code: "SUPPORT_NOTE_FORBIDDEN", Message: "only platform sellers or admins can access the account note"}

// ErrSupportNoteTooLong caps the shared note (it is a scratchpad, not a CRM).
var ErrSupportNoteTooLong = &domain.AppError{Code: "SUPPORT_NOTE_TOO_LONG", Message: "note must be <= 4000 chars"}

const supportNoteMaxLen = 4000

// sellerSupportRepo is the narrow port over postgres.SellerSupportRepository.
type sellerSupportRepo interface {
	ListCarteiraThreads(ctx context.Context, sellerUserID string) ([]domain.SellerSupportThreadRow, error)
	ResolveThreadAccount(ctx context.Context, conversationID string) (accountID, accountName string, err error)
	GetNote(ctx context.Context, accountID string) (*domain.AccountInternalNote, error)
	UpsertNote(ctx context.Context, accountID, note, editorUserID string) error
}

// supportSellerFlagChecker mirrors middleware.SellerFlagChecker without
// importing the adapter layer.
type supportSellerFlagChecker interface {
	IsSeller(ctx context.Context, userID string) (bool, error)
}

type SellerSupportUseCase struct {
	repo        sellerSupportRepo
	sellerFlags supportSellerFlagChecker
	auditRepo   domain.AuditLogRepository
}

func NewSellerSupportUseCase(repo sellerSupportRepo, sellerFlags supportSellerFlagChecker, auditRepo domain.AuditLogRepository) *SellerSupportUseCase {
	return &SellerSupportUseCase{repo: repo, sellerFlags: sellerFlags, auditRepo: auditRepo}
}

// SellerSupportView is the GET /seller/support-threads payload.
type SellerSupportView struct {
	TotalUnread int                                `json:"total_unread"`
	Accounts    []domain.SellerSupportAccountGroup `json:"accounts"`
}

// ListGrouped folds the flat thread rows into account groups, preserving
// the repo's ordering (account name, then most recent thread first).
// Routes are already gated by RequireSeller, so no flag re-check here.
func (uc *SellerSupportUseCase) ListGrouped(ctx context.Context, sellerUserID string) (*SellerSupportView, error) {
	rows, err := uc.repo.ListCarteiraThreads(ctx, sellerUserID)
	if err != nil {
		return nil, err
	}

	view := &SellerSupportView{Accounts: []domain.SellerSupportAccountGroup{}}
	idx := map[string]int{}
	for _, r := range rows {
		i, ok := idx[r.AccountID]
		if !ok {
			group := domain.SellerSupportAccountGroup{
				AccountID:   r.AccountID,
				AccountName: r.AccountName,
				Threads:     []domain.SellerSupportThread{},
			}
			if r.Note != "" || r.NoteUpdatedAt != nil {
				group.Note = &domain.AccountInternalNote{
					AccountID:     r.AccountID,
					Note:          r.Note,
					UpdatedByName: r.NoteUpdatedBy,
					UpdatedAt:     r.NoteUpdatedAt,
				}
			}
			view.Accounts = append(view.Accounts, group)
			i = len(view.Accounts) - 1
			idx[r.AccountID] = i
		}
		view.Accounts[i].Threads = append(view.Accounts[i].Threads, domain.SellerSupportThread{
			ConversationID: r.ConversationID,
			UserID:         r.UserID,
			UserName:       r.UserName,
			UserRole:       r.UserRole,
			LastMessage:    r.LastMessage,
			LastMessageAt:  r.LastMessageAt,
			UnreadCount:    r.UnreadCount,
		})
		view.Accounts[i].UnreadCount += r.UnreadCount
		view.TotalUnread += r.UnreadCount
	}
	return view, nil
}

// authorizeNote allows platform admins and flagged sellers (US-007:
// "editável pelo vendedor/admin da plataforma"). The route is mounted
// behind RequirePlatformAccount WITHOUT RequireSeller, so an unflagged
// platform admin still reaches the note; everyone else is refused here.
// Fail-closed on a flag lookup error.
func (uc *SellerSupportUseCase) authorizeNote(ctx context.Context, userID, role string) error {
	if role == "admin" {
		return nil
	}
	if uc.sellerFlags == nil {
		return ErrSupportNoteForbidden
	}
	isSeller, err := uc.sellerFlags.IsSeller(ctx, userID)
	if err != nil {
		return err
	}
	if !isSeller {
		return ErrSupportNoteForbidden
	}
	return nil
}

// GetNote resolves the conversation's customer account and returns the
// shared note (empty note when never written). A conversation that isn't
// an internal thread is a 404 — the note only exists for CS threads.
func (uc *SellerSupportUseCase) GetNote(ctx context.Context, userID, role, conversationID string) (*domain.AccountInternalNote, error) {
	if err := uc.authorizeNote(ctx, userID, role); err != nil {
		return nil, err
	}
	accountID, _, err := uc.repo.ResolveThreadAccount(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return nil, domain.ErrConversationNotFound
	}
	return uc.repo.GetNote(ctx, accountID)
}

// SetNote validates + persists the shared note and audits the edit
// (who/when land on the row for display; the trail goes to audit_logs).
// Returns the refreshed note so the client renders the stamped metadata.
func (uc *SellerSupportUseCase) SetNote(ctx context.Context, userID, role, conversationID, note string) (*domain.AccountInternalNote, error) {
	if err := uc.authorizeNote(ctx, userID, role); err != nil {
		return nil, err
	}
	note = strings.TrimSpace(note)
	if len(note) > supportNoteMaxLen {
		return nil, ErrSupportNoteTooLong
	}
	accountID, _, err := uc.repo.ResolveThreadAccount(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		return nil, domain.ErrConversationNotFound
	}
	if err := uc.repo.UpsertNote(ctx, accountID, note, userID); err != nil {
		return nil, err
	}
	uc.audit(ctx, userID, accountID, note)
	return uc.repo.GetNote(ctx, accountID)
}

// audit writes the edit trail. Best-effort: the note is already
// persisted, and the visible who/when live on the row itself.
func (uc *SellerSupportUseCase) audit(ctx context.Context, userID, accountID, note string) {
	if uc.auditRepo == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{"note_len": len(note)})
	if err != nil {
		payload = nil
	}
	actorID := userID
	log := &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		ActorID:    &actorID,
		Action:     "support.account_note_updated",
		EntityType: "account",
		EntityID:   accountID,
		Payload:    payload,
		CreatedAt:  time.Now().UTC(),
	}
	if err := uc.auditRepo.Create(ctx, log); err != nil {
		slog.Warn("seller support: audit write failed",
			"account_id", accountID, "actor_id", userID, "error", err)
	}
}
