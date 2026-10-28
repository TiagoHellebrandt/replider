package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Electronic signature (§6.11, G7).
//
// The last link in the chain: a document goes out, the status lands on the card,
// and a signature BECOMES A SALE — which is what makes it reach the portfolio,
// the repurchase engine and somebody's commission statement. Without that last
// step this is a filing cabinet with a courier attached.
//
// The security spine of the whole file: the provider publishes no webhook HMAC
// and no shared secret. So the callback is treated as a HINT — "go look at this
// document" — and never as a fact. Every state change is confirmed against the
// provider's own API with the customer's own token before anything is written,
// because a payload that could register a sale is a payload worth forging.

// SignatureArchive stores the signed copy in our own private bucket.
//
// Narrow port over the GCS adapter. Archiving is the point: a signed document
// that lives only in the provider's account is a document the customer loses the
// day they stop paying that provider.
type SignatureArchive interface {
	UploadBytes(ctx context.Context, objectPath string, data []byte, contentType string) error
}

// signedPDFMaxBytes bounds the archived copy. A signed PDF carries the original
// plus an audit page; twice the upload ceiling is generous and still refuses to
// pull something absurd into memory.
const signedPDFMaxBytes = 40 * 1024 * 1024

type SignatureUseCase struct {
	// lifecycle emits contract.signed (J2). Optional.
	lifecycle LifecycleEmitter
	repo      domain.SignatureRepository
	settings  domain.SignatureSettingsRepository
	gateway   domain.SignatureGateway

	// docs reads the PDF we are sending; archive stores the signed one.
	docs    ContractDocReader
	archive SignatureArchive

	// sales turns a signature into a Sale. Optional: without it the signature is
	// still tracked and archived, and the sale is a manual step — which is better
	// than a half-registered one.
	sales     *SaleUseCase
	contracts domain.ContractRepository

	now func() time.Time
}

func NewSignatureUseCase(
	repo domain.SignatureRepository,
	settings domain.SignatureSettingsRepository,
	gateway domain.SignatureGateway,
) *SignatureUseCase {
	return &SignatureUseCase{
		repo: repo, settings: settings, gateway: gateway, now: time.Now,
	}
}

func (uc *SignatureUseCase) SetStorage(docs ContractDocReader, archive SignatureArchive) {
	uc.docs, uc.archive = docs, archive
}

func (uc *SignatureUseCase) SetSales(sales *SaleUseCase, contracts domain.ContractRepository) {
	uc.sales, uc.contracts = sales, contracts
}

func (uc *SignatureUseCase) SetClock(now func() time.Time) { uc.now = now }

var errSignatureNotFound = &domain.AppError{
	Code: "SIGNATURE_NOT_FOUND", Message: "pedido de assinatura não encontrado",
}

// ── Settings: the customer's own provider account ──────────────────────────

// Settings returns the integration state WITHOUT the token.
func (uc *SignatureUseCase) Settings(ctx context.Context, accountID string) (*domain.SignatureSettings, error) {
	current, err := uc.settings.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		// Not configured yet is a state, not an error: the screen shows the empty
		// form rather than a red box.
		return &domain.SignatureSettings{
			AccountID: accountID, Provider: domain.SignatureZapSign,
		}, nil
	}
	// A COPY: blanking the token on the struct the repository handed over would
	// poison it for any caller that caches reads, and "the credential vanished
	// after somebody opened Settings" is a bug nobody would look for here.
	view := *current
	view.HasToken = current.Configured()
	view.APIToken = ""
	return &view, nil
}

// SaveSettings stores the BYO token and mints the inbound webhook token.
func (uc *SignatureUseCase) SaveSettings(
	ctx context.Context, accountID, apiToken string, sandbox bool,
) (*domain.SignatureSettings, error) {
	current, err := uc.settings.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		current = &domain.SignatureSettings{
			AccountID: accountID, Provider: domain.SignatureZapSign,
		}
	}
	if current.WebhookToken == "" {
		// Minted once and kept: rotating it on every save would silently break
		// the callback the customer already pasted into their provider.
		current.WebhookToken = ulid.Make().String()
	}
	current.Sandbox = sandbox
	// An empty token on save KEEPS the stored one — the same posture as channel
	// credentials, so a settings screen that shows a mask does not wipe the
	// credential when somebody edits the checkbox next to it.
	if trimmed := strings.TrimSpace(apiToken); trimmed != "" {
		current.APIToken = trimmed
	}
	if err := uc.settings.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return uc.Settings(ctx, accountID)
}

// ── Sending ────────────────────────────────────────────────────────────────

