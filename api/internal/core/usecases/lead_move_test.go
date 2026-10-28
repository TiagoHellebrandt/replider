package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockStageRepo is a testify mock for domain.FunnelStageRepository, scoped
// to the board-move tests.
type mockStageRepo struct{ mock.Mock }

func (m *mockStageRepo) ListByFunnel(ctx context.Context, accountID, funnelID string) ([]*domain.FunnelStage, error) {
	args := m.Called(ctx, accountID, funnelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunnelStage), args.Error(1)
}

func (m *mockStageRepo) GetByID(ctx context.Context, accountID, id string) (*domain.FunnelStage, error) {
	args := m.Called(ctx, accountID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FunnelStage), args.Error(1)
}

func (m *mockStageRepo) Create(ctx context.Context, stage *domain.FunnelStage) (*domain.FunnelStage, error) {
	args := m.Called(ctx, stage)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FunnelStage), args.Error(1)
}

func (m *mockStageRepo) Update(ctx context.Context, stage *domain.FunnelStage) (*domain.FunnelStage, error) {
	args := m.Called(ctx, stage)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FunnelStage), args.Error(1)
}

func (m *mockStageRepo) Delete(ctx context.Context, accountID, id string) error {
	return m.Called(ctx, accountID, id).Error(0)
}

func (m *mockStageRepo) EnsureDefaults(ctx context.Context, accountID, funnelID string) ([]*domain.FunnelStage, error) {
	args := m.Called(ctx, accountID, funnelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunnelStage), args.Error(1)
}

func (m *mockStageRepo) MoveLead(ctx context.Context, accountID, leadID, stageID string, status domain.LeadStatus, boardPos *float64, touch bool) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, leadID, stageID, status, boardPos, touch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockStageRepo) BoardNeighborPositions(ctx context.Context, accountID, stageID, beforeLeadID, afterLeadID string) (*float64, *float64, *float64, error) {
	args := m.Called(ctx, accountID, stageID, beforeLeadID, afterLeadID)
	get := func(i int) *float64 {
		if args.Get(i) == nil {
			return nil
		}
		return args.Get(i).(*float64)
	}
	return get(0), get(1), get(2), args.Error(3)
}

func (m *mockStageRepo) ResetBoardOrder(ctx context.Context, accountID, funnelID, stageID string) error {
	return m.Called(ctx, accountID, funnelID, stageID).Error(0)
}

func (m *mockStageRepo) SeedBoardOrder(ctx context.Context, accountID, stageID string) error {
	return m.Called(ctx, accountID, stageID).Error(0)
}

func posPtr(v float64) *float64 { return &v }

func moveFixture(t *testing.T, lead *domain.Lead, stage *domain.FunnelStage) (*LeadUseCase, *mockLeadRepo, *mockStageRepo, *MockAuditLogRepo) {
	t.Helper()
	leadRepo := new(mockLeadRepo)
	stageRepo := new(mockStageRepo)
	auditRepo := new(MockAuditLogRepo)
	uc := NewLeadUseCase(leadRepo, new(MockSalesFunnelRepo))
	uc.SetStageRepo(stageRepo)
	uc.SetAuditRepo(auditRepo)
	leadRepo.On("GetByID", mock.Anything, lead.ID).Return(lead, nil)
	stageRepo.On("GetByID", mock.Anything, lead.AccountID, stage.ID).Return(stage, nil)
	return uc, leadRepo, stageRepo, auditRepo
}

// A same-stage move is a REORDER: it must skip the backward-move guard, keep
// the canonical status (even one the target column does not map to), leave
// updated_at alone (touch=false) and write no audit row — reordering inside
// "Ganhos" is not winning the deal twice.
func TestLeadUseCase_MoveLead_SameStageReordersWithoutLifecycle(t *testing.T) {
	stageID := "st-1"
	lead := &domain.Lead{
		ID: "lead-1", AccountID: "acc-1", FunnelID: "fn-1",
		Status: domain.LeadStatusContacted, StageID: &stageID,
	}
	// The column deliberately maps to an EARLY status: a cross-stage move
	// here would trip IsBackwardMove, so a green run proves the guard was
	// skipped, not merely not triggered.
	stage := &domain.FunnelStage{
		ID: stageID, AccountID: "acc-1", FunnelID: "fn-1",
		MapsToStatus: domain.LeadStatusNew,
	}
	uc, _, stageRepo, auditRepo := moveFixture(t, lead, stage)

	stageRepo.On("BoardNeighborPositions", mock.Anything, "acc-1", stageID, "lead-b", "lead-c").
		Return(posPtr(1024), posPtr(2048), posPtr(4096), nil)
	stageRepo.On("MoveLead", mock.Anything, "acc-1", "lead-1", stageID,
		domain.LeadStatusContacted, posPtr(1536), false).
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusContacted}, nil)

	actor := domain.Actor{UserID: "u-1", AccountID: "acc-1", Role: domain.RoleGestor}
	moved, err := uc.MoveLead(context.Background(), actor, "lead-1", stageID, "lead-b", "lead-c")
	require.NoError(t, err)
	assert.Equal(t, domain.LeadStatusContacted, moved.Status, "a reorder never rewrites the pipeline")
	stageRepo.AssertExpectations(t)
	auditRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// A stage-less lead sitting in its RESOLVED column (status maps to it) gets
