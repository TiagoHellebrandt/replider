package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Contracts (§6.11, G4).
//
// Sensitive data with two properties the rest of the API does not have: what a
// caller may SEE depends on their role, and every read is logged. Both are
// enforced here, on the way out — a client that decides for itself what to hide
// is a client one devtools tab away from showing everything.
type ContractHandler struct {
	uc *usecases.ContractUseCase

	// storage mints the signed URLs. Nil in an environment with no bucket,
	// which makes upload and download 503 while the rest of the repository
	// keeps working — a contract typed by hand needs no object storage.
	storage contractStorage
	// audit is the access log. Nil disables logging, and that is a deliberate
	// startup decision rather than a silent one: see main.go.
	audit domain.AuditLogRepository
	// adjustments prices the renegotiation each contract is owed. Nil leaves the
	// field off the response entirely rather than reporting zero — "no
	// adjustment computed" and "no adjustment owed" are different answers.
	adjustments *usecases.ContractAdjustmentUseCase
}

// contractStorage is the slice of the GCS adapter this handler needs.
type contractStorage interface {
	GenerateSignedURL(ctx context.Context, objectName, contentType string, expiry time.Duration) (string, error)
	GenerateReadSignedURL(ctx context.Context, objectPath string, expiry time.Duration) (string, error)
}

func NewContractHandler(uc *usecases.ContractUseCase) *ContractHandler {
	return &ContractHandler{uc: uc}
}

func (h *ContractHandler) SetStorage(s contractStorage)         { h.storage = s }
func (h *ContractHandler) SetAudit(a domain.AuditLogRepository) { h.audit = a }
func (h *ContractHandler) SetAdjustments(uc *usecases.ContractAdjustmentUseCase) {
	h.adjustments = uc
}

// contractUploadTTL is how long the signed PUT stays valid. Long enough for a
// slow mobile upload of a scanned 20MB contract, short enough that a URL leaked
// from a log is useless by the time anybody reads it.
const contractUploadTTL = 15 * time.Minute

// contractDownloadTTL is shorter: the client redirects immediately, and a
// contract's signed URL in somebody's browser history is a copy of the document.
const contractDownloadTTL = 5 * time.Minute

// contractMaxUploadBytes caps the PDF. Contracts with annexes are big; a 20MB
// ceiling covers scanned ones and still refuses a video somebody renamed.
const contractMaxUploadBytes = 20 * 1024 * 1024

type contractUploadURLRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type contractUploadURLResponse struct {
	UploadURL  string `json:"upload_url"`
	ObjectPath string `json:"object_path"`
}

// HandleUploadURL serves POST /v1/contracts/upload-url.
//
// @Summary  Mint a signed PUT URL for a contract PDF
// @Tags     contracts
// @Accept   json
// @Produce  json
// @Success  200 {object} contractUploadURLResponse
// @Failure  400 {object} domain.AppError
// @Failure  503 {object} domain.AppError
// @Router   /contracts/upload-url [post]
func (h *ContractHandler) HandleUploadURL(c *gin.Context) {
	var req contractUploadURLRequest
	if !bindJSON(c, &req) {
		return
	}
	// Validated before storage is probed, so a client uploading a .docx gets the
	// same answer whether or not the bucket is configured.
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(req.Filename)), ".pdf") {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "INVALID_FILE_TYPE", Message: "envie o contrato em PDF"})
		return
	}
	if req.Size <= 0 {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "INVALID_REQUEST", Message: "size deve ser maior que zero"})
		return
	}
	if req.Size > contractMaxUploadBytes {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "CONTRACT_FILE_TOO_LARGE",
			Message: fmt.Sprintf("o arquivo excede o limite de %d MB",
				contractMaxUploadBytes/(1024*1024))})
		return
	}
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "STORAGE_UNAVAILABLE", Message: "armazenamento de arquivos não configurado"})
		return
	}

	accountID := getAccountID(c)
	// The path is namespaced by account and unguessable: the bucket is private,
	// but a predictable path is one fewer thing between a misconfiguration and
	// somebody else's contract.
	objectPath := fmt.Sprintf("contracts/%s/%s-%s", accountID,
		ulid.Make().String(), sanitizeFilename(req.Filename))

	url, err := h.storage.GenerateSignedURL(
		c.Request.Context(), objectPath, "application/pdf", contractUploadTTL)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contractUploadURLResponse{UploadURL: url, ObjectPath: objectPath})
}

