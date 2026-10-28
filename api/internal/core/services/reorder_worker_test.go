package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This worker writes into the approval queue. Nothing it does reaches a
// customer without a human — which is the whole design, and what these tests
// pin.

type fakeReorderSource struct {
	due      []*domain.ReorderCandidate
	dueErr   error
	nudged   []string
	nudgeErr error
	horizon  time.Time
}

func (f *fakeReorderSource) DueForReorder(_ context.Context, within time.Time, _ int) ([]*domain.ReorderCandidate, error) {
	f.horizon = within
	return f.due, f.dueErr
}

func (f *fakeReorderSource) RecordNudge(_ context.Context, _, _, _, saleID, productKey string, _ time.Time) error {
	if f.nudgeErr != nil {
		return f.nudgeErr
	}
	key := saleID
	if productKey != "" {
		key = saleID + "|" + productKey
	}
	f.nudged = append(f.nudged, key)
	return nil
}

type fakeDrafter struct {
	queued []string
	texts  []string
	err    error
}

func (f *fakeDrafter) QueueReorderApproach(_ context.Context, _, contactID, text string) error {
	if f.err != nil {
		return f.err
	}
	f.queued = append(f.queued, contactID)
	f.texts = append(f.texts, text)
	return nil
}

var reorderNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func reorderCandidate(saleID string, dueIn time.Duration) *domain.ReorderCandidate {
	return &domain.ReorderCandidate{
		AccountID: "acc-1", ContactID: "c-" + saleID,
		ContactName: "Clínica Bem Viver", SaleID: saleID,
		Items: []domain.SaleItem{
			{Description: "Luvas de procedimento cx 100", Quantity: 200},
		},
		LastSoldAt: reorderNow.Add(-30 * 24 * time.Hour),
		DueAt:      reorderNow.Add(dueIn),
		GrossCents: 450000,
	}
}

func runReorder(source *fakeReorderSource, drafter *fakeDrafter) {
	NewReorderWorker(source, drafter).
		WithClock(func() time.Time { return reorderNow }).
		RunOnce(context.Background())
}

func TestReorder_QueuesASuggestionForApproval(t *testing.T) {
	source := &fakeReorderSource{due: []*domain.ReorderCandidate{
		reorderCandidate("s1", 3*24*time.Hour),
	}}
	drafter := &fakeDrafter{}

	runReorder(source, drafter)

	require.Len(t, drafter.queued, 1)
	assert.Equal(t, []string{"s1"}, source.nudged)
	// Names what they bought: a reminder that says "o material" is one the
	// customer has to decode.
	assert.Contains(t, drafter.texts[0], "Luvas de procedimento cx 100")
}

// The horizon includes what is ALREADY overdue: a customer who ran out last
// week is the most expensive state in the model, not one to skip because the
// window passed.
func TestReorder_AsksAheadByTheLeadTime(t *testing.T) {
	source := &fakeReorderSource{}

	runReorder(source, &fakeDrafter{})

	assert.Equal(t, reorderNow.Add(domain.ReorderLeadTimeDays*24*time.Hour),
		source.horizon)
}

// A worker that runs every six hours must not turn a helpful reminder into a
// week of daily ones.
func TestReorder_MarksWhatItAsked(t *testing.T) {
	source := &fakeReorderSource{due: []*domain.ReorderCandidate{
		reorderCandidate("s1", 24*time.Hour),
		reorderCandidate("s2", 48*time.Hour),
	}}

	runReorder(source, &fakeDrafter{})

	assert.Equal(t, []string{"s1", "s2"}, source.nudged)
}

// A nudge marked as sent that never reached the queue is a customer nobody
// ever asks.
func TestReorder_DoesNotMarkWhatItCouldNotQueue(t *testing.T) {
	source := &fakeReorderSource{due: []*domain.ReorderCandidate{
		reorderCandidate("s1", 24*time.Hour),
	}}
	drafter := &fakeDrafter{err: errors.New("no funnel")}

	runReorder(source, drafter)

	assert.Empty(t, source.nudged)
}

