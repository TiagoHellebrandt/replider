package usecases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// Assessing inbound email for impersonation (§6.10, camada 1).
//
// Orchestration only: the rules live in domain.AssessEmailRisk, which is a
// pure function. This gathers what those rules need — what the account
// already corresponds with, whether this address has written before, how old
// the sender's domain is — and records the verdict.
// ContactPaymentMemory is what this account has seen a contact use before.
//
// A narrow port: the fraud check is a comparison against history, and the
// history is the only thing a generic filter cannot have.
type ContactPaymentMemory interface {
	Known(ctx context.Context, accountID, contactKey string) (map[string]struct{}, error)
	Record(ctx context.Context, accountID, contactKey string, details []domain.PaymentDetail) error
}

type EmailRiskUseCase struct {
	repo domain.EmailRiskRepository
	// age is optional. A lookup that did not run is NOT a pass, and the
	// domain layer already treats an absent date as "no answer".
	age domain.DomainAgeLookup
	// payments is optional: without it the technical checks still run, but
	// the golpe do boleto goes undetected.
	payments ContactPaymentMemory
	// allowlist is optional: without it a false positive cannot be
	// dismissed, which teaches the reader to scroll past the banner.
	allowlist domain.SenderAllowlistRepository
	// scams is optional: without it the platform protects each customer
	// alone, and the network effect — the whole point of camada 4 — does not
	// exist.
	scams domain.ScamReportRepository
}

// SetScamReports wires the collective blocklist.
func (uc *EmailRiskUseCase) SetScamReports(r domain.ScamReportRepository) { uc.scams = r }

// SetAllowlist wires the "é legítimo" memory.
func (uc *EmailRiskUseCase) SetAllowlist(a domain.SenderAllowlistRepository) { uc.allowlist = a }

// MarkLegitimate records that a human vouched for this sender and clears the
// warning on the message they were looking at.
//
// Clears rather than hides: the banner disappears because it was answered.
// Future messages from the same sender skip the checks entirely, which is the
// point — a warning the reader has already dismissed once and that keeps
// coming back is the same as no warning at all.
func (uc *EmailRiskUseCase) MarkLegitimate(ctx context.Context, actor domain.Actor, messageID, address string) error {
	if uc.allowlist == nil {
		return &domain.AppError{Code: "EMAIL_RISK_UNAVAILABLE", Message: "allowlist not configured"}
	}
	if err := uc.allowlist.Allow(ctx, actor.AccountID, address, actor.UserID); err != nil {
		return err
	}
	// Best effort: the vouch is the durable decision, and failing to clear
	// one banner must not undo it.
	if err := uc.repo.Clear(ctx, messageID, actor.AccountID); err != nil {
		slog.WarnContext(ctx, "email risk: vouched but could not clear the banner",
			"message_id", messageID, "error", err)
	}
	return nil
}

func NewEmailRiskUseCase(repo domain.EmailRiskRepository) *EmailRiskUseCase {
	return &EmailRiskUseCase{repo: repo}
}

// SetPaymentMemory wires the per-contact payment history.
func (uc *EmailRiskUseCase) SetPaymentMemory(m ContactPaymentMemory) { uc.payments = m }

// SetDomainAgeLookup wires the registry client. Nil disables the age check
// without disabling anything else.
func (uc *EmailRiskUseCase) SetDomainAgeLookup(l domain.DomainAgeLookup) { uc.age = l }

// knownDomainsLimit bounds the lookalike comparison. A thousand domains is
// far more than any account corresponds with, and the check is O(n) per
// message on the inbound hot path.
const knownDomainsLimit = 1000

// domainAgeTimeout keeps a slow registry from holding up a customer's reply.
// The check is worth having and not worth waiting for.
const domainAgeTimeout = 4 * time.Second

// EmailRiskParams is what the caller knows at inbound time.
type EmailRiskParams struct {
	AccountID   string
	MessageID   string
	FromAddress string
	ReplyTo     string
	Auth        domain.InboundAuthVerdict
	// Body is the message text, scanned for payment instructions.
	Body string
}