// SignatureInput is what a client may ask for.
type SignatureInput struct {
	// ContractID is the usual source: the PDF already in our bucket. ObjectPath
	// overrides it for a proposal that is not a contract yet.
	ContractID string
	ObjectPath string
	LeadID     string
	ContactID  string

	Title   string
	Signers []domain.SignatureSigner

	// ValueCents is what the signature is worth. Zero tracks the document and
	// registers NO sale.
	ValueCents int64
	DeadlineOn *time.Time
}

// Send puts a document out for signature.
func (uc *SignatureUseCase) Send(
	ctx context.Context, accountID, userID string, in SignatureInput,
) (*domain.SignatureRequest, error) {
	settings, err := uc.settings.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !settings.Configured() {
		return nil, domain.ErrSignatureNotConfigured
	}
	if uc.docs == nil {
		return nil, &domain.AppError{
			Code:    "SIGNATURE_STORAGE_UNAVAILABLE",
			Message: "armazenamento de arquivos não configurado",
		}
	}

	req := &domain.SignatureRequest{
		ID: ulid.Make().String(), AccountID: accountID,
		ContractID:      strings.TrimSpace(in.ContractID),
		LeadID:          strings.TrimSpace(in.LeadID),
		ContactID:       strings.TrimSpace(in.ContactID),
		Provider:        uc.gateway.Name(),
		Title:           strings.TrimSpace(in.Title),
		Status:          domain.SignatureDraft,
		Signers:         in.Signers,
		ObjectPath:      strings.TrimSpace(in.ObjectPath),
		ValueCents:      in.ValueCents,
		Currency:        domain.CommissionLedgerCurrency,
		CreatedByUserID: userID,
	}

	// A contract fills in what it already knows, so the caller does not repeat
	// the title, the counterpart or the value it can read off the record.
	if req.ContractID != "" && uc.contracts != nil {
		contract, err := uc.contracts.Get(ctx, accountID, req.ContractID)
		if err != nil {
			return nil, err
		}
		if contract == nil {
			return nil, errContractNotFound
		}
		if req.ObjectPath == "" {
			req.ObjectPath = contract.ObjectPath
		}
		if req.Title == "" {
			req.Title = contract.Title
		}
		if req.ContactID == "" {
			req.ContactID = contract.ContactID
		}
		if req.ValueCents == 0 {
			req.ValueCents = contract.ValueCents
		}
		if in.DeadlineOn == nil && contract.EndsOn != nil {
			in.DeadlineOn = contract.EndsOn
		}
	}
	if req.ObjectPath == "" {
		return nil, &domain.AppError{
			Code:    "SIGNATURE_NO_DOCUMENT",
			Message: "não há arquivo para enviar — anexe o PDF ao contrato primeiro",
		}
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	pdf, err := uc.docs.DownloadObject(ctx, req.ObjectPath, signedPDFMaxBytes)
	if err != nil {
		return nil, &domain.AppError{
			Code: "SIGNATURE_NO_DOCUMENT", Message: "could not abrir o arquivo"}
	}

	// Persisted BEFORE the provider call, as a draft: a document that reaches the
	// provider and never reaches our table is a signature nobody can track, and
	// the customer's quota was spent either way.
	if err := uc.repo.Create(ctx, req); err != nil {
		return nil, err
	}

	result, err := uc.gateway.Create(ctx, settings.APIToken, domain.SignatureDocument{
		Title: req.Title, PDF: pdf, Signers: req.Signers,
		ExternalID: req.ID, Sandbox: settings.Sandbox, DeadlineOn: in.DeadlineOn,
	})
	if err != nil {
		req.Status = domain.SignatureFailed
		req.FailureReason = signatureFailureReason(err)
		if updateErr := uc.repo.Update(ctx, req); updateErr != nil {
			slog.ErrorContext(ctx, "signature: could not record the send failure",
				"request_id", req.ID, "error", updateErr)
		}
		return req, err
	}

	sentAt := uc.now()
	req.ProviderDocToken = result.ProviderDocToken
	req.Status = result.Status
	req.SentAt = &sentAt
	req.Signers = mergeSigners(req.Signers, result.Signers)
	if err := uc.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func signatureFailureReason(err error) string {
	if appErr, ok := err.(*domain.AppError); ok {
		return appErr.Message
	}
	// The provider's own words, trimmed: a person can act on "signers: this field
	// is required" and cannot act on "erro ao enviar".
	msg := err.Error()
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
}

// mergeSigners keeps what WE know (the phone we were given) and takes what only
// the provider knows (the token, the signing link, their status).
func mergeSigners(ours, theirs []domain.SignatureSigner) []domain.SignatureSigner {
	if len(theirs) == 0 {
		return ours
	}
	out := make([]domain.SignatureSigner, 0, len(theirs))
	for i, remote := range theirs {
		merged := remote
		if i < len(ours) {
			// Matched by position: the provider returns the signers in the order
			// they were sent. Matching by email would drop a signer who has only
			// a phone.
			merged.Phone = ours[i].Phone
			if merged.Email == "" {
				merged.Email = ours[i].Email
			}
			if merged.Name == "" {
				merged.Name = ours[i].Name
			}
		}
		out = append(out, merged)
	}
	return out
}

// ── Reading ────────────────────────────────────────────────────────────────

func (uc *SignatureUseCase) Get(ctx context.Context, accountID, id string) (*domain.SignatureRequest, error) {
	req, err := uc.repo.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errSignatureNotFound
	}
	return req, nil
}

func (uc *SignatureUseCase) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.SignatureRequest, error) {
	list, err := uc.repo.List(ctx, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.SignatureRequest{}
	}
	return list, nil
}

