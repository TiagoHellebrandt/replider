package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The product asks people to open files sent by strangers it prospected
// itself. These pin the one rule that matters: nothing is served unless a
// scanner actually said it was clean.

type fakeAttachmentStore struct {
	uploaded  map[string][]byte
	failWrite bool
	failRead  bool
	signed    []string
}

func newFakeStore() *fakeAttachmentStore {
	return &fakeAttachmentStore{uploaded: map[string][]byte{}}
}

func (f *fakeAttachmentStore) UploadBytes(_ context.Context, path string, data []byte, _ string) error {
	if f.failWrite {
		return errors.New("bucket down")
	}
	f.uploaded[path] = data
	return nil
}

func (f *fakeAttachmentStore) GenerateReadSignedURL(_ context.Context, path string, _ time.Duration) (string, error) {
	f.signed = append(f.signed, path)
	return "https://signed.example/" + path, nil
}

func (f *fakeAttachmentStore) DownloadObject(_ context.Context, path string, _ int64) ([]byte, error) {
	if f.failRead {
		return nil, errors.New("bucket down")
	}
	data, ok := f.uploaded[path]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

type fakeAttachmentRepo struct {
	rows map[string]*domain.MessageAttachment
}

func newFakeRepo() *fakeAttachmentRepo {
	return &fakeAttachmentRepo{rows: map[string]*domain.MessageAttachment{}}
}

func (f *fakeAttachmentRepo) Create(_ context.Context, a *domain.MessageAttachment) error {
	copied := *a
	f.rows[a.ID] = &copied
	return nil
}

func (f *fakeAttachmentRepo) ListByMessage(_ context.Context, messageID string) ([]*domain.MessageAttachment, error) {
	var out []*domain.MessageAttachment
	for _, a := range f.rows {
		if a.MessageID == messageID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAttachmentRepo) ListByMessages(_ context.Context, ids []string) (map[string][]*domain.MessageAttachment, error) {
	out := map[string][]*domain.MessageAttachment{}
	for _, a := range f.rows {
		for _, id := range ids {
			if a.MessageID == id {
				out[id] = append(out[id], a)
			}
		}
	}
	return out, nil
}

func (f *fakeAttachmentRepo) GetForAccount(_ context.Context, id, accountID string) (*domain.MessageAttachment, error) {
	a, ok := f.rows[id]
	if !ok || a.AccountID != accountID {
		return nil, nil
	}
	return a, nil
}

func (f *fakeAttachmentRepo) SetScanResult(_ context.Context, id string, status domain.ScanStatus, sig string) error {
	if a, ok := f.rows[id]; ok {
		a.ScanStatus, a.ScanSignature = status, sig
	}
	return nil
}

func (f *fakeAttachmentRepo) ListUnscanned(_ context.Context, limit int32) ([]*domain.MessageAttachment, error) {
	var out []*domain.MessageAttachment
	for _, a := range f.rows {
		if a.ScanStatus == domain.ScanPending || a.ScanStatus == domain.ScanFailed {
			out = append(out, a)
			if int32(len(out)) >= limit {
				break
			}
		}
	}
	return out, nil
}

type fakeScanner struct {
	verdict domain.ScanVerdict
	err     error
	calls   int
}

func (f *fakeScanner) Scan(context.Context, []byte) (domain.ScanVerdict, error) {
	f.calls++
	return f.verdict, f.err
}

func actorFor(account string) domain.Actor {
	return domain.Actor{AccountID: account, UserID: "user-1", Role: domain.RoleAdmin}
}

func TestStore_PersistsAndScans(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	uc := NewMessageAttachmentUseCase(repo, store)
	scanner := &fakeScanner{verdict: domain.ScanVerdict{Status: domain.ScanClean}}
	uc.SetScanner(scanner)

	got := uc.Store(context.Background(), "acc-1", "msg-1", []IncomingAttachment{
		{Filename: "termo.pdf", ContentType: "application/pdf", Content: []byte("%PDF")},
	})

	require.Len(t, got, 1)
	assert.Equal(t, domain.ScanClean, got[0].ScanStatus)
	assert.Equal(t, 1, scanner.calls)
	assert.Len(t, store.uploaded, 1)
	// The stored path must be account-scoped and must not carry the
	// sender-supplied filename.
	for path := range store.uploaded {
		assert.Contains(t, path, "accounts/acc-1/")
		assert.NotContains(t, path, "termo.pdf")
	}
}

// The single most important behaviour in this file.
func TestDownloadURL_RefusesEverythingButClean(t *testing.T) {
	for _, status := range []domain.ScanStatus{
		domain.ScanPending, domain.ScanSkipped, domain.ScanFailed,
	} {
		repo, store := newFakeRepo(), newFakeStore()
		repo.rows["a1"] = &domain.MessageAttachment{
			ID: "a1", AccountID: "acc-1", ObjectPath: "p", ScanStatus: status,
		}
		uc := NewMessageAttachmentUseCase(repo, store)

		_, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", false)

		assert.ErrorIs(t, err, domain.ErrAttachmentNotDownloadable, "status %q", status)
		assert.Empty(t, store.signed, "no URL may be minted for status %q", status)
	}
}

// An infected file gets its own error: the UI must not offer a retry,
// because there is nothing to wait for.
func TestDownloadURL_InfectedHasItsOwnError(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	repo.rows["a1"] = &domain.MessageAttachment{
		ID: "a1", AccountID: "acc-1", ObjectPath: "p",
		ScanStatus: domain.ScanInfected, ScanSignature: "Eicar-Test-Signature",
	}
	uc := NewMessageAttachmentUseCase(repo, store)

	_, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", false)

	assert.ErrorIs(t, err, domain.ErrAttachmentInfected)
	assert.Empty(t, store.signed)
}

func TestDownloadURL_CleanFileIsServed(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	repo.rows["a1"] = &domain.MessageAttachment{
		ID: "a1", AccountID: "acc-1", ObjectPath: "accounts/acc-1/x", ScanStatus: domain.ScanClean,
	}
	uc := NewMessageAttachmentUseCase(repo, store)

	url, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", false)

	require.NoError(t, err)
	assert.Contains(t, url, "accounts/acc-1/x")
}

// The tenant check is the query itself. Another account's clean file is
// "not found", not "forbidden" — probing ids must not distinguish the two.
func TestDownloadURL_AnotherAccountsFileIsNotFound(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	repo.rows["a1"] = &domain.MessageAttachment{
		ID: "a1", AccountID: "acc-OTHER", ObjectPath: "p", ScanStatus: domain.ScanClean,
	}
	uc := NewMessageAttachmentUseCase(repo, store)

	_, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", false)

	assert.ErrorIs(t, err, domain.ErrAttachmentNotFound)
	assert.Empty(t, store.signed)
}

// No scanner configured is "skipped", never "clean" — and skipped is not
// downloadable, so an unscanned file cannot be served as a checked one.
func TestStore_NoScannerRecordsSkippedNotClean(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	uc := NewMessageAttachmentUseCase(repo, store)

	got := uc.Store(context.Background(), "acc-1", "msg-1", []IncomingAttachment{
		{Filename: "x.pdf", Content: []byte("data")},
	})

	require.Len(t, got, 1)
	assert.Equal(t, domain.ScanSkipped, got[0].ScanStatus)
	assert.False(t, got[0].Downloadable())
}

// A scanner that errors must not produce a clean file.
func TestStore_ScannerErrorIsNotClean(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	uc := NewMessageAttachmentUseCase(repo, store)
	uc.SetScanner(&fakeScanner{
		verdict: domain.ScanVerdict{Status: domain.ScanFailed},
		err:     errors.New("clamd down"),
	})

	got := uc.Store(context.Background(), "acc-1", "msg-1", []IncomingAttachment{
		{Filename: "x.pdf", Content: []byte("data")},
	})

	require.Len(t, got, 1)
	assert.Equal(t, domain.ScanFailed, got[0].ScanStatus)
	assert.False(t, got[0].Downloadable())
}

// One bad file must not take its siblings with it: the message is already
// persisted, and dropping the whole batch loses documents that uploaded fine.
func TestStore_OneFailureDoesNotDropTheOthers(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	uc := NewMessageAttachmentUseCase(repo, store)

	got := uc.Store(context.Background(), "acc-1", "msg-1", []IncomingAttachment{
		{Filename: "vazio.pdf", Content: nil},
		{Filename: "gigante.pdf", Content: make([]byte, MaxAttachmentBytes+1)},
		{Filename: "bom.pdf", Content: []byte("ok")},
	})

	require.Len(t, got, 1)
	assert.Equal(t, "bom.pdf", got[0].Filename)
}

// A file stored while clamd was down must not stay unopenable forever.
func TestRescan_SettlesFilesStoredWhileTheScannerWasDown(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	uc := NewMessageAttachmentUseCase(repo, store)
	uc.SetScanner(&fakeScanner{
		verdict: domain.ScanVerdict{Status: domain.ScanFailed},
		err:     errors.New("down"),
	})

	got := uc.Store(context.Background(), "acc-1", "msg-1", []IncomingAttachment{
		{Filename: "termo.pdf", Content: []byte("%PDF")},
	})
	require.Len(t, got, 1)
	require.Equal(t, domain.ScanFailed, got[0].ScanStatus)

	// Daemon comes back.
	uc.SetScanner(&fakeScanner{verdict: domain.ScanVerdict{Status: domain.ScanClean}})
	settled, err := uc.Rescan(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 1, settled)
	assert.Equal(t, domain.ScanClean, repo.rows[got[0].ID].ScanStatus)
}

// Without a scanner there is nothing to re-check, and marking rows clean
// would be the exact lie this whole design avoids.
func TestRescan_NoScannerIsANoOp(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	repo.rows["a1"] = &domain.MessageAttachment{ID: "a1", ScanStatus: domain.ScanPending}
	uc := NewMessageAttachmentUseCase(repo, store)

	settled, err := uc.Rescan(context.Background(), 10)

	require.NoError(t, err)
	assert.Zero(t, settled)
	assert.Equal(t, domain.ScanPending, repo.rows["a1"].ScanStatus)
}

// An UNSCANNED file is a different decision from an infected one: nobody
// looked, and the person may have been expecting that document. Possible, but
// only through an explicit acknowledgement — never the default click.
func TestDownloadURL_AcknowledgementOpensAnUnscannedFile(t *testing.T) {
	for _, status := range []domain.ScanStatus{
		domain.ScanPending, domain.ScanSkipped, domain.ScanFailed,
	} {
		repo, store := newFakeRepo(), newFakeStore()
		repo.rows["a1"] = &domain.MessageAttachment{
			ID: "a1", AccountID: "acc-1", ObjectPath: "p", ScanStatus: status,
		}
		uc := NewMessageAttachmentUseCase(repo, store)

		url, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", true)

		require.NoError(t, err, "status %q", status)
		assert.NotEmpty(t, url)
	}
}

// The one that must never bend. A "download anyway" on a file a scanner
// flagged would turn the whole scan into decoration.
func TestDownloadURL_AcknowledgementNeverUnlocksAnInfectedFile(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	repo.rows["a1"] = &domain.MessageAttachment{
		ID: "a1", AccountID: "acc-1", ObjectPath: "p",
		ScanStatus: domain.ScanInfected, ScanSignature: "Eicar-Test-Signature",
	}
	uc := NewMessageAttachmentUseCase(repo, store)

	_, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", true)

	assert.ErrorIs(t, err, domain.ErrAttachmentInfected)
	assert.Empty(t, store.signed)
}

// Acknowledging does not widen the tenant boundary.
func TestDownloadURL_AcknowledgementDoesNotCrossAccounts(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	repo.rows["a1"] = &domain.MessageAttachment{
		ID: "a1", AccountID: "acc-OTHER", ObjectPath: "p", ScanStatus: domain.ScanSkipped,
	}
	uc := NewMessageAttachmentUseCase(repo, store)

	_, err := uc.DownloadURL(context.Background(), actorFor("acc-1"), "a1", true)

	assert.ErrorIs(t, err, domain.ErrAttachmentNotFound)
	assert.Empty(t, store.signed)
}

type fakeInlineSigner struct{ calls []string }

func (f *fakeInlineSigner) SignObjectURL(accountID, objectPath string, _ time.Duration) string {
	f.calls = append(f.calls, objectPath)
	return "https://api.example/v1/media/file?a=" + accountID + "&p=" + objectPath
}

// The fast path: cid: images become signed proxy URLs — no download, no
// base64 — keyed and filtered exactly like the data-URI variant.
func TestInlineImageURLsByCID_SignsInsteadOfDownloading(t *testing.T) {
	repo, store := newFakeRepo(), newFakeStore()
	uc := NewMessageAttachmentUseCase(repo, store)
	signer := &fakeInlineSigner{}
	uc.SetMediaSigner(signer)

	seed := func(id, cid, ctype, account string) {
		_ = repo.Create(context.Background(), &domain.MessageAttachment{
			ID: id, MessageID: "m1", AccountID: account,
			ContentID: cid, ContentType: ctype,
			ObjectPath: "accounts/" + account + "/attachments/m1/" + id,
		})
	}
	seed("a1", "foto@mail", "image/jpeg", "acc-1")
	seed("a2", "", "image/png", "acc-1")               // no cid → not inline
	seed("a3", "doc@mail", "application/pdf", "acc-1") // not an image
	seed("a4", "alheia@mail", "image/png", "acc-2")    // another tenant

	out := uc.InlineImageURLsByCID(context.Background(), "acc-1", "m1")

	assert.Len(t, out, 1)
	assert.Contains(t, out["foto@mail"], "/v1/media/file?a=acc-1")
	// Nothing was fetched from storage — that is the whole point.
	assert.Empty(t, store.signed)
}

func TestInlineImageURLsByCID_NilWithoutSigner(t *testing.T) {
	uc := NewMessageAttachmentUseCase(newFakeRepo(), newFakeStore())
	assert.Nil(t, uc.InlineImageURLsByCID(context.Background(), "acc-1", "m1"))
}
