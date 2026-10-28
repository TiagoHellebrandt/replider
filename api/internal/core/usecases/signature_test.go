package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Electronic signature (§6.11, G7).
//
// The two properties worth a test suite of their own:
//
//  1. the webhook is NOT trusted. The provider signs nothing, so a forged
//     `doc_signed` must cause an authenticated read and no sale.
//  2. a signature becomes AT MOST ONE sale. A duplicate sale is a duplicate
//     commission, which is money out the door.

type fakeSignatureRepo struct {
	stored map[string]*domain.SignatureRequest
	// attachCalls records every claim attempt, so the idempotency guard can be
	// observed rather than inferred.
	attachCalls []string
}

func newFakeSignatureRepo() *fakeSignatureRepo {
	return &fakeSignatureRepo{stored: map[string]*domain.SignatureRequest{}}
}

func (f *fakeSignatureRepo) Create(_ context.Context, r *domain.SignatureRequest) error {
	clone := *r
	f.stored[r.ID] = &clone
	return nil
}

func (f *fakeSignatureRepo) Get(_ context.Context, _, id string) (*domain.SignatureRequest, error) {
	r, ok := f.stored[id]
	if !ok {
		return nil, nil
	}
	clone := *r
	return &clone, nil
}

func (f *fakeSignatureRepo) GetByProviderToken(_ context.Context, _, token string) (*domain.SignatureRequest, error) {
	for _, r := range f.stored {
		if r.ProviderDocToken == token && token != "" {
			clone := *r
			return &clone, nil
		}
	}
	return nil, nil
}

