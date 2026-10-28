package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Contracts: the repository and the reading (§6.11, G4).
//
// Upload a PDF, the model proposes the fields that matter (parties, term,
// automatic renewal, notice window, index, SLA, penalties, exclusivity), and a
// person confirms them. Nothing downstream — no renewal alert, no renegotiation
// task — fires off an unconfirmed reading: a machine deciding on its own that a
// contract renews in 30 days is a machine making a commercial decision.

// ContractDocReader downloads the stored PDF and turns it into text.
//
// Two narrow ports rather than one storage interface: the download is the same
// method the knowledge ingester already uses (DownloadObject), and the extractor
// is the same PDF text pass. Tests substitute both without a bucket.
type ContractDocReader interface {
	DownloadObject(ctx context.Context, objectPath string, maxBytes int64) ([]byte, error)
}

// contractPDFMaxBytes mirrors the upload endpoint's ceiling. Re-checked here
// because the worker re-reads the object, and a file that slipped past the
// upload guard must not be able to pull 200MB into memory.
const contractPDFMaxBytes = 20 * 1024 * 1024

// contractTextLimit is how much of the document the model reads.
//
// The first ~60k characters, which in practice is the whole agreement plus the
// start of the annexes. Sending an entire annexed price list costs tokens to
// re-read what the clauses already said.
const contractTextLimit = 60000

// minContractTextChars is the floor below which a PDF has no usable text layer.
// Below this there is nothing to read from the bytes, and asking a model to work
// from three characters produces a confident invention.
const minContractTextChars = 200

// contractInlineDocMaxBytes caps what goes to a multimodal model INLINE.
//
// Twelve MiB of PDF, because base64 inflates by a third and the request has to
// fit in one call. Above it the honest answer is the scanned-PDF message: the
// alternative is a second upload protocol (a Files API handle to create, track
// and clean up) for the rare 40MB scan, and a feature that only works sometimes
// is worse than one with a stated limit.
const contractInlineDocMaxBytes = 12 * 1024 * 1024

type ContractUseCase struct {
	repo     domain.ContractRepository
	accounts domain.AccountRepository
	plans    domain.PlanRepository

	ai        domain.AIFactory
	docs      ContractDocReader
	extractor PDFExtractor
	meter     *AIMeter
	model     string
	// spend is the per-plan ceiling on document reads (K4). Optional: nil
	// extracts exactly as before the caps existed.
	spend *spendGate

	// dispatch runs the extraction. Overridden in tests to run inline; in
	// production it is a detached goroutine, because reading a 40-page contract
	// takes longer than an HTTP request should live.
	dispatch func(func())
	now      func() time.Time
}

func NewContractUseCase(repo domain.ContractRepository) *ContractUseCase {
	return &ContractUseCase{
		repo:     repo,
		dispatch: func(f func()) { go f() },
		now:      time.Now,
	}
}

// SetAI wires the reading. Without it, an uploaded contract stays a filed PDF
// with whatever a human typed — which is still useful, and is why extraction is
// optional rather than a hard dependency.
func (uc *ContractUseCase) SetAI(
	f domain.AIFactory, accounts domain.AccountRepository, plans domain.PlanRepository,
	docs ContractDocReader, extractor PDFExtractor,
) {
	uc.ai, uc.accounts, uc.plans = f, accounts, plans
	uc.docs, uc.extractor = docs, extractor
}

func (uc *ContractUseCase) SetMeter(m *AIMeter) { uc.meter = m }

// SetSpendCounter wires the per-plan document-read ceiling (K4).
//
// The counter is the AI cost ledger, so the cap and the invoice line read the
// same rows and can never disagree about how many documents were read.
func (uc *ContractUseCase) SetSpendCounter(c domain.SpendCounter) {
	uc.spend = newSpendGate(uc.accounts, uc.plans, c)
	// The counter main.go wires IS the cost ledger; when it also satisfies
	// the ledger interface the customer's own monthly budget check lights
	// up with no extra wiring.
	if costs, ok := c.(domain.AICostRepository); ok {
		uc.spend.SetCostLedger(costs)
	}
}

func (uc *ContractUseCase) SetModel(m string)             { uc.model = strings.TrimSpace(m) }
func (uc *ContractUseCase) SetDispatcher(d func(func()))  { uc.dispatch = d }
func (uc *ContractUseCase) SetClock(now func() time.Time) { uc.now = now }

