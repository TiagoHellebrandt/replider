package services

import (
	"context"
	"sync"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// raceAssignmentRepo models Postgres' atomic upsert-and-read: the first
// writer for a key commits its variant; every later AssignReturning for
// the same key reads back THAT committed variant, regardless of what it
// proposed. This is exactly the contract the real
// `INSERT ... ON CONFLICT DO UPDATE ... RETURNING variant` guarantees,
// so a passing test here proves the call site converges under
// concurrency. Assign/Get are kept for interface completeness.
type raceAssignmentRepo struct {
	mu      sync.Mutex
	store   map[string]string // key -> committed variant
	assigns int
}

func newRaceAssignmentRepo() *raceAssignmentRepo {
	return &raceAssignmentRepo{store: map[string]string{}}
}

func keyOf(experimentID, conversationID string) string {
	return experimentID + "|" + conversationID
}

func (r *raceAssignmentRepo) AssignReturning(_ context.Context, a *domain.ExperimentAssignment) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assigns++
	k := keyOf(a.ExperimentID, a.ConversationID)
	if v, ok := r.store[k]; ok {
		return v, nil // a concurrent writer already committed — honor it
	}
	r.store[k] = a.Variant
	return a.Variant, nil
}

func (r *raceAssignmentRepo) Assign(_ context.Context, a *domain.ExperimentAssignment) error {
	_, err := r.AssignReturning(context.Background(), a)
	return err
}

func (r *raceAssignmentRepo) Get(_ context.Context, experimentID, conversationID string) (*domain.ExperimentAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.store[keyOf(experimentID, conversationID)]; ok {
		return &domain.ExperimentAssignment{ExperimentID: experimentID, ConversationID: conversationID, Variant: v}, nil
	}
	return nil, nil
}

func (r *raceAssignmentRepo) GetByConversation(_ context.Context, conversationID string) (*domain.ExperimentAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range r.store {
		if len(k) > len(conversationID) && k[len(k)-len(conversationID):] == conversationID {
			return &domain.ExperimentAssignment{ConversationID: conversationID, Variant: v}, nil
		}
	}
	return nil, nil
}

var _ domain.ExperimentAssignmentRepository = (*raceAssignmentRepo)(nil)

// TestAgentRunner_ABAssignment_RaceConsistent fires 100 concurrent
// resolutions for the SAME fresh conversation and asserts they all land
// on one variant. The pre-fix code (Assign DO NOTHING + separate Get)
// had a window where a reader could observe a different variant than the
// committed one; AssignReturning closes it.
func TestAgentRunner_ABAssignment_RaceConsistent(t *testing.T) {
	exp := &domain.PromptExperiment{
		ID:           "exp-1",
		AccountID:    "acc-1",
		AgentID:      "agent-1",
		Status:       domain.ExperimentStatusRunning,
		VariantABody: "BODY-A",
		VariantBBody: "BODY-B",
		SplitPctB:    50,
	}
	assignRepo := newRaceAssignmentRepo()
	r := &AgentRunner{
		experimentRepo: &stubExpRepo{exp: exp},
		assignmentRepo: assignRepo,
	}
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1"}

	const n = 100
	results := make([]string, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // line everyone up to maximize contention
			_, variant, expID := r.resolveSystemPromptBody(context.Background(), agent, "conv-1")
			if assert.NotNil(t, variant) {
				results[idx] = *variant
			}
			assert.NotNil(t, expID)
		}(i)
	}
	close(start)
	wg.Wait()

	first := results[0]
	assert.Contains(t, []string{domain.VariantA, domain.VariantB}, first)
	for i, v := range results {
		assert.Equalf(t, first, v, "goroutine %d flipped variant under race", i)
	}
	// Exactly one variant must be committed for the conversation.
	committed, _ := assignRepo.Get(context.Background(), exp.ID, "conv-1")
	if assert.NotNil(t, committed) {
		assert.Equal(t, first, committed.Variant)
	}
}