// contractRequest is the write shape. Dates arrive as YYYY-MM-DD strings, the
// way the form shows them.
type contractRequest struct {
	ContactID string `json:"contact_id"`
	Title     string `json:"title"`
	// ParentContractID files this as an amendment (aditivo) of another contract.
	ParentContractID string `json:"parent_contract_id"`

	ObjectPath string `json:"object_path"`
	FileName   string `json:"file_name"`
	// Text is a pasted contract body, for cases with no PDF.
	Text string `json:"text"`

	Parties []domain.ContractParty `json:"parties"`
	Subject string                 `json:"subject"`

	ValueCents   int64  `json:"value_cents"`
	Currency     string `json:"currency"`
	PaymentTerms string `json:"payment_terms"`

	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`

	AutoRenewal bool `json:"auto_renewal"`
	NoticeDays  int  `json:"notice_days"`

	AdjustmentIndex string `json:"adjustment_index"`
	AdjustmentMonth *int   `json:"adjustment_month"`

	PriceRegistry        bool   `json:"price_registry"`
	RegistryNumber       string `json:"registry_number"`
	RegistryBalanceCents int64  `json:"registry_balance_cents"`
	RegistryBalanceOn    string `json:"registry_balance_on"`
	LastAdjustedOn       string `json:"last_adjusted_on"`

	SLAText         string                   `json:"sla_text"`
	Penalties       []domain.ContractPenalty `json:"penalties"`
	Exclusivity     bool                     `json:"exclusivity"`
	ExclusivityText string                   `json:"exclusivity_text"`
	Guarantees      string                   `json:"guarantees"`
	Termination     string                   `json:"termination"`
	Notes           string                   `json:"notes"`

	// Confirm records that a person validated the reading. The person is the
	// session, never a field in the body.
	Confirm bool `json:"confirm"`
}

func (req contractRequest) toInput() usecases.ContractInput {
	return usecases.ContractInput{
		ContactID:            req.ContactID,
		Title:                req.Title,
		ParentContractID:     req.ParentContractID,
		ObjectPath:           req.ObjectPath,
		FileName:             req.FileName,
		Text:                 req.Text,
		Parties:              req.Parties,
		Subject:              req.Subject,
		ValueCents:           req.ValueCents,
		Currency:             req.Currency,
		PaymentTerms:         req.PaymentTerms,
		StartsOn:             parseDateOnly(req.StartsOn),
		EndsOn:               parseDateOnly(req.EndsOn),
		AutoRenewal:          req.AutoRenewal,
		NoticeDays:           req.NoticeDays,
		AdjustmentIndex:      req.AdjustmentIndex,
		AdjustmentMonth:      req.AdjustmentMonth,
		PriceRegistry:        req.PriceRegistry,
		RegistryNumber:       req.RegistryNumber,
		RegistryBalanceCents: req.RegistryBalanceCents,
		RegistryBalanceOn:    parseDateOnly(req.RegistryBalanceOn),
		LastAdjustedOn:       parseDateOnly(req.LastAdjustedOn),
		SLAText:              req.SLAText,
		Penalties:            req.Penalties,
		Exclusivity:          req.Exclusivity,
		ExclusivityText:      req.ExclusivityText,
		Guarantees:           req.Guarantees,
		Termination:          req.Termination,
		Notes:                req.Notes,
		Confirm:              req.Confirm,
	}
}

// parseDateOnly accepts YYYY-MM-DD and nothing else. An unparseable date
// becomes absent rather than "now": a renewal alert on today's date is worse
// than no alert.
func parseDateOnly(raw string) *time.Time {
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

// contractResponse adds the two derived dates every screen would otherwise
// compute (and get differently).
type contractResponse struct {
	*domain.Contract
	// DaysUntilEnd is negative once the contract has expired, which is the state
	// that costs money and must not read as zero.
	DaysUntilEnd *int `json:"days_until_end,omitempty"`
	// NoticeDeadline is the last day to give notice before it renews itself.
	NoticeDeadline *time.Time `json:"notice_deadline,omitempty"`
	// Confirmed says a human validated the reading. Everything downstream (G5's
	// alerts, G6's renewal leads) keys on this, so the client shows it too.
	Confirmed bool `json:"confirmed"`
	// Adjustment is what the price is owed: the accumulated index since the base
	// date, and the new value it implies. Absent when the contract states no
	// adjustment month — a price nobody agreed to raise gets no suggestion.
	Adjustment *domain.ContractAdjustment `json:"adjustment,omitempty"`
}

func decorateContract(c *domain.Contract, now time.Time) contractResponse {
	return contractResponse{
		Contract: c, DaysUntilEnd: c.DaysUntilEnd(now),
		NoticeDeadline: c.NoticeDeadline(), Confirmed: c.Confirmed(),
	}
}

// decorate adds the priced adjustment when the use case is wired.
//
// Only for a CONFIRMED reading: suggesting a new price from an index month a
// model guessed would be the product inventing a number to quote a customer.
func (h *ContractHandler) decorate(c *gin.Context, contract *domain.Contract, now time.Time) contractResponse {
	out := decorateContract(contract, now)
	if h.adjustments == nil || !contract.Confirmed() {
		return out
	}
	out.Adjustment = h.adjustments.For(c.Request.Context(), contract)
	return out
}

// HandleList serves GET /v1/contracts.
//
// @Summary  List contracts
// @Tags     contracts
// @Produce  json
// @Success  200 {array} contractResponse
// @Router   /contracts [get]
func (h *ContractHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	list, err := h.uc.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	now := time.Now()
	role := getUserRole(c)
	out := make([]contractResponse, 0, len(list))
	for _, contract := range list {
		out = append(out, h.decorate(c, contract.RedactForRole(role), now))
	}
	c.JSON(http.StatusOK, out)
}

// HandleGet serves GET /v1/contracts/:id.
//
// Reading a contract is an event worth recording: these documents carry prices,
// penalties and exclusivity, and "who opened this" is the question an incident
// starts with.
//
// @Summary  One contract with its amendment chain
// @Tags     contracts
// @Param    id path string true "Contract ID"
// @Produce  json
// @Success  200 {object} contractResponse
// @Failure  404 {object} domain.AppError
// @Router   /contracts/{id} [get]
func (h *ContractHandler) HandleGet(c *gin.Context) {
	contract, err := h.uc.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	h.logAccess(c, contract, "contract.viewed")
	c.JSON(http.StatusOK, h.decorate(c, contract.RedactForRole(getUserRole(c)), time.Now()))
}

// HandleCreate serves POST /v1/contracts.
//
// @Summary  File a contract (and read it, when there is a document)
// @Tags     contracts
// @Accept   json
// @Produce  json
// @Success  200 {object} contractResponse
// @Failure  400 {object} domain.AppError
// @Router   /contracts [post]
func (h *ContractHandler) HandleCreate(c *gin.Context) {
	var req contractRequest
	if !bindJSON(c, &req) {
		return
	}
	contract, err := h.uc.Create(c.Request.Context(), getAccountID(c), getUserID(c), req.toInput())
	if err != nil {
		respondError(c, err)
		return
	}
	h.logAccess(c, contract, "contract.created")
	c.JSON(http.StatusOK, decorateContract(contract, time.Now()))
}

// HandleUpdate serves PUT /v1/contracts/:id.
//
// @Summary  Correct and confirm a contract's fields
// @Tags     contracts
// @Accept   json
// @Produce  json
// @Success  200 {object} contractResponse
// @Failure  400 {object} domain.AppError
// @Failure  404 {object} domain.AppError
// @Router   /contracts/{id} [put]
func (h *ContractHandler) HandleUpdate(c *gin.Context) {
	var req contractRequest
	if !bindJSON(c, &req) {
		return
	}
	contract, err := h.uc.Update(c.Request.Context(), getAccountID(c), getUserID(c),
		c.Param("id"), req.toInput())
	if err != nil {
		respondError(c, err)
		return
	}
	action := "contract.updated"
	if req.Confirm {
		action = "contract.confirmed"
	}
	h.logAccess(c, contract, action)
	c.JSON(http.StatusOK, decorateContract(contract, time.Now()))
}

// HandleDelete serves DELETE /v1/contracts/:id.
//
// @Summary  Delete a contract and its amendments
// @Tags     contracts
// @Param    id path string true "Contract ID"
// @Success  204
// @Router   /contracts/{id} [delete]
func (h *ContractHandler) HandleDelete(c *gin.Context) {
	id := c.Param("id")
	// Read before deleting so the audit row can say WHAT was deleted; after the
	// delete there is nothing left to describe.
	contract, err := h.uc.Get(c.Request.Context(), getAccountID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if err := h.uc.Delete(c.Request.Context(), getAccountID(c), id); err != nil {
		respondError(c, err)
		return
	}
	h.logAccess(c, contract, "contract.deleted")
	c.Status(http.StatusNoContent)
}

// HandleReextract serves POST /v1/contracts/:id/reextract.
//
// @Summary  Read the stored document again
// @Tags     contracts
// @Produce  json
// @Success  202 {object} contractResponse
// @Failure  400 {object} domain.AppError
// @Failure  503 {object} domain.AppError
// @Router   /contracts/{id}/reextract [post]
func (h *ContractHandler) HandleReextract(c *gin.Context) {
	contract, err := h.uc.Reextract(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, decorateContract(contract, time.Now()))
}

type contractDownloadResponse struct {
	URL string `json:"url"`
	// ExpiresIn is seconds. Told to the client so a UI can say the link is
	// temporary instead of letting somebody paste an expiring URL into a chat.
	ExpiresIn int `json:"expires_in"`
}

// HandleDownload serves GET /v1/contracts/:id/download.
//
// Admin only, and always logged: a signed URL IS the document. Handing one to
// an attendant would make the role redaction on the JSON above pointless — the
// PDF has every clause that was stripped out.
//
// @Summary  Signed URL to read the stored PDF
// @Tags     contracts
// @Param    id path string true "Contract ID"
// @Produce  json
// @Success  200 {object} contractDownloadResponse
// @Failure  404 {object} domain.AppError
// @Failure  503 {object} domain.AppError
// @Router   /contracts/{id}/download [get]
func (h *ContractHandler) HandleDownload(c *gin.Context) {
	contract, err := h.uc.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if strings.TrimSpace(contract.ObjectPath) == "" {
		c.JSON(http.StatusNotFound, domain.AppError{
			Code: "CONTRACT_NO_DOCUMENT", Message: "este contrato não tem arquivo"})
		return
	}
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "STORAGE_UNAVAILABLE", Message: "armazenamento de arquivos não configurado"})
		return
	}
	url, err := h.storage.GenerateReadSignedURL(
		c.Request.Context(), contract.ObjectPath, contractDownloadTTL)
	if err != nil {
		respondError(c, err)
		return
	}
	h.logAccess(c, contract, "contract.downloaded")
	c.JSON(http.StatusOK, contractDownloadResponse{
		URL: url, ExpiresIn: int(contractDownloadTTL.Seconds())})
}

// logAccess appends to the ONE audit trail (audit_logs), which already captures
// the caller's IP and User-Agent from the request context.
//
// Best-effort by design: a contract read must not fail because the log write
// did, but the failure is logged loudly — an access trail with silent holes is
// worse than none, because it will be trusted.
func (h *ContractHandler) logAccess(c *gin.Context, contract *domain.Contract, action string) {
	if h.audit == nil || contract == nil {
		return
	}
	actor := getUserID(c)
	entry := &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  getAccountID(c),
		Action:     action,
		EntityType: "contract",
		EntityID:   contract.ID,
	}
	if actor != "" {
		entry.ActorID = &actor
	}
	// The payload names the document without copying its terms: an audit row is
	// read by more people than the contract is.
	if payload, err := json.Marshal(map[string]any{
		"title":   contract.Title,
		"version": contract.Version,
		"role":    getUserRole(c),
	}); err == nil {
		entry.Payload = payload
	}
	if err := h.audit.Create(c.Request.Context(), entry); err != nil {
		slog.ErrorContext(c.Request.Context(), "contract access log write failed",
			"action", action, "contract_id", contract.ID, "error", err)
	}
}