// ContractInput is what a client may write. Deliberately not the domain struct:
// extraction status, the AI proposal and the confirmation stamp are the
// server's to set, and accepting them from a request would let a client mark
// its own reading as human-confirmed.
type ContractInput struct {
	ID        string
	ContactID string
	Title     string

	// ParentContractID makes this an amendment. The version is assigned by the
	// server from the chain, never by the client.
	ParentContractID string

	ObjectPath string
	FileName   string
	// Text is the pasted contract body, for a co-pilot or a paste-in that has
	// no PDF. Read instead of the document when present.
	Text string

	Parties []domain.ContractParty
	Subject string

	ValueCents   int64
	Currency     string
	PaymentTerms string

	StartsOn *time.Time
	EndsOn   *time.Time

	AutoRenewal bool
	NoticeDays  int

	AdjustmentIndex string
	AdjustmentMonth *int

	// The price registry (ata de registro de preços) and its remaining balance.
	PriceRegistry        bool
	RegistryNumber       string
	RegistryBalanceCents int64
	RegistryBalanceOn    *time.Time
	// LastAdjustedOn is when the price was actually raised. It is what an index
	// accumulation counts from, so it is a field a person sets deliberately —
	// never inferred from "the contract was updated today".
	LastAdjustedOn *time.Time

	SLAText         string
	Penalties       []domain.ContractPenalty
	Exclusivity     bool
	ExclusivityText string
	Guarantees      string
	Termination     string
	Notes           string

	// Confirm marks the reading as validated by the caller. The caller's id is
	// taken from the session, not from the body.
	Confirm bool
}

// Create files a contract and, when there is something to read, starts the
// extraction in the background.
func (uc *ContractUseCase) Create(
	ctx context.Context, accountID, userID string, in ContractInput,
) (*domain.Contract, error) {
	contract := &domain.Contract{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		Version:   1,
	}
	applyContractInput(contract, in)

	if in.ParentContractID != "" {
		parent, err := uc.repo.Get(ctx, accountID, in.ParentContractID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, errContractNotFound
		}
		// An amendment of an amendment flattens onto the same chain: the thing
		// people mean by "aditivo 3" is the third change to the original, not a
		// tree.
		root := parent.ID
		if parent.IsAmendment() {
			root = parent.ParentContractID
		}
		contract.ParentContractID = root
		version, err := uc.repo.NextVersion(ctx, accountID, root)
		if err != nil {
			return nil, err
		}
		contract.Version = version
		if strings.TrimSpace(contract.Title) == "" {
			contract.Title = parent.Title
		}
	}

	if strings.TrimSpace(contract.Currency) == "" {
		contract.Currency = domain.CommissionLedgerCurrency
	}
	readable := strings.TrimSpace(in.Text) != "" || strings.TrimSpace(in.ObjectPath) != ""
	contract.ExtractionStatus = domain.ContractExtractionSkipped
	if readable && uc.canExtract() {
		contract.ExtractionStatus = domain.ContractExtractionPending
	}
	if in.Confirm {
		uc.stampConfirmation(contract, userID)
	}

	if err := contract.Validate(); err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, contract); err != nil {
		return nil, err
	}

	if contract.ExtractionStatus == domain.ContractExtractionPending {
		// Detached from the request: context.WithoutCancel so closing the
		// browser tab does not abandon a reading halfway.
		detached := context.WithoutCancel(ctx)
		text := in.Text
		uc.dispatch(func() { uc.extract(detached, accountID, contract.ID, text) })
	}
	return contract, nil
}

func (uc *ContractUseCase) canExtract() bool {
	return uc.ai != nil && uc.accounts != nil
}

var errContractNotFound = &domain.AppError{
	Code: "CONTRACT_NOT_FOUND", Message: "contrato não encontrado",
}

// Get is one contract with its amendment chain.
func (uc *ContractUseCase) Get(ctx context.Context, accountID, id string) (*domain.Contract, error) {
	contract, err := uc.repo.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, errContractNotFound
	}
	// The chain hangs off the ROOT, so asking for an amendment shows the same
	// history as asking for its parent — otherwise "which version am I looking
	// at" depends on which link somebody clicked.
	root := contract.ID
	if contract.IsAmendment() {
		root = contract.ParentContractID
	}
	amendments, err := uc.repo.ListAmendments(ctx, accountID, root)
	if err != nil {
		return nil, err
	}
	contract.Amendments = amendments
	return contract, nil
}

func (uc *ContractUseCase) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.Contract, error) {
	list, err := uc.repo.List(ctx, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Contract{}
	}
	return list, nil
}

