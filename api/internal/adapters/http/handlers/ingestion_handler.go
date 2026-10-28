package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

const (
	// knowledgePDFMaxSize is the upload cap we enforce at /upload-url.
	// Mission spec: 10 MB. Mirrors knowledgePDFMaxBytes in the use
	// case so the worker re-applies the same limit defensively.
	knowledgePDFMaxSize = int64(10 * 1024 * 1024)
	// uploadURLTTL is how long the signed PUT URL stays valid. 15min
	// matches the existing /media/upload contract; large enough for a
	// slow mobile upload, small enough that a leaked URL is useless
	// by the time anyone finds it.
	uploadURLTTL = 15 * time.Minute
)

// IngestionHandler exposes the async URL-ingest pipeline:
//   - POST /knowledge/sources/:id/ingest-url  → enqueue
//   - GET  /knowledge/ingestion-jobs/:id      → poll status
//   - GET  /knowledge/sources/:id/jobs        → history
//
// All routes are scoped by account_id (JWT claim) and the underlying
// use case enforces ownership through agent → account.
type IngestionHandler struct {
	uc         *usecases.IngestionUseCase
	gcsAdapter *storage.GCSAdapter
}

func NewIngestionHandler(uc *usecases.IngestionUseCase, gcsAdapter *storage.GCSAdapter) *IngestionHandler {
	return &IngestionHandler{uc: uc, gcsAdapter: gcsAdapter}
}

// IngestURLRequest is the single-URL variant body. The multi-page
// crawler still lives at POST /knowledge/sources/url and takes a
// different shape — kept separate on purpose so the two modes don't
// collide on one handler.
type IngestURLRequest struct {
	URL string `json:"url" binding:"required"`
}

// HandleIngestURL godoc
// @Summary      Queue a URL ingestion job
// @Description  Creates a pending ingestion_job for the given knowledge
// @Description  source. The worker picks it up within ~3 s, fetches the
// @Description  URL (text/html only, max 2 MiB, 15 s timeout), chunks +
// @Description  embeds the content, and persists chunks on the source.
// @Description  The client should poll GET /knowledge/ingestion-jobs/:id
// @Description  until status is completed or failed.
// @Tags         Knowledge Base
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Knowledge Source ID"
// @Param        request body handlers.IngestURLRequest true "Ingest payload"
// @Success      202 {object} domain.IngestionJob
// @Failure      404 {object} domain.AppError "code: KNOWLEDGE_NOT_FOUND"
// @Router       /knowledge/sources/{id}/ingest-url [post]
func (h *IngestionHandler) HandleIngestURL(c *gin.Context) {
	var req IngestURLRequest
	if !bindJSON(c, &req) {
		return
	}

	job, err := h.uc.CreateURLJob(c.Request.Context(), usecases.CreateURLJobDTO{
		AccountID: c.GetString("accountID"),
		SourceID:  c.Param("id"),
		URL:       req.URL,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, job)
}

// HandleGetIngestionJob godoc
// @Summary      Poll an ingestion job
// @Description  Returns the latest status, chunks_created, and
// @Description  error_message if terminal. Dashboards poll every ~2 s
// @Description  until status ∈ {completed, failed}.
// @Tags         Knowledge Base
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Ingestion Job ID"
// @Success      200 {object} domain.IngestionJob
// @Failure      404 {object} domain.AppError "code: KNOWLEDGE_NOT_FOUND"
// @Router       /knowledge/ingestion-jobs/{id} [get]
func (h *IngestionHandler) HandleGetIngestionJob(c *gin.Context) {
	job, err := h.uc.GetJob(c.Request.Context(), c.Param("id"), c.GetString("accountID"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

// UploadURLRequest is the body for minting a signed PUT URL for a
// PDF upload. Size is required so the handler can 400-early before
// the app streams bytes to GCS and discovers the cap at the end.
type UploadURLRequest struct {
	Filename string `json:"filename" binding:"required"`
	Size     int64  `json:"size" binding:"required"`
}

// UploadURLResponse is the signed-URL payload the app uses to PUT
// the file. object_path is what the client echoes back on
// /ingest-pdf so the worker knows where to fetch from.
type UploadURLResponse struct {
	UploadURL  string `json:"upload_url"`
	ObjectPath string `json:"object_path"`
}

// HandleUploadURL godoc
// @Summary      Mint a signed PUT URL for a knowledge PDF upload
// @Description  Validates filename (.pdf) + size (<=10 MiB), then
// @Description  generates a V4 signed PUT URL scoped to the caller's
// @Description  account. Client must PUT the file with
// @Description  Content-Type: application/pdf, then confirm via
// @Description  POST /knowledge/sources/{id}/ingest-pdf.
// @Tags         Knowledge Base
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Knowledge Source ID"
// @Param        request body handlers.UploadURLRequest true "Upload request"
// @Success      200 {object} handlers.UploadURLResponse
// @Failure      400 {object} domain.AppError "code: INVALID_REQUEST | PDF_TOO_LARGE | INVALID_FILE_TYPE"
// @Failure      404 {object} domain.AppError "code: KNOWLEDGE_NOT_FOUND"
// @Router       /knowledge/sources/{id}/upload-url [post]
func (h *IngestionHandler) HandleUploadURL(c *gin.Context) {
	// Validate inputs BEFORE probing storage so 400-level responses
	// don't depend on server health. A client uploading a DOCX
	// should see INVALID_FILE_TYPE whether or not GCS is wired.
	var req UploadURLRequest
	if !bindJSON(c, &req) {
		return
	}
	if !strings.HasSuffix(strings.ToLower(req.Filename), ".pdf") {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "INVALID_FILE_TYPE", Message: "apenas PDFs são aceitos nesta fase",
		})
		return
	}
	if req.Size <= 0 {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "INVALID_REQUEST", Message: "size deve ser maior que zero",
		})
		return
	}
	if req.Size > knowledgePDFMaxSize {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "PDF_TOO_LARGE",
			Message: fmt.Sprintf("o arquivo excede o limite de %d MB", knowledgePDFMaxSize/(1024*1024)),
		})
		return
	}

	if h.gcsAdapter == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "STORAGE_UNAVAILABLE", Message: "object storage not configured",
		})
		return
	}

	accountID := c.GetString("accountID")
	safeName := sanitizeFilename(req.Filename)
	objectPath := fmt.Sprintf("accounts/%s/%d/%s", accountID, time.Now().UnixMilli(), safeName)

	url, err := h.gcsAdapter.GenerateSignedURL(
		c.Request.Context(), objectPath, "application/pdf", uploadURLTTL,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{
			Code: "INTERNAL_ERROR", Message: "failed to mint upload URL",
		})
		return
	}
	c.JSON(http.StatusOK, UploadURLResponse{
		UploadURL:  url,
		ObjectPath: objectPath,
	})
}