// the same reorder semantics: pin the stage, keep the status.
func TestLeadUseCase_MoveLead_ResolvedColumnReorderCountsAsSameStage(t *testing.T) {
	lead := &domain.Lead{
		ID: "lead-1", AccountID: "acc-1", FunnelID: "fn-1",
		Status: domain.LeadStatusWon, StageID: nil,
	}
	stage := &domain.FunnelStage{
		ID: "st-won", AccountID: "acc-1", FunnelID: "fn-1",
		MapsToStatus: domain.LeadStatusWon, IsTerminal: true,
	}
	uc, _, stageRepo, auditRepo := moveFixture(t, lead, stage)

	stageRepo.On("BoardNeighborPositions", mock.Anything, "acc-1", "st-won", "lead-b", "").
		Return(posPtr(512), nil, posPtr(512), nil)
	stageRepo.On("MoveLead", mock.Anything, "acc-1", "lead-1", "st-won",
		domain.LeadStatusWon, posPtr(512+float64(domain.BoardPositionStep)), false).
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusWon}, nil)

	actor := domain.Actor{UserID: "u-1", AccountID: "acc-1", Role: domain.RoleGestor}
	_, err := uc.MoveLead(context.Background(), actor, "lead-1", "st-won", "lead-b", "")
	require.NoError(t, err)
	stageRepo.AssertExpectations(t)
	// A terminal-column entry is normally audited; a reorder inside it must not be.
	auditRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// Cross-stage moves keep their guards: the backward rule still refuses, and