// Assess runs the technical checks and stores the result when there is one.
//
// Returns the assessment so the caller can act immediately. Never returns an
// error to the caller's critical path: a message must be delivered even when
// the risk layer cannot run, and a failure here is logged rather than
// propagated — refusing to deliver mail because a WHOIS lookup timed out
// would be a worse product than not checking at all.
func (uc *EmailRiskUseCase) Assess(ctx context.Context, p EmailRiskParams) domain.EmailRiskAssessment {
	// A human already vouched for this sender. Running the checks anyway and
	// then hiding the result would be theatre — and would still cost the
	// registry lookup on every message.
	if uc.allowlist != nil {
		if ok, err := uc.allowlist.IsAllowed(ctx, p.AccountID, p.FromAddress); err == nil && ok {
			return domain.EmailRiskAssessment{}
		}
	}

	in := domain.EmailRiskInput{
		FromAddress: p.FromAddress,
		ReplyTo:     p.ReplyTo,
		Auth:        p.Auth,
		Now:         time.Now(),
	}

	if known, err := uc.repo.KnownDomains(ctx, p.AccountID, knownDomainsLimit); err == nil {
		in.KnownDomains = known
	} else {
		slog.WarnContext(ctx, "email risk: could not read known domains", "error", err)
	}

	if seen, err := uc.repo.SeenBefore(ctx, p.AccountID, p.FromAddress, p.MessageID); err == nil {
		in.SeenBefore = seen
	} else {
		// Failing OPEN here: treating an unknown history as "first contact"
		// would add a signal to every message during a database blip.
		in.SeenBefore = true
		slog.WarnContext(ctx, "email risk: could not check sender history", "error", err)
	}

	in.DomainRegisteredAt = uc.domainAge(ctx, domain.DomainOf(p.FromAddress))
	in.ChangedPayments = uc.paymentChanges(ctx, p)
	in.NetworkReports = uc.networkReports(ctx, p)

	assessment := domain.AssessEmailRisk(in)
	if assessment.Clean() {
		// Nothing found is the normal case and is not stored: recording it
		// would double the write volume of the busiest table in the product
		// to say "tudo certo".
		return assessment
	}

	if err := uc.repo.Save(ctx, &domain.EmailRiskRecord{
		MessageID:    p.MessageID,
		AccountID:    p.AccountID,
		Assessment:   assessment,
		SenderDomain: domain.DomainOf(p.FromAddress),
	}); err != nil {
		slog.ErrorContext(ctx, "email risk: could not store the assessment",
			"message_id", p.MessageID, "error", err)
	}
	if assessment.Quarantined() {
		slog.WarnContext(ctx, "inbound email quarantined",
			"message_id", p.MessageID, "account_id", p.AccountID,
			"score", assessment.Score, "from", p.FromAddress)
	}
	return assessment
}

// paymentChanges compares the money instructions in this message against
// everything this contact has used before.
//
// Records what it saw either way, INCLUDING the changed details: the second
// message from the same new account should not raise the same alarm, or the
// warning becomes a fixture of the thread and stops meaning anything. The
// alarm fires on the change, once.
func (uc *EmailRiskUseCase) paymentChanges(ctx context.Context, p EmailRiskParams) []domain.PaymentDetail {
	if uc.payments == nil {
		return nil
	}
	found := domain.ExtractPaymentDetails(p.Body)
	if len(found) == 0 {
		return nil
	}

	known, err := uc.payments.Known(ctx, p.AccountID, p.FromAddress)
	if err != nil {
		// Fail QUIET, not open: an unreadable history would otherwise make
		// every payment detail look new and alarm on every invoice during a
		// database blip.
		slog.WarnContext(ctx, "email risk: could not read payment history", "error", err)
		return nil
	}

	changed := domain.NewPaymentDetails(found, known)
	if err := uc.payments.Record(ctx, p.AccountID, p.FromAddress, found); err != nil {
		slog.WarnContext(ctx, "email risk: could not record payment details", "error", err)
	}

	// Only an instruction to PAY is worth alarming about. A changed CNPJ in a
	// signature is a company that renamed itself, not a redirected transfer.
	if !domain.HasPaymentInstruction(changed) {
		return nil
	}
	return changed
}

