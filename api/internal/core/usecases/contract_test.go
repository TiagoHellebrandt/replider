package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// The contract reading (§6.11, G4).
//
// Everything here is about the model NOT being trusted: a misread renewal date
// becomes a task on a day nobody agreed to, and a misread notice window becomes
// a contract that renews itself while somebody thought they had time.

func TestParseContractExtraction_ReadsTheFieldsThatCostMoney(t *testing.T) {
	raw := "```json\n" + `{
      "title":"Fornecimento de luvas",
      "subject":"Fornecimento mensal de EPI",
      "parties":[{"name":"Hospital São Lucas","role":"contratante","document":"12.345.678/0001-90"},
                 {"name":"MedDistribui","role":"contratada"}],
      "value_cents":50000000,
      "payment_terms":"30 dias após a nota",
      "starts_on":"2026-04-01","ends_on":"2027-03-31",
      "auto_renewal":true,"notice_days":60,
      "adjustment_index":"IPCA","adjustment_month":3,
      "sla_text":"entrega em 48h",
      "penalties":[{"description":"atraso na entrega","percent":10,"trigger":"atraso superior a 5 dias"},
                   {"description":"rescisão antecipada","amount_cents":2000000}],
      "exclusivity":true,"exclusivity_text":"exclusividade na região sul",
      "guarantees":"fiança bancária","termination":"aviso prévio de 90 dias"
    }` + "\n```"

	proposal, fill := parseContractExtraction(raw)
	if proposal == nil || fill == nil {
		t.Fatal("a well-formed reading must produce both the record and the fill")
	}
	// The proposal is kept verbatim, so the original reading survives a human
	// editing the fields afterwards.
	if proposal["title"] != "Fornecimento de luvas" {
		t.Fatalf("proposal not stored verbatim: %v", proposal["title"])
	}

	if fill.ValueCents != 50000000 {
		t.Fatalf("value = %d, want 50000000", fill.ValueCents)
	}
	if !fill.AutoRenewal || fill.NoticeDays != 60 {
		t.Fatalf("renewal pair wrong: auto=%v notice=%d", fill.AutoRenewal, fill.NoticeDays)
	}
	if fill.AdjustmentIndex != "IPCA" || fill.AdjustmentMonth == nil || *fill.AdjustmentMonth != 3 {
		t.Fatalf("adjustment wrong: %q %v", fill.AdjustmentIndex, fill.AdjustmentMonth)
	}
	if fill.StartsOn == nil || fill.StartsOn.Format("2006-01-02") != "2026-04-01" {
		t.Fatalf("start date wrong: %v", fill.StartsOn)
	}
	if len(fill.Parties) != 2 || fill.Parties[0].Document != "12.345.678/0001-90" {
		t.Fatalf("parties wrong: %+v", fill.Parties)
	}
	// Both penalties, with what triggers them: the multa is the clause nobody
	// remembers and the one that gets invoked.
	if len(fill.Penalties) != 2 {
		t.Fatalf("penalties = %d, want 2", len(fill.Penalties))
	}
	if fill.Penalties[0].Percent != 10 || fill.Penalties[0].Trigger == "" {
		t.Fatalf("first penalty wrong: %+v", fill.Penalties[0])
	}
	if fill.Penalties[1].AmountCents != 2000000 {
		t.Fatalf("fixed penalty wrong: %+v", fill.Penalties[1])
	}
	// The notice deadline is derived, not read: 60 days before 31/03/2027.
	contract := &domain.Contract{
		EndsOn: fill.EndsOn, AutoRenewal: fill.AutoRenewal, NoticeDays: fill.NoticeDays,
	}
	deadline := contract.NoticeDeadline()
	if deadline == nil || deadline.Format("2006-01-02") != "2027-01-30" {
		t.Fatalf("notice deadline = %v, want 2027-01-30", deadline)
	}
}

