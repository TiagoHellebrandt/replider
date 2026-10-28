package usecases

import (
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Lifecycle webhook emission (§6.19, J2).
//
// One narrow port and one helper, shared by every use case that has a lifecycle
// moment worth telling an integration about. The alternative — each use case
// holding the dispatcher and assembling its own payload — is five slightly
// different payload shapes and five places to forget the account id.

// LifecycleEmitter is the slice of the outbound dispatcher these use cases need.
//
// Narrow on purpose: a use case that can Emit but not configure or test webhooks
// cannot accidentally grow into a second webhook administration surface.
type LifecycleEmitter interface {
	Emit(accountID, event string, payload any)
}

// emitLifecycle sends one lifecycle event, doing nothing when unwired.
//
// Fire-and-forget by construction (the dispatcher spawns its own goroutine), and
// deliberately without an error return: a webhook nobody is listening to must
// never be able to fail the sale that produced it.
func emitLifecycle(
	emitter LifecycleEmitter, accountID, event string, payload domain.LifecyclePayload,
) {
	if emitter == nil || accountID == "" {
		return
	}
	if payload.OccurredAt == "" {
		// When it HAPPENED, not when it was delivered: a retry an hour later
		// must not look like a second event to whoever is deduplicating.
		payload.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	emitter.Emit(accountID, event, payload)
}