// Update writes the human's version of the fields.
//
// Editing IS confirming when the caller asks for it: somebody who corrected the
// end date and pressed save has validated the reading, and making them press a
// second button only trains them to press buttons.
func (uc *ContractUseCase) Update(
	ctx context.Context, accountID, userID, id string, in ContractInput,
) (*domain.Contract, error) {
	existing, err := uc.repo.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errContractNotFound
	}

	applyContractInput(existing, in)
	if strings.TrimSpace(existing.Currency) == "" {
		existing.Currency = domain.CommissionLedgerCurrency
	}
	if in.Confirm && !existing.Confirmed() {
		uc.stampConfirmation(existing, userID)
	}
	if err := existing.Validate(); err != nil {
		return nil, err
	}
	if err := uc.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (uc *ContractUseCase) Delete(ctx context.Context, accountID, id string) error {
	return uc.repo.Delete(ctx, accountID, id)
}

func (uc *ContractUseCase) stampConfirmation(c *domain.Contract, userID string) {
	if strings.TrimSpace(userID) == "" {
		// A confirmation with no name is not a confirmation. Silently dropping
		// it is better than recording that "somebody" validated the numbers.
		return
	}
	at := uc.now()
	c.ConfirmedAt = &at
	c.ConfirmedByUserID = userID
}

// applyContractInput copies the fields a client owns onto the entity.
func applyContractInput(c *domain.Contract, in ContractInput) {
	c.ContactID = strings.TrimSpace(in.ContactID)
	if title := strings.TrimSpace(in.Title); title != "" {
		c.Title = title
	}
	if path := strings.TrimSpace(in.ObjectPath); path != "" {
		c.ObjectPath = path
	}
	if name := strings.TrimSpace(in.FileName); name != "" {
		c.FileName = name
	}
	c.Parties = in.Parties
	c.Subject = strings.TrimSpace(in.Subject)
	c.ValueCents = in.ValueCents
	if cur := strings.TrimSpace(strings.ToLower(in.Currency)); cur != "" {
		c.Currency = cur
	}
	c.PaymentTerms = strings.TrimSpace(in.PaymentTerms)
	c.StartsOn = in.StartsOn
	c.EndsOn = in.EndsOn
	c.AutoRenewal = in.AutoRenewal
	c.NoticeDays = in.NoticeDays
	c.AdjustmentIndex = strings.TrimSpace(in.AdjustmentIndex)
	c.AdjustmentMonth = in.AdjustmentMonth
	c.PriceRegistry = in.PriceRegistry
	c.RegistryNumber = strings.TrimSpace(in.RegistryNumber)
	c.RegistryBalanceCents = in.RegistryBalanceCents
	c.RegistryBalanceOn = in.RegistryBalanceOn
	c.LastAdjustedOn = in.LastAdjustedOn
	c.SLAText = strings.TrimSpace(in.SLAText)
	c.Penalties = in.Penalties
	c.Exclusivity = in.Exclusivity
	c.ExclusivityText = strings.TrimSpace(in.ExclusivityText)
	c.Guarantees = strings.TrimSpace(in.Guarantees)
	c.Termination = strings.TrimSpace(in.Termination)
	c.Notes = strings.TrimSpace(in.Notes)
}

