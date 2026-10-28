package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A shared commercial mailbox holds, in one morning: a lead answering a cold
// email, a supplier's invoice, three newsletters and a phishing attempt.
// Chronological order buries the first under the last three.

func TestClassifyInbox_QuarantineOutranksEverything(t *testing.T) {
	// Dressed as an ordinary invoice, which is exactly how invoice fraud
	// works — filing it as "financial" IS the fraud succeeding.
	c, ok := ClassifyInbox(InboxSignals{
		Quarantined:           true,
		HasPaymentInstruction: true,
		HasSale:               true,
		LeadStatus:            LeadStatusResponded,
		LastWasInbound:        true,
	})

	assert.True(t, ok)
	assert.Equal(t, CategoryQuarantine, c)
}

// The header is the honest signal. A newsletter can say "sua fatura chegou"
// and can come from a company we buy from, so it is checked before both.
func TestClassifyInbox_ListUnsubscribeMeansNewsletter(t *testing.T) {
	c, ok := ClassifyInbox(InboxSignals{
		ListUnsubscribe:       true,
		HasPaymentInstruction: true,
		Subject:               "Sua fatura de janeiro",
	})

	assert.True(t, ok)
	assert.Equal(t, CategoryNewsletter, c)
}

func TestClassifyInbox_PaymentInstructionMeansFinancial(t *testing.T) {
	c, ok := ClassifyInbox(InboxSignals{
		HasPaymentInstruction: true,
		Body:                  "Segue o boleto em anexo.",
		LastWasInbound:        true,
	})

	assert.True(t, ok)
	assert.Equal(t, CategoryFinancial, c)
}

// The event the product exists to surface, and the one a chronological inbox
// buries.
func TestClassifyInbox_ARespondingLeadIsHot(t *testing.T) {
	for _, status := range []LeadStatus{LeadStatusResponded, LeadStatusQualified} {
		c, ok := ClassifyInbox(InboxSignals{
			LeadStatus: status, LastWasInbound: true,
		})
		assert.True(t, ok)
		assert.Equal(t, CategoryHotLead, c, "status %q", status)
	}
}

// A lead we merely WROTE to is not hot. Otherwise every cold email we send
// would climb to the top of our own inbox.
func TestClassifyInbox_AContactedLeadIsNotHot(t *testing.T) {
	c, _ := ClassifyInbox(InboxSignals{
		LeadStatus: LeadStatusContacted, LastWasInbound: true,
	})

	assert.NotEqual(t, CategoryHotLead, c)
}

// A sale is the one fact that separates a customer from a prospect.
func TestClassifyInbox_AContactWhoBoughtIsACustomer(t *testing.T) {
	c, ok := ClassifyInbox(InboxSignals{HasSale: true, LastWasInbound: true})

	assert.True(t, ok)
	assert.Equal(t, CategoryCustomer, c)
}

// A customer writing "quero cancelar minha assinatura" is not a newsletter.
// The prose fallback only applies to mail we did not receive as a reply.
func TestClassifyInbox_ACustomerAskingToCancelIsNotANewsletter(t *testing.T) {
	c, _ := ClassifyInbox(InboxSignals{
		Body:           "Preciso cancelar inscrição no plano, como faço?",
		LastWasInbound: true,
		HasSale:        true,
	})

	assert.Equal(t, CategoryCustomer, c)
}

// Nothing decisive. "other" is the honest answer, and it is still WRITTEN —
// a conversation with no category sorts at zero and vanishes under
// everything, which is worse than a coarse label.
func TestClassifyInbox_UnknownIsOtherAndSaysSo(t *testing.T) {
	c, ok := ClassifyInbox(InboxSignals{
		Subject: "Oi", Body: "Tudo bem?", LastWasInbound: true,
	})

	assert.False(t, ok, "the caller's cue to ask a model")
	assert.Equal(t, CategoryOther, c)
}

// ── Ordering ──────────────────────────────────────────────────────────────

func TestInboxImportance_PutsWhatPaysAboveWhatArrivesLast(t *testing.T) {
	hot := InboxImportance(CategoryHotLead, InboxSignals{})
	news := InboxImportance(CategoryNewsletter, InboxSignals{})
	customer := InboxImportance(CategoryCustomer, InboxSignals{})

	assert.Greater(t, hot, customer)
	assert.Greater(t, customer, news)
}

// Not because it is urgent business — because it is the one thing that costs
// money if it is not seen before it is trusted.
func TestInboxImportance_QuarantineIsTop(t *testing.T) {
	q := InboxImportance(CategoryQuarantine, InboxSignals{})

	for _, c := range []InboxCategory{
		CategoryHotLead, CategoryFinancial, CategoryCustomer,
		CategorySupplier, CategoryOther, CategoryNewsletter,
	} {
		assert.Greater(t, q, InboxImportance(c, InboxSignals{}), "vs %q", c)
	}
}

// Bottom, never zero: filing something as junk and then not showing it is how
// a real message disappears.
func TestInboxImportance_NewsletterIsLowNotHidden(t *testing.T) {
	assert.Positive(t, InboxImportance(CategoryNewsletter, InboxSignals{}))
}

// The value reaches SQL, so "whatever the client sent" must never.
func TestValidInboxCategory_IsAClosedSet(t *testing.T) {
	assert.True(t, ValidInboxCategory("hot_lead"))
	assert.True(t, ValidInboxCategory("QUARANTINE"))
	assert.False(t, ValidInboxCategory("'; DROP TABLE conversations; --"))
	assert.False(t, ValidInboxCategory(""))
	assert.False(t, ValidInboxCategory("inventada"))
}