func TestParseContractExtraction_DropsWhatTheModelCannotHaveRead(t *testing.T) {
	// A notice window of 3650 days, a 400% penalty, a swapped term, and a date
	// written in prose. Each one keeps its field EMPTY rather than scheduling
	// something on a date nobody agreed to.
	_, fill := parseContractExtraction(`{
      "title":"Contrato","notice_days":3650,
      "penalties":[{"description":"multa","percent":400}],
      "starts_on":"2027-03-01","ends_on":"2026-03-01",
      "adjustment_month":13,
      "value_cents":-500
    }`)
	if fill == nil {
		t.Fatal("a partially wrong reading is still worth showing")
	}
	if fill.NoticeDays != 0 {
		t.Fatalf("absurd notice window kept: %d", fill.NoticeDays)
	}
	if len(fill.Penalties) != 1 || fill.Penalties[0].Percent != 0 {
		t.Fatalf("out-of-range penalty percent kept: %+v", fill.Penalties)
	}
	// Swapped dates poison every "expira em", so neither is kept.
	if fill.StartsOn != nil || fill.EndsOn != nil {
		t.Fatalf("inverted term kept: %v → %v", fill.StartsOn, fill.EndsOn)
	}
	if fill.AdjustmentMonth != nil {
		t.Fatalf("month 13 kept: %v", *fill.AdjustmentMonth)
	}
	if fill.ValueCents != 0 {
		t.Fatalf("negative value kept: %d", fill.ValueCents)
	}

	// Prose instead of a date: empty. The day is what a notice deadline counts
	// back from, and guessing the 1st is inventing a deadline.
	_, prose := parseContractExtraction(`{"title":"x","ends_on":"março de 2027"}`)
	if prose == nil || prose.EndsOn != nil {
		t.Fatalf("prose date accepted: %v", prose.EndsOn)
	}
}

func TestParseContractExtraction_RefusesUnparseableAnswers(t *testing.T) {
	for _, raw := range []string{"", "não consegui ler o contrato", "{broken"} {
		proposal, fill := parseContractExtraction(raw)
		if proposal != nil || fill != nil {
			t.Fatalf("%q: a half-read contract offered as a proposal is worse than "+
				"none, because somebody will confirm it", raw)
		}
	}
}

// ── The pipeline, with a fake repository ───────────────────────────────────

type fakeContractRepo struct {
	stored    map[string]*domain.Contract
	extracted []domain.ContractExtractionStatus
	lastFill  *domain.Contract
}

func newFakeContractRepo() *fakeContractRepo {
	return &fakeContractRepo{stored: map[string]*domain.Contract{}}
}

func (f *fakeContractRepo) Create(_ context.Context, c *domain.Contract) error {
	clone := *c
	f.stored[c.ID] = &clone
	return nil
}

func (f *fakeContractRepo) Get(_ context.Context, _, id string) (*domain.Contract, error) {
	c, ok := f.stored[id]
	if !ok {
		return nil, nil
	}
	clone := *c
	return &clone, nil
}

func (f *fakeContractRepo) Update(_ context.Context, c *domain.Contract) error {
	clone := *c
	f.stored[c.ID] = &clone
	return nil
}

func (f *fakeContractRepo) Delete(_ context.Context, _, id string) error {
	delete(f.stored, id)
	return nil
}

