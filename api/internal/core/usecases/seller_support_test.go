package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// ── fakes ────────────────────────────────────────────────────────────────

type fakeSellerSupportRepo struct {
	rows    []domain.SellerSupportThreadRow
	listErr error

	resolveAccountID   string
	resolveAccountName string
	resolveErr         error

	note    *domain.AccountInternalNote
	noteErr error

	upsertErr      error
	upsertedID     string
	upsertedNote   string
	upsertedEditor string
}

func (f *fakeSellerSupportRepo) ListCarteiraThreads(_ context.Context, _ string) ([]domain.SellerSupportThreadRow, error) {
	return f.rows, f.listErr
}

func (f *fakeSellerSupportRepo) ResolveThreadAccount(_ context.Context, _ string) (string, string, error) {
	return f.resolveAccountID, f.resolveAccountName, f.resolveErr
}

func (f *fakeSellerSupportRepo) GetNote(_ context.Context, accountID string) (*domain.AccountInternalNote, error) {
	if f.noteErr != nil {
		return nil, f.noteErr
	}
	if f.note != nil {
		return f.note, nil
	}
	return &domain.AccountInternalNote{AccountID: accountID}, nil
}

func (f *fakeSellerSupportRepo) UpsertNote(_ context.Context, accountID, note, editorUserID string) error {
	f.upsertedID, f.upsertedNote, f.upsertedEditor = accountID, note, editorUserID
	return f.upsertErr
}

type fakeSellerFlags struct {
	isSeller bool
	err      error
}

func (f *fakeSellerFlags) IsSeller(_ context.Context, _ string) (bool, error) {
	return f.isSeller, f.err
}

type fakeAuditRepo struct {
	created []*domain.AuditLog
}

func (f *fakeAuditRepo) Create(_ context.Context, log *domain.AuditLog) error {
	f.created = append(f.created, log)
	return nil
}
func (f *fakeAuditRepo) ListByAccount(context.Context, string, int32, int32) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (f *fakeAuditRepo) CountByAccount(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeAuditRepo) ListFiltered(context.Context, domain.AuditLogFilter) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (f *fakeAuditRepo) CountFiltered(context.Context, domain.AuditLogFilter) (int64, error) {
	return 0, nil
}

// ── ListGrouped ──────────────────────────────────────────────────────────

func TestSellerSupportListGrouped_GroupsByAccount(t *testing.T) {
	at := time.Now()
	repo := &fakeSellerSupportRepo{rows: []domain.SellerSupportThreadRow{
		{AccountID: "acc1", AccountName: "Acme", ConversationID: "c1", UserID: "u1",
			UserName: "Ana", UserRole: "admin", LastMessage: "oi", LastMessageAt: &at,
			UnreadCount: 2, Note: "cliente VIP", NoteUpdatedBy: "Vend", NoteUpdatedAt: &at},
		{AccountID: "acc1", AccountName: "Acme", ConversationID: "c2", UserID: "u2",
			UserName: "Bia", UserRole: "attendant", UnreadCount: 1,
			Note: "cliente VIP", NoteUpdatedBy: "Vend", NoteUpdatedAt: &at},
		{AccountID: "acc2", AccountName: "Beta", ConversationID: "c3", UserID: "u3",
			UserName: "Caio", UserRole: "admin", UnreadCount: 0},
	}}
	uc := NewSellerSupportUseCase(repo, &fakeSellerFlags{isSeller: true}, nil)

	view, err := uc.ListGrouped(context.Background(), "seller1")
	assert.NoError(t, err)
	assert.Equal(t, 3, view.TotalUnread)
	assert.Len(t, view.Accounts, 2)

	acme := view.Accounts[0]
	assert.Equal(t, "acc1", acme.AccountID)
	assert.Equal(t, 3, acme.UnreadCount)
	assert.Len(t, acme.Threads, 2)
	assert.Equal(t, "Ana", acme.Threads[0].UserName)
	assert.Equal(t, "attendant", acme.Threads[1].UserRole)
	// The shared note rides the group once, with its audit metadata.
	if assert.NotNil(t, acme.Note) {
		assert.Equal(t, "cliente VIP", acme.Note.Note)
		assert.Equal(t, "Vend", acme.Note.UpdatedByName)
	}

	beta := view.Accounts[1]
	assert.Equal(t, "Beta", beta.AccountName)
	assert.Nil(t, beta.Note) // never written
	assert.Equal(t, 0, beta.UnreadCount)
}

func TestSellerSupportListGrouped_EmptyCarteira(t *testing.T) {
	uc := NewSellerSupportUseCase(&fakeSellerSupportRepo{}, &fakeSellerFlags{}, nil)
	view, err := uc.ListGrouped(context.Background(), "seller1")
	assert.NoError(t, err)
	assert.NotNil(t, view.Accounts)
	assert.Len(t, view.Accounts, 0)
	assert.Equal(t, 0, view.TotalUnread)
}

func TestSellerSupportListGrouped_RepoError(t *testing.T) {
	uc := NewSellerSupportUseCase(&fakeSellerSupportRepo{listErr: errors.New("boom")}, &fakeSellerFlags{}, nil)
	_, err := uc.ListGrouped(context.Background(), "seller1")
	assert.Error(t, err)
}

