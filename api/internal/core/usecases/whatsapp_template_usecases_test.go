package usecases

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
)

// MockWhatsAppTemplateRepo is a testify mock for domain.WhatsAppTemplateRepository.
type MockWhatsAppTemplateRepo struct{ mock.Mock }

func (m *MockWhatsAppTemplateRepo) Upsert(ctx context.Context, t *domain.WhatsAppTemplate) error {
	return m.Called(ctx, t).Error(0)
}

func (m *MockWhatsAppTemplateRepo) ListByChannel(ctx context.Context, channelID string) ([]*domain.WhatsAppTemplate, error) {
	args := m.Called(ctx, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.WhatsAppTemplate), args.Error(1)
}

func waTemplateChannel() *domain.Channel {
	return &domain.Channel{
		ID:        "chan-1",
		AccountID: "acc-1",
		Type:      "whatsapp",
		Config: map[string]interface{}{
			"waba_id":         "waba-123",
			"phone_number_id": "pn-456",
			"access_token":    "tok-789",
		},
	}
}

func TestSyncTemplates_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "meta-tpl-1",
					"name": "boas_vindas",
					"status": "APPROVED",
					"language": "pt_BR",
					"category": "MARKETING",
					"components": [
						{"type": "BODY", "text": "Olá {{1}}, seu pedido {{2}} foi confirmado!"}
					]
				}
			],
			"paging": {}
		}`))
	}))
	defer srv.Close()
	prev := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(prev) })

	chRepo := new(MockChannelRepo)
	chRepo.On("GetByID", mock.Anything, "chan-1").Return(waTemplateChannel(), nil)

	tplRepo := new(MockWhatsAppTemplateRepo)
	var captured *domain.WhatsAppTemplate
	tplRepo.On("Upsert", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		captured = args.Get(1).(*domain.WhatsAppTemplate)
	}).Return(nil)
	tplRepo.On("ListByChannel", mock.Anything, "chan-1").Return([]*domain.WhatsAppTemplate{{Name: "boas_vindas"}}, nil)

	uc := NewSyncTemplatesUseCase(tplRepo, chRepo)
	out, err := uc.Execute(context.Background(), "acc-1", "chan-1")
	require.NoError(t, err)
	require.Len(t, out, 1)

	require.NotNil(t, captured)
	assert.Equal(t, "boas_vindas", captured.Name)
	assert.Equal(t, "pt_BR", captured.Language)
	assert.Equal(t, "MARKETING", captured.Category)
	assert.Equal(t, "APPROVED", captured.MetaStatus)
	assert.Equal(t, "meta-tpl-1", captured.MetaTemplateID)
	assert.Equal(t, "acc-1", captured.AccountID)
	assert.Equal(t, "chan-1", captured.ChannelID)
	assert.Contains(t, captured.Body, "{{1}}")

	var vars []string
	require.NoError(t, json.Unmarshal(captured.Variables, &vars))
	assert.Equal(t, []string{"1", "2"}, vars)

	tplRepo.AssertExpectations(t)
}

func TestSyncTemplates_OwnershipDenied(t *testing.T) {
	ch := waTemplateChannel()
	ch.AccountID = "someone-else"
	chRepo := new(MockChannelRepo)
	chRepo.On("GetByID", mock.Anything, "chan-1").Return(ch, nil)

	tplRepo := new(MockWhatsAppTemplateRepo)

	uc := NewSyncTemplatesUseCase(tplRepo, chRepo)
	_, err := uc.Execute(context.Background(), "acc-1", "chan-1")

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "CHANNEL_NOT_FOUND", appErr.Code)
	tplRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestSyncTemplates_MissingCredentials(t *testing.T) {
	ch := waTemplateChannel()
	ch.Config = map[string]interface{}{"phone_number_id": "pn-456"} // no waba_id / access_token
	chRepo := new(MockChannelRepo)
	chRepo.On("GetByID", mock.Anything, "chan-1").Return(ch, nil)

	tplRepo := new(MockWhatsAppTemplateRepo)

	uc := NewSyncTemplatesUseCase(tplRepo, chRepo)
	_, err := uc.Execute(context.Background(), "acc-1", "chan-1")

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "CHANNEL_MISSING_CONFIG", appErr.Code)
	tplRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestExtractTemplateVariables_Dedup(t *testing.T) {
	got := extractTemplateVariables("Oi {{1}}, confirma {{2}}? Repetindo {{1}}.")
	var vars []string
	require.NoError(t, json.Unmarshal(got, &vars))
	assert.Equal(t, []string{"1", "2"}, vars)

	empty := extractTemplateVariables("sem variaveis")
	assert.Equal(t, "[]", string(empty))
}