// networkReports asks how many OTHER companies reported this sender.
//
// Checks the payment identifiers too, so a scammer who burns a domain but
// keeps the bank account is still caught — the account is the more expensive
// half of their setup and the one they reuse.
func (uc *EmailRiskUseCase) networkReports(ctx context.Context, p EmailRiskParams) int {
	if uc.scams == nil {
		return 0
	}
	best := 0
	if n, err := uc.scams.CountReportingAccounts(ctx,
		domain.DomainOf(p.FromAddress), p.AccountID); err == nil {
		best = n
	} else {
		slog.WarnContext(ctx, "email risk: could not read network reports", "error", err)
	}

	for _, d := range domain.ExtractPaymentDetails(p.Body) {
		n, err := uc.scams.CountByPaymentValue(ctx, d.Value, p.AccountID)
		if err == nil && n > best {
			best = n
		}
	}
	return best
}

// ReportScam records one company's verdict and returns what the platform
// already knew.
//
// The investigation is deliberately NOT a model call. Asked "is this a scam",
// a model produces a confident sentence; what the reader needs is evidence
// they can check — the domain's age, the resemblance to a supplier they know,
// how many other companies said the same. Those are facts the platform
// already holds, and a fact beats a fluent opinion in the one place where
// being wrong costs money.
func (uc *EmailRiskUseCase) ReportScam(
	ctx context.Context,
	actor domain.Actor,
	messageID, fromAddress, body, reason string,
) (*domain.EmailRiskRecord, error) {
	if uc.scams == nil {
		return nil, &domain.AppError{Code: "EMAIL_RISK_UNAVAILABLE", Message: "scam reports not configured"}
	}

	// The bank details the scam used travel with the report, so the same
	// account under a new domain is still caught. Never nil: most reported
	// messages carry no payment instruction at all, and a nil slice reaches
	// Postgres as NULL against a NOT NULL column.
	values := []string{}
	for _, d := range domain.ExtractPaymentDetails(body) {
		values = append(values, d.Value)
	}

	if err := uc.scams.Report(ctx, &domain.ScamReport{
		ID:            ulid.Make().String(),
		AccountID:     actor.AccountID,
		MessageID:     messageID,
		SenderDomain:  domain.DomainOf(fromAddress),
		PaymentValues: values,
		Reason:        strings.TrimSpace(reason),
	}); err != nil {
		return nil, err
	}

	// Re-assess with the report now on file, so the reporter immediately sees
	// what the platform makes of it — including whether anyone else had
	// already said the same.
	assessment := uc.Assess(ctx, EmailRiskParams{
		AccountID:   actor.AccountID,
		MessageID:   messageID,
		FromAddress: fromAddress,
		Body:        body,
		// A reported message is being judged on its own merits, not on an
		// authentication verdict the caller could supply.
		Auth: domain.InboundAuthVerdict{},
	})
	return &domain.EmailRiskRecord{
		MessageID:    messageID,
		AccountID:    actor.AccountID,
		Assessment:   assessment,
		SenderDomain: domain.DomainOf(fromAddress),
	}, nil
}

func (uc *EmailRiskUseCase) domainAge(ctx context.Context, dom string) *time.Time {
	if uc.age == nil || dom == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, domainAgeTimeout)
	defer cancel()

	at, err := uc.age.RegisteredAt(ctx, dom)
	if err != nil {
		slog.WarnContext(ctx, "email risk: domain age lookup failed", "domain", dom, "error", err)
		return nil
	}
	return at
}