// extract reads the document and stores the proposal.
//
// Never touches a field a human already confirmed: somebody who fixed a date
// while the worker ran must not have their correction overwritten by the
// reading that got it wrong in the first place.
func (uc *ContractUseCase) extract(ctx context.Context, accountID, contractID, pastedText string) {
	if err := uc.repo.SetExtraction(ctx, accountID, contractID,
		domain.ContractExtractionRunning, nil, "", nil); err != nil {
		slog.ErrorContext(ctx, "contract extraction: could not mark running",
			"contract_id", contractID, "error", err)
		return
	}

	fail := func(reason string) {
		if err := uc.repo.SetExtraction(ctx, accountID, contractID,
			domain.ContractExtractionFailed, nil, reason, nil); err != nil {
			slog.ErrorContext(ctx, "contract extraction: could not mark failed",
				"contract_id", contractID, "error", err)
		}
	}

	// The ceiling is checked before the DOWNLOAD, not just before the model
	// call: pulling a 40-page PDF out of the bucket to then refuse it wastes
	// the egress and the wait. The contract row is left as `failed` with the
	// reason, so the screen says why instead of leaving a document stuck on
	// "lendo…" forever.
	if err := uc.spend.ensure(ctx, accountID, domain.SpendDocumentExtraction); err != nil {
		var appErr *domain.AppError
		reason := "limite de leitura de documentos do plano esgotado"
		if errors.As(err, &appErr) {
			reason = appErr.Message
		}
		fail(reason)
		return
	}

	text := strings.TrimSpace(pastedText)
	// The raw bytes, kept for the scanned-PDF fallback below: a document with no
	// text layer is not a document with nothing in it.
	var data []byte
	if text == "" {
		contract, err := uc.repo.Get(ctx, accountID, contractID)
		if err != nil || contract == nil {
			fail("contrato não encontrado ao ler o documento")
			return
		}
		if uc.docs == nil || uc.extractor == nil {
			fail("leitura de PDF não configurada neste ambiente")
			return
		}
		data, err = uc.docs.DownloadObject(ctx, contract.ObjectPath, contractPDFMaxBytes)
		if err != nil {
			fail("could not abrir o arquivo enviado")
			return
		}
		text, err = uc.extractor.Extract(data)
		if err != nil {
			// A parse failure is not the end: the bytes are still a document a
			// multimodal model can read.
			text = ""
		}
	}

	provider, plan, err := uc.provider(ctx, accountID)
	if err != nil {
		fail("nenhum provedor de IA configurado para esta conta")
		return
	}

	text = strings.TrimSpace(text)
	var resp *domain.AICallbackResponse
	operation := "contract_extract"

	if len(text) >= minContractTextChars {
		if len(text) > contractTextLimit {
			text = text[:contractTextLimit]
		}
		resp, err = provider.GenerateResponse(ctx, contractExtractSystemPrompt,
			[]domain.ChatMessage{{Role: "user", Content: contractExtractPrompt(text)}}, nil)
	} else {
		// No usable text layer — the scanned contract, which in this market is
		// the common case rather than the edge one. The pipeline was going to end
		// in a model call anyway, so the FILE goes instead of the text and the
		// model does the reading.
		reader, canRead := provider.(domain.DocumentReader)
		switch {
		case !canRead:
			// This account's provider cannot read a file. Keep the honest answer
			// rather than pretending the contract is unreadable in principle.
			fail("o PDF parece ser digitalizado (sem texto selecionável) e o provedor " +
				"de IA desta conta não lê arquivos. Envie a versão com texto ou " +
				"preencha os campos à mão.")
			return
		case len(data) == 0:
			fail("o PDF parece ser digitalizado (sem texto selecionável). " +
				"Envie a versão com texto ou preencha os campos à mão.")
			return
		case len(data) > contractInlineDocMaxBytes:
			fail(fmt.Sprintf("o PDF é digitalizado e grande demais para a leitura "+
				"automática (limite de %d MB). Envie a versão com texto ou preencha "+
				"os campos à mão.", contractInlineDocMaxBytes/(1024*1024)))
			return
		}
		// Metered separately: reading pages as images costs a different order of
		// tokens from reading text, and a cost line that hides which one ran is a
		// cost line nobody can act on.
		operation = "contract_extract_vision"
		resp, err = reader.GenerateFromDocument(ctx, contractExtractSystemPrompt,
			contractExtractPrompt(""), domain.AIDocument{
				MIMEType: "application/pdf", Data: data,
			})
	}

	// The decorator chain declares GenerateFromDocument unconditionally (see
	// domain.ErrAIDocumentUnsupported), so a provider that cannot read files
	// now says so HERE instead of failing the assertion above. Same answer.
	if errors.Is(err, domain.ErrAIDocumentUnsupported) {
		fail("o PDF parece ser digitalizado (sem texto selecionável) e o provedor " +
			"de IA desta conta não lê arquivos. Envie a versão com texto ou " +
			"preencha os campos à mão.")
		return
	}
	if err != nil || resp == nil {
		fail("a leitura automática falhou; tente de novo ou preencha à mão")
		return
	}
	if uc.meter != nil && plan != nil {
		uc.meter.Record(ctx, accountID, operation, plan.AIProvider, plan.AIModel, resp.Usage)
	}

	proposal, fill := parseContractExtraction(resp.Content)
	if proposal == nil {
		fail("could not entender a resposta da leitura automática")
		return
	}
	if err := uc.repo.SetExtraction(ctx, accountID, contractID,
		domain.ContractExtractionDone, proposal, "", fill); err != nil {
		slog.ErrorContext(ctx, "contract extraction: could not store proposal",
			"contract_id", contractID, "error", err)
	}
}

