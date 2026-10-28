package domain

// StripeInvoiceSummary is the slim payload surfaced to the owner panel
// for the customer's last invoices. Lives in the domain package so
// both the Stripe adapter (payment/) and the use case (usecases/) can
// import it without creating a cyclic dependency. Mirrors a subset of
// stripe-go's *Invoice — line items + tax detail are intentionally
// stripped (drilldowns happen in the Stripe dashboard).
type StripeInvoiceSummary struct {
	ID            string  `json:"id"`
	Number        string  `json:"number"`
	Status        string  `json:"status"`      // draft|open|paid|uncollectible|void
	AmountDue     int64   `json:"amount_due"`  // cents
	AmountPaid    int64   `json:"amount_paid"` // cents
	Currency      string  `json:"currency"`
	Created       int64   `json:"created"` // unix
	HostedURL     *string `json:"hosted_url,omitempty"`
	InvoicePDFURL *string `json:"invoice_pdf,omitempty"`
}

// StripePaidInvoice is one invoice PAID inside a profit cycle (US-006),
// carrying the REAL processing fee pulled from the charge's
// balance_transaction (expanded on the list call) — never an estimated
// percentage. AmountPaidCents is in the invoice currency; FeeCents is
// in the balance_transaction's own SETTLEMENT currency, reported in
// FeeCurrency — the profit computation must never assume it is BRL
// (US-008 review fix).
type StripePaidInvoice struct {
	ID              string
	AmountPaidCents int64
	// RefundedCents is the charge's amount_refunded, in the same
	// currency as AmountPaidCents — covers refunds and refund-backed
	// credit notes. Revenue is paid − refunded (US-007 review fix).
	RefundedCents int64
	Currency      string // lowercase ISO: "brl" | "usd"
	FeeCents      int64
	// FeeCurrency is the balance_transaction's currency (lowercase
	// ISO). Empty when the charge had no expanded balance_transaction
	// (FeeCents is 0 then).
	FeeCurrency string
	PaidAt      int64 // unix
}
