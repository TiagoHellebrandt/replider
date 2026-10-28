package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Signal ingestion (§2.1, D2).
//
// PNCP we read ourselves, because it is public and free. The other useful
// Brazilian signals — a freshly opened CNPJ, a CNAE change, a job posting —
// live behind paid vendors (Speedio and friends) whose contracts differ per
// customer. Writing a first-party adapter for each would mean shipping code
// against an API we cannot test, for a key we do not hold.
//
// So this is the seam instead: any feed the customer ALREADY pays for can
// POST here — through Zapier, Make, n8n, an ERP job, a scraper they run —
// and the signal lands in the same inbox, with the same dedup, expiry and
// watch matching as a tender. The customer's existing vendor contract does
// the part we cannot honestly do for them.
type SignalIngestHandler struct {
	repo domain.SignalRepository
	// secret is the deployment-wide SIGNAL_INGEST_SECRET. It never signs a
	// request directly: each account gets its own key derived from it (see
	// AccountIngestSecret), so a signature captured for one account is
	// worthless against another even though the account is named in the
	// URL.
	secret string
}

// AccountIngestSecret derives the per-account signing key the customer's
// feed uses: hex(HMAC-SHA256(key = SIGNAL_INGEST_SECRET, msg = account_id)).
// The operator hands this value to the customer; the global secret stays
// on the server. Documented in docs/webhooks.md ("Inbound signals").
func AccountIngestSecret(globalSecret, accountID string) string {
	mac := hmac.New(sha256.New, []byte(globalSecret))
	mac.Write([]byte(accountID))
	return hex.EncodeToString(mac.Sum(nil))
}

func NewSignalIngestHandler(repo domain.SignalRepository, secret string) *SignalIngestHandler {
	return &SignalIngestHandler{repo: repo, secret: secret}
}

// maxIngestBody bounds one push. A signal is a few hundred bytes of text; a
// megabyte of it is either a mistake or an attack.
const maxIngestBody = 512 << 10

type ingestRequest struct {
	// Source must be one of the known kinds. An invented one would create
	// signals no watch can ever match, which looks like a silent failure.
	Source string `json:"source"`
	// ExternalID is the dedup key. REQUIRED, because a feed that pushes the
	// same company every hour would otherwise fill the inbox with one
	// company.
	ExternalID   string `json:"external_id"`
	Organization string `json:"organization"`
	CNPJ         string `json:"cnpj"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	ValueCents   int64  `json:"value_cents"`
	City         string `json:"city"`
	State        string `json:"state"`
	PublishedAt  string `json:"published_at"`
	ClosesAt     string `json:"closes_at"`
}

// HandleIngest serves POST /v1/webhooks/signals/:account_id.
//
// Unauthenticated by JWT — the caller is a machine, not a session — and
// authenticated instead by an HMAC of the body. The account is named in the
// path, so the signature is what stops anyone from writing into somebody
// else's feed.
//
// @Summary  Push an external buying signal
// @Tags     signals
// @Accept   json
// @Param    account_id path string true "Account ID"
// @Success  202
// @Failure  401 {object} domain.AppError
// @Router   /webhooks/signals/{account_id} [post]
func (h *SignalIngestHandler) HandleIngest(c *gin.Context) {
	if h.secret == "" {
		// Refuse rather than accept unsigned writes. An open endpoint that
		// inserts into a customer's prospecting feed is a way to make a
		// salesperson email whoever an attacker chooses.
		respondError(c, &domain.AppError{
			Code:    "SIGNAL_INGEST_UNAVAILABLE",
			Message: "ingestão de sinais não configurada",
		})
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxIngestBody))
	if err != nil {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "corpo ilegível"})
		return
	}
	if !h.validSignature(c.Param("account_id"), c.GetHeader("X-Signal-Signature"), body) {
		respondError(c, domain.ErrUnauthorized)
		return
	}

	// Decoded from the bytes we already read, not from the request body: the
	// signature was computed over exactly these bytes, and re-reading the
	// stream would verify one thing and store another.
	var req ingestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "JSON inválido"})
		return
	}

	source := domain.SignalSource(strings.ToLower(strings.TrimSpace(req.Source)))
	switch source {
	case domain.SignalNewCompany, domain.SignalJobPost, domain.SignalPNCP:
	default:
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "fonte de sinal desconhecida"})
		return
	}
	if strings.TrimSpace(req.ExternalID) == "" || strings.TrimSpace(req.Title) == "" {
		respondError(c, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "external_id e title são obrigatórios",
		})
		return
	}

	signal := &domain.Signal{
		ID: ulid.Make().String(), AccountID: c.Param("account_id"),
		Source: source, ExternalID: strings.TrimSpace(req.ExternalID),
		Organization: req.Organization, CNPJ: onlyDigits(req.CNPJ),
		Title: req.Title, Description: req.Description, URL: req.URL,
		ValueCents: req.ValueCents, City: req.City,
		State:       strings.ToUpper(strings.TrimSpace(req.State)),
		PublishedAt: parseIngestTime(req.PublishedAt, time.Now()),
	}
	if closes := parseIngestTime(req.ClosesAt, time.Time{}); !closes.IsZero() {
		signal.ClosesAt = &closes
	}

	if err := h.repo.Upsert(c.Request.Context(), signal); err != nil {
		respondError(c, err)
		return
	}
	// 202, not 200: the signal is stored, but whether any watch matches it
	// is decided when the feed is read. Claiming otherwise would promise
	// something this handler does not know.
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

// validSignature compares an HMAC-SHA256 of the raw body under the
// ACCOUNT's derived key. The path names the account; the key is derived
// from it, so a body+signature pair replayed against another account_id
// verifies under a different key and fails.
//
// Constant time: a plain == leaks how much of a forged signature was right,
// one byte at a time.
func (h *SignalIngestHandler) validSignature(accountID, header string, body []byte) bool {
	provided := strings.TrimPrefix(strings.TrimSpace(header), "sha256=")
	if provided == "" || accountID == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(AccountIngestSecret(h.secret, accountID)))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(provided), []byte(expected))
}

func parseIngestTime(raw string, fallback time.Time) time.Time {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return fallback
}

// onlyDigits strips CNPJ punctuation. The CNPJ is a join key to everything
// else we know about a company, and a punctuated one joins to nothing.
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
