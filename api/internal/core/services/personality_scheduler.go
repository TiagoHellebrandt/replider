package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// PersonalityLearner is the minimal contract we need from the learning
// use case. Defined here (instead of importing usecases) so this package
// stays free of upward dependencies.
type PersonalityLearner interface {
	Execute(ctx context.Context, agentID, accountID string) (*domain.Agent, error)
}

// PersonalityScheduler triggers personality learning after every N human
// replies on an agent's conversations. Previously the admin had to click a
// "learn" button in settings — this keeps the bot's voice tracking how the
// team actually talks without any manual step.
//
// The counter is in-memory on purpose: precision isn't important
// (learning is idempotent), the worst case on restart is we re-hit the
// threshold a little sooner, and persisting it would require a new table.
type PersonalityScheduler struct {
	learner   PersonalityLearner
	threshold int           // human replies between learn runs
	cooldown  time.Duration // minimum gap between two runs on the same agent

	mu        sync.Mutex
	counters  map[string]int       // agentID -> pending human replies
	lastRunAt map[string]time.Time // agentID -> last successful run
}

// NewPersonalityScheduler wires the scheduler. threshold 20 + cooldown 1h
// is conservative for SMB volumes: a busy team hits it daily, a slow one
// weekly — never on the same minute.
func NewPersonalityScheduler(learner PersonalityLearner) *PersonalityScheduler {
	return &PersonalityScheduler{
		learner:   learner,
		threshold: 20,
		cooldown:  time.Hour,
		counters:  make(map[string]int),
		lastRunAt: make(map[string]time.Time),
	}
}

// OnHumanReply is the hook called after a human attendant ships a reply.
// Bumps the per-agent counter and fires a learn job in the background
// when the threshold is crossed and cooldown is satisfied. Safe to call
// from any goroutine.
func (s *PersonalityScheduler) OnHumanReply(agentID, accountID string) {
	if agentID == "" || accountID == "" {
		return
	}
	if !s.bump(agentID) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := s.learner.Execute(ctx, agentID, accountID); err != nil {
		slog.Warn("auto personality learning failed", "agent_id", agentID, "error", err)
		return
	}
	slog.Info("auto personality learning succeeded", "agent_id", agentID)
}

// bump returns true when the caller should run the learner now. It
// atomically resets the counter and stamps the last-run time; concurrent
// replies that also cross the threshold will simply be deduped by the
// cooldown check on the next cycle.
func (s *PersonalityScheduler) bump(agentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[agentID]++
	if s.counters[agentID] < s.threshold {
		return false
	}
	if time.Since(s.lastRunAt[agentID]) < s.cooldown {
		return false
	}
	s.counters[agentID] = 0
	s.lastRunAt[agentID] = time.Now()
	return true
}
