package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeFunnelWebhookTester is a stub FunnelWebhookTester capturing the URL/secret
// it was called with and returning a canned status/body/err.
type fakeFunnelWebhookTester struct {
	called    bool
	gotURL    string
	gotSecret string
	status    int
	body      string
	err       error
}

func (f *fakeFunnelWebhookTester) DeliverFunnelTest(_ context.Context, _, url, secret string) (int, string, error) {
	f.called = true
	f.gotURL = url
	f.gotSecret = secret
	return f.status, f.body, f.err
}

// MockSalesFunnelRepo is a testify mock for domain.SalesFunnelRepository.
type MockSalesFunnelRepo struct {
	mock.Mock
}

func (m *MockSalesFunnelRepo) Create(ctx context.Context, funnel *domain.SalesFunnel) error {
	return m.Called(ctx, funnel).Error(0)
}

func (m *MockSalesFunnelRepo) GetByID(ctx context.Context, id string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SalesFunnel), args.Error(1)
}

func (m *MockSalesFunnelRepo) GetByChannelID(ctx context.Context, channelID string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SalesFunnel), args.Error(1)
}

func (m *MockSalesFunnelRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.SalesFunnel, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SalesFunnel), args.Error(1)
}

func (m *MockSalesFunnelRepo) Update(ctx context.Context, funnel *domain.SalesFunnel) error {
	return m.Called(ctx, funnel).Error(0)
}

func (m *MockSalesFunnelRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockSalesFunnelRepo) EnsureDefault(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, accountID)
	if f, _ := args.Get(0).(*domain.SalesFunnel); f != nil {
		return f, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestFunnelUseCase_Create_ValidSchema(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	schema := json.RawMessage(`[
		{"key":"empresa","label":"Empresa","type":"text","required":true},
		{"key":"orcamento","label":"Orçamento","type":"number","required":false}
	]`)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(f *domain.SalesFunnel) bool {
		return f.AccountID == "acc-1" && f.Name == "Funil Vendas" && f.Active && len(f.MetadataSchema) > 0
	})).Return(nil)

	f, err := uc.Create(context.Background(), CreateFunnelDTO{
		AccountID:      "acc-1",
		Name:           "Funil Vendas",
		MetadataSchema: schema,
	})

	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.NotEmpty(t, f.ID)
	// Schema round-trips into typed field defs.
	fields, ferr := f.MetadataFields()
	assert.NoError(t, ferr)
	assert.Len(t, fields, 2)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_Create_DuplicateKeyRejected(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	schema := json.RawMessage(`[
		{"key":"empresa","label":"Empresa","type":"text","required":true},
		{"key":"empresa","label":"Empresa 2","type":"text","required":false}
	]`)

	f, err := uc.Create(context.Background(), CreateFunnelDTO{
		AccountID:      "acc-1",
		Name:           "Funil",
		MetadataSchema: schema,
	})

	assert.Nil(t, f)
	assert.ErrorIs(t, err, domain.ErrDuplicateMetadataKey)
	// Validation fails before persistence.
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestFunnelUseCase_Create_InvalidTypeRejected(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	schema := json.RawMessage(`[{"key":"campo","label":"Campo","type":"bogus"}]`)

	_, err := uc.Create(context.Background(), CreateFunnelDTO{
		AccountID:      "acc-1",
		Name:           "Funil",
		MetadataSchema: schema,
	})

	assert.ErrorIs(t, err, domain.ErrInvalidMetadataType)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestFunnelUseCase_Get_OwnershipDenied(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID:        "f-1",
		AccountID: "other-acc",
	}, nil)

	f, err := uc.Get(context.Background(), "acc-1", "f-1")

	assert.Nil(t, f)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_Get_NotFound(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	repo.On("GetByID", mock.Anything, "missing").Return(nil, nil)

	f, err := uc.Get(context.Background(), "acc-1", "missing")

	assert.Nil(t, f)
	assert.ErrorIs(t, err, domain.ErrSalesFunnelNotFound)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_TestWebhook_Success(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)
	tester := &fakeFunnelWebhookTester{status: 200, body: `{"ok":true}`}
	uc.SetWebhookTester(tester)

	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID:            "f-1",
		AccountID:     "acc-1",
		WebhookURL:    "https://hook.example.com/wh",
		WebhookSecret: "shh",
	}, nil)

	res, err := uc.TestWebhook(context.Background(), "acc-1", "f-1")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, `{"ok":true}`, res.ResponseBody)
	assert.True(t, tester.called)
	assert.Equal(t, "https://hook.example.com/wh", tester.gotURL)
	assert.Equal(t, "shh", tester.gotSecret)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_TestWebhook_OwnershipDenied(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)
	tester := &fakeFunnelWebhookTester{status: 200}
	uc.SetWebhookTester(tester)

	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID:         "f-1",
		AccountID:  "other-acc",
		WebhookURL: "https://hook.example.com/wh",
	}, nil)

	res, err := uc.TestWebhook(context.Background(), "acc-1", "f-1")

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.False(t, tester.called, "must not ping a funnel the caller doesn't own")
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_TestWebhook_NoURLConfigured(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)
	tester := &fakeFunnelWebhookTester{status: 200}
	uc.SetWebhookTester(tester)

	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID:         "f-1",
		AccountID:  "acc-1",
		WebhookURL: "   ",
	}, nil)

	res, err := uc.TestWebhook(context.Background(), "acc-1", "f-1")

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrFunnelWebhookNotSet)
	assert.False(t, tester.called)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_TestWebhook_TransportFailure(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)
	tester := &fakeFunnelWebhookTester{err: errors.New("connection refused")}
	uc.SetWebhookTester(tester)

	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID:         "f-1",
		AccountID:  "acc-1",
		WebhookURL: "https://hook.example.com/wh",
	}, nil)

	res, err := uc.TestWebhook(context.Background(), "acc-1", "f-1")

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrFunnelWebhookTestFailed)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_TestWebhook_TesterUnavailable(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo) // no SetWebhookTester

	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID:         "f-1",
		AccountID:  "acc-1",
		WebhookURL: "https://hook.example.com/wh",
	}, nil)

	res, err := uc.TestWebhook(context.Background(), "acc-1", "f-1")

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrFunnelWebhookTesterUnavailable)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_Create_EmptySchemaDefaultsToArray(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	f, err := uc.Create(context.Background(), CreateFunnelDTO{
		AccountID: "acc-1",
		Name:      "Sem schema",
	})

	assert.NoError(t, err)
	assert.Equal(t, json.RawMessage("[]"), f.MetadataSchema)
	repo.AssertExpectations(t)
}

