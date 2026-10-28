package domain

import (
	"errors"
	"strings"
	"testing"
)

// The lifecycle event catalogue (J2).
//
// The failure this guards against is the hardest integration bug to notice: a
// webhook that is configured, looks healthy, and never fires because a name
// disagrees by one character between the subscription and the emit site.

func TestWebhookCatalogue_KeepsThePreExistingEvents(t *testing.T) {
	// These have been subscribable since the first webhook shipped. Dropping one
	// from the catalogue would silently break every live subscription that names
	// it — the customer's endpoint simply stops receiving.
	for _, name := range []string{
		"message.received", "message.sent",
		"conversation.created", "conversation.assigned", "conversation.closed",
		"whatsapp.quality_changed", "whatsapp.template_status_changed",
	} {
		if !ValidWebhookEvent(name) {
			t.Fatalf("%q dropped out of the catalogue", name)
		}
	}
}

func TestWebhookCatalogue_CarriesTheLifecycleEvents(t *testing.T) {
	for _, name := range []string{
		EventLeadWon, EventSaleRegistered, EventContractSigned,
		EventReorderPredicted, EventCommissionClosed,
	} {
		if !ValidWebhookEvent(name) {
			t.Fatalf("%q is emitted but not subscribable", name)
		}
	}
}

func TestWebhookCatalogue_EveryEventExplainsItself(t *testing.T) {
	for _, info := range WebhookEventCatalogue() {
		if strings.TrimSpace(info.Description) == "" {
			// Somebody wiring an integration reads this list to decide what to
			// subscribe to; a bare name is not enough to decide.
			t.Fatalf("%q has no description", info.Name)
		}
	}
}

func TestWebhookCatalogue_HasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, info := range WebhookEventCatalogue() {
		if seen[info.Name] {
			t.Fatalf("%q listed twice", info.Name)
		}
		seen[info.Name] = true
	}
}

func TestValidateWebhookEvents_NamesTheOffender(t *testing.T) {
	err := ValidateWebhookEvents([]string{"sale.registered", "sale.registred"})

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("err = %v, want an AppError", err)
	}
	// "evento inválido" with no name makes somebody compare two lists by eye.
	if !strings.Contains(appErr.Message, "sale.registred") {
		t.Fatalf("message %q does not name the typo", appErr.Message)
	}
}

func TestValidateWebhookEvents_RefusesAnEmptySubscription(t *testing.T) {
	// A subscription to nothing presents as "the webhook never fires", which is
	// exactly the bug the catalogue exists to prevent.
	if err := ValidateWebhookEvents(nil); err == nil {
		t.Fatal("a subscription to no events must be refused")
	}
}

func TestValidWebhookEvent_ToleratesSurroundingSpace(t *testing.T) {
	// A name pasted out of documentation carries whitespace, and refusing it
	// would be a support thread about an invisible character.
	if !ValidWebhookEvent("  sale.registered  ") {
		t.Fatal("a padded name must still validate")
	}
	if ValidWebhookEvent("sale.registered.v2") {
		t.Fatal("a near-miss must not validate")
	}
}