func (uc *ContractUseCase) provider(ctx context.Context, accountID string) (domain.AIProvider, *domain.Plan, error) {
	if uc.ai == nil || uc.accounts == nil {
		return nil, nil, errContractExtractUnavailable
	}
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, nil, domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && uc.plans != nil {
		if p, err := uc.plans.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	// A contract is long and the reading is extraction, not writing: the
	// cheaper task-specific model keeps a 40-page PDF off whatever expensive
	// model was chosen for talking to customers.
	if uc.model != "" && uc.model != plan.AIModel {
		clone := *plan
		clone.AIModel = uc.model
		plan = &clone
	}
	provider, err := uc.ai.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil || provider == nil {
		return nil, nil, errContractExtractUnavailable
	}
	return provider, plan, nil
}

var errContractExtractUnavailable = &domain.AppError{
	Code:    "CONTRACT_EXTRACT_UNAVAILABLE",
	Message: "leitura automática indisponível: nenhum provedor de IA configurado",
}

// Reextract runs the reading again on demand.
//
// The button exists because the first pass runs on whatever model the account
// had at the time, and because a re-upload of the same contract as a
// text-bearing PDF should not need a new record.
func (uc *ContractUseCase) Reextract(ctx context.Context, accountID, id string) (*domain.Contract, error) {
	contract, err := uc.repo.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, errContractNotFound
	}
	if !uc.canExtract() {
		return nil, errContractExtractUnavailable
	}
	if strings.TrimSpace(contract.ObjectPath) == "" {
		return nil, &domain.AppError{
			Code:    "CONTRACT_NO_DOCUMENT",
			Message: "este contrato não tem arquivo para ler",
		}
	}
	contract.ExtractionStatus = domain.ContractExtractionPending
	detached := context.WithoutCancel(ctx)
	uc.dispatch(func() { uc.extract(detached, accountID, id, "") })
	return contract, nil
}

const contractExtractSystemPrompt = "Você lê contratos comerciais brasileiros e extrai " +
	"os campos pedidos. Responde só com JSON válido, sem cercas de código. " +
	"Nunca invente: campo ausente no contrato fica vazio."

// contractExtractPrompt asks for the fields.
//
// An EMPTY text is the document path: the file is attached to the request, so
// the instruction refers to it instead of quoting it. The rules below are the
// same either way — the shape of the answer must not depend on how the contract
// reached the model.
func contractExtractPrompt(text string) string {
	var b strings.Builder
	b.WriteString("Leia o contrato e extraia os campos.\n\n")
	if strings.TrimSpace(text) == "" {
		b.WriteString("O contrato está no arquivo anexado a esta mensagem. " +
			"Se o documento for digitalizado, leia o que estiver legível e deixe " +
			"vazio o que não conseguir ler com certeza.\n")
	} else {
		b.WriteString("--- CONTRATO ---\n")
		b.WriteString(text)
		b.WriteString("\n--- FIM ---\n")
	}
	b.WriteString(`
Responda APENAS com JSON:
{"title":"","subject":"",
 "parties":[{"name":"","role":"contratante","document":""}],
 "value_cents":0,"payment_terms":"",
 "starts_on":"AAAA-MM-DD","ends_on":"AAAA-MM-DD",
 "auto_renewal":false,"notice_days":0,
 "adjustment_index":"","adjustment_month":0,
 "sla_text":"",
 "penalties":[{"description":"","percent":0,"amount_cents":0,"trigger":""}],
 "exclusivity":false,"exclusivity_text":"",
 "guarantees":"","termination":""}

Regras:
- value_cents em CENTAVOS (R$ 1.500,00 = 150000). 0 se o contrato não trouxer valor.
- Datas em AAAA-MM-DD. Vazio se o contrato não trouxer.
- auto_renewal=true só se houver renovação automática escrita.
- notice_days é o prazo de aviso prévio para NÃO renovar, em dias.
- adjustment_index é o índice de reajuste ("IPCA", "IGP-M") e adjustment_month o
  mês-base (1 a 12), 0 se não houver.
- penalties: TODAS as multas, cada uma com o que a dispara.
- Não invente nada. Campo que não está no contrato fica vazio ou 0.`)
	return b.String()
}

