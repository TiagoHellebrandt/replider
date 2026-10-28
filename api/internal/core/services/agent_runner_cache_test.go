package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type stubExpRepo struct {
	calls atomic.Int32
	exp   *domain.PromptExperiment
}

func (s *stubExpRepo) Create(context.Context, *domain.PromptExperiment) error { return nil }
func (s *stubExpRepo) GetByID(context.Context, string) (*domain.PromptExperiment, error) {
	return nil, nil
}
func (s *stubExpRepo) GetRunningByAgentID(context.Context, string) (*domain.PromptExperiment, error) {
	s.calls.Add(1)
	return s.exp, nil
}
func (s *stubExpRepo) ListByAccountID(context.Context, string, int, int) ([]*domain.PromptExperiment, error) {
	return nil, nil
}
func (s *stubExpRepo) ListByAgentID(context.Context, string, int, int) ([]*domain.PromptExperiment, error) {
	return nil, nil
}
func (s *stubExpRepo) UpdateStatus(context.Context, string, string, *time.Time, *time.Time) error {
	return nil
}
func (s *stubExpRepo) SetWinner(context.Context, string, *string) error { return nil }
func (s *stubExpRepo) Delete(context.Context, string) error             { return nil }
func (s *stubExpRepo) MetricsForVariant(context.Context, string, string) (*domain.VariantMetrics, error) {
	return nil, nil
}
func (s *stubExpRepo) SampleConversations(context.Context, string, string, int, int) ([]domain.ExperimentSample, []domain.ExperimentSample, error) {
	return nil, nil, nil
}

func TestAgentRunner_ExperimentCache_HitsRepoOnce(t *testing.T) {
	stub := &stubExpRepo{exp: nil}
	r := &AgentRunner{experimentRepo: stub}

	for i := 0; i < 10; i++ {
		got := r.lookupRunningExperiment(context.Background(), "agent-x")
		assert.Nil(t, got)
	}
	assert.Equal(t, int32(1), stub.calls.Load(), "cache should collapse repeat lookups")
}

func TestAgentRunner_ExperimentCache_InvalidateForcesRefetch(t *testing.T) {
	stub := &stubExpRepo{exp: nil}
	r := &AgentRunner{experimentRepo: stub}

	_ = r.lookupRunningExperiment(context.Background(), "agent-y")
	r.InvalidateExperimentCache("agent-y")
	_ = r.lookupRunningExperiment(context.Background(), "agent-y")

	assert.Equal(t, int32(2), stub.calls.Load())
}

func TestAgentRunner_ExperimentCache_PerAgent(t *testing.T) {
	stub := &stubExpRepo{exp: nil}
	r := &AgentRunner{experimentRepo: stub}

	_ = r.lookupRunningExperiment(context.Background(), "a")
	_ = r.lookupRunningExperiment(context.Background(), "b")
	_ = r.lookupRunningExperiment(context.Background(), "a")
	_ = r.lookupRunningExperiment(context.Background(), "b")

	assert.Equal(t, int32(2), stub.calls.Load())
}