// ListByContract is the contract card's read.
func (uc *SignatureUseCase) ListByContract(
	ctx context.Context, accountID, contractID string,
) ([]*domain.SignatureRequest, error) {
	list, err := uc.repo.ListByContract(ctx, accountID, contractID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.SignatureRequest{}
	}
	return list, nil
}

// Refresh asks the provider what the truth is and applies it.
//
// The one place status changes. Both the manual "atualizar" button and the
// webhook come through here, so a forged callback and a real one do exactly the
// same thing: cause a read.
func (uc *SignatureUseCase) Refresh(
	ctx context.Context, accountID, id string,
) (*domain.SignatureRequest, error) {
	req, err := uc.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.ProviderDocToken) == "" {
		// Never reached the provider: there is nothing to ask about.
		return req, nil
	}
	settings, err := uc.settings.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !settings.Configured() {
		return nil, domain.ErrSignatureNotConfigured
	}

	result, err := uc.gateway.Get(ctx, settings.APIToken, req.ProviderDocToken)
	if err != nil {
		return nil, err
	}
	return uc.apply(ctx, settings, req, result)
}

// apply writes a confirmed state and, on a signature, archives and registers.
func (uc *SignatureUseCase) apply(
	ctx context.Context, settings *domain.SignatureSettings,
	req *domain.SignatureRequest, result *domain.SignatureResult,
) (*domain.SignatureRequest, error) {
	now := uc.now()
	previous := req.Status

	req.Status = result.Status
	req.Signers = mergeSigners(req.Signers, result.Signers)

	switch result.Status {
	case domain.SignatureViewed:
		if req.ViewedAt == nil {
			req.ViewedAt = &now
		}
		// A signed document that gets a late "viewed" event must not walk
		// backwards: terminal is terminal.
		if previous.Terminal() {
			req.Status = previous
		}
	case domain.SignatureSigned:
		if req.SignedAt == nil {
			req.SignedAt = &now
		}
	case domain.SignatureRefused:
		if req.RefusedAt == nil {
			req.RefusedAt = &now
		}
	}

	// Archive before anything else depends on it: the signed copy is the evidence,
	// and a failure here must not stop the status from being recorded.
	if req.Status == domain.SignatureSigned && req.SignedObjectPath == "" &&
		result.SignedFileURL != "" && uc.archive != nil {
		path := fmt.Sprintf("contracts/%s/signed/%s.pdf", req.AccountID, req.ID)
		data, err := uc.gateway.DownloadSigned(ctx, result.SignedFileURL, signedPDFMaxBytes)
		if err == nil {
			if err = uc.archive.UploadBytes(ctx, path, data, "application/pdf"); err == nil {
				req.SignedObjectPath = path
			}
		}
		if err != nil {
			// Loudly, because the customer believes they have a copy: the status is
			// still correct and the file can be re-fetched by pressing refresh.
			slog.ErrorContext(ctx, "signature: signed copy not archived",
				"request_id", req.ID, "error", err)
		}
	}

	if err := uc.repo.Update(ctx, req); err != nil {
		return nil, err
	}

	if req.BecomesSale() {
		uc.registerSale(ctx, req)
	}

	// J2: only on the TRANSITION into signed. `apply` runs on every refresh and
	// every callback, so emitting on the state rather than on the change would
	// fire the same event on every poll.
	if previous != domain.SignatureSigned && req.Signed() {
		emitLifecycle(uc.lifecycle, req.AccountID, domain.EventContractSigned,
			domain.LifecyclePayload{
				ID:          req.ID,
				ContactID:   req.ContactID,
				AmountCents: req.ValueCents,
				Currency:    domain.CommissionLedgerCurrency,
				Confirmed:   true,
			})
	}
	return req, nil
}

// SetLifecycleEmitter wires the contract.signed webhook (J2). Optional.
func (uc *SignatureUseCase) SetLifecycleEmitter(e LifecycleEmitter) {
	uc.lifecycle = e
}

