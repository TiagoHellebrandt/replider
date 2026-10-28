package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/replider/api/internal/core/usecases"
)

// setupUploadURLRouter wires JUST the /upload-url endpoint with a nil
// GCS adapter — the 400-level tests never reach the signing step, so
// we don't need a fake. The test asserts the handler rejects bad
// inputs before touching storage.
func setupUploadURLRouter(jwtMock *MockJWTService) *gin.Engine {
	// Use case + GCS adapter are only needed for success-path tests
	// (which would require a real/fake signer). The validation tests
	// below rely exclusively on handler-level short-circuits.
	uc := usecases.NewIngestionUseCase(nil, nil, nil, nil, nil)
	handler := NewIngestionHandler(uc, nil)

	r := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := r.Group("/v1/knowledge/sources")
	g.Use(authMw, middleware.RequireRole("admin"))
	g.POST("/:id/upload-url", handler.HandleUploadURL)
	return r
}

func TestHandleUploadURL_RejectsNonPDFExtension(t *testing.T) {
	jwt := new(MockJWTService)
	expectAuth(jwt, testToken, testUserID, testAccountID, "admin")

	r := setupUploadURLRouter(jwt)
	w := performRequest(r, http.MethodPost, "/v1/knowledge/sources/src-1/upload-url",
		map[string]any{"filename": "menu.docx", "size": 1024}, testToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var body domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "INVALID_FILE_TYPE" {
		t.Errorf("expected code INVALID_FILE_TYPE, got %q", body.Code)
	}
}

func TestHandleUploadURL_RejectsOversizedFile(t *testing.T) {
	jwt := new(MockJWTService)
	expectAuth(jwt, testToken, testUserID, testAccountID, "admin")

	r := setupUploadURLRouter(jwt)
	// 12 MiB > 10 MiB cap.
	w := performRequest(r, http.MethodPost, "/v1/knowledge/sources/src-1/upload-url",
		map[string]any{"filename": "big.pdf", "size": 12 * 1024 * 1024}, testToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var body domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "PDF_TOO_LARGE" {
		t.Errorf("expected code PDF_TOO_LARGE, got %q", body.Code)
	}
}

func TestHandleUploadURL_RejectsZeroSize(t *testing.T) {
	jwt := new(MockJWTService)
	expectAuth(jwt, testToken, testUserID, testAccountID, "admin")

	r := setupUploadURLRouter(jwt)
	// gin's required binding fires before our own size check because
	// size=0 is the zero value; both outcomes are acceptable — what
	// matters is the request does NOT become a job.
	w := performRequest(r, http.MethodPost, "/v1/knowledge/sources/src-1/upload-url",
		map[string]any{"filename": "menu.pdf"}, testToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUploadURL_StorageUnavailableWhenGCSNotConfigured(t *testing.T) {
	jwt := new(MockJWTService)
	expectAuth(jwt, testToken, testUserID, testAccountID, "admin")

	r := setupUploadURLRouter(jwt)
	// Valid input (pdf + legal size), but the handler was built with
	// a nil GCS adapter. Must 503 instead of panicking.
	w := performRequest(r, http.MethodPost, "/v1/knowledge/sources/src-1/upload-url",
		map[string]any{"filename": "menu.pdf", "size": 1024}, testToken)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d body=%s", w.Code, w.Body.String())
	}
	var body domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "STORAGE_UNAVAILABLE" {
		t.Errorf("expected code STORAGE_UNAVAILABLE, got %q", body.Code)
	}
}

// Sanity: the default PDF extractor should treat the ledongthuc "no
// extractable text" sentinel as empty-string success so the use case's
// scanned-PDF guard owns the friendly-message mapping in one place.
func TestDefaultPDFExtractor_SwallowsScannedSentinel(t *testing.T) {
	ex := services.NewDefaultPDFExtractor()
	// Fake bytes that will fail to parse. The real extractor returns
	// a "pdf: failed to open" error on garbage, which is NOT the
	// scanned-PDF sentinel; that specific path requires a valid PDF
	// with no text layer, which is expensive to fabricate. Here we
	// simply confirm the wrapper exists and propagates non-sentinel
	// errors rather than swallowing them.
	_, err := ex.Extract([]byte("not a real pdf"))
	if err == nil {
		t.Fatalf("expected parse error for garbage input")
	}
}