func (f *fakeContractRepo) List(_ context.Context, _ string, _, _ int) ([]*domain.Contract, error) {
	out := make([]*domain.Contract, 0, len(f.stored))
	for _, c := range f.stored {
		if !c.IsAmendment() {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeContractRepo) ListAmendments(_ context.Context, _, parentID string) ([]*domain.Contract, error) {
	var out []*domain.Contract
	for _, c := range f.stored {
		if c.ParentContractID == parentID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeContractRepo) NextVersion(_ context.Context, _, parentID string) (int, error) {
	next := 2
	for _, c := range f.stored {
		if c.ParentContractID == parentID && c.Version >= next {
			next = c.Version + 1
		}
	}
	return next, nil
}

// DueForAttention mirrors the SQL sweep: confirmed contracts only, and the
// domain decides which of them actually owe somebody a deadline.
func (f *fakeContractRepo) DueForAttention(_ context.Context, through time.Time, _ int) ([]*domain.Contract, error) {
	var out []*domain.Contract
	for _, c := range f.stored {
		if !c.Confirmed() {
			continue
		}
		endsInside := c.EndsOn != nil && !c.EndsOn.After(through)
		if endsInside || c.AdjustmentMonth != nil ||
			(c.PriceRegistry && c.RegistryBalanceCents > 0) {
			clone := *c
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (f *fakeContractRepo) SetExtraction(_ context.Context, _, id string,
	status domain.ContractExtractionStatus, proposal map[string]any,
	extractionError string, fill *domain.Contract) error {
	f.extracted = append(f.extracted, status)
	if fill != nil {
		f.lastFill = fill
	}
	if c, ok := f.stored[id]; ok {
		c.ExtractionStatus = status
		c.ExtractionError = extractionError
		if proposal != nil {
			c.AIProposal = proposal
		}
	}
	return nil
}

func TestContractCreate_AmendmentJoinsTheParentsChain(t *testing.T) {
	repo := newFakeContractRepo()
	uc := NewContractUseCase(repo)
	ctx := context.Background()

	parent, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{Title: "Fornecimento"})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if parent.Version != 1 {
		t.Fatalf("original version = %d, want 1", parent.Version)
	}
	// No document, no AI: the status is "skipped" rather than a pending read
	// that will never happen.
	if parent.ExtractionStatus != domain.ContractExtractionSkipped {
		t.Fatalf("status = %s, want skipped", parent.ExtractionStatus)
	}

	first, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{
		ParentContractID: parent.ID,
	})
	if err != nil {
		t.Fatalf("create amendment: %v", err)
	}
	if first.Version != 2 || first.ParentContractID != parent.ID {
		t.Fatalf("amendment wrong: v%d parent=%s", first.Version, first.ParentContractID)
	}
	// An amendment inherits the parent's title when none is given — "Aditivo"
	// with no subject is a row nobody can identify in a list.
	if first.Title != "Fornecimento" {
		t.Fatalf("amendment title = %q, want the parent's", first.Title)
	}

	// An amendment OF an amendment flattens onto the same chain: "aditivo 3" is
	// the third change to the original, not a tree.
	second, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{
		ParentContractID: first.ID, Title: "Aditivo 2",
	})
	if err != nil {
		t.Fatalf("create nested amendment: %v", err)
	}
	if second.ParentContractID != parent.ID || second.Version != 3 {
		t.Fatalf("nested amendment wrong: parent=%s v%d", second.ParentContractID, second.Version)
	}
}

func TestContractUpdate_ConfirmationNamesWhoConfirmed(t *testing.T) {
	repo := newFakeContractRepo()
	uc := NewContractUseCase(repo)
	fixed := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	uc.SetClock(func() time.Time { return fixed })
	ctx := context.Background()

	contract, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{Title: "Fornecimento"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if contract.Confirmed() {
		t.Fatal("a fresh contract is not a confirmed reading")
	}

	updated, err := uc.Update(ctx, "acc-1", "user-9", contract.ID, ContractInput{
		Title: "Fornecimento", NoticeDays: 60, Confirm: true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.Confirmed() || updated.ConfirmedByUserID != "user-9" {
		t.Fatalf("confirmation must name the person: %+v", updated.ConfirmedAt)
	}
	if !updated.ConfirmedAt.Equal(fixed) {
		t.Fatalf("confirmed at %v, want %v", updated.ConfirmedAt, fixed)
	}

	// A confirmation with no session behind it is not a confirmation: recording
	// that "somebody" validated the numbers is worse than recording nothing.
	fresh, err := uc.Create(ctx, "acc-1", "", ContractInput{Title: "Outro", Confirm: true})
	if err != nil {
		t.Fatalf("create anonymous: %v", err)
	}
	if fresh.Confirmed() {
		t.Fatal("an unnamed confirmation must not stamp")
	}
}

func TestContractExtraction_ScannedPDFSaysSo(t *testing.T) {
	repo := newFakeContractRepo()
	uc := NewContractUseCase(repo)
	uc.SetDispatcher(func(f func()) { f() })
	// A provider that CAN read files is wired, deliberately: the point is that
	// pasted text too short to read, with no file behind it, has nothing to fall
	// back to. The fallback needs bytes, and there are none.
	provider := &documentProvider{docAnswer: validExtraction}
	factory := new(MockAIFactory)
	factory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil).Maybe()
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil).Maybe()
	uc.SetAI(factory, accounts, nil, nil, nil)
	ctx := context.Background()

	// A short text is the scanned-PDF case: below the floor there is nothing to
	// read, and asking a model anyway produces a confident invention.
	contract, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{
		Title: "Digitalizado", Text: "CONTRATO",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stored, _ := repo.Get(ctx, "acc-1", contract.ID)
	if stored.ExtractionStatus != domain.ContractExtractionFailed {
		t.Fatalf("status = %s, want failed", stored.ExtractionStatus)
	}
	if stored.ExtractionError == "" {
		t.Fatal("the failure has to say WHY, or the user re-uploads the same file")
	}
	if provider.calledDoc {
		t.Fatal("with no bytes there is nothing to send as a file")
	}
}

// ── The scanned-PDF fallback ───────────────────────────────────────────────
//
// A contract that was signed, scanned and re-sent has no text layer. The
// pipeline was going to end in a model call anyway, so the FILE goes instead of
// the text. These tests pin when that happens and — more importantly — when it
// deliberately does not.

// textOnlyProvider answers text prompts and CANNOT read a file: the shape of an
// account on a provider (or model) with no document support.
type textOnlyProvider struct {
	answer     string
	calledText bool
}

func (p *textOnlyProvider) GenerateStream(context.Context, string, []domain.ChatMessage, []domain.AITool) (domain.StreamResult, error) {
	return domain.StreamResult{}, nil
}

func (p *textOnlyProvider) GenerateResponse(_ context.Context, _ string, _ []domain.ChatMessage, _ []domain.AITool) (*domain.AICallbackResponse, error) {
	p.calledText = true
	return &domain.AICallbackResponse{Content: p.answer}, nil
}

func (p *textOnlyProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, nil
}

// documentProvider also implements domain.DocumentReader.
type documentProvider struct {
	textOnlyProvider
	docAnswer  string
	calledDoc  bool
	receivedMB int
	prompt     string
}

func (p *documentProvider) GenerateFromDocument(
	_ context.Context, _, prompt string, doc domain.AIDocument,
) (*domain.AICallbackResponse, error) {
	p.calledDoc = true
	p.receivedMB = len(doc.Data) / (1024 * 1024)
	p.prompt = prompt
	if doc.MIMEType != "application/pdf" {
		return nil, errors.New("unexpected mime")
	}
	return &domain.AICallbackResponse{Content: p.docAnswer}, nil
}

// stubDocs hands back canned bytes for the stored object.
type stubDocs struct{ data []byte }

func (s stubDocs) DownloadObject(context.Context, string, int64) ([]byte, error) {
	return s.data, nil
}

// stubExtractor returns whatever text the PDF "had".
type stubExtractor struct{ text string }

func (s stubExtractor) Extract([]byte) (string, error) { return s.text, nil }

const validExtraction = `{"title":"Fornecimento de luvas","value_cents":10000000,` +
	`"auto_renewal":true,"notice_days":60,"adjustment_index":"IPCA","adjustment_month":3}`

func contractExtractionCase(
	t *testing.T, provider domain.AIProvider, pdf []byte, textLayer string,
) (*fakeContractRepo, *domain.Contract) {
	t.Helper()
	repo := newFakeContractRepo()
	uc := NewContractUseCase(repo)
	uc.SetDispatcher(func(f func()) { f() })

	factory := new(MockAIFactory)
	factory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil).Maybe()
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil).Maybe()

	uc.SetAI(factory, accounts, nil, stubDocs{data: pdf}, stubExtractor{text: textLayer})

	ctx := context.Background()
	created, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{
		Title: "Contrato", ObjectPath: "contracts/acc-1/file.pdf",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stored, _ := repo.Get(ctx, "acc-1", created.ID)
	return repo, stored
}

func TestContractExtraction_TextLayerWins(t *testing.T) {
	provider := &documentProvider{
		textOnlyProvider: textOnlyProvider{answer: validExtraction},
		docAnswer:        validExtraction,
	}
	// A text-bearing PDF: reading the bytes as images would cost a different
	// order of tokens for the same answer.
	_, stored := contractExtractionCase(t, provider, []byte("%PDF-1.7 ..."),
		strings.Repeat("CLÁUSULA PRIMEIRA — DO OBJETO. ", 40))

	if !provider.calledText {
		t.Fatal("a usable text layer must go as TEXT")
	}
	if provider.calledDoc {
		t.Fatal("the document path must not run when there is text to read")
	}
	if stored.ExtractionStatus != domain.ContractExtractionDone {
		t.Fatalf("status = %s, want extracted", stored.ExtractionStatus)
	}
}

func TestContractExtraction_ScannedPDFGoesToTheModelAsAFile(t *testing.T) {
	provider := &documentProvider{docAnswer: validExtraction}
	// No text layer — the scanned contract.
	repo, stored := contractExtractionCase(t, provider, []byte("%PDF-1.7 scanned"), "")

	if !provider.calledDoc {
		t.Fatal("a scanned PDF must be sent as a FILE instead of being refused")
	}
	if provider.calledText {
		t.Fatal("there was no text to send")
	}
	// The instruction refers to the attachment rather than quoting a body.
	if !strings.Contains(provider.prompt, "arquivo anexado") {
		t.Fatalf("the document prompt must point at the attachment: %q", provider.prompt)
	}
	if stored.ExtractionStatus != domain.ContractExtractionDone {
		t.Fatalf("status = %s, want extracted", stored.ExtractionStatus)
	}
	// And the fields it read are pre-filled for a human to confirm.
	if repo.lastFill == nil || repo.lastFill.NoticeDays != 60 {
		t.Fatalf("the reading must pre-fill the fields: %+v", repo.lastFill)
	}
}

func TestContractExtraction_NoFallbackWhenTheProviderCannotReadFiles(t *testing.T) {
	provider := &textOnlyProvider{answer: validExtraction}
	_, stored := contractExtractionCase(t, provider, []byte("%PDF-1.7 scanned"), "")

	if stored.ExtractionStatus != domain.ContractExtractionFailed {
		t.Fatalf("status = %s, want failed", stored.ExtractionStatus)
	}
	// The message has to say WHICH limit was hit — the provider, not the
	// contract. "Unreadable in principle" would send somebody hunting for another
	// copy of a file that is fine.
	if !strings.Contains(stored.ExtractionError, "provedor") {
		t.Fatalf("the failure must name the provider limit: %q", stored.ExtractionError)
	}
}

func TestContractExtraction_ScannedButTooBigIsRefusedWithItsLimit(t *testing.T) {
	provider := &documentProvider{docAnswer: validExtraction}
	huge := make([]byte, contractInlineDocMaxBytes+1)
	_, stored := contractExtractionCase(t, provider, huge, "")

	if provider.calledDoc {
		t.Fatal("a document over the inline cap must not be sent")
	}
	if stored.ExtractionStatus != domain.ContractExtractionFailed {
		t.Fatalf("status = %s, want failed", stored.ExtractionStatus)
	}
	// A stated limit beats a feature that works sometimes.
	if !strings.Contains(stored.ExtractionError, "12 MB") {
		t.Fatalf("the failure must state the limit: %q", stored.ExtractionError)
	}
}

// A PDF the parser chokes on is still a document a model can read: the parse
// error must not shortcut the fallback.
func TestContractExtraction_ParseFailureStillTriesTheFile(t *testing.T) {
	provider := &documentProvider{docAnswer: validExtraction}
	repo := newFakeContractRepo()
	uc := NewContractUseCase(repo)
	uc.SetDispatcher(func(f func()) { f() })

	factory := new(MockAIFactory)
	factory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil).Maybe()
	accounts := new(MockAccountRepo)
	accounts.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil).Maybe()
	uc.SetAI(factory, accounts, nil,
		stubDocs{data: []byte("%PDF-1.7 broken")}, failingExtractor{})

	ctx := context.Background()
	created, err := uc.Create(ctx, "acc-1", "user-1", ContractInput{
		Title: "Contrato", ObjectPath: "contracts/acc-1/file.pdf",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !provider.calledDoc {
		t.Fatal("a parse failure must fall through to the document read")
	}
	stored, _ := repo.Get(ctx, "acc-1", created.ID)
	if stored.ExtractionStatus != domain.ContractExtractionDone {
		t.Fatalf("status = %s, want extracted", stored.ExtractionStatus)
	}
}

type failingExtractor struct{}

func (failingExtractor) Extract([]byte) (string, error) {
	return "", errors.New("pdf: malformed")
}