// a plain drop (no neighbors) clears the manual position and bumps
// updated_at (touch=true).
func TestLeadUseCase_MoveLead_CrossStageKeepsGuardAndClearsPos(t *testing.T) {
	fromID := "st-late"
	lead := &domain.Lead{
		ID: "lead-1", AccountID: "acc-1", FunnelID: "fn-1",
		Status: domain.LeadStatusContacted, StageID: &fromID,
	}
	early := &domain.FunnelStage{
		ID: "st-early", AccountID: "acc-1", FunnelID: "fn-1",
		MapsToStatus: domain.LeadStatusNew,
	}
	uc, _, stageRepo, _ := moveFixture(t, lead, early)

	actor := domain.Actor{UserID: "u-1", AccountID: "acc-1", Role: domain.RoleGestor}
	_, err := uc.MoveLead(context.Background(), actor, "lead-1", "st-early", "", "")
	assert.ErrorIs(t, err, domain.ErrStageBackward)
	stageRepo.AssertNotCalled(t, "MoveLead",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// Forward, without neighbors: status advances, pos clears, clock bumps.
	forward := &domain.FunnelStage{
		ID: "st-resp", AccountID: "acc-1", FunnelID: "fn-1",
		MapsToStatus: domain.LeadStatusResponded,
	}
	uc2, _, stageRepo2, _ := moveFixture(t, lead, forward)
	stageRepo2.On("MoveLead", mock.Anything, "acc-1", "lead-1", "st-resp",
		domain.LeadStatusResponded, (*float64)(nil), true).
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusResponded}, nil)

	_, err = uc2.MoveLead(context.Background(), actor, "lead-1", "st-resp", "", "")
	require.NoError(t, err)
	stageRepo2.AssertExpectations(t)
	stageRepo2.AssertNotCalled(t, "BoardNeighborPositions",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The reset is stage-scoped and refuses a stage of another funnel — the same
// no-cross-funnel rule every board write obeys.
func TestLeadUseCase_ResetBoardOrder_ChecksStageOwnership(t *testing.T) {
	stageRepo := new(mockStageRepo)
	uc := NewLeadUseCase(new(mockLeadRepo), new(MockSalesFunnelRepo))
	uc.SetStageRepo(stageRepo)
	actor := domain.Actor{UserID: "u-1", AccountID: "acc-1", Role: domain.RoleGestor}

	stageRepo.On("GetByID", mock.Anything, "acc-1", "st-1").
		Return(&domain.FunnelStage{ID: "st-1", AccountID: "acc-1", FunnelID: "fn-other"}, nil)
	err := uc.ResetBoardOrder(context.Background(), actor, "fn-1", "st-1")
	assert.ErrorIs(t, err, domain.ErrStageWrongFunnel)
	stageRepo.AssertNotCalled(t, "ResetBoardOrder", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	stageRepo2 := new(mockStageRepo)
	uc2 := NewLeadUseCase(new(mockLeadRepo), new(MockSalesFunnelRepo))
	uc2.SetStageRepo(stageRepo2)
	stageRepo2.On("GetByID", mock.Anything, "acc-1", "st-1").
		Return(&domain.FunnelStage{ID: "st-1", AccountID: "acc-1", FunnelID: "fn-1"}, nil)
	stageRepo2.On("ResetBoardOrder", mock.Anything, "acc-1", "fn-1", "st-1").Return(nil)
	require.NoError(t, uc2.ResetBoardOrder(context.Background(), actor, "fn-1", "st-1"))
	stageRepo2.AssertExpectations(t)
}

// The bug the seeding exists for: the client names a neighbor the column has
// never placed. Treating it as "no neighbor" made the dropped card the only
// placed one — and the placed-first rule threw it to the TOP, wherever the
// slot was. Naming an unplaced neighbor must seed the column and resolve
// again.
func TestLeadUseCase_MoveLead_UnplacedNeighborSeedsTheColumnFirst(t *testing.T) {
	stageID := "st-1"
	lead := &domain.Lead{
		ID: "lead-1", AccountID: "acc-1", FunnelID: "fn-1",
		Status: domain.LeadStatusProspected, StageID: &stageID,
	}
	stage := &domain.FunnelStage{
		ID: stageID, AccountID: "acc-1", FunnelID: "fn-1",
		MapsToStatus: domain.LeadStatusProspected,
	}
	uc, _, stageRepo, _ := moveFixture(t, lead, stage)

	// First resolution: the named neighbors exist but carry no position.
	stageRepo.On("BoardNeighborPositions", mock.Anything, "acc-1", stageID, "lead-b", "lead-c").
		Return(nil, nil, nil, nil).Once()
	stageRepo.On("SeedBoardOrder", mock.Anything, "acc-1", stageID).Return(nil).Once()
	// After the seed the same neighbors have concrete positions.
	stageRepo.On("BoardNeighborPositions", mock.Anything, "acc-1", stageID, "lead-b", "lead-c").
		Return(posPtr(1024), posPtr(2048), posPtr(3072), nil).Once()
	stageRepo.On("MoveLead", mock.Anything, "acc-1", "lead-1", stageID,
		domain.LeadStatusProspected, posPtr(1536), false).
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusProspected}, nil)

	actor := domain.Actor{UserID: "u-1", AccountID: "acc-1", Role: domain.RoleGestor}
	_, err := uc.MoveLead(context.Background(), actor, "lead-1", stageID, "lead-b", "lead-c")
	require.NoError(t, err)
	stageRepo.AssertExpectations(t)
}

// A plain cross-column drop (no neighbors named) still seeds nothing.
func TestLeadUseCase_MoveLead_NoNeighborsNeverSeeds(t *testing.T) {
	lead := &domain.Lead{
		ID: "lead-1", AccountID: "acc-1", FunnelID: "fn-1",
		Status: domain.LeadStatusNew,
	}
	stage := &domain.FunnelStage{
		ID: "st-2", AccountID: "acc-1", FunnelID: "fn-1",
		MapsToStatus: domain.LeadStatusProspected,
	}
	uc, _, stageRepo, auditRepo := moveFixture(t, lead, stage)
	auditRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	stageRepo.On("MoveLead", mock.Anything, "acc-1", "lead-1", "st-2",
		domain.LeadStatusProspected, (*float64)(nil), true).
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusProspected}, nil)

	actor := domain.Actor{UserID: "u-1", AccountID: "acc-1", Role: domain.RoleGestor}
	_, err := uc.MoveLead(context.Background(), actor, "lead-1", "st-2", "", "")
	require.NoError(t, err)
	stageRepo.AssertNotCalled(t, "SeedBoardOrder", mock.Anything, mock.Anything, mock.Anything)
}