func (f *fakeSignatureRepo) List(_ context.Context, _ string, _, _ int) ([]*domain.SignatureRequest, error) {
	out := make([]*domain.SignatureRequest, 0, len(f.stored))
	for _, r := range f.stored {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeSignatureRepo) ListByContract(_ context.Context, _, contractID string) ([]*domain.SignatureRequest, error) {
	var out []*domain.SignatureRequest
	for _, r := range f.stored {
		if r.ContractID == contractID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeSignatureRepo) Update(_ context.Context, r *domain.SignatureRequest) error {
	existing, ok := f.stored[r.ID]
	if !ok {
		return errors.New("not found")
	}
	clone := *r
	// Mirrors the SQL: Update never writes sale_id, which moves only through
	// AttachSale's conditional claim.
	clone.SaleID = existing.SaleID
	f.stored[r.ID] = &clone
	return nil
}

// AttachSale mirrors the conditional UPDATE: the claim succeeds only while the
// slot is empty or still held by the "pending" marker.
func (f *fakeSignatureRepo) AttachSale(_ context.Context, _, id, saleID string) (bool, error) {
	f.attachCalls = append(f.attachCalls, saleID)
	r, ok := f.stored[id]
	if !ok {
		return false, nil
	}
	if saleID == "" {
		r.SaleID = ""
		return true, nil
	}
	if r.SaleID != "" && r.SaleID != "pending" {
		return false, nil
	}
	r.SaleID = saleID
	return true, nil
}

type fakeSignatureSettings struct {
	settings *domain.SignatureSettings
}

// Reads CLONE, like the SQL repository does: a fake that hands back its own
// pointer lets a caller mutate the "database" by accident and hides exactly the
// bug that would bite in production.
func (f *fakeSignatureSettings) Get(context.Context, string) (*domain.SignatureSettings, error) {
	if f.settings == nil {
		return nil, nil
	}
	clone := *f.settings
	return &clone, nil
}

func (f *fakeSignatureSettings) ByWebhookToken(_ context.Context, token string) (*domain.SignatureSettings, error) {
	if f.settings != nil && f.settings.WebhookToken == token && token != "" {
		clone := *f.settings
		return &clone, nil
	}
	return nil, nil
}

func (f *fakeSignatureSettings) Upsert(_ context.Context, s *domain.SignatureSettings) error {
	clone := *s
	// Mirrors the COALESCE in the SQL: an empty token keeps the stored one.
	if clone.APIToken == "" && f.settings != nil {
		clone.APIToken = f.settings.APIToken
	}
	f.settings = &clone
	return nil
}

// fakeGateway is the provider. `truth` is what its API answers, deliberately
// independent of whatever a webhook payload claims.
type fakeGateway struct {
	truth        *domain.SignatureResult
	createResult *domain.SignatureResult
	createErr    error
	getCalls     int
	downloaded   int
}

func (f *fakeGateway) Name() domain.SignatureProvider { return domain.SignatureZapSign }

func (f *fakeGateway) Create(context.Context, string, domain.SignatureDocument) (*domain.SignatureResult, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResult != nil {
		return f.createResult, nil
	}
	return &domain.SignatureResult{
		ProviderDocToken: "doc-1", Status: domain.SignatureSent,
		Signers: []domain.SignatureSigner{
			{Name: "Ana", ProviderToken: "s1", SignURL: "https://sign/s1", Status: "new"},
		},
	}, nil
}

func (f *fakeGateway) Get(context.Context, string, string) (*domain.SignatureResult, error) {
	f.getCalls++
	if f.truth == nil {
		return &domain.SignatureResult{ProviderDocToken: "doc-1", Status: domain.SignatureSent}, nil
	}
	return f.truth, nil
}

func (f *fakeGateway) DownloadSigned(context.Context, string, int64) ([]byte, error) {
	f.downloaded++
	return []byte("%PDF signed"), nil
}

type fakeArchive struct {
	paths []string
	err   error
}

func (f *fakeArchive) UploadBytes(_ context.Context, path string, _ []byte, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.paths = append(f.paths, path)
	return nil
}

func signatureFixture(t *testing.T, gw *fakeGateway) (
	*SignatureUseCase, *fakeSignatureRepo, *fakeSignatureSettings, *fakeArchive,
) {
	t.Helper()
	repo := newFakeSignatureRepo()
	settings := &fakeSignatureSettings{settings: &domain.SignatureSettings{
		AccountID: "acc-1", Provider: domain.SignatureZapSign,
		APIToken: "tok-123", WebhookToken: "hook-abc",
	}}
	archive := &fakeArchive{}

	uc := NewSignatureUseCase(repo, settings, gw)
	uc.SetStorage(stubDocs{data: []byte("%PDF contract")}, archive)
	SetCallbackMapper(func(event string) domain.SignatureStatus {
		switch event {
		case "doc_signed":
			return domain.SignatureSigned
		case "doc_viewed":
			return domain.SignatureViewed
		default:
			return ""
		}
	})
	return uc, repo, settings, archive
}

func sendFixture(t *testing.T, uc *SignatureUseCase) *domain.SignatureRequest {
	t.Helper()
	req, err := uc.Send(context.Background(), "acc-1", "user-1", SignatureInput{
		ObjectPath: "contracts/acc-1/file.pdf",
		Title:      "Contrato de fornecimento",
		ContactID:  "contact-1",
		ValueCents: 10000000,
		Signers: []domain.SignatureSigner{
			{Name: "Dra. Ana", Email: "ana@clinica.com", Phone: "+5565999990000"},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	return req
}

func TestSignatureSend_TracksTheDocumentAndTheSigningLink(t *testing.T) {
	gw := &fakeGateway{}
	uc, repo, _, _ := signatureFixture(t, gw)

	req := sendFixture(t, uc)
	if req.Status != domain.SignatureSent || req.SentAt == nil {
		t.Fatalf("status = %q, sentAt = %v", req.Status, req.SentAt)
	}
	if req.ProviderDocToken != "doc-1" {
		t.Fatalf("provider token = %q", req.ProviderDocToken)
	}
	// The phone we were given survives the merge; the signing link only the
	// provider knows comes back.
	if len(req.Signers) != 1 || req.Signers[0].Phone != "+5565999990000" ||
		req.Signers[0].SignURL == "" {
		t.Fatalf("signers = %+v", req.Signers)
	}
	// Persisted before the provider call: a document that reaches the provider and
	// not our table is a signature nobody can track, on quota already spent.
	if stored, _ := repo.Get(context.Background(), "acc-1", req.ID); stored == nil {
		t.Fatal("the request must be stored")
	}
}

func TestSignatureSend_RefusalsAndFailures(t *testing.T) {
	// No provider token: the integration is missing, and saying so points at
	// Settings instead of at the document.
	uc := NewSignatureUseCase(newFakeSignatureRepo(),
		&fakeSignatureSettings{settings: nil}, &fakeGateway{})
	uc.SetStorage(stubDocs{data: []byte("%PDF")}, &fakeArchive{})
	if _, err := uc.Send(context.Background(), "acc-1", "u", SignatureInput{
		ObjectPath: "p", Title: "x",
		Signers: []domain.SignatureSigner{{Name: "A", Email: "a@b.c"}},
	}); !errors.Is(err, domain.ErrSignatureNotConfigured) {
		t.Fatalf("err = %v, want ErrSignatureNotConfigured", err)
	}

	// A signer nobody can notify is refused: a document nobody is told about is a
	// document nobody signs.
	full, _, _, _ := signatureFixture(t, &fakeGateway{})
	if _, err := full.Send(context.Background(), "acc-1", "u", SignatureInput{
		ObjectPath: "p", Title: "x",
		Signers: []domain.SignatureSigner{{Name: "Sem contato"}},
	}); err == nil {
		t.Fatal("a signer with no email and no phone must be refused")
	}

	// The provider failing is recorded as OUR failure, with its reason, and kept
	// apart from the customer refusing to sign.
	failing, repo, _, _ := signatureFixture(t, &fakeGateway{
		createErr: errors.New("zapsign: signers: this field is required"),
	})
	req, err := failing.Send(context.Background(), "acc-1", "u", SignatureInput{
		ObjectPath: "p", Title: "x",
		Signers: []domain.SignatureSigner{{Name: "A", Email: "a@b.c"}},
	})
	if err == nil {
		t.Fatal("the provider error must surface")
	}
	stored, _ := repo.Get(context.Background(), "acc-1", req.ID)
	if stored.Status != domain.SignatureFailed {
		t.Fatalf("status = %q, want failed", stored.Status)
	}
	if stored.FailureReason == "" {
		t.Fatal("the reason has to be kept — a person can act on the provider's words")
	}
}

// The security spine: a forged callback causes a READ, not a sale.
func TestSignatureCallback_NeverTrustsThePayload(t *testing.T) {
	gw := &fakeGateway{}
	uc, repo, _, _ := signatureFixture(t, gw)
	req := sendFixture(t, uc)

	// The provider's API still says the document is out for signature...
	gw.truth = &domain.SignatureResult{
		ProviderDocToken: "doc-1", Status: domain.SignatureSent,
	}
	// ...while the callback claims it was signed.
	if err := uc.HandleCallback(context.Background(), "hook-abc", SignatureCallback{
		EventType: "doc_signed", ExternalID: req.ID,
	}); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}

	if gw.getCalls == 0 {
		t.Fatal("the callback must cause an authenticated read")
	}
	stored, _ := repo.Get(context.Background(), "acc-1", req.ID)
	if stored.Status == domain.SignatureSigned {
		t.Fatal("a forged callback must not mark a document signed")
	}
	if stored.SaleID != "" {
		t.Fatal("and must not register a sale")
	}

	// An unknown webhook token says nothing about which accounts exist.
	if err := uc.HandleCallback(context.Background(), "wrong-token", SignatureCallback{
		EventType: "doc_signed", ExternalID: req.ID,
	}); err == nil {
		t.Fatal("an unknown token must be refused")
	}

	// An event we do not act on causes no provider read at all: a bounced
	// notification email must not become an API call per notification.
	before := gw.getCalls
	if err := uc.HandleCallback(context.Background(), "hook-abc", SignatureCallback{
		EventType: "email_bounce", ExternalID: req.ID,
	}); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if gw.getCalls != before {
		t.Fatal("an unmapped event must not hit the provider")
	}
}

func TestSignatureCallback_SignedArchivesAndRegistersOneSale(t *testing.T) {
	signedAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	gw := &fakeGateway{truth: &domain.SignatureResult{
		ProviderDocToken: "doc-1", Status: domain.SignatureSigned,
		SignedFileURL: "https://cdn/signed.pdf",
		Signers: []domain.SignatureSigner{
			{Name: "Dra. Ana", ProviderToken: "s1", Status: "signed", SignedAt: &signedAt},
		},
	}}
	uc, repo, _, archive := signatureFixture(t, gw)
	req := sendFixture(t, uc)

	// The sale side, wired with the fake sale repo the F1 tests use.
	saleRepo := &fakeSaleRepo{}
	uc.SetSales(NewSaleUseCase(saleRepo, nil), nil)

	for i := 0; i < 3; i++ {
		// Three callbacks for the same event: providers retry, and a retry must not
		// become a second sale — a duplicate sale is a duplicate commission.
		if err := uc.HandleCallback(context.Background(), "hook-abc", SignatureCallback{
			EventType: "doc_signed", DocToken: "doc-1",
		}); err != nil {
			t.Fatalf("HandleCallback: %v", err)
		}
	}

	stored, _ := repo.Get(context.Background(), "acc-1", req.ID)
	if stored.Status != domain.SignatureSigned || stored.SignedAt == nil {
		t.Fatalf("status = %q, signedAt = %v", stored.Status, stored.SignedAt)
	}
	// The signed copy is archived in OUR bucket: a document that lives only in the
	// provider's account is one the customer loses when they leave that provider.
	if stored.SignedObjectPath == "" || len(archive.paths) != 1 {
		t.Fatalf("archived = %v, path = %q", archive.paths, stored.SignedObjectPath)
	}
	// Exactly one sale, and the request points at it.
	if len(saleRepo.created) != 1 {
		t.Fatalf("registered %d sales, want exactly 1", len(saleRepo.created))
	}
	if stored.SaleID == "" || stored.SaleID == "pending" {
		t.Fatalf("sale_id = %q, want the registered sale", stored.SaleID)
	}
	sale := saleRepo.created[0]
	if sale.GrossCents != 10000000 || sale.ContactID != "contact-1" {
		t.Fatalf("sale = %+v", sale)
	}
	// The salesperson who sent it is the one the sale (and the commission) belongs
	// to.
	if sale.SellerUserID != "user-1" {
		t.Fatalf("seller = %q, want the sender", sale.SellerUserID)
	}
}

func TestSignatureCallback_NoSaleWithoutValueOrCustomer(t *testing.T) {
	gw := &fakeGateway{truth: &domain.SignatureResult{
		ProviderDocToken: "doc-1", Status: domain.SignatureSigned,
	}}

	// Signed, but worth nothing recorded: track it, register no sale. Inventing a
	// value would put a number in somebody's commission.
	uc, repo, _, _ := signatureFixture(t, gw)
	saleRepo := &fakeSaleRepo{}
	uc.SetSales(NewSaleUseCase(saleRepo, nil), nil)
	req, err := uc.Send(context.Background(), "acc-1", "user-1", SignatureInput{
		ObjectPath: "p", Title: "Termo de confidencialidade",
		ContactID: "contact-1", ValueCents: 0,
		Signers: []domain.SignatureSigner{{Name: "Ana", Email: "a@b.c"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := uc.HandleCallback(context.Background(), "hook-abc", SignatureCallback{
		EventType: "doc_signed", ExternalID: req.ID,
	}); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	stored, _ := repo.Get(context.Background(), "acc-1", req.ID)
	if stored.Status != domain.SignatureSigned {
		t.Fatalf("status = %q, want signed", stored.Status)
	}
	if len(saleRepo.created) != 0 {
		t.Fatal("a signed document with no value is not a sale")
	}

	// Signed with a value but no customer: same answer. A sale with no customer
	// feeds no portfolio.
	noContact, repo2, _, _ := signatureFixture(t, gw)
	saleRepo2 := &fakeSaleRepo{}
	noContact.SetSales(NewSaleUseCase(saleRepo2, nil), nil)
	req2, err := noContact.Send(context.Background(), "acc-1", "user-1", SignatureInput{
		ObjectPath: "p", Title: "Proposta", ValueCents: 5000000,
		Signers: []domain.SignatureSigner{{Name: "Ana", Email: "a@b.c"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := noContact.HandleCallback(context.Background(), "hook-abc", SignatureCallback{
		EventType: "doc_signed", ExternalID: req2.ID,
	}); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	stored2, _ := repo2.Get(context.Background(), "acc-1", req2.ID)
	if stored2.Status != domain.SignatureSigned {
		t.Fatalf("status = %q", stored2.Status)
	}
	if len(saleRepo2.created) != 0 {
		t.Fatal("no customer means no sale to register")
	}
}

// A late "viewed" must not walk a signed document backwards.
func TestSignatureApply_TerminalStatusDoesNotRegress(t *testing.T) {
	gw := &fakeGateway{truth: &domain.SignatureResult{
		ProviderDocToken: "doc-1", Status: domain.SignatureSigned,
	}}
	uc, repo, _, _ := signatureFixture(t, gw)
	req := sendFixture(t, uc)

	if _, err := uc.Refresh(context.Background(), "acc-1", req.ID); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	gw.truth = &domain.SignatureResult{
		ProviderDocToken: "doc-1", Status: domain.SignatureViewed,
	}
	if _, err := uc.Refresh(context.Background(), "acc-1", req.ID); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	stored, _ := repo.Get(context.Background(), "acc-1", req.ID)
	if stored.Status != domain.SignatureSigned {
		t.Fatalf("status = %q — terminal is terminal", stored.Status)
	}
}

func TestSignatureSettings_KeepsTheTokenAndTheWebhook(t *testing.T) {
	settings := &fakeSignatureSettings{}
	uc := NewSignatureUseCase(newFakeSignatureRepo(), settings, &fakeGateway{})

	first, err := uc.SaveSettings(context.Background(), "acc-1", "tok-123", false)
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if first.WebhookToken == "" {
		t.Fatal("the inbound webhook token must be minted")
	}
	if !first.HasToken {
		t.Fatal("the integration must report itself configured")
	}
	// Never handed back: it is the customer's credential at their provider.
	if first.APIToken != "" {
		t.Fatal("the token must not be returned")
	}

	// Saving the sandbox switch with an EMPTY token keeps the stored one, and keeps
	// the same webhook URL the customer already pasted at their provider.
	second, err := uc.SaveSettings(context.Background(), "acc-1", "", true)
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if second.WebhookToken != first.WebhookToken {
		t.Fatal("rotating the webhook token would silently break the callback")
	}
	if !second.HasToken || !second.Sandbox {
		t.Fatalf("settings = %+v", second)
	}
	if settings.settings.APIToken != "tok-123" {
		t.Fatalf("stored token = %q, want it kept", settings.settings.APIToken)
	}
}

// fakeSaleRepo records what the signature registered. The commission statement
// reads these rows, so "how many were created" is the assertion that matters.
type fakeSaleRepo struct {
	created []*domain.Sale
}

func (f *fakeSaleRepo) Create(_ context.Context, s *domain.Sale) error {
	clone := *s
	f.created = append(f.created, &clone)
	return nil
}

func (f *fakeSaleRepo) Get(_ context.Context, id, _ string) (*domain.Sale, error) {
	for _, s := range f.created {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (f *fakeSaleRepo) Update(context.Context, *domain.Sale) error   { return nil }
func (f *fakeSaleRepo) Delete(context.Context, string, string) error { return nil }

func (f *fakeSaleRepo) List(context.Context, string, int, int) ([]*domain.Sale, error) {
	return f.created, nil
}

func (f *fakeSaleRepo) ListByContact(context.Context, string, string) ([]*domain.Sale, error) {
	return f.created, nil
}

func (f *fakeSaleRepo) Totals(context.Context, string, time.Time, time.Time) (domain.SaleTotals, error) {
	return domain.SaleTotals{}, nil
}

func (f *fakeSaleRepo) HasSale(context.Context, string, string) (bool, error) {
	return len(f.created) > 0, nil
}

func (f *fakeSaleRepo) ListByPeriod(context.Context, string, time.Time, time.Time) ([]*domain.Sale, error) {
	return f.created, nil
}

func (f *fakeSaleRepo) CountByContactBefore(context.Context, string, string, time.Time) (int, error) {
	return 0, nil
}
