package mcp

import (
	"context"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRelationshipRepo struct{ saved []*domain.RelationshipDate }

func (s *stubRelationshipRepo) List(context.Context, string) ([]*domain.RelationshipDate, error) {
	return s.saved, nil
}
func (s *stubRelationshipRepo) Upsert(_ context.Context, d *domain.RelationshipDate) error {
	s.saved = append(s.saved, d)
	return nil
}
func (s *stubRelationshipRepo) Delete(context.Context, string, string) error { return nil }
func (s *stubRelationshipRepo) DueDates(context.Context, time.Time, int) ([]*domain.RelationshipDate, error) {
	return nil, nil
}
func (s *stubRelationshipRepo) MarkFired(context.Context, string, time.Time) error { return nil }
func (s *stubRelationshipRepo) DueForCheckIn(context.Context, time.Time, int) ([]*domain.CheckInCandidate, error) {
	return nil, nil
}
func (s *stubRelationshipRepo) RecordCheckIn(context.Context, string, string, string, time.Time) error {
	return nil
}

type stubCrossSellRepo struct{}

func (stubCrossSellRepo) Pairs(context.Context, string) ([]domain.CoPurchasePair, error) {
	return []domain.CoPurchasePair{
		{Product: "luva", Partner: "mascara", Customers: 8, ProductCustomers: 10},
	}, nil
}
func (stubCrossSellRepo) ItemsBoughtBy(context.Context, string, string) ([]string, error) {
	return []string{"luva"}, nil
}

func connectWith(t *testing.T, deps Deps, role string) *mcpsdk.ClientSession {
	t.Helper()
	server := buildServer(deps, role)
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func relationshipDeps() Deps {
	return Deps{
		RelationshipUC: usecases.NewRelationshipUseCase(&stubRelationshipRepo{}),
		CrossSellUC:    usecases.NewCrossSellUseCase(stubCrossSellRepo{}),
	}
}

// The two tools the PRD names, on the surface a salesperson reaches from a
// phone.
func TestMCPServer_RelationshipToolsAreListed(t *testing.T) {
	names := listToolNames(t, connectWith(t, relationshipDeps(), "attendant"))

	for _, tool := range []string{
		"schedule_greeting", "list_relationship_dates", "suggest_cross_sell",
	} {
		assert.True(t, names[tool], "tools/list missing %s", tool)
	}
}

// A deployment that has not wired the feature must not advertise it — a tool
// that always errors is worse than one that is absent.
func TestMCPServer_RelationshipToolsAbsentWhenUnwired(t *testing.T) {
	names := listToolNames(t, connect(t, "admin"))

	for _, tool := range []string{"schedule_greeting", "suggest_cross_sell"} {
		assert.False(t, names[tool], "tools/list must not offer an unwired %s", tool)
	}
}