// ── Note authorization ───────────────────────────────────────────────────

func TestSellerSupportGetNote_AdminAllowed(t *testing.T) {
	repo := &fakeSellerSupportRepo{resolveAccountID: "acc1", note: &domain.AccountInternalNote{AccountID: "acc1", Note: "n"}}
	uc := NewSellerSupportUseCase(repo, &fakeSellerFlags{isSeller: false}, nil)

	note, err := uc.GetNote(context.Background(), "u1", "admin", "conv1")
	assert.NoError(t, err)
	assert.Equal(t, "n", note.Note)
}

func TestSellerSupportGetNote_FlaggedSellerAllowed(t *testing.T) {
	repo := &fakeSellerSupportRepo{resolveAccountID: "acc1"}
	uc := NewSellerSupportUseCase(repo, &fakeSellerFlags{isSeller: true}, nil)

	note, err := uc.GetNote(context.Background(), "u1", "attendant", "conv1")
	assert.NoError(t, err)
	assert.Equal(t, "acc1", note.AccountID)
	assert.Equal(t, "", note.Note) // never written → editable blank
}

func TestSellerSupportGetNote_AttendantForbidden(t *testing.T) {
	uc := NewSellerSupportUseCase(&fakeSellerSupportRepo{resolveAccountID: "acc1"}, &fakeSellerFlags{isSeller: false}, nil)
	_, err := uc.GetNote(context.Background(), "u1", "attendant", "conv1")
	assert.ErrorIs(t, err, ErrSupportNoteForbidden)
}

func TestSellerSupportGetNote_FlagLookupErrorFailsClosed(t *testing.T) {
	uc := NewSellerSupportUseCase(&fakeSellerSupportRepo{resolveAccountID: "acc1"}, &fakeSellerFlags{err: errors.New("db down")}, nil)
	_, err := uc.GetNote(context.Background(), "u1", "attendant", "conv1")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrSupportNoteForbidden)
}

func TestSellerSupportGetNote_NotInternalThreadIs404(t *testing.T) {
	// ResolveThreadAccount returns "" for a normal (non-internal)
	// conversation or an unknown id.
	uc := NewSellerSupportUseCase(&fakeSellerSupportRepo{}, &fakeSellerFlags{isSeller: true}, nil)
	_, err := uc.GetNote(context.Background(), "u1", "admin", "conv1")
	assert.ErrorIs(t, err, domain.ErrConversationNotFound)
}

// ── SetNote ──────────────────────────────────────────────────────────────

func TestSellerSupportSetNote_PersistsAndAudits(t *testing.T) {
	repo := &fakeSellerSupportRepo{resolveAccountID: "acc1"}
	audit := &fakeAuditRepo{}
	uc := NewSellerSupportUseCase(repo, &fakeSellerFlags{isSeller: true}, audit)

	_, err := uc.SetNote(context.Background(), "seller1", "attendant", "conv1", "  contexto do cliente  ")
	assert.NoError(t, err)
	assert.Equal(t, "acc1", repo.upsertedID)
	assert.Equal(t, "contexto do cliente", repo.upsertedNote) // trimmed
	assert.Equal(t, "seller1", repo.upsertedEditor)

	if assert.Len(t, audit.created, 1) {
		entry := audit.created[0]
		assert.Equal(t, "support.account_note_updated", entry.Action)
		assert.Equal(t, "acc1", entry.AccountID)
		if assert.NotNil(t, entry.ActorID) {
			assert.Equal(t, "seller1", *entry.ActorID)
		}
	}
}

func TestSellerSupportSetNote_TooLong(t *testing.T) {
	uc := NewSellerSupportUseCase(&fakeSellerSupportRepo{resolveAccountID: "acc1"}, &fakeSellerFlags{isSeller: true}, nil)
	_, err := uc.SetNote(context.Background(), "u1", "admin", "conv1", strings.Repeat("a", 4001))
	assert.ErrorIs(t, err, ErrSupportNoteTooLong)
}

func TestSellerSupportSetNote_EmptyClears(t *testing.T) {
	repo := &fakeSellerSupportRepo{resolveAccountID: "acc1"}
	uc := NewSellerSupportUseCase(repo, &fakeSellerFlags{isSeller: true}, nil)
	_, err := uc.SetNote(context.Background(), "u1", "admin", "conv1", "   ")
	assert.NoError(t, err)
	assert.Equal(t, "", repo.upsertedNote)
}

func TestSellerSupportSetNote_ForbiddenDoesNotWrite(t *testing.T) {
	repo := &fakeSellerSupportRepo{resolveAccountID: "acc1"}
	audit := &fakeAuditRepo{}
	uc := NewSellerSupportUseCase(repo, &fakeSellerFlags{isSeller: false}, audit)
	_, err := uc.SetNote(context.Background(), "u1", "attendant", "conv1", "x")
	assert.ErrorIs(t, err, ErrSupportNoteForbidden)
	assert.Empty(t, repo.upsertedID)
	assert.Empty(t, audit.created)
}