// parseContractExtraction returns the raw proposal (for the record) and the
// fields to pre-fill (for the human to check).
//
// Anything unparseable returns nil, nil: a half-read contract offered as a
// proposal is worse than none, because somebody will confirm it.
func parseContractExtraction(raw string) (map[string]any, *domain.Contract) {
	body := extractJSONObject(raw)
	if body == "" {
		return nil, nil
	}
	var proposal map[string]any
	if json.Unmarshal([]byte(body), &proposal) != nil {
		return nil, nil
	}

	var parsed struct {
		Title   string `json:"title"`
		Subject string `json:"subject"`
		Parties []struct {
			Name     string `json:"name"`
			Role     string `json:"role"`
			Document string `json:"document"`
		} `json:"parties"`
		ValueCents      int64  `json:"value_cents"`
		PaymentTerms    string `json:"payment_terms"`
		StartsOn        string `json:"starts_on"`
		EndsOn          string `json:"ends_on"`
		AutoRenewal     bool   `json:"auto_renewal"`
		NoticeDays      int    `json:"notice_days"`
		AdjustmentIndex string `json:"adjustment_index"`
		AdjustmentMonth int    `json:"adjustment_month"`
		SLAText         string `json:"sla_text"`
		Penalties       []struct {
			Description string  `json:"description"`
			Percent     float64 `json:"percent"`
			AmountCents int64   `json:"amount_cents"`
			Trigger     string  `json:"trigger"`
		} `json:"penalties"`
		Exclusivity     bool   `json:"exclusivity"`
		ExclusivityText string `json:"exclusivity_text"`
		Guarantees      string `json:"guarantees"`
		Termination     string `json:"termination"`
	}
	if json.Unmarshal([]byte(body), &parsed) != nil {
		// The proposal is still worth keeping: a human can read it even when its
		// shape does not fit the form.
		return proposal, nil
	}

	fill := &domain.Contract{
		Title:           strings.TrimSpace(parsed.Title),
		Subject:         strings.TrimSpace(parsed.Subject),
		ValueCents:      max64(parsed.ValueCents, 0),
		PaymentTerms:    strings.TrimSpace(parsed.PaymentTerms),
		StartsOn:        parseContractDate(parsed.StartsOn),
		EndsOn:          parseContractDate(parsed.EndsOn),
		AutoRenewal:     parsed.AutoRenewal,
		AdjustmentIndex: strings.TrimSpace(parsed.AdjustmentIndex),
		SLAText:         strings.TrimSpace(parsed.SLAText),
		Exclusivity:     parsed.Exclusivity,
		ExclusivityText: strings.TrimSpace(parsed.ExclusivityText),
		Guarantees:      strings.TrimSpace(parsed.Guarantees),
		Termination:     strings.TrimSpace(parsed.Termination),
	}
	// A notice window out of range is a misread number, not a clause. Dropping
	// it leaves the field empty for a human instead of scheduling an alert on a
	// date nobody agreed to.
	if parsed.NoticeDays > 0 && parsed.NoticeDays <= domain.MaxContractNoticeDays {
		fill.NoticeDays = parsed.NoticeDays
	}
	if parsed.AdjustmentMonth >= 1 && parsed.AdjustmentMonth <= 12 {
		month := parsed.AdjustmentMonth
		fill.AdjustmentMonth = &month
	}
	// A term that ends before it starts means the model swapped the dates;
	// keeping neither is honest, keeping both would poison every "expira em".
	if fill.StartsOn != nil && fill.EndsOn != nil && fill.EndsOn.Before(*fill.StartsOn) {
		fill.StartsOn, fill.EndsOn = nil, nil
	}
	for _, p := range parsed.Parties {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		fill.Parties = append(fill.Parties, domain.ContractParty{
			Name: strings.TrimSpace(p.Name), Role: strings.TrimSpace(p.Role),
			Document: strings.TrimSpace(p.Document),
		})
	}
	for _, p := range parsed.Penalties {
		if strings.TrimSpace(p.Description) == "" && p.Percent == 0 && p.AmountCents == 0 {
			continue
		}
		percent := p.Percent
		if percent < 0 || percent > 100 {
			percent = 0
		}
		fill.Penalties = append(fill.Penalties, domain.ContractPenalty{
			Description: strings.TrimSpace(p.Description), Percent: percent,
			AmountCents: max64(p.AmountCents, 0), Trigger: strings.TrimSpace(p.Trigger),
		})
	}
	return proposal, fill
}

// parseContractDate accepts the date the prompt asked for and nothing else.
//
// A model that answers "março de 2027" gets an empty field rather than a guessed
// day: the day is what a notice deadline is counted from.
func parseContractDate(raw string) *time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &parsed
}

func max64(v, floor int64) int64 {
	if v < floor {
		return floor
	}
	return v
}
