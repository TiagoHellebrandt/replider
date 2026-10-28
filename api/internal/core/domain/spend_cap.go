package domain

import (
	"context"
	"fmt"
	"time"
)

// Per-plan ceilings on the AI work that is not a message (§6.14, K4).
//
// Three things already had a ceiling: prospecting (leads), enrichment, and
// outbound sends. Two expensive ones did not — AI-written approaches and
// document extraction — and both can be spent in bulk from a single request.
// `draft_approaches_batch` fills a queue in one call; the scanned-contract path
// sends whole PDFs to a multimodal model.
//
// The daily token cap is NOT a substitute. It is an anti-runaway rail: it stops
// one bad afternoon, but it lets an account burn the same daily budget every day
// of the month no matter what they pay. A per-plan allowance is what makes the
// margin a function of the price.
//
// One table below holds everything each kind of spend needs, so adding the sixth
// capped operation is one entry rather than a fourth copy of the same twenty
// lines.

// SpendKind is one metered, capped operation.
type SpendKind string

const (
	// SpendAIDraft is one AI-written approach. Counted per DRAFT the customer
	// asked for, not per provider call — see SpendOperations.
	SpendAIDraft SpendKind = "ai_draft"
	// SpendDocumentExtraction is one document read by a model: a contract
	// parsed from text, or a scanned PDF sent to a multimodal model. The
	// single most expensive operation in the product.
	SpendDocumentExtraction SpendKind = "document_extraction"
)

// SpendOperations maps a kind onto the ledger operations that count against it.
//
// `approach_draft_retry` is deliberately ABSENT from the draft list. A retry is
// OUR repair of a malformed model response, not something the customer asked
// for, so charging it against their allowance would bill them for our bug. It
// still costs real money and still lands in the cost ledger — it is just our
// problem, visible in the margin rather than in their cap.
func SpendOperations(kind SpendKind) []string {
	switch kind {
	case SpendAIDraft:
		return []string{"approach_draft"}
	case SpendDocumentExtraction:
		// Text and vision are two routes to the same request: one document,
		// one unit. The pipeline takes one or the other, never both.
		// A product brochure read (PDF import) is the same unit of work as a
		// contract read — one document through a model — so it shares the
		// allowance instead of minting a plan column of its own.
		return []string{"contract_extract", "contract_extract_vision", "product_pdf_extract"}
	}
	return nil
}

// SpendCapFor resolves one kind's monthly allowance for an account.
//
// Goes through the Effective* accessors, never the raw plan column: a contract
// plan stores 0 and resolves through the account override, and reading the
// column directly is the mistake that once blocked an enterprise account from
// inviting an attendant.
func SpendCapFor(kind SpendKind, account *Account, plan *Plan) int {
	if account == nil || plan == nil {
		return 0
	}
	switch kind {
	case SpendAIDraft:
		return account.EffectiveMaxAIDraftsPerMonth(plan)
	case SpendDocumentExtraction:
		return account.EffectiveMaxDocumentExtractionsPerMonth(plan)
	}
	return 0
}

// SpendNotAllowedCode / SpendExceededCode are the error codes a refusal carries.
//
// Separate codes per kind, and separate "not included" from "used up": they are
// different conversations. One is an upgrade, the other is "wait for the cycle
// or buy a pack", and a single code would make the client guess which.
func SpendNotAllowedCode(kind SpendKind) string {
	switch kind {
	case SpendAIDraft:
		return "AI_DRAFTS_NOT_ALLOWED"
	case SpendDocumentExtraction:
		return "DOC_EXTRACTION_NOT_ALLOWED"
	}
	return "SPEND_NOT_ALLOWED"
}

func SpendExceededCode(kind SpendKind) string {
	switch kind {
	case SpendAIDraft:
		return "AI_DRAFTS_BUDGET_EXCEEDED"
	case SpendDocumentExtraction:
		return "DOC_EXTRACTION_BUDGET_EXCEEDED"
	}
	return "SPEND_BUDGET_EXCEEDED"
}

// ErrSpendNotAllowed is the refusal for a plan that does not include the
// operation at all.
func ErrSpendNotAllowed(kind SpendKind) *AppError {
	message := "seu plano não inclui esta operação — faça upgrade para habilitar"
	switch kind {
	case SpendAIDraft:
		message = "seu plano não inclui rascunhos por IA — faça upgrade para habilitar"
	case SpendDocumentExtraction:
		message = "seu plano não inclui leitura de documentos por IA — faça upgrade para habilitar"
	}
	return &AppError{Code: SpendNotAllowedCode(kind), Message: message}
}

// ErrSpendExceeded is the refusal for an allowance that has run out.
//
// The message carries the numbers. "Limite excedido" makes somebody open a
// support thread to find out by how much; "1.000 de 1.000 usados neste ciclo"
// answers it, and the reset date says when it stops being true.
func ErrSpendExceeded(kind SpendKind, used, cap int, resetsAt time.Time) *AppError {
	what := "operações"
	switch kind {
	case SpendAIDraft:
		what = "rascunhos por IA"
	case SpendDocumentExtraction:
		what = "leituras de documento"
	}
	return &AppError{
		Code: SpendExceededCode(kind),
		Message: fmt.Sprintf(
			"limite de %s do ciclo esgotado: %d de %d usados. Renova em %s.",
			what, used, cap, resetsAt.Format("02/01/2006")),
	}
}

// SpendWindow is the cycle a cap is measured over: the current month in the
// ACCOUNT's timezone.
//
// The account's month, not a rolling thirty days, and not UTC's month: the
// allowance is sold per billing cycle, and a customer who is told "renova dia 1"
// has to see it renew on THEIR first — a window that rolls over mid-afternoon
// for an account in Manaus is a support ticket.
func SpendWindow(now time.Time, timezone string) (from, resetsAt time.Time) {
	loc := LoadLocation(timezone)
	local := now.In(loc)
	from = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	return from, from.AddDate(0, 1, 0)
}

// SpendCounter counts completed operations against an allowance.
//
// A narrow port on purpose: the only implementation is the AI cost ledger, and
// the alternative — a per-kind counter column — is a second source of truth that
// drifts the first time a write is missed (the same reasoning that made the
// case-metric caps read the event log).
type SpendCounter interface {
	// CountOperations counts this account's ledger rows for any of the given
	// operations since a moment.
	CountOperations(ctx context.Context, accountID string, operations []string, since time.Time) (int64, error)
}

// ErrAIBudgetExceeded is the refusal for the CUSTOMER's own monthly AI
// ceiling. Distinct from the plan caps: this one the account set on itself,
// and the fix is raising it in settings, not upgrading a plan.
func ErrAIBudgetExceeded(usedCents, budgetCents int64, resetsAt time.Time) *AppError {
	return &AppError{
		Code: "AI_BUDGET_EXCEEDED",
		Message: fmt.Sprintf(
			"o teto de IA da conta foi atingido (R$ %.2f de R$ %.2f). Aumente o teto em Ajustes ou aguarde o ciclo reiniciar em %s",
			float64(usedCents)/100, float64(budgetCents)/100, resetsAt.Format("02/01")),
	}
}
