package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/replider/api/internal/core/usecases"
)

type KnowledgeHandler struct {
	knowledgeUC *usecases.KnowledgeUseCase
}

func NewKnowledgeHandler(uc *usecases.KnowledgeUseCase) *KnowledgeHandler {
	return &KnowledgeHandler{knowledgeUC: uc}
}

type CreateSourceReq struct {
	AgentID     string  `json:"agent_id" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

// @Summary      Create Knowledge Source
// @Description  Creates a new knowledge source for a specific agent
// @Tags         Knowledge Base
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.CreateSourceReq true "Source Payload"
// @Success      201  {object}  domain.KnowledgeSource
// @Failure      400  {object}  domain.AppError
// @Router       /knowledge/sources [post]
func (h *KnowledgeHandler) HandleCreateSource(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req CreateSourceReq
	if !bindJSON(c, &req) {
		return
	}

	dto := usecases.CreateSourceDTO{
		AccountID:   accountID,
		AgentID:     req.AgentID,
		Name:        req.Name,
		Description: req.Description,
	}

	source, err := h.knowledgeUC.CreateSource(c.Request.Context(), dto)
	if err != nil {
		slog.Error("failed to create knowledge source", "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, source)
}

// @Summary      List Knowledge Sources
// @Description  Returns all knowledge sources for a specific agent
// @Tags         Knowledge Base
// @Produce      json
// @Security     BearerAuth
// @Param        agent_id query string true "Agent ID"
// @Success      200  {array}   domain.KnowledgeSource
// @Router       /knowledge/sources [get]
func (h *KnowledgeHandler) HandleListSources(c *gin.Context) {
	accountID := c.GetString("accountID")
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "agent_id query param is required"})
		return
	}

	sources, err := h.knowledgeUC.ListSources(c.Request.Context(), accountID, agentID)
	if err != nil {
		slog.Error("failed to list sources", "agent_id", agentID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list sources"})
		return
	}

	c.JSON(http.StatusOK, sources)
}

type IngestKnowledgeRequest struct {
	Content    string `json:"content" binding:"required"`
	SourceName string `json:"source_name"` // removed binding:"required"
	Metadata   any    `json:"metadata" swaggertype:"object"`
}

// @Summary      Ingest Knowledge Base Content
// @Description  Generates embeddings and stores in the specific knowledge source
// @Tags         Knowledge Base
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Source ID"
// @Param        request body handlers.IngestKnowledgeRequest true "Knowledge Content"
// @Success      201  {object}  domain.KnowledgeChunk
// @Failure      400  {object}  domain.AppError
// @Router       /knowledge/sources/{id}/chunks [post]
func (h *KnowledgeHandler) HandleIngestKnowledge(c *gin.Context) {
	accountID := c.GetString("accountID")
	sourceID := c.Param("id")

	var req IngestKnowledgeRequest
	if !bindJSON(c, &req) {
		return
	}

	dto := usecases.IngestKnowledgeDTO{
		AccountID:  accountID,
		SourceID:   sourceID,
		Content:    req.Content,
		SourceName: req.SourceName,
		Metadata:   req.Metadata,
	}

	chunk, err := h.knowledgeUC.Ingest(c.Request.Context(), dto)
	if err != nil {
		slog.Error("failed to ingest knowledge", "source_id", sourceID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, chunk)
}

// @Summary      List Knowledge Chunks Context
// @Description  Returns all chunks for a source
// @Tags         Knowledge Base
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Source ID"
// @Success      200  {array}   domain.KnowledgeChunk
// @Router       /knowledge/sources/{id}/chunks [get]
func (h *KnowledgeHandler) HandleListChunks(c *gin.Context) {
	accountID := c.GetString("accountID")
	sourceID := c.Param("id")

	chunks, err := h.knowledgeUC.ListChunks(c.Request.Context(), accountID, sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list chunks"})
		return
	}

	c.JSON(http.StatusOK, chunks)
}

// @Summary      Delete a knowledge chunk
// @Description  Deletes a knowledge base chunk by ID.
// @Tags         Knowledge Base
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Knowledge Chunk ID"
// @Success      200  {object}  map[string]string
// @Router       /knowledge/chunks/{id} [delete]
func (h *KnowledgeHandler) HandleDeleteKnowledge(c *gin.Context) {
	accountID := c.GetString("accountID")
	chunkID := c.Param("id")

	err := h.knowledgeUC.DeleteChunk(c.Request.Context(), accountID, chunkID)
	if err != nil {
		if err == domain.ErrKnowledgeNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to delete knowledge chunk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "knowledge chunk deleted"})
}

// @Summary      Delete Knowledge Source
// @Description  Deletes a knowledge source and every chunk under it (FK CASCADE).
// @Tags         Knowledge Base
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Knowledge Source ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  domain.AppError
// @Router       /knowledge/sources/{id} [delete]
func (h *KnowledgeHandler) HandleDeleteSource(c *gin.Context) {
	accountID := c.GetString("accountID")
	sourceID := c.Param("id")

	if err := h.knowledgeUC.DeleteSource(c.Request.Context(), accountID, sourceID); err != nil {
		if err == domain.ErrKnowledgeNotFound || err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to delete knowledge source"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "knowledge source deleted"})
}

// ──────────────────── Bulk ingestion endpoints ────────────────────

// knowledgeFileMaxBytes caps any single upload at 10 MB. Matches the
// contract in docs/BACKEND_TODOS.md and mirrors Meta's attachment cap
// so users don't hit a surprise limit after DOCX→PDF exports.
const knowledgeFileMaxBytes = 10 * 1024 * 1024

var knowledgeAllowedMimes = map[string]struct{}{
	"application/pdf": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	"text/plain; charset=utf-8": {},
}

// HandleUploadFile godoc
// @Summary      Upload a file as a knowledge source
// @Description  Accepts a PDF, DOCX or TXT (<=10 MB), extracts text, chunks
//
//	it using the default budget, and creates a single knowledge source
//	with all chunks indexed for RAG. `agent_id` is required as a
//	multipart field alongside `file`.
//
// @Tags         Knowledge Base
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        agent_id  formData  string  true  "Target agent ID"
// @Param        name      formData  string  false "Source name (defaults to filename)"
// @Param        file      formData  file    true  "Document to index"
// @Success      201  {object}  map[string]interface{}  "{source_id, chunks_created, preview}"
// @Failure      400  {object}  domain.AppError
// @Failure      413  {object}  domain.AppError
// @Router       /knowledge/sources/upload [post]
func (h *KnowledgeHandler) HandleUploadFile(c *gin.Context) {
	accountID := c.GetString("accountID")

	// Limit the total request body before Gin materializes the form —
	// otherwise a 10 GB upload lives briefly in memory before we can
	// reject it.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, knowledgeFileMaxBytes)

	if err := c.Request.ParseMultipartForm(knowledgeFileMaxBytes); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, domain.AppError{
			Code:    "FILE_TOO_LARGE",
			Message: "file must be <= 10 MB",
		})
		return
	}

	agentID := c.PostForm("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "INVALID_REQUEST",
			Message: "agent_id is required",
		})
		return
	}
	name := c.PostForm("name")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "FILE_REQUIRED",
			Message: "multipart field 'file' is required",
		})
		return
	}
	if fileHeader.Size > knowledgeFileMaxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, domain.AppError{
			Code:    "FILE_TOO_LARGE",
			Message: "file must be <= 10 MB",
		})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "FILE_READ_FAILED",
			Message: err.Error(),
		})
		return
	}
	defer f.Close()
	buf := make([]byte, fileHeader.Size)
	if _, err := f.Read(buf); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "FILE_READ_FAILED",
			Message: err.Error(),
		})
		return
	}

	// Magic-byte sniff beats client-supplied Content-Type (trivially
	// forged) and covers the PDF/DOCX/TXT cases we support.
	mime := services.DetectMimeByMagic(buf)
	if _, ok := knowledgeAllowedMimes[mime]; !ok {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "INVALID_FILE_TYPE",
			Message: "only PDF, DOCX and TXT files are supported",
		})
		return
	}

	if name == "" {
		name = fileHeader.Filename
	}
	if name == "" {
		name = "Upload"
	}

	res, err := h.knowledgeUC.CreateSourceFromFile(c.Request.Context(),
		usecases.CreateSourceFromFileDTO{
			AccountID: accountID,
			AgentID:   agentID,
			Name:      name,
			FileBytes: buf,
			Mime:      mime,
		})
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, domain.AppError{
				Code: "AGENT_NOT_FOUND", Message: "agent not found",
			})
			return
		}
		slog.Error("knowledge upload failed",
			"account_id", accountID, "agent_id", agentID, "error", err)
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "KNOWLEDGE_UPLOAD_FAILED", Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"source_id":      res.Source.ID,
		"chunks_created": res.ChunksCreated,
		"preview":        res.Preview,
	})
}

type CrawlURLReq struct {
	AgentID  string `json:"agent_id" binding:"required"`
	URL      string `json:"url" binding:"required"`
	MaxPages int    `json:"max_pages"`
	Name     string `json:"name"`
}

// HandleCrawlURL godoc
// @Summary      Crawl a URL and index it as a knowledge source
// @Description  Fetches `url` over HTTPS and BFS-walks same-origin links
//
//	up to `max_pages` (default 25), extracts visible text per page,
//	and indexes everything under a single knowledge source. SSRF-
//	protected: rejects HTTP, non-default ports, private IPs and the
//	cloud metadata services.
//
// @Tags         Knowledge Base
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.CrawlURLReq true "Crawl payload"
// @Success      201  {object}  map[string]interface{}  "{source_id, pages_crawled, chunks_created}"
// @Failure      400  {object}  domain.AppError
// @Router       /knowledge/sources/url [post]
func (h *KnowledgeHandler) HandleCrawlURL(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req CrawlURLReq
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.knowledgeUC.CreateSourceFromURL(c.Request.Context(),
		usecases.CreateSourceFromURLDTO{
			AccountID: accountID,
			AgentID:   req.AgentID,
			URL:       req.URL,
			MaxPages:  req.MaxPages,
			Name:      req.Name,
		})
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, domain.AppError{
				Code: "AGENT_NOT_FOUND", Message: "agent not found",
			})
			return
		}
		// Crawler validation errors map to 400 so the UI can render
		// them against the input field; anything else is surfaced as
		// a generic upload failure.
		slog.Warn("knowledge url crawl failed",
			"account_id", accountID, "agent_id", req.AgentID, "url", req.URL, "error", err)
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "KNOWLEDGE_URL_CRAWL_FAILED", Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"source_id":      res.Source.ID,
		"pages_crawled":  res.PagesCrawled,
		"chunks_created": res.ChunksCreated,
	})
}