// registerSale turns a signature into a Sale.
//
// This is the encaixe: the moment somebody signs, the deal reaches the portfolio,
// the repurchase prediction and the commission statement without anybody typing
// it again. Best-effort and loud: the signature is already recorded, and failing
// the whole call here would leave a signed document looking unsigned.
func (uc *SignatureUseCase) registerSale(ctx context.Context, req *domain.SignatureRequest) {
	if uc.sales == nil {
		return
	}
	// Claim the slot FIRST. Two webhooks arriving together would otherwise both
	// see an empty sale_id and register two sales — and a duplicate sale is a
	// duplicate commission.
	claimed, err := uc.repo.AttachSale(ctx, req.AccountID, req.ID, "pending")
	if err != nil || !claimed {
		return
	}

	sale, err := uc.sales.Register(ctx, req.AccountID, &domain.Sale{
		ContactID:    req.ContactID,
		LeadID:       req.LeadID,
		SellerUserID: req.CreatedByUserID,
		GrossCents:   req.ValueCents,
		Currency:     req.Currency,
		Source:       domain.SaleManual,
		Notes:        "Registrada pela assinatura de " + req.Title,
	})
	if err != nil {
		// Release the claim so a retry (the refresh button, the next webhook) can
		// try again: a slot held by a failure is a sale nobody can ever register.
		if _, releaseErr := uc.repo.AttachSale(ctx, req.AccountID, req.ID, ""); releaseErr != nil {
			slog.ErrorContext(ctx, "signature: could not release the sale claim",
				"request_id", req.ID, "error", releaseErr)
		}
		slog.ErrorContext(ctx, "signature: signed but the sale was not registered",
			"request_id", req.ID, "error", err)
		return
	}
	if _, err := uc.repo.AttachSale(ctx, req.AccountID, req.ID, sale.ID); err != nil {
		slog.ErrorContext(ctx, "signature: sale registered but not linked — "+
			"a later refresh may register it twice",
			"request_id", req.ID, "sale_id", sale.ID, "error", err)
		return
	}
	req.SaleID = sale.ID
}

// ── The callback ───────────────────────────────────────────────────────────

// SignatureCallback is the shape we read off an inbound webhook. Deliberately
// minimal: two fields, both used only to FIND the document.
type SignatureCallback struct {
	EventType  string
	DocToken   string
	ExternalID string
}

// HandleCallback resolves an inbound webhook and re-reads the truth.
//
// The provider signs nothing, so this treats the payload as a hint. It carries no
// status into the database: it finds the row, then asks the provider — with the
// customer's own token — what actually happened. Somebody who forges a
// `doc_signed` callback causes an API read that returns "pending", and nothing
// changes.
func (uc *SignatureUseCase) HandleCallback(
	ctx context.Context, webhookToken string, cb SignatureCallback,
) error {
	settings, err := uc.settings.ByWebhookToken(ctx, strings.TrimSpace(webhookToken))
	if err != nil {
		return err
	}
	if settings == nil {
		// An unknown token gets the same answer as a wrong one, and says nothing
		// about which accounts exist.
		return errSignatureNotFound
	}
	if !settings.Configured() {
		return domain.ErrSignatureNotConfigured
	}

	// Events we do not act on stop here rather than causing a provider read on
	// every notification email that bounces.
	if domain.SignatureStatus(mapCallbackStatus(cb.EventType)) == "" {
		return nil
	}

	var req *domain.SignatureRequest
	if id := strings.TrimSpace(cb.ExternalID); id != "" {
		// Our own id, echoed back: the reliable match.
		req, err = uc.repo.Get(ctx, settings.AccountID, id)
		if err != nil {
			return err
		}
	}
	if req == nil && strings.TrimSpace(cb.DocToken) != "" {
		req, err = uc.repo.GetByProviderToken(ctx, settings.AccountID, cb.DocToken)
		if err != nil {
			return err
		}
	}
	if req == nil {
		return errSignatureNotFound
	}

	result, err := uc.gateway.Get(ctx, settings.APIToken, req.ProviderDocToken)
	if err != nil {
		return err
	}
	_, err = uc.apply(ctx, settings, req, result)
	return err
}

// mapCallbackStatus is set by the adapter package at wiring time.
//
// A function variable rather than an import: the domain and the use case must not
// depend on a specific provider's event vocabulary, and this keeps the mapping in
// the one place that knows it.
var mapCallbackStatus = func(string) string { return "" }

// SetCallbackMapper wires the provider's event vocabulary.
func SetCallbackMapper(fn func(string) domain.SignatureStatus) {
	mapCallbackStatus = func(event string) string { return string(fn(event)) }
}