// US-F011: a per-funnel webhook URL is customer-supplied and reaches the
// dispatcher, so an internal target must be refused at write time — for both
// the HTTP handler and the MCP upsert_funnel tool, which share this use case.
func TestFunnelUseCase_Create_BlocksInternalWebhookURL(t *testing.T) {
	prev := httpsafe.AllowsPrivateTargets()
	httpsafe.SetAllowPrivateTargets(false)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(prev) })

	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	for _, raw := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:5432/",
		"http://10.1.2.3/hook",
	} {
		f, err := uc.Create(context.Background(), CreateFunnelDTO{
			AccountID:  "acc-1",
			Name:       "Funil",
			WebhookURL: raw,
		})
		assert.Nil(t, f, raw)
		var appErr *domain.AppError
		if assert.ErrorAs(t, err, &appErr, raw) {
			assert.Equal(t, "WEBHOOK_URL_BLOCKED", appErr.Code)
		}
	}
	// Nothing hostile ever reached the DB.
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestFunnelUseCase_Update_BlocksInternalWebhookURL(t *testing.T) {
	prev := httpsafe.AllowsPrivateTargets()
	httpsafe.SetAllowPrivateTargets(false)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(prev) })

	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)
	repo.On("GetByID", mock.Anything, "f-1").Return(&domain.SalesFunnel{
		ID: "f-1", AccountID: "acc-1", Name: "Funil",
	}, nil)

	f, err := uc.Update(context.Background(), UpdateFunnelDTO{
		AccountID:  "acc-1",
		ID:         "f-1",
		Name:       "Funil",
		WebhookURL: "http://192.168.0.10/hook",
	})

	assert.Nil(t, f)
	var appErr *domain.AppError
	if assert.ErrorAs(t, err, &appErr) {
		assert.Equal(t, "WEBHOOK_URL_BLOCKED", appErr.Code)
	}
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// An empty webhook URL means "no webhook" and must stay valid.
func TestFunnelUseCase_Create_EmptyWebhookURLAllowed(t *testing.T) {
	prev := httpsafe.AllowsPrivateTargets()
	httpsafe.SetAllowPrivateTargets(false)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(prev) })

	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.SalesFunnel")).Return(nil)

	f, err := uc.Create(context.Background(), CreateFunnelDTO{AccountID: "acc-1", Name: "Funil"})

	assert.NoError(t, err)
	assert.NotNil(t, f)
}

// The default funnel is the one thing an account always has. Listing makes
// sure of it, so the Clientes tab never opens on "you have no funnel yet".
func TestFunnelUseCase_List_MakesSureTheDefaultExists(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	def := &domain.SalesFunnel{ID: "f-def", AccountID: "acc-1", Name: domain.DefaultFunnelName, IsDefault: true}
	repo.On("EnsureDefault", mock.Anything, "acc-1").Return(def, nil)
	repo.On("ListByAccountID", mock.Anything, "acc-1").Return([]*domain.SalesFunnel{def}, nil)

	got, err := uc.List(context.Background(), "acc-1")

	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.True(t, got[0].IsDefault)
	repo.AssertExpectations(t)
}

func TestFunnelUseCase_List_FailsLoudlyWhenTheDefaultCannotBeMade(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	repo.On("EnsureDefault", mock.Anything, "acc-1").Return(nil, assert.AnError)

	got, err := uc.List(context.Background(), "acc-1")

	assert.Nil(t, got)
	assert.ErrorIs(t, err, assert.AnError)
	repo.AssertNotCalled(t, "ListByAccountID", mock.Anything, mock.Anything)
}

// It can be renamed and reconfigured, not removed — deleting it would leave
// the account with nowhere for its customers to be.
func TestFunnelUseCase_Delete_RefusesTheDefault(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	repo.On("GetByID", mock.Anything, "f-def").Return(&domain.SalesFunnel{
		ID: "f-def", AccountID: "acc-1", IsDefault: true,
	}, nil)

	err := uc.Delete(context.Background(), "acc-1", "f-def")

	assert.ErrorIs(t, err, domain.ErrCannotDeleteDefault)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestFunnelUseCase_Delete_StillRemovesAnOrdinaryFunnel(t *testing.T) {
	repo := new(MockSalesFunnelRepo)
	uc := NewFunnelUseCase(repo)

	repo.On("GetByID", mock.Anything, "f-2").Return(&domain.SalesFunnel{
		ID: "f-2", AccountID: "acc-1",
	}, nil)
	repo.On("Delete", mock.Anything, "f-2").Return(nil)

	assert.NoError(t, uc.Delete(context.Background(), "acc-1", "f-2"))
	repo.AssertExpectations(t)
}
