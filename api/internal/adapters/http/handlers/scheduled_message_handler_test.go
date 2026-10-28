package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// fakeScheduledRepo implements domain.OutboundScheduledRepository
// without testify/mock so the tests stay readable. Records every
// call so the assertions can lock the wire shape that hits the repo.
type fakeScheduledRepo struct {
	mu        sync.Mutex
	created   []*domain.OutboundScheduled
	cancelled []string
	listed    []*domain.OutboundScheduled
	listErr   error
}

func (f *fakeScheduledRepo) Create(_ context.Context, row *domain.OutboundScheduled) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, row)
	return nil
}
func (f *fakeScheduledRepo) ListDue(context.Context, int) ([]*domain.OutboundScheduled, error) {
	return nil, nil
}
func (f *fakeScheduledRepo) MarkSent(context.Context, string, string) error     { return nil }
func (f *fakeScheduledRepo) MarkFailed(context.Context, string, string) error   { return nil }
func (f *fakeScheduledRepo) TouchAttempt(context.Context, string, string) error { return nil }
func (f *fakeScheduledRepo) Cancel(_ context.Context, id, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled = append(f.cancelled, id)
	return nil
}
func (f *fakeScheduledRepo) ListByAccount(context.Context, string, int, int) ([]*domain.OutboundScheduled, error) {
	return f.listed, f.listErr
}
func (f *fakeScheduledRepo) RecoverStale(context.Context, int) (int64, error) { return 0, nil }

func setupScheduledRouter(t *testing.T, repo *fakeScheduledRepo, channel *domain.Channel) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	channelRepo := new(MockChannelRepo)
	if channel != nil {
		channelRepo.On("GetByID", mock.Anything, channel.ID).Return(channel, nil).Maybe()
	}
	channelRepo.On("GetByID", mock.Anything, "ch-other").Return(&domain.Channel{
		ID: "ch-other", AccountID: "different-account", Type: "whatsapp",
	}, nil).Maybe()

	h := NewScheduledMessageHandler(repo, channelRepo)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accountID", testAccountID)
		c.Set("userRole", "admin")
		c.Next()
	})
	r.POST("/v1/scheduled-messages", h.HandleSchedule)
	r.GET("/v1/scheduled-messages", h.HandleList)
	r.DELETE("/v1/scheduled-messages/:id", h.HandleCancel)
	return r
}

func TestScheduledMessage_Create_HappyPath(t *testing.T) {
	repo := &fakeScheduledRepo{}
	router := setupScheduledRouter(t, repo, &domain.Channel{
		ID: "ch-1", AccountID: testAccountID, Type: "whatsapp",
	})

	body, _ := json.Marshal(map[string]any{
		"channel_id": "ch-1",
		"recipient":  "5511999999999",
		"send_at":    time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		"payload": map[string]any{
			"type":    "text",
			"content": "Lembrete: consulta amanhã às 10h",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/scheduled-messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 scheduled row created, got %d", len(repo.created))
	}
	row := repo.created[0]
	assert.Equal(t, "ch-1", row.ChannelID)
	assert.Equal(t, testAccountID, row.AccountID)
	assert.Equal(t, "5511999999999", row.Recipient)
	assert.Contains(t, string(row.Payload), "Lembrete")
}

func TestScheduledMessage_Create_RejectsSendAtInPast(t *testing.T) {
	repo := &fakeScheduledRepo{}
	router := setupScheduledRouter(t, repo, &domain.Channel{
		ID: "ch-1", AccountID: testAccountID, Type: "whatsapp",
	})

	body, _ := json.Marshal(map[string]any{
		"channel_id": "ch-1",
		"recipient":  "5511999999999",
		"send_at":    time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		"payload":    map[string]any{"type": "text", "content": "x"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/scheduled-messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	if len(repo.created) != 0 {
		t.Errorf("expected NO row created for past send_at, got %d", len(repo.created))
	}
}

func TestScheduledMessage_Create_RejectsCrossAccountChannel(t *testing.T) {
	repo := &fakeScheduledRepo{}
	router := setupScheduledRouter(t, repo, nil)

	body, _ := json.Marshal(map[string]any{
		"channel_id": "ch-other",
		"recipient":  "5511999999999",
		"send_at":    time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		"payload":    map[string]any{"type": "text", "content": "x"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/scheduled-messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestScheduledMessage_Cancel(t *testing.T) {
	repo := &fakeScheduledRepo{}
	router := setupScheduledRouter(t, repo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/v1/scheduled-messages/sched-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"sched-1"}, repo.cancelled)
}

func TestScheduledMessage_ListReturnsItems(t *testing.T) {
	repo := &fakeScheduledRepo{
		listed: []*domain.OutboundScheduled{
			{ID: "a", AccountID: testAccountID},
			{ID: "b", AccountID: testAccountID},
		},
	}
	router := setupScheduledRouter(t, repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/scheduled-messages", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 2, len(resp.Items))
}