// IngestPDFRequest is the confirm-side body: the app hands back the
// object_path it received from /upload-url after the PUT completed.
type IngestPDFRequest struct {
	ObjectPath string `json:"object_path" binding:"required"`
}

// HandleIngestPDF godoc
// @Summary      Queue a PDF ingestion job
// @Description  Creates a pending ingestion_job of type=pdf pointing
// @Description  at the given GCS object_path (minted earlier by
// @Description  POST /upload-url). The worker picks it up within ~3 s,
// @Description  downloads the PDF, extracts text, chunks + embeds, and
// @Description  persists chunks. Client polls via
// @Description  GET /knowledge/ingestion-jobs/{id}.
// @Tags         Knowledge Base
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Knowledge Source ID"
// @Param        request body handlers.IngestPDFRequest true "Ingest payload"
// @Success      202 {object} domain.IngestionJob
// @Failure      404 {object} domain.AppError "code: KNOWLEDGE_NOT_FOUND"
// @Router       /knowledge/sources/{id}/ingest-pdf [post]
func (h *IngestionHandler) HandleIngestPDF(c *gin.Context) {
	var req IngestPDFRequest
	if !bindJSON(c, &req) {
		return
	}
	job, err := h.uc.CreatePDFJob(c.Request.Context(), usecases.CreatePDFJobDTO{
		AccountID:  c.GetString("accountID"),
		SourceID:   c.Param("id"),
		ObjectPath: req.ObjectPath,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, job)
}

// HandleListIngestionJobs godoc
// @Summary      List ingestion history for a knowledge source
// @Description  Returns the newest `limit` jobs (max 100, default 20)
// @Description  for the given source. Used to render the "Últimas
// @Description  importações" strip next to the URL tab.
// @Tags         Knowledge Base
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string true  "Knowledge Source ID"
// @Param        limit query int    false "Max results (1-100)" default(20)
// @Success      200 {array} domain.IngestionJob
// @Failure      404 {object} domain.AppError
// @Router       /knowledge/sources/{id}/jobs [get]
func (h *IngestionHandler) HandleListIngestionJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	jobs, err := h.uc.ListJobsForSource(
		c.Request.Context(),
		c.Param("id"),
		c.GetString("accountID"),
		int32(limit),
	)
	if err != nil {
		respondError(c, err)
		return
	}
	if jobs == nil {
		// Non-nil empty array: the Flutter serializer is strict about
		// List<T> vs null and crashes on an unexpected null.
		jobs = []*domain.IngestionJob{}
	}
	c.JSON(http.StatusOK, jobs)
}