// One customer failing is a fact about that customer — a missing funnel, an
// opted-out contact — and must not cost the batch.
func TestReorder_AFailedMarkDoesNotStopTheRest(t *testing.T) {
	source := &fakeReorderSource{
		due: []*domain.ReorderCandidate{
			reorderCandidate("s1", 24*time.Hour),
			reorderCandidate("s2", 48*time.Hour),
		},
		nudgeErr: errors.New("db down"),
	}
	drafter := &fakeDrafter{}

	runReorder(source, drafter)

	assert.Len(t, drafter.queued, 2)
}

func TestReorder_ReadFailureIsNotFatal(t *testing.T) {
	source := &fakeReorderSource{dueErr: errors.New("db down")}
	drafter := &fakeDrafter{}

	require.NotPanics(t, func() { runReorder(source, drafter) })
	assert.Empty(t, drafter.queued)
}

func TestReorder_UnwiredIsANoOp(t *testing.T) {
	require.NotPanics(t, func() {
		NewReorderWorker(nil, nil).RunOnce(context.Background())
	})
}

// ── The message ───────────────────────────────────────────────────────────

// Saying so plainly beats pretending to have noticed on time, and invites a
// correction if the prediction was wrong.
func TestReorderMessage_OverdueSaysSo(t *testing.T) {
	text := ReorderMessage(reorderCandidate("s1", -5*24*time.Hour), reorderNow)

	assert.Contains(t, text, "deve ter acabado")
}

func TestReorderMessage_AheadOfTimeGivesTheCountdown(t *testing.T) {
	text := ReorderMessage(reorderCandidate("s1", 5*24*time.Hour), reorderNow)

	assert.Contains(t, text, "5 dias")
}

// Vague is better than inventing a product name the customer never ordered.
func TestReorderMessage_NoItemsStaysVagueRatherThanWrong(t *testing.T) {
	c := reorderCandidate("s1", 3*24*time.Hour)
	c.Items = nil

	text := ReorderMessage(c, reorderNow)

	assert.Contains(t, text, "material da última compra")
}

// Listing six items turns a short message into a packing slip nobody reads.
func TestReorderMessage_ManyItemsBecomeOneNameAndACount(t *testing.T) {
	c := reorderCandidate("s1", 3*24*time.Hour)
	c.Items = append(c.Items,
		domain.SaleItem{Description: "Seringa 5ml"},
		domain.SaleItem{Description: "Gaze estéril"})

	text := ReorderMessage(c, reorderNow)

	assert.Contains(t, text, "Luvas de procedimento cx 100 (e mais 2 itens)")
	assert.NotContains(t, text, "Seringa")
}

// It offers the same quantity rather than asking what they want: the whole
// point is that reordering takes one word.
func TestReorderMessage_OffersTheSameOrder(t *testing.T) {
	text := ReorderMessage(reorderCandidate("s1", 3*24*time.Hour), reorderNow)

	assert.Contains(t, text, "mesma quantidade da última compra")
}

// One sale, two cycled products: BOTH predictions are offered and marked —
// the old per-sale key silenced the second one forever (migration 000224).
func TestReorder_TwoProductsOfOneSaleAreTwoNudges(t *testing.T) {
	due := time.Now().Add(24 * time.Hour)
	source := &fakeReorderSource{due: []*domain.ReorderCandidate{
		{AccountID: "acc1", ContactID: "c1", SaleID: "s1", ProductKey: "prod-luvas", DueAt: due},
		{AccountID: "acc1", ContactID: "c1", SaleID: "s1", ProductKey: "prod-gaze", DueAt: due},
	}}
	drafter := &fakeDrafter{}
	NewReorderWorker(source, drafter).RunOnce(context.Background())

	assert.Len(t, drafter.queued, 2)
	assert.Equal(t, []string{"s1|prod-luvas", "s1|prod-gaze"}, source.nudged)
}
