package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oklog/ulid/v2"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// approachMetaPrompt instructs the model to write ONE short, personalized
// first-contact (Social Selling) WhatsApp message in PT-BR. The output is a
// SUGGESTION a human reviews/edits before sending — never sent automatically
// (US-020). The anti-ban posture (warm, consultative, opt-out friendly) is
// baked into the rules so a careless approach can't read as spam.
const approachMetaPrompt = `Você é um especialista em Social Selling que escreve a PRIMEIRA mensagem de abordagem, via WhatsApp, em português do Brasil, para um possível cliente que ainda NÃO conhece a empresa.

A partir do contexto do funil, do produto e dos dados do lead abaixo, escreva UMA única mensagem curta de abordagem. Regras:
- Escreva SOMENTE o texto da mensagem, em português do Brasil — sem preâmbulo, sem aspas, sem explicações, sem títulos.
- Tom humano, caloroso e consultivo. Apresente-se brevemente, mostre que entendeu o contexto do lead e proponha uma conversa — nunca pressione nem prometa o que não foi informado.
- Personalize com os dados do lead quando existirem (nome, empresa, cargo, necessidade). Quando faltar um dado, NÃO invente: seja genérico mas relevante.
- Curta: no máximo 3 frases / ~60 palavras. Pode usar no máximo 1 emoji discreto.
- Respeite o opt-out: deixe claro, de forma leve, que a pessoa pode pedir para não receber mais mensagens.
- NUNCA escreva placeholder. Nada de [Seu Nome], [Empresa], {{nome}} ou colchetes de preencher depois. Assine com o nome da empresa informado acima; se ele não vier, escreva sem assinatura em vez de inventar um.
- NUNCA garanta prazo, disponibilidade, resultado ou capacidade técnica: nada de "garantimos", "atendimento em X horas", "resolvemos qualquer". Quem envia responde juridicamente por isso, e a mensagem sai antes de qualquer engenheiro conferir.
- NUNCA afirme capacidade sobre um equipamento, marca ou serviço só porque ele apareceu nos dados do lead — o dado diz o que o CLIENTE tem, não o que quem envia sabe fazer.
- NUNCA afirme como fato algo que você não recebeu: nada de "soube que", "vi que", "sei que vocês estão buscando". Se for suposição, escreva como pergunta.`

// SetDraftModel pins the model used to WRITE outbound approaches, overriding
// the account plan's model for this task only.
//
// The plan picks one model for everything, which is the wrong shape here:
// answering "quantos leads eu tenho" and writing a cold message a stranger
// reads in the customer's name are not the same risk. A pilot user named
// the stakes — a draft promising fast guaranteed repair on a hospital
// ventilator is a commitment made in her name, sent before any engineer saw
// it. This lets the expensive-to-get-wrong task run on a stronger model
// without paying for it on every chat turn.
//
// Empty (the default) keeps the plan's model, so nothing has to be
// configured for the product to work.
// SetExperimentRepo wires the A/B of approach writing. Optional: without it
// drafting behaves as it always did, which is what keeps this measurable
// without becoming load-bearing.
func (uc *ApproachUseCase) SetExperimentRepo(r domain.ApproachExperimentRepository) {
	uc.experiments = r
}

func (uc *ApproachUseCase) SetDraftModel(model string) {
	uc.draftModel = strings.TrimSpace(model)
}

// ApproachSender is the narrow port the approval flow needs to deliver one
// outbound message and learn its provider-side id. Satisfied by the WhatsApp
// adapter's SendMessageAndGetID — wired in main.go. Keeping it narrow lets the
// use case be unit-tested without a real channel adapter.
type ApproachSender interface {
	SendMessageAndGetID(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) (string, error)
}

// ApproachUseCase implements the human-in-the-loop outbound approach (US-020):
// the AI only SUGGESTS a draft; a human reviews/edits it and explicitly
// approves the send. There is deliberately NO path that sends a prospected
// lead's approach without passing through approval — automatic mass-send is
// forbidden by design.
type ApproachUseCase struct {
	// batchJobs stores provider batch jobs (overnight 50%-off drafts).
	// Optional — nil keeps the synchronous night pass only.
	batchJobs domain.AIBatchJobRepository
	// brandRepo is optional: nil means an account drafts exactly as it did
	// before the brand kit existed.
	brandRepo domain.BrandKitRepository
	// suppressions is optional: nil keeps the pre-bounce behaviour, where a
	// dead address is written to over and over.
	suppressions domain.EmailSuppressionRepository
	// autopilot is optional: nil means the streak is never counted, so the
	// offer is never made — which is the correct behaviour for a deployment
	// that has not wired the feature.
	autopilot domain.AutopilotRepository
	// strategies stamps which strategy version reached a lead (E3). Without
	// it nothing downstream measures anything: an A/B compares two unnamed
	// things and "essa abordagem funciona" is a feeling.
	strategies  LeadStrategyAttributor
	leadRepo    domain.LeadRepository
	funnelRepo  domain.SalesFunnelRepository
	channelRepo domain.ChannelRepository
	// sendChannels answers the one question the no-channel gate asks: does
	// this account have any door at all. A narrow port with an optional
	// setter rather than reusing channelRepo directly — `ListByAccountID`
	// also serves ActiveEmailChannel, and a generic stub for one shadows the
	// specific expectations the other's tests register.
	sendChannels draftSendChannelLister
	contactRepo  domain.ContactRepository
	accountRepo  domain.AccountRepository
	planRepo     domain.PlanRepository
	productRepo  domain.ProductRepository
	usageRepo    domain.UsageRepository
	// spend is the per-plan ceiling on AI drafts (K4). Optional: nil drafts
	// exactly as before the caps existed — a deployment that forgot to wire
	// it must keep working, since this is a margin control and not a security
	// boundary.
	spend     *spendGate
	aiFactory domain.AIFactory

	// dossier gathers what the account already knows about the lead — its own
	// co-purchase patterns, this customer's purchase history, recorded signals,
	// the registered activity. Optional: nil drafts from exactly what it drafted
	// from before, which is the funnel's persona and one product.
	dossier *LeadDossierBuilder
	// differentiators is the account's "why ours" library. Optional: without
	// it the draft argues from the product alone.
	differentiators domain.DifferentiatorRepository
	// matcher picks which of the account's products this lead's offer is
	// about, when the funnel does not name one. Optional: nil drafts as
	// before — the funnel's product or none.
	matcher *OfferMatcher
	// draftModel overrides the plan's model for approach writing only.
	// See SetDraftModel.
	draftModel string
	// experiments is optional (US-F216). nil → no A/B, everything else
	// works exactly as before.
	experiments domain.ApproachExperimentRepository
	sender      ApproachSender
	// emailSender delivers the email path. The default `sender` is the
	// WhatsApp adapter; using it for an email channel would send through
	// the wrong provider entirely.
	emailSender ApproachSender
	// bulkSender delivers BATCH email approvals through the platform's bulk
	// provider (hybrid model). See SetBulkSender.
	bulkSender ApproachSender
	// mailboxRepo enforces the email deliverability guards (US-M007).
	// Optional wiring — see SetMailboxRepo; unwired REFUSES email approvals
	// rather than sending unguarded.
	mailboxRepo domain.EmailMailboxRepository
	// now is injectable so cap/warmup behaviour is testable without
	// waiting three weeks.
	now func() time.Time
	// cases records what happened for the case report (US-M011). Optional:
	// instrumentation must never be able to break the send it measures.
	cases *CaseRecorder
	// meter prices the AI calls that never become a message (US-M015).
	meter *AIMeter
	// scheduleRepo turns a daily-cap refusal into a scheduled send (phase
	// 1b). Optional: unwired keeps the hard refusal.
	scheduleRepo domain.ApproachScheduleRepository
	// convRepo + msgRepo persist the approved send as an outbound message
	// (see lead_approach_outbound.go). Optional — SetOutboundRecorder.
	convRepo domain.ConversationRepository
	msgRepo  domain.MessageRepository
}

func NewApproachUseCase(
	leadRepo domain.LeadRepository,
	funnelRepo domain.SalesFunnelRepository,
	channelRepo domain.ChannelRepository,
	contactRepo domain.ContactRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	productRepo domain.ProductRepository,
	usageRepo domain.UsageRepository,
	aiFactory domain.AIFactory,
	sender ApproachSender,
) *ApproachUseCase {
	return &ApproachUseCase{
		leadRepo:    leadRepo,
		funnelRepo:  funnelRepo,
		channelRepo: channelRepo,
		contactRepo: contactRepo,
		accountRepo: accountRepo,
		planRepo:    planRepo,
		productRepo: productRepo,
		usageRepo:   usageRepo,
		aiFactory:   aiFactory,
		sender:      sender,
		now:         time.Now,
	}
}

// draftableStatuses are the pre-approach pipeline stages from which a lead may
// be drafted. Anything later (contacted onward) is rejected so a human can't
// accidentally re-approach a lead that was already contacted.
func draftable(status domain.LeadStatus) bool {
	switch status {
	case domain.LeadStatusNew, domain.LeadStatusProspected,
		domain.LeadStatusEnriching, domain.LeadStatusEnriched,
		domain.LeadStatusPendingApproval:
		return true
	default:
		return false
	}
}

// Draft generates an AI-suggested approach for one lead and queues it for human
// approval (status=pending_approval). It does NOT send anything. An opted-out
// contact is rejected up front — there's no point drafting an approach we can
// never deliver.
func (uc *ApproachUseCase) Draft(ctx context.Context, accountID, leadID string) (*domain.Lead, error) {
	lead, funnel, err := uc.ownedLeadAndFunnel(ctx, accountID, leadID)
	if err != nil {
		return nil, err
	}
	if !draftable(lead.Status) {
		return nil, domain.ErrLeadAlreadyContacted
	}
	if uc.leadOptedOut(ctx, accountID, lead) {
		return nil, domain.ErrContactOptedOut
	}
	// Checked BEFORE the provider call: a cap enforced after the money is
	// spent is an error message, not a cap.
	if err := uc.spend.ensure(ctx, accountID, domain.SpendAIDraft); err != nil {
		return nil, err
	}

	text, err := uc.generateDraft(ctx, accountID, funnel, lead)
	if err != nil {
		return nil, err
	}

	lead.ApproachDraft = text
	lead.Status = domain.LeadStatusPendingApproval
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		return nil, fmt.Errorf("queue lead draft: %w", err)
	}
	return lead, nil
}

// SaveDraft persists a human-written approach WITHOUT sending it and without
// touching the lead's status.
//
// Until now the only ways to write `approach_draft` were Draft (the AI writes
// it) and ApproveSend (which delivers it to a real person). A salesperson who
// typed their own text and closed the panel lost it: the words lived in a text
// field and nowhere else. Losing somebody's writing is the one bug that makes
// them stop writing in the tool.
//
// It deliberately does NOT move the lead into pending_approval. Queuing is a
// decision ("this is ready for review"), and autosaving a half-finished
// sentence is not that decision. A lead already in the queue keeps its place —
// the text under review is simply the current text.
func (uc *ApproachUseCase) SaveDraft(ctx context.Context, accountID, leadID, text string) (*domain.Lead, error) {
	lead, err := uc.leadRepo.GetByID(ctx, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, domain.ErrLeadNotFound
	}
	// Missing and cross-account answer the same so a guessed id cannot probe
	// another tenant's funnel.
	if lead.AccountID != accountID {
		return nil, domain.ErrLeadNotFound
	}
	// A lead already contacted has a draft that is now history — rewriting it
	// would rewrite what was sent, as far as every screen showing it knows.
	if lead.Status == domain.LeadStatusContacted {
		return nil, domain.ErrLeadAlreadyContacted
	}
	if lead.ApproachDraft == text {
		return lead, nil
	}
	lead.ApproachDraft = text
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		return nil, fmt.Errorf("save lead draft: %w", err)
	}
	return lead, nil
}

// draftSendChannelLister is the one channel fact the drafting gate needs.
type draftSendChannelLister interface {
	ListByAccountID(ctx context.Context, accountID string) ([]*domain.Channel, error)
}

// SetSendChannels wires the "does this account have a door" lookup. Nil skips
// the gate, which is the permissive answer — same posture as the rest of the
// optional wiring here.
func (uc *ApproachUseCase) SetSendChannels(l draftSendChannelLister) { uc.sendChannels = l }

// BatchDraftResult reports the outcome of a bulk draft enqueue.
type BatchDraftResult struct {
	// Drafted is the number of leads moved into the approval queue.
	Drafted int `json:"drafted"`
	// SkippedOptedOut is the number of leads skipped because the contact opted
	// out of receiving messages (filtered BEFORE enqueuing, per US-020 AC).
	SkippedOptedOut int `json:"skipped_opted_out"`
	// SkippedError is the number of leads whose draft generation failed and
	// were left untouched (best-effort; the batch never aborts on one failure).
	SkippedError int `json:"skipped_error"`
	// SkippedBudget is the number of leads the plan's monthly draft allowance
	// did not cover (K4). Reported rather than swallowed: a batch that
	// silently returns 40 of 100 reads as "only 40 were possible", and the
	// customer never learns there is an upgrade that would have drafted the
	// rest.
	SkippedBudget int `json:"skipped_budget"`
	// LeadIDs are the ids successfully queued, so the UI can refresh them.
	LeadIDs []string `json:"lead_ids"`
	// StoppedReason is set when the batch gave up because every draft was
	// failing the same way. Empty means the batch ran to the end. It is the
	// difference between "this lead did not work" and "nothing can work for
	// this account until somebody fixes X", and the screen shows it.
	StoppedReason string `json:"stopped_reason,omitempty"`

	// lastFailure/failureStreak track consecutive identical failures. Not
	// serialized — bookkeeping for stopOnRepeatedFailure.
	lastFailure   string
	failureStreak int
}

const (
	defaultDraftBatch = 25
	maxDraftBatch     = 100
)

// DraftBatch bulk-generates drafts for a funnel's not-yet-approached leads and
// queues them for approval, FILTERING opted-out contacts before enqueuing and
// reporting how many were skipped (US-020 AC). It still sends nothing — every
// lead lands in pending_approval for a human to review. Mass auto-send remains
// impossible: even this batch only fills the queue.
func (uc *ApproachUseCase) DraftBatch(ctx context.Context, accountID, funnelID string, limit int) (*BatchDraftResult, error) {
	funnel, err := uc.funnelRepo.GetByID(ctx, funnelID)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != accountID {
		return nil, domain.ErrUnauthorized
	}

	if limit <= 0 {
		limit = defaultDraftBatch
	}
	if limit > maxDraftBatch {
		limit = maxDraftBatch
	}

	// Pull a window of the funnel's leads; we draft only the draftable ones.
	leads, err := uc.leadRepo.ListByFunnel(ctx, funnelID, "", int32(maxDraftBatch), 0) //nolint:gosec // G115: maxDraftBatch is a small constant
	if err != nil {
		return nil, err
	}

	// How many the plan's remaining allowance covers. Resolved ONCE for the
	// batch and then counted down locally: re-reading the ledger per lead
	// would be a query per draft, and the ledger only learns about a draft
	// after its provider call returns anyway.
	budget, err := uc.spend.remainingFor(ctx, accountID, domain.SpendAIDraft, limit)
	if err != nil {
		// Nothing at all is allowed — that is a refusal for the whole call,
		// not a batch that quietly drafts zero.
		return nil, err
	}

	res := &BatchDraftResult{LeadIDs: []string{}}
	for _, lead := range leads {
		if res.Drafted >= limit {
			break
		}
		if !draftable(lead.Status) {
			continue
		}
		if uc.leadOptedOut(ctx, accountID, lead) {
			res.SkippedOptedOut++
			continue
		}
		if res.Drafted >= budget {
			// Count the rest so the caller can say how many the allowance
			// did not cover, instead of returning a short list with no
			// explanation.
			res.SkippedBudget++
			continue
		}
		text, err := uc.generateDraft(ctx, accountID, funnel, lead)
		if err != nil {
			res.SkippedError++
			if uc.stopOnRepeatedFailure(ctx, res, accountID, funnelID, err) {
				break
			}
			continue
		}
		lead.ApproachDraft = text
		lead.Status = domain.LeadStatusPendingApproval
		if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
			res.SkippedError++
			if uc.stopOnRepeatedFailure(ctx, res, accountID, funnelID, err) {
				break
			}
			continue
		}
		res.Drafted++
		res.LeadIDs = append(res.LeadIDs, lead.ID)
		// A success clears the streak: an isolated bad lead between good
		// ones is not an account-level fault.
		res.failureStreak, res.lastFailure = 0, ""
	}
	return res, nil
}

// draftBatchFailureStreak is how many identical failures in a row mean the
// cause is the ACCOUNT, not the lead.
const draftBatchFailureStreak = 3

// stopOnRepeatedFailure ends a batch whose failures are all the same.
//
// The pattern this replaces ran in production for nine consecutive nights:
// an account with no catalogue failed EVERY lead for the same reason, the
// loop counted 70 skips and moved on, and the only trace was a number.
// "Could not reach the model for lead X" and "this account has no catalogue,
// so no offer can be written at all" produced identical output.
//
// Three of the same message is not three problems. Stopping also stops
// spending: 70 doomed provider calls cost real money to learn one fact.
func (uc *ApproachUseCase) stopOnRepeatedFailure(
	ctx context.Context, res *BatchDraftResult, accountID, funnelID string, err error,
) bool {
	// An account-level refusal needs no streak: no other lead in this funnel
	// will get past it either. Counting to three would just be three reads
	// spent learning the same fact.
	if code := appErrorCode(err); domain.IsAccountLevelDraftRefusal(code) {
		res.StoppedReason = err.Error()
		slog.WarnContext(ctx, "approach batch: stopped, the account cannot draft at all",
			"account_id", accountID, "funnel_id", funnelID, "code", code)
		return true
	}
	msg := err.Error()
	if msg == res.lastFailure {
		res.failureStreak++
	} else {
		res.lastFailure, res.failureStreak = msg, 1
	}
	if res.failureStreak < draftBatchFailureStreak {
		return false
	}
	res.StoppedReason = msg
	slog.WarnContext(ctx, "approach batch: stopped, every draft failing the same way",
		"account_id", accountID, "funnel_id", funnelID,
		"failed", res.SkippedError, "error", err)
	return true
}

// ApproachTemplateButton is one declared button of the approved template,
// forwarded so the anti-ban opt-out guard can see the opt-out quick-reply and
// so dynamic quick-reply payloads bind by index.
type ApproachTemplateButton struct {
	SubType string
	Text    string
	Payload string
}

// ApproachSendInput is the human-approved send: the edited text plus the
// approved WhatsApp template it is delivered through. Cold first-contact on the
// WhatsApp Cloud API is business-initiated and ONLY deliverable via a
// pre-approved template — a free-form text is rejected outside the 24h
// customer-service window, which a prospected lead never opened. The edited
// Text becomes the template's BODY parameter(s) (the approach template's body
// is a single {{1}} that receives the personalized message).
type ApproachSendInput struct {
	Text string
	// ActorUserID is who approved. Recorded as the sender of the outbound
	// message row so the timeline and the "human reply" queries attribute
	// the send to a person. Optional (scheduled replay, MCP without actor).
	ActorUserID string
	// Subject is used only by the email path (US-M009). An empty subject
	// falls back to a default in the adapter — never sent blank, since a
	// missing subject is a spam signal.
	Subject          string
	TemplateName     string
	TemplateLanguage string // default pt_BR
	TemplateCategory string // default MARKETING
	// StrategyID and StrategyVersion attribute the send (E3). Optional: an
	// approach written without a strategy is an honest state, and defaulting
	// to some arbitrary strategy would make the first measurement a lie.
	StrategyID      string
	StrategyVersion int
	Buttons         []ApproachTemplateButton
	// EmailTemplateID and EmailTemplateVersion say which stored email
	// template this text came from, so the send — and any reply it earns —
	// counts against the exact words that earned it. Email path only, and
	// optional: a text written from scratch simply has no template.
	EmailTemplateID      string
	EmailTemplateVersion int
	// ViaBulk routes this send through the platform's bulk provider (hybrid
	// model, 2026-08-25): every BATCH approval sets it; one-by-one approvals
	// never do. Requires the mailbox's domain registered+verified with the
	// provider (BulkReady), else the send refuses with
	// BULK_SENDING_NOT_READY. Bulk sends skip the per-mailbox warmup/daily
	// cap — the provider's reputation carries the volume — but keep every
	// people-facing guard: suppression, opt-out, broadcast budget.
	ViaBulk bool
}

// ApproveSend sends the human-edited approach for a lead awaiting approval,
// delivered through the approved WhatsApp template (so cold first-contact is
// actually deliverable and the US-019 anti-ban opt-out guard runs). It re-checks
// the opt-out flag and plan limits and validates the contact is a real phone
// immediately before sending; on success the lead moves to contacted. On a send
// failure the lead stays pending_approval and the error is surfaced (US-020).
func (uc *ApproachUseCase) ApproveSend(ctx context.Context, accountID, leadID string, in ApproachSendInput) (*domain.Lead, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return nil, domain.ErrLeadApproachTextRequired
	}
	// NOTE: the "approved template required" rule is WhatsApp-specific and
	// is therefore checked AFTER the channel is resolved (below). Checking
	// it here — as this did before US-M009 — made every email approval fail
	// with a WhatsApp error before the email path could run.

	// An unfilled placeholder blocks the send, and is the ONLY outbound check
	// that does. Everything else this system finds — a promise, an invented
	// fact, a changed payment detail — is a judgement the approver may
	// legitimately make. "Olá {{nome}}" is never a judgement: it is a broken
	// message, and it burns a lead for a reason nobody chose.
	//
	// Checked here rather than only in the reviewer because the reviewer
	// costs a model call and can be unavailable. This one cannot.
	if ph := domain.PlaceholderFindings(text); len(ph) > 0 {
		quotes := make([]string, 0, len(ph))
		for _, p := range ph {
			quotes = append(quotes, p.Quote)
		}
		return nil, &domain.AppError{
			Code: domain.ErrApproachHasPlaceholder.Code,
			Message: domain.ErrApproachHasPlaceholder.Message + ": " +
				strings.Join(quotes, ", "),
		}
	}

	lead, funnel, err := uc.ownedLeadAndFunnel(ctx, accountID, leadID)
	if err != nil {
		return nil, err
	}
	// Approval-queue gate: only a drafted lead can be approved. There is no
	// route that sends an approach skipping this state.
	if lead.Status != domain.LeadStatusPendingApproval {
		return nil, domain.ErrLeadNotPendingApproval
	}

	// The funnel's channel, or the account's active email channel when the
	// funnel never got one (the default funnel is born without a channel —
	// see resolveSendChannel).
	channel, err := uc.resolveSendChannel(ctx, accountID, funnel)
	if err != nil {
		return nil, err
	}

	// Email takes a different path entirely (US-M004/M007): a subject and a
	// body instead of a WhatsApp template, and the deliverability guards
	// instead of the anti-ban template rules. Routing it through the
	// template builder would produce a nonsense message AND skip the caps
	// that protect the customer's domain.
	if channel.Type == "email" {
		return uc.approveSendEmail(ctx, accountID, lead, channel, in, text)
	}

	// Cold approach must go through an approved template — a free-form text
	// would be rejected outside the 24h window.
	if strings.TrimSpace(in.TemplateName) == "" {
		return nil, domain.ErrLeadApproachTemplateRequired
	}

	// The contact must be reachable by phone — on a TELEPHONY channel.
	// Prospecting keys a phone-less lead on its Google place_id, which is
	// not messageable. An email lead legitimately has no phone at all, so
	// this check moved below the channel lookup in US-M009.
	var phone string
	if channel.Type != "email" {
		phone = uc.leadPhone(ctx, accountID, lead)
		if phone == "" {
			return nil, domain.ErrLeadContactNotMessageable
		}
	}

	// Opt-out compliance (US-025): never send to an opted-out contact, and make
	// the refusal explicit rather than relying solely on the adapter guard.
	// contacts.opted_out_at is keyed by external_id, so the email path
	// checks the contact itself (leadOptedOut) instead of a phone.
	if channel.Type == "email" {
		if uc.leadOptedOut(ctx, accountID, lead) {
			return nil, domain.ErrContactOptedOut
		}
	} else if optedOut, _ := uc.contactRepo.IsOptedOut(ctx, accountID, phone); optedOut {
		return nil, domain.ErrContactOptedOut
	}

	// Plan limits: an outbound approach is a business-initiated message, billed
	// like a broadcast — enforce the same monthly cap before sending.
	if err := uc.checkBroadcastBudget(ctx, accountID); err != nil {
		return nil, err
	}

	// Build a TEMPLATE message: the edited text is the BODY param, routed through
	// the high-level template path so buildTemplatePayload's anti-ban opt-out
	// guard runs before any network call (US-019).
	msg := &domain.Message{
		Type:      "template",
		Direction: "outbound",
		From:      "bot",
		Content:   &text,
		Metadata:  buildApproachTemplateMeta(in, text),
	}
	// Claim BEFORE the provider call: the atomic status flip is what stops
	// a second tab (or the schedule worker) from sending this twice.
	if err := uc.claimForSend(ctx, lead); err != nil {
		return nil, err
	}
	externalID, err := uc.sender.SendMessageAndGetID(ctx, channel, phone, msg)
	if err != nil {
		// Send failure: put the lead back in the queue and surface the reason.
		uc.releaseClaim(ctx, lead)
		if appErr, ok := err.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, &domain.AppError{Code: domain.ErrLeadSendFailed.Code, Message: fmt.Sprintf("%s: %v", domain.ErrLeadSendFailed.Message, err)}
	}
	uc.recordOutbound(ctx, accountID, channel, phone, lead, msg, externalID, approachSender(in))

	// Reserve the broadcast budget best-effort (worst case a tiny over-count).
	if uc.usageRepo != nil {
		_ = uc.usageRepo.AddBroadcastMessages(ctx, accountID, 1)
	}
	// Recorded only after the provider accepted it: a case report that
	// counts sends which failed is worse than no report.
	uc.cases.ApproachSent(ctx, accountID, lead.ID, channel.Type)

	// A/B outcome, recorded BEFORE the draft field is overwritten below —
	// afterwards the generated text is gone and every approval would score
	// as unedited.
	uc.closeExperiment(ctx, lead.FunnelID, lead.ID, text, false)
	// Same window, same reason: the streak that earns autopilot is measured
	// against the text AS DRAFTED, and lead.ApproachDraft is overwritten two
	// lines below.
	uc.recordAutopilotStreak(ctx, accountID, lead.ApproachDraft, text)
	uc.attributeStrategy(ctx, lead.ID, in)

	lead.ApproachDraft = text
	lead.Status = domain.LeadStatusContacted
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		// The message went out; the status write failed. Surface it, but the
		// send already happened so this is a best-effort consistency error.
		return nil, fmt.Errorf("mark lead contacted: %w", err)
	}
	return lead, nil
}

// buildApproachTemplateMeta assembles the msg.Metadata the WhatsApp adapter's
// template path reads (parseTemplateSpecFromMeta): the approved template name,
// language (default pt_BR), category (default MARKETING — which makes the
// adapter's opt-out guard mandatory), the edited text as the single BODY param,
// and the declared buttons (so the opt-out quick-reply is visible to the guard).
func buildApproachTemplateMeta(in ApproachSendInput, text string) map[string]interface{} {
	lang := strings.TrimSpace(in.TemplateLanguage)
	if lang == "" {
		lang = "pt_BR"
	}
	category := strings.TrimSpace(in.TemplateCategory)
	if category == "" {
		category = "MARKETING"
	}
	meta := map[string]interface{}{
		"template_name":        strings.TrimSpace(in.TemplateName),
		"template_language":    lang,
		"template_category":    category,
		"template_body_params": []string{text},
	}
	if len(in.Buttons) > 0 {
		buttons := make([]map[string]interface{}, 0, len(in.Buttons))
		for _, b := range in.Buttons {
			buttons = append(buttons, map[string]interface{}{
				"sub_type": b.SubType,
				"text":     b.Text,
				"payload":  b.Payload,
			})
		}
		meta["template_buttons"] = buttons
	}
	return meta
}

// ownedLeadAndFunnel loads the lead (ownership-checked) and its funnel.
func (uc *ApproachUseCase) ownedLeadAndFunnel(ctx context.Context, accountID, leadID string) (*domain.Lead, *domain.SalesFunnel, error) {
	lead, err := uc.leadRepo.GetByID(ctx, leadID)
	if err != nil {
		return nil, nil, err
	}
	if lead == nil {
		return nil, nil, domain.ErrLeadNotFound
	}
	if lead.AccountID != accountID {
		return nil, nil, domain.ErrUnauthorized
	}
	funnel, err := uc.funnelRepo.GetByID(ctx, lead.FunnelID)
	if err != nil {
		return nil, nil, err
	}
	if funnel == nil {
		return nil, nil, domain.ErrSalesFunnelNotFound
	}
	return lead, funnel, nil
}

// generateDraft renders the lead/funnel/product context and asks the account's
// AI provider (BYOK) for a suggested approach message.
// draftPrep is everything a draft needs BEFORE the model answers — shared
// by the synchronous path and the overnight batch path, so the two can
// never drift on prompt assembly, experiments or metering identity.
type draftPrep struct {
	accountID    string
	lead         *domain.Lead
	provider     domain.AIProvider
	plan         *domain.Plan
	systemPrompt string
	payload      string
	brand        *domain.BrandKit
	exp          *domain.ApproachExperiment
	variant      string
}

func (uc *ApproachUseCase) generateDraft(ctx context.Context, accountID string, funnel *domain.SalesFunnel, lead *domain.Lead) (string, error) {
	prep, err := uc.prepareDraft(ctx, accountID, funnel, lead)
	if err != nil {
		return "", err
	}
	resp, err := prep.provider.GenerateResponse(ctx, prep.systemPrompt, []domain.ChatMessage{
		{Role: "user", Content: prep.payload},
	}, nil)
	if err != nil || resp == nil {
		return "", &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("AI provider failed to generate approach: %v", err),
		}
	}
	return uc.completeDraft(ctx, prep, resp.Content, resp.Usage)
}

// prepareDraft assembles the prompt and resolves the provider.
func (uc *ApproachUseCase) prepareDraft(ctx context.Context, accountID string, funnel *domain.SalesFunnel, lead *domain.Lead) (*draftPrep, error) {
	prep, err := uc.prepareDraftContext(ctx, accountID, funnel, lead)
	if err != nil {
		return nil, err
	}
	payload := uc.buildApproachPayload(ctx, accountID, funnel, lead)
	if prep.exp != nil {
		if suffix := strings.TrimSpace(prep.exp.SuffixFor(prep.variant)); suffix != "" {
			payload += "\n\nOrientação adicional:\n" + suffix
		}
	}
	prep.payload = payload
	return prep, nil
}

// prepareDraftContext resolves everything a draft needs EXCEPT the lead
// payload: account, catalogue gate, plan/model, A/B arm, provider and the
// brand-layered system prompt. Cheap (no model call). The payload — dossier
// plus offer matcher, which IS a model call — is added by prepareDraft; the
// batch collector skips it by reusing the payload the job carried.
func (uc *ApproachUseCase) prepareDraftContext(ctx context.Context, accountID string, funnel *domain.SalesFunnel, lead *domain.Lead) (*draftPrep, error) {
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}
	// No catalogue, no draft. With nothing true to offer the model pads the
	// message with generic filler — and a queue full of "possíveis
	// sinergias" is spam wearing the product's name. Fail-open on a read
	// error (the funnel-pinned-product path still guards itself), closed on
	// a confirmed empty list.
	if uc.productRepo != nil {
		if products, perr := uc.productRepo.ListByAccountID(ctx, accountID); perr == nil && len(products) == 0 {
			return nil, domain.ErrApproachNeedsProducts
		}
	}
	// No door, no draft — the same posture, one step further along the
	// pipeline. An account with nothing to offer writes filler; an account
	// with nowhere to send writes offers that age in a queue. Production had
	// 89 of those, growing by forty a night.
	//
	// This gate sits here, in the one place every drafting path passes
	// through, rather than on the nightly worker alone: signals draft,
	// the radar drafts, a person drafts, and a fourth caller added later
	// would have missed a check bolted onto only one of them.
	//
	// Fail-OPEN on a read error, closed on a confirmed empty list — same as
	// the catalogue gate. A database blip must not stop a working account
	// from drafting.
	if uc.sendChannels != nil {
		if channels, cerr := uc.sendChannels.ListByAccountID(ctx, accountID); cerr == nil {
			if !domain.AccountCanSend(channels) {
				return nil, domain.ErrApproachNeedsChannel
			}
			// And the envelope needs an address, not just the company a
			// postbox. A contact resolved to nothing our channels can dial is
			// a message we can finish writing and never post — production had
			// 50 of those in one queue.
			//
			// Only checked when the contact resolves: an unreadable contact is
			// a lookup problem, not a proven gap, and the other gates here all
			// fail open on a read error.
			if contact := uc.leadContact(ctx, accountID, lead); contact != nil &&
				!domain.CanReachContact(channels, contact) {
				return nil, domain.ErrApproachNeedsAddress
			}
		}
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}

	// A/B: the arm is decided by the lead id, so a re-draft lands in the
	// SAME arm. Random assignment would let someone regenerate until they
	// got the variant they preferred, turning the experiment into a poll.
	exp, variant := uc.runningExperiment(ctx, funnel.ID, lead.ID)

	// The task's model wins over the plan's, and a variant's model wins over
	// both — testing a MODEL is the same experiment as testing a prompt.
	// Cloned rather than mutated: plan is shared, and rewriting it here
	// would silently re-point every other task on this request.
	effectivePlan := plan
	model := uc.draftModel
	if exp != nil {
		if m := exp.ModelFor(variant); m != "" {
			model = m
		}
	}
	if model != "" && model != plan.AIModel {
		clone := *plan
		clone.AIModel = model
		effectivePlan = &clone
	}
	provider, err := uc.aiFactory.GetProviderForAccount(ctx, effectivePlan.AIProvider, account, effectivePlan)
	if err != nil {
		if errors.Is(err, domain.ErrBYOKKeyMissing) {
			return nil, domain.ErrBYOKNotConfigured
		}
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("failed to initialize AI provider: %v", err),
		}
	}

	// The company layer leads the instruction, so the provider can cache it:
	// it is identical for every message this account ever sends, and a
	// cached prefix is up to 90% cheaper. The forbidden claims are repeated
	// at the very end by the composer, where they are the last thing the
	// model reads.
	systemPrompt := approachMetaPrompt
	brand, overlay := uc.brandLayers(ctx, accountID, lead)
	if layered := domain.ComposeSystemPrompt(domain.PromptLayers{
		Brand: brand, Seller: overlay,
	}); layered != "" {
		systemPrompt = layered + "\n\n" + approachMetaPrompt
	}

	return &draftPrep{
		accountID: accountID, lead: lead, provider: provider,
		plan: effectivePlan, systemPrompt: systemPrompt,
		brand: brand, exp: exp, variant: variant,
	}, nil
}

// completeDraft meters, checks and repairs one raw model answer — the SAME
// post-processing whether the answer arrived synchronously or from a batch
// job collected hours later.
func (uc *ApproachUseCase) completeDraft(ctx context.Context, prep *draftPrep, rawText string, usage domain.AIUsage) (string, error) {
	accountID, lead := prep.accountID, prep.lead
	effectivePlan, provider := prep.plan, prep.provider
	systemPrompt, payload, brand := prep.systemPrompt, prep.payload, prep.brand
	exp, variant := prep.exp, prep.variant

	// Metered BEFORE the empty-response check: the provider was called and
	// charged us either way, and a draft that came back blank is exactly
	// the case where silently eating the cost is tempting.
	uc.meter.Record(ctx, accountID, "approach_draft", effectivePlan.AIProvider, effectivePlan.AIModel, usage)

	text := strings.TrimSpace(rawText)
	if text == "" {
		return "", &domain.AppError{Code: "APPROACH_GENERATION_EMPTY", Message: "AI returned an empty approach"}
	}

	// One retry when the draft promises something. The meta-prompt already
	// forbids it and the model does it anyway — measured across real
	// generations, "garantir" survived in most drafts despite an explicit
	// ban. Naming the exact offending fragments back to the model is far
	// more effective than repeating the rule, so that is what this does.
	//
	// Bounded to a single extra call: this runs per lead inside a batch, and
	// a loop here would multiply the account's AI bill by however stubborn
	// the model is feeling. If the second attempt still promises, the draft
	// is kept — a human approves every outbound anyway, and dropping their
	// draft to zero would be a worse answer than handing them a flawed one.
	// A claim the company forbids is not a style problem — in healthcare it
	// is a regulatory one — so it is checked deterministically rather than
	// trusted to the instruction above. Measured on this codebase: an
	// explicit ban in a prompt survives most of the time, not all of it, and
	// "most" is not a compliance posture.
	if brand != nil {
		if bad := domain.ViolatedClaims(text, brand.ForbiddenClaims); len(bad) > 0 {
			slog.WarnContext(ctx, "approach draft used a forbidden claim, retrying",
				"lead_id", lead.ID, "claims", bad)
			retry := payload + "\n\nO texto anterior usou termos proibidos: " +
				strings.Join(bad, ", ") +
				". Reescreva sem eles, sem trocá-los por sinônimos."
			if second, err := provider.GenerateResponse(ctx, systemPrompt,
				[]domain.ChatMessage{{Role: "user", Content: retry}}, nil); err == nil && second != nil {
				uc.meter.Record(ctx, accountID, "approach_draft", effectivePlan.AIProvider, effectivePlan.AIModel, second.Usage)
				if clean := strings.TrimSpace(second.Content); clean != "" &&
					len(domain.ViolatedClaims(clean, brand.ForbiddenClaims)) == 0 {
					text = clean
				}
			}
		}
	}

	uc.recordDraft(ctx, exp, variant, lead.ID, text)

	if promises := domain.ApproachPromises(text); len(promises) > 0 {
		retry, retryErr := provider.GenerateResponse(ctx, approachMetaPrompt, []domain.ChatMessage{
			{Role: "user", Content: payload},
			{Role: "assistant", Content: text},
			{Role: "user", Content: fmt.Sprintf(
				"Esta mensagem promete algo que quem envia não pode sustentar: %s. "+
					"Reescreva SEM prometer prazo, disponibilidade, resultado ou capacidade técnica, "+
					"e sem afirmar como fato o que não foi informado. Devolva SOMENTE a mensagem reescrita.",
				strings.Join(promises, ", "))},
		}, nil)
		if retryErr == nil && retry != nil {
			uc.meter.Record(ctx, accountID, "approach_draft_retry", effectivePlan.AIProvider, effectivePlan.AIModel, retry.Usage)
			if cleaned := strings.TrimSpace(retry.Content); cleaned != "" && !domain.HasApproachPromise(cleaned) {
				return cleaned, nil
			}
		}
		slog.WarnContext(ctx, "approach draft still contains a promise after one retry",
			"lead_id", lead.ID, "fragments", promises)
	}
	return text, nil
}

// brandLayers reads the company and seller layers for this draft.
//
// Best effort in both directions: an account with no brand kit drafts exactly
// as it did before, and a failed read degrades to the same. Refusing to draft
// because a configuration table was unreachable would be a worse product than
// drafting without the extra voice.
func (uc *ApproachUseCase) brandLayers(ctx context.Context, accountID string, lead *domain.Lead) (*domain.BrandKit, *domain.SellerOverlay) {
	if uc.brandRepo == nil {
		return nil, nil
	}
	brand, err := uc.brandRepo.Get(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "approach draft: could not read the brand kit", "error", err)
		return nil, nil
	}
	// The seller layer only applies when the lead HAS an owner: an unassigned
	// lead has no personal voice to speak in, and borrowing someone else's
	// signature would put a name on a message they did not send.
	var overlay *domain.SellerOverlay
	if lead.OwnerUserID != nil && *lead.OwnerUserID != "" {
		if o, err := uc.brandRepo.GetOverlay(ctx, accountID, *lead.OwnerUserID); err == nil {
			overlay = o
		}
	}
	return brand, overlay
}

// buildApproachPayload flattens everything the model needs into a plain text
// block: funnel persona, product, collected metadata, enrichment, contact name.
func (uc *ApproachUseCase) buildApproachPayload(ctx context.Context, accountID string, funnel *domain.SalesFunnel, lead *domain.Lead) string {
	var b strings.Builder
	// WHO IS SIGNING. Without a name the model fills the gap itself and
	// writes "Sou [Seu Nome], da AssistMed" — a raw placeholder that goes
	// out to a real customer the moment someone approves a batch without
	// reading every line. Three separate pilot users hit this; one measured
	// it in 5 of 8 drafts. The company name is what the message can honestly
	// be signed with, so it is stated rather than left to be guessed.
	if uc.accountRepo != nil {
		if account, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && account != nil {
			if name := strings.TrimSpace(account.Name); name != "" {
				fmt.Fprintf(&b, "Empresa que envia (assine com este nome): %s\n\n", name)
			}
		}
	}
	if p := strings.TrimSpace(funnel.PersonaPrompt); p != "" {
		fmt.Fprintf(&b, "Persona de vendas do funil:\n%s\n\n", p)
	}
	fmt.Fprintf(&b, "Funil: %s\n", funnel.Name)

	// The dossier is rendered FIRST because the matcher reads it: the pick
	// and the text must come from the same facts.
	rendered := ""
	if uc.dossier != nil {
		rendered = uc.dossier.Build(
			ctx, accountID, lead, uc.leadContact(ctx, accountID, lead),
		).Render()
	}

	// Product context. The funnel's product, when configured, is an explicit
	// choice and stays the anchor. Without one — the default funnel has none —
	// the matcher picks from the catalogue for THIS lead and the pick is
	// stamped on the lead (the caller's Upsert persists it), so the card and
	// the approval screen show the same product the draft is about.
	funnelProduct := funnel.ProductID != nil && *funnel.ProductID != ""
	if funnelProduct && uc.productRepo != nil {
		if product, err := uc.productRepo.GetByID(ctx, *funnel.ProductID); err == nil && product != nil && product.AccountID == accountID {
			b.WriteString(buildProductPersonaPayload(product, ""))
		}
	}
	if !funnelProduct && uc.matcher != nil {
		picks, objections, err := uc.matcher.Match(ctx, accountID, lead, rendered)
		if err != nil {
			slog.WarnContext(ctx, "offer matcher failed; drafting without a pick", "lead_id", lead.ID, "error", err)
		}
		if len(picks) > 0 {
			if raw, err := json.Marshal(picks); err == nil {
				lead.MatchedProducts = raw
			}
			b.WriteString(uc.matcher.Payload(ctx, accountID, picks))
		}
		// The predicted pushback rides the same stamp-and-render path as the
		// pick: persisted on the lead (the card and the WhatsApp queue show
		// it) and handed to the writer so the text can preempt it.
		if len(objections) > 0 {
			if raw, err := json.Marshal(objections); err == nil {
				lead.AnticipatedObjections = raw
			}
			b.WriteString("\nOBJEÇÕES PROVÁVEIS DESTE CLIENTE (antecipe no texto quando couber com naturalidade; nunca as liste todas):\n")
			for _, o := range objections {
				fmt.Fprintf(&b, "  - %s → %s\n", o.Label, o.Counter)
			}
		}
	}

	// Why yours rather than the one they already own.
	//
	// The account's OWN words: there is no seed for this and there must not
	// be, because a default differentiator is the product inventing an
	// advantage about somebody's equipment. Empty means the draft argues from
	// the product alone — which is what "genérico" meant before this existed.
	uc.writeDifferentiators(&b, accountID)

	// The rule that keeps a comparison from becoming an accusation.
	//
	// The dossier may carry the incumbent supplier, and only ever from the
	// public procurement record or the customer's own words — CNES publishes
	// equipment counts and never a brand. Everything else the model might
	// "know" about who serves this hospital is a guess, and a guess about a
	// supplier, in a message to the hospital that signed with them, ends the
	// conversation.
	b.WriteString("\nREGRA SOBRE CONCORRENTES: só cite um fornecedor atual se ele " +
		"aparecer nos fatos acima (registro público de contratos ou algo que o " +
		"próprio cliente disse). Nunca deduza, nunca chute a marca a partir do " +
		"parque instalado — o CNES informa quantidade, jamais fabricante. " +
		"Sem fonte, defenda seus diferenciais sem comparar com ninguém.\n")

	// Contact name, if known.
	if uc.contactRepo != nil {
		if contact := uc.leadContact(ctx, accountID, lead); contact != nil && contact.Name != nil && strings.TrimSpace(*contact.Name) != "" {
			fmt.Fprintf(&b, "Nome do contato: %s\n", strings.TrimSpace(*contact.Name))
		}
	}

	// Everything the account already knows, grouped by HOW it is known.
	//
	// This replaced two raw dumps: the collected fields and the enrichment blob,
	// each printed as `key: value` with no indication of where the value came
	// from. A registered activity code and a name a model read off a page arrived
	// with identical authority, so the draft would assert both — and asserting an
	// invented fact to somebody who knows better is the one thing a cold approach
	// cannot survive.
	//
	// The dossier also brings what those dumps never had: the account's own
	// co-purchase patterns with their evidence, this customer's purchase history
	// and reorder date, and the signals somebody recorded. All of it already
	// existed; none of it reached the message.
	if uc.dossier != nil {
		if rendered != "" {
			b.WriteString("\n")
			b.WriteString(rendered)
		}
	} else {
		// Unwired: exactly the previous payload, so an account that has not been
		// migrated drafts as it always did.
		if len(lead.CollectedMetadata) > 0 {
			var collected map[string]services.CollectedField
			if err := json.Unmarshal(lead.CollectedMetadata, &collected); err == nil && len(collected) > 0 {
				b.WriteString("Dados coletados do lead:\n")
				for k, v := range collected {
					if strings.TrimSpace(v.Value) != "" {
						fmt.Fprintf(&b, "  - %s: %s\n", k, v.Value)
					}
				}
			}
		}
		if len(lead.Enrichment) > 0 {
			var enrich map[string]any
			if err := json.Unmarshal(lead.Enrichment, &enrich); err == nil && len(enrich) > 0 {
				b.WriteString("Enriquecimento (dados do decisor/empresa):\n")
				for k, v := range enrich {
					fmt.Fprintf(&b, "  - %s: %v\n", k, v)
				}
			}
		}
	}

	return b.String()
}

// SetOfferMatcher wires the product pick into the draft. Nil keeps the
// previous behaviour exactly.
func (uc *ApproachUseCase) SetOfferMatcher(m *OfferMatcher) { uc.matcher = m }

// SetDossierBuilder wires the account's own knowledge into the draft.
//
// The pieces existed and never met: the generator saw the persona and a product
// while the same account already held the patterns, the history and the signals.
// Nil keeps the previous behaviour exactly.
func (uc *ApproachUseCase) SetDossierBuilder(b *LeadDossierBuilder) {
	uc.dossier = b
}

// checkBroadcastBudget enforces the monthly business-initiated message cap
// (same accounting as campaigns) before an approach is sent.
func (uc *ApproachUseCase) checkBroadcastBudget(ctx context.Context, accountID string) error {
	if uc.accountRepo == nil || uc.planRepo == nil {
		return nil
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && *account.PlanID != "" {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	broadcastCap := account.EffectiveMaxBroadcastMessagesPerMonth(plan)
	if broadcastCap <= 0 {
		return &domain.AppError{
			Code:    "BROADCAST_NOT_ALLOWED",
			Message: "your plan does not include outbound approaches — upgrade to enable",
		}
	}
	if uc.usageRepo != nil {
		usage, err := uc.usageRepo.GetByAccountID(ctx, accountID)
		if err != nil {
			return err
		}
		used := 0
		if usage != nil && usage.BroadcastMessagesUsed != nil {
			used = int(*usage.BroadcastMessagesUsed)
		}
		if used+1 > broadcastCap {
			return &domain.AppError{
				Code:    "BROADCAST_BUDGET_EXCEEDED",
				Message: fmt.Sprintf("monthly outbound budget exceeded: %d used of %d allowed", used, broadcastCap),
			}
		}
	}
	return nil
}

// ── Contact resolution (US-F005/F006) ───────────────────────────────────────
//
// Before the identity migration `lead.ContactID` WAS the phone number, so
// every send/opt-out/enrich path used it directly. It is now an opaque ULID,
// and the phone lives on the contact row (external_id / phone) and in
// contact_identities. These helpers are the single place that translates
// "which lead" into "how do I reach them".
//
// The fallback matters: a caller may still hand us a lead whose ContactID is
// a raw E.164 (a row written by an older binary mid-deploy, or a unit test
// with a hand-built lead). Treating that as the phone keeps those paths
// working instead of silently refusing to send.

// leadContact loads the contact behind a lead. Returns (nil, nil) when the
// contact row is gone — callers degrade rather than fail.
func (uc *ApproachUseCase) leadContact(ctx context.Context, accountID string, lead *domain.Lead) *domain.Contact {
	if uc.contactRepo == nil || lead == nil || lead.ContactID == "" {
		return nil
	}
	if c, err := uc.contactRepo.GetByID(ctx, lead.ContactID, accountID); err == nil && c != nil {
		return c
	}
	// Legacy shape: ContactID is the external id (the phone) itself.
	if c, err := uc.contactRepo.GetByExternalID(ctx, accountID, lead.ContactID); err == nil && c != nil {
		return c
	}
	return nil
}

// leadPhone returns the E.164 number to send to, or "" when the lead has no
// messageable phone (prospecting keys a phone-less lead on its Google
// place_id, which is not addressable).
func (uc *ApproachUseCase) leadPhone(ctx context.Context, accountID string, lead *domain.Lead) string {
	if contact := uc.leadContact(ctx, accountID, lead); contact != nil {
		if contact.Phone != nil {
			if phone, ok := domain.NormalizeE164(*contact.Phone); ok {
				return phone
			}
		}
		if phone, ok := domain.NormalizeE164(contact.ExternalID); ok {
			return phone
		}
	}
	if lead != nil {
		if phone, ok := domain.NormalizeE164(lead.ContactID); ok {
			return phone
		}
	}
	return ""
}

// leadOptedOut answers the opt-out question in the contact's own terms.
// contacts.opted_out_at is keyed by external_id, so the lookup needs the
// phone, never the lead's ULID.
func (uc *ApproachUseCase) leadOptedOut(ctx context.Context, accountID string, lead *domain.Lead) bool {
	if uc.contactRepo == nil {
		return false
	}
	if contact := uc.leadContact(ctx, accountID, lead); contact != nil {
		// The resolved contact carries its own flag. Re-asking by
		// external_id missed a person keyed by the OTHER address (opted out
		// by e-mail, keyed by phone) and answered "not opted out".
		return contact.OptedOutAt != nil
	}
	optedOut, _ := uc.contactRepo.IsOptedOut(ctx, accountID, lead.ContactID)
	return optedOut
}

// ── Email approval path (US-M009) ───────────────────────────────────────────

// approveSendEmail delivers an approved approach by email, under the
// deliverability guards.
//
// The guards run BEFORE the send and the counter is bumped AFTER it: a cap
// that counted attempts rather than deliveries would silently shrink the
// customer's budget every time their SMTP provider hiccuped.
func (uc *ApproachUseCase) approveSendEmail(
	ctx context.Context,
	accountID string,
	lead *domain.Lead,
	channel *domain.Channel,
	in ApproachSendInput,
	text string,
) (*domain.Lead, error) {
	if in.ViaBulk {
		// A batch must never silently fall back to the customer's mailbox —
		// that is the burn the hybrid model exists to prevent.
		if uc.bulkSender == nil {
			return nil, domain.ErrBulkSendingUnavailable
		}
	} else if uc.emailSender == nil {
		return nil, &domain.AppError{
			Code:    "EMAIL_SENDER_UNAVAILABLE",
			Message: "envio por e-mail indisponível: canal não conectado",
		}
	}
	to := uc.leadEmail(ctx, accountID, lead)
	if to == "" {
		return nil, domain.ErrLeadContactNotMessageable
	}

	// Checked before the deliverability guards, and deliberately so: this is
	// a fact about THE ADDRESS, not about how well this account is set up.
	// An address that bounced hard or reported us as spam must be refused
	// even on an account whose guards were never wired.
	//
	// A harder gate than opt-out, too: an opted-out person could still
	// receive mail, they asked not to. A suppressed address either does not
	// exist or told its provider we are abusive — writing to it again is
	// what costs the customer their sending reputation.
	//
	// Fails CLOSED, unlike the opt-out guard. There the inbound keyword is
	// the durable enforcement and the send-side check is belt-and-braces; a
	// suppression has no second enforcement point, so a lookup we cannot
	// perform must not be read as permission.
	if uc.suppressions != nil {
		suppressed, err := uc.suppressions.IsSuppressed(ctx, accountID, to)
		if err != nil {
			slog.ErrorContext(ctx, "suppression lookup failed — refusing the send",
				"account_id", accountID, "error", err)
			return nil, domain.ErrEmailAddressSuppressed
		}
		if suppressed {
			return nil, domain.ErrEmailAddressSuppressed
		}
	}

	if uc.mailboxRepo == nil {
		// No guard wiring means no protection, and sending unguarded cold
		// mail from a customer's domain is exactly the harm US-M007 exists
		// to prevent. Refuse rather than "degrade".
		return nil, &domain.AppError{
			Code:    "EMAIL_GUARDS_UNAVAILABLE",
			Message: "envio por e-mail indisponível: proteções de entregabilidade não configuradas",
		}
	}

	from := channelStringField(channel, "from_address")
	if from == "" {
		return nil, &domain.AppError{
			Code:    "CHANNEL_MISSING_CONFIG",
			Message: "canal de e-mail sem from_address configurado",
		}
	}
	mailbox, err := uc.mailboxRepo.EnsureForChannel(ctx, accountID, channel.ID, from)
	if err != nil {
		return nil, err
	}

	// An approach is a FIRST touch — cold by definition, so it faces every
	// gate (DNS, warmup, daily cap).
	//
	// Except through the bulk provider: there the provider's own
	// infrastructure carries the volume, so the per-mailbox warmup/cap gates
	// don't apply — the gate is the domain being registered AND verified
	// with the provider, which is what BulkReady answers.
	now := uc.now()
	if in.ViaBulk {
		if !mailbox.BulkReady() {
			return nil, domain.ErrBulkSendingNotReady
		}
	} else if decision := mailbox.CanSend(now, true); !decision.Allowed {
		switch decision.Reason {
		case domain.BlockDNSUnverified:
			return nil, domain.ErrEmailDomainNotVerified
		case domain.BlockWarmupUnknown:
			// NULL warmup fails CLOSED — see domain.BlockWarmupUnknown.
			return nil, domain.ErrEmailWarmupNotStarted
		default:
			// The refusal says how much and why: a bare "limite atingido"
			// is a wall; used/cap plus the warmup day is a plan.
			msg := fmt.Sprintf("limite diário desta caixa atingido (%d de %d hoje)",
				decision.Used, decision.Cap)
			if mailbox.IsWarming(now) {
				msg += fmt.Sprintf(" — caixa em aquecimento, dia %d de %d; o limite cresce a cada dia",
					mailbox.WarmupDay(now), domain.WarmupRampDays())
			}
			// Honest until scheduling lands (phase 1b): nothing sends by
			// itself tomorrow — the person approves again when the cap renews.
			msg += ". Amanhã o limite renova — aprove de novo"
			return nil, &domain.AppError{Code: domain.ErrEmailDailyCap.Code, Message: msg}
		}
	}

	if err := uc.checkBroadcastBudget(ctx, accountID); err != nil {
		return nil, err
	}

	subject := strings.TrimSpace(in.Subject)
	msg := &domain.Message{
		Type:      "text",
		Direction: "outbound",
		From:      "bot",
		Content:   &text,
		Metadata:  emailApproachMeta(subject, in),
	}
	sender := uc.emailSender
	if in.ViaBulk {
		sender = uc.bulkSender
	}
	// Claim BEFORE the provider call — see claimForSend.
	if err := uc.claimForSend(ctx, lead); err != nil {
		return nil, err
	}
	externalID, err := sender.SendMessageAndGetID(ctx, channel, to, msg)
	if err != nil {
		uc.releaseClaim(ctx, lead)
		// errors.As, not a type assertion: the adapter WRAPS the OAuth
		// refusal ("%w"), and a plain assertion turned a revoked Google
		// grant into a generic LEAD_SEND_FAILED.
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, &domain.AppError{Code: domain.ErrLeadSendFailed.Code, Message: fmt.Sprintf("%s: %v", domain.ErrLeadSendFailed.Message, err)}
	}
	// The sent offer becomes a message on the contact's conversation: the
	// timeline shows it, a bounce can find it, a reply threads onto it.
	uc.recordOutbound(ctx, accountID, channel, to, lead, msg, externalID, approachSender(in))

	// Counted only after a real delivery — and only against the mailbox's
	// daily cap when the mailbox actually carried it. A bulk send spends the
	// provider's reputation, not the mailbox's warmup budget.
	if !in.ViaBulk {
		if _, err := uc.mailboxRepo.RecordSend(ctx, mailbox.ID, now); err != nil {
			slog.WarnContext(ctx, "email approach: failed to record send against the daily cap",
				"mailbox_id", mailbox.ID, "error", err)
		}
	}
	if uc.usageRepo != nil {
		_ = uc.usageRepo.AddBroadcastMessages(ctx, accountID, 1)
	}
	uc.cases.ApproachSent(ctx, accountID, lead.ID, channel.Type)

	// Same bookkeeping the WhatsApp path does — it used to be skipped here,
	// so an email approval (the product's main channel) never closed its A/B
	// arm nor attributed its strategy. Before the draft field is overwritten,
	// for the same reason as there.
	uc.closeExperiment(ctx, lead.FunnelID, lead.ID, text, false)
	uc.recordAutopilotStreak(ctx, accountID, lead.ApproachDraft, text)
	uc.attributeStrategy(ctx, lead.ID, in)

	lead.ApproachDraft = text
	lead.Status = domain.LeadStatusContacted
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		return nil, err
	}
	return lead, nil
}

// leadEmail resolves the address to write to: the contact's own email, or
// any registered email identity.
func (uc *ApproachUseCase) leadEmail(ctx context.Context, accountID string, lead *domain.Lead) string {
	if contact := uc.leadContact(ctx, accountID, lead); contact != nil {
		if contact.Email != nil && domain.IsEmailAddress(*contact.Email) {
			return strings.ToLower(strings.TrimSpace(*contact.Email))
		}
		if domain.IsEmailAddress(contact.ExternalID) {
			return strings.ToLower(strings.TrimSpace(contact.ExternalID))
		}
	}
	if lead != nil && domain.IsEmailAddress(lead.ContactID) {
		return strings.ToLower(strings.TrimSpace(lead.ContactID))
	}
	return ""
}

// channelStringField reads one string out of a channel's config.
func channelStringField(channel *domain.Channel, key string) string {
	if channel == nil || channel.Config == nil {
		return ""
	}
	// Config arrives as raw JSON bytes from the repository and as a map from
	// in-process callers. The []byte case must be unmarshalled DIRECTLY:
	// json.Marshal of a []byte produces a base64 string, so the previous
	// marshal-then-unmarshal round trip turned every repo-loaded channel into
	// an empty map — and the email approval path answered CHANNEL_MISSING_CONFIG
	// for channels whose from_address was sitting right there in the row. Only
	// map-shaped unit-test fixtures ever passed. Same normalisation as
	// handlers.emailFromAddressOf, which had already learned this.
	var m map[string]any
	switch v := channel.Config.(type) {
	case []byte:
		if json.Unmarshal(v, &m) != nil {
			return ""
		}
	case json.RawMessage:
		if json.Unmarshal(v, &m) != nil {
			return ""
		}
	default:
		raw, err := json.Marshal(v)
		if err != nil || json.Unmarshal(raw, &m) != nil {
			return ""
		}
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

// SetEmailSender wires the email channel adapter for the approval path.
func (uc *ApproachUseCase) SetEmailSender(s ApproachSender) {
	uc.emailSender = s
}

// SetBulkSender wires the platform's bulk provider for BATCH email approvals
// (hybrid model). Optional: unwired, a ViaBulk send refuses with
// BULK_SENDING_UNAVAILABLE — it never falls back to the customer's mailbox,
// because a batch through a warming mailbox is exactly the burn the hybrid
// model exists to prevent.
func (uc *ApproachUseCase) SetBulkSender(s ApproachSender) {
	uc.bulkSender = s
}

// SetCaseRecorder wires case instrumentation (US-M011). Optional: unwired,
// approaches still send — they just are not counted.
func (uc *ApproachUseCase) SetCaseRecorder(r *CaseRecorder) {
	uc.cases = r
}

// SetAIMeter wires transparent token metering (US-M015). Unwired, drafts
// still work — they are just not billed, which is why main.go always wires
// it.
func (uc *ApproachUseCase) SetAIMeter(m *AIMeter) {
	uc.meter = m
}

// SetSpendCounter wires the per-plan AI-draft ceiling (K4).
//
// The counter is the AI cost ledger — the same rows the invoice line reads, so
// the cap and the bill can never disagree about how much was drafted.
func (uc *ApproachUseCase) SetSpendCounter(c domain.SpendCounter) {
	uc.spend = newSpendGate(uc.accountRepo, uc.planRepo, c)
	// The counter main.go wires IS the cost ledger; when it also satisfies
	// the ledger interface the customer's own monthly budget check lights
	// up with no extra wiring.
	if costs, ok := c.(domain.AICostRepository); ok {
		uc.spend.SetCostLedger(costs)
	}
}

// SetMailboxRepo wires the deliverability guards (US-M007). Optional setter
// for the same reason as the other seams; without it, email approvals are
// REFUSED rather than sent unguarded.
func (uc *ApproachUseCase) SetMailboxRepo(r domain.EmailMailboxRepository) {
	uc.mailboxRepo = r
}

// runningExperiment returns the funnel's live experiment and this lead's arm.
// Any failure degrades to "no experiment": measurement must never be able to
// stop a draft from being written.
func (uc *ApproachUseCase) runningExperiment(ctx context.Context, funnelID, leadID string) (*domain.ApproachExperiment, string) {
	if uc.experiments == nil {
		return nil, ""
	}
	exp, err := uc.experiments.GetRunningByFunnel(ctx, funnelID)
	if err != nil || exp == nil {
		return nil, ""
	}
	return exp, domain.AssignVariant(leadID, exp.SplitPctB)
}

// recordDraft files the generated text under its arm, with how many risks it
// carries. Risk count is measurable at draft time — no waiting for anyone to
// approve — so a variant that produces fewer warnings shows up immediately.
func (uc *ApproachUseCase) recordDraft(ctx context.Context, exp *domain.ApproachExperiment, variant, leadID, text string) {
	if exp == nil || uc.experiments == nil {
		return
	}
	err := uc.experiments.UpsertResult(ctx, &domain.ApproachVariantResult{
		ID:           ulid.Make().String(),
		ExperimentID: exp.ID,
		LeadID:       leadID,
		Variant:      variant,
		DraftedText:  text,
		RiskCount:    len(domain.ApproachFindings(text)),
	})
	if err != nil {
		slog.WarnContext(ctx, "approach A/B: could not record draft", "error", err)
	}
}

// closeExperiment records what the human did with the draft.
//
// The comparison is against the text AS GENERATED, read back from the
// experiment row — not from the lead, whose draft field is overwritten on
// approval. Reading it there would compare the sent text with itself and
// score every approval as unedited, which is the one number this whole
// experiment exists to measure.
func (uc *ApproachUseCase) closeExperiment(ctx context.Context, funnelID, leadID, sent string, discarded bool) {
	if uc.experiments == nil {
		return
	}
	exp, err := uc.experiments.GetRunningByFunnel(ctx, funnelID)
	if err != nil || exp == nil {
		return
	}
	outcome := domain.ApproachOutcomeDiscarded
	var ratio float32
	if !discarded {
		res, err := uc.experiments.GetResult(ctx, exp.ID, leadID)
		if err != nil || res == nil {
			return
		}
		ratio = domain.EditRatio(res.DraftedText, sent)
		outcome = domain.ApproachOutcomeUnedited
		if ratio > 0 {
			outcome = domain.ApproachOutcomeEdited
		}
	}
	if err := uc.experiments.SetOutcome(ctx, exp.ID, leadID, outcome, ratio); err != nil {
		slog.WarnContext(ctx, "approach A/B: could not close result", "error", err)
	}
}

// LeadStrategyAttributor stamps a lead with the strategy that reached it.
//
// A narrow port rather than the full StrategyRepository: this use case only
// ever writes the attribution, and widening the dependency would drag every
// approach test into knowing about strategy CRUD.
type LeadStrategyAttributor interface {
	AttributeLead(ctx context.Context, leadID, strategyID string, version int) error
}

// SetStrategyAttributor wires strategy attribution (E3).
func (uc *ApproachUseCase) SetStrategyAttributor(a LeadStrategyAttributor) {
	uc.strategies = a
}

// SetAutopilotRepo wires the progressive-autopilot streak (§3.4).
func (uc *ApproachUseCase) SetAutopilotRepo(r domain.AutopilotRepository) {
	uc.autopilot = r
}

// attributeStrategy stamps which strategy version reached this lead (E3).
//
// Written on the SEND rather than on the draft: a draft that was discarded
// never reached anybody, and counting it would credit a strategy with an
// approach that does not exist. Write-once at the repository, so a second
// send under a newer strategy cannot rewrite the history of the first.
//
// Best effort — this runs after the message is out, and losing an
// attribution costs a row in a report, never a message.
func (uc *ApproachUseCase) attributeStrategy(ctx context.Context, leadID string, in ApproachSendInput) {
	if uc.strategies == nil || strings.TrimSpace(in.StrategyID) == "" {
		return
	}
	version := in.StrategyVersion
	if version <= 0 {
		version = 1
	}
	if err := uc.strategies.AttributeLead(ctx, leadID, in.StrategyID, version); err != nil {
		slog.WarnContext(ctx, "could not attribute the lead to its strategy",
			"lead_id", leadID, "strategy_id", in.StrategyID, "error", err)
	}
}

// recordAutopilotStreak moves the counter that earns the autopilot offer.
//
// An approval sent EXACTLY as drafted advances it; any edit resets it to
// zero. The streak is evidence that the model writes what this person would
// have written, and an edit is that evidence expiring — so a single reset
// method, never an increment the caller could forget to pair with a reset.
//
// Best effort: this runs after the message has already gone out, and losing
// a counter must not surface as a failed send.
func (uc *ApproachUseCase) recordAutopilotStreak(ctx context.Context, accountID, drafted, sent string) {
	if uc.autopilot == nil {
		return
	}
	unedited := strings.TrimSpace(drafted) != "" &&
		strings.TrimSpace(drafted) == strings.TrimSpace(sent)
	if err := uc.autopilot.RecordApproval(ctx, accountID, unedited); err != nil {
		slog.WarnContext(ctx, "could not record the autopilot streak",
			"account_id", accountID, "error", err)
	}
}

// SetBrandRepo wires the company and seller voice layers.
func (uc *ApproachUseCase) SetBrandRepo(r domain.BrandKitRepository) { uc.brandRepo = r }

// SetSuppressionRepo wires the do-not-send list.
func (uc *ApproachUseCase) SetSuppressionRepo(r domain.EmailSuppressionRepository) {
	uc.suppressions = r
}

// emailApproachMeta builds the outbound metadata for an email approach.
//
// The template id travels ON THE MESSAGE rather than being recorded here so
// the reply side can find it: a reply arrives days later on the inbound
// webhook with nothing but a threading header, and the sent row is the only
// place that header can be resolved back to a template.
func emailApproachMeta(subject string, in ApproachSendInput) map[string]any {
	meta := map[string]any{"subject": subject}
	if id := strings.TrimSpace(in.EmailTemplateID); id != "" {
		version := in.EmailTemplateVersion
		if version <= 0 {
			version = 1
		}
		meta["template_id"] = id
		meta["template_version"] = version
	}
	return meta
}

// QueueReorderApproach puts a repurchase suggestion into the approval queue
// (§6.5, F3).
//
// It writes a lead in pending_approval with the draft already filled, which
// means a repurchase goes through the SAME human gate as a cold approach.
// That is the point: a machine that decides on its own to ask a hospital to
// buy more gloves is one wrong prediction away from a customer who stops
// trusting anything the system says.
//
// An existing lead for this contact is REUSED rather than duplicated — the
// customer is one relationship, not one row per reorder — and a lead already
// mid-conversation is left alone, because dropping a draft on top of a live
// negotiation is worse than not suggesting anything.
func (uc *ApproachUseCase) QueueReorderApproach(ctx context.Context, accountID, contactID, text string) error {
	_, err := uc.queueSuggested(ctx, accountID, contactID, "reorder", text)
	return err
}

// queueSuggested is the shared body behind every machine-drafted approach to
// an EXISTING customer (repurchase, greeting, check-in, maintenance).
//
// Returns the lead it queued, or nil when it deliberately queued nothing —
// the caller must treat a nil lead as "leave it alone", never as an error.
func (uc *ApproachUseCase) queueSuggested(ctx context.Context, accountID, contactID, source, text string) (*domain.Lead, error) {
	if uc.funnelRepo == nil || uc.leadRepo == nil {
		return nil, &domain.AppError{
			Code: "SALES_FUNNEL_UNAVAILABLE", Message: "funil de vendas não configurado"}
	}
	funnels, err := uc.funnelRepo.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// The DEFAULT funnel, when the account has one: it is where imports and
	// finds land when nobody picked a funnel, so a suggested draft belongs
	// there too. The list is newest-first, so "first active" used to mean
	// "whichever funnel was created last" — a reorder draft for a
	// long-standing customer landed in a funnel somebody made yesterday.
	var funnel *domain.SalesFunnel
	for _, f := range funnels {
		if f != nil && f.Active && f.IsDefault {
			funnel = f
			break
		}
	}
	if funnel == nil {
		for _, f := range funnels {
			if f != nil && f.Active {
				funnel = f
				break
			}
		}
	}
	if funnel == nil {
		// No funnel means no queue to put this in. Reported rather than
		// silently dropped: an account with sales and no funnel is a setup
		// problem somebody can fix.
		return nil, domain.ErrSalesFunnelNotFound
	}

	existing, err := uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contactID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		switch existing.Status {
		case domain.LeadStatusPendingApproval, domain.LeadStatusContacted,
			domain.LeadStatusResponded, domain.LeadStatusQualified:
			// Already being worked. Adding a draft on top would put two
			// different conversations in front of the same salesperson
			// about the same customer.
			return nil, nil
		}
		existing.ApproachDraft = text
		existing.Status = domain.LeadStatusPendingApproval
		// Source is NOT overwritten. It records where the lead came from —
		// prospecting, a signal, an import — and a reorder draft is not a new
		// origin. Rewriting it would quietly destroy the attribution the
		// strategy and case reports are built on.
		if err := uc.leadRepo.Upsert(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	lead := &domain.Lead{
		ID:                ulid.Make().String(),
		AccountID:         accountID,
		FunnelID:          funnel.ID,
		ContactID:         contactID,
		Status:            domain.LeadStatusPendingApproval,
		Source:            source,
		ApproachDraft:     text,
		CollectedMetadata: json.RawMessage("{}"),
		Enrichment:        json.RawMessage("{}"),
	}
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		return nil, err
	}
	return lead, nil
}

// QueueRelationshipApproach queues a relationship routine and, for a greeting
// on an account running autopilot, sends it (§6.5, F4).
//
// The exception is narrow and every part of it is enforced here rather than
// in the worker:
//
//   - lowRisk comes from domain.GreetingIsLowRisk — a maintenance reminder
//     implies a service the company may not be able to schedule, and a
//     check-in is a sales message wearing a friendly hat. Neither qualifies.
//   - the send goes through ApproveSend, the SAME path a human approval
//     takes, so opt-out, suppression, plan budget and deliverability caps all
//     still apply. An automated send that skipped any of them would be a
//     second, weaker set of rules for exactly the traffic nobody is watching.
//   - domain.Autopilot.CanAutoSendGreeting keeps the email-only rule and the
//     shared daily cap.
//
// Anything that refuses simply LEAVES the draft in the queue, which is the
// product's normal state — a greeting nobody approved is a greeting that does
// not go out, not an error.
func (uc *ApproachUseCase) QueueRelationshipApproach(
	ctx context.Context, accountID, contactID, source, subject, text string, lowRisk bool,
) error {
	// Queue only. This used to auto-send "low risk" greetings when the
	// account's autopilot was enabled — the one path in the product that
	// wrote to a customer without a person approving it, contradicting the
	// rule the product is sold on ("nada sai sem você"). Removed 2026-09-01;
	// the parameters stay so callers (and the seeded strategies) need no
	// change. lowRisk and subject are now informational only.
	_, _ = lowRisk, subject
	_, err := uc.queueSuggested(ctx, accountID, contactID, source, text)
	return err
}

// funnelChannelType resolves what an account's active funnel sends through.
func (uc *ApproachUseCase) funnelChannelType(ctx context.Context, accountID, funnelID string) (string, error) {
	if uc.funnelRepo == nil || uc.channelRepo == nil {
		return "", nil
	}
	funnel, err := uc.funnelRepo.GetByID(ctx, funnelID)
	if err != nil || funnel == nil || funnel.ChannelID == nil || *funnel.ChannelID == "" {
		return "", err
	}
	channel, err := uc.channelRepo.GetByID(ctx, *funnel.ChannelID)
	if err != nil || channel == nil || channel.AccountID != accountID {
		return "", err
	}
	return channel.Type, nil
}

// accountTimezone reads where the customer actually is.
//
// The daily cap is a promise to the person who turned autopilot on — "no more
// than N while I am away" — and a day is their day, not the server's. An
// account in Lisbon whose cap rolled over at Brazilian midnight would get its
// reset in the middle of the working afternoon.
//
// Best effort: a lookup failure falls back to UTC rather than blocking the
// send path over a timezone.
func (uc *ApproachUseCase) accountTimezone(ctx context.Context, accountID string) string {
	if uc.accountRepo == nil {
		return ""
	}
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return ""
	}
	return account.Timezone
}

// clock is uc.now with a fallback, so a use case built by a test that skipped
// the constructor does not panic on a nil func.
func (uc *ApproachUseCase) clock() time.Time {
	if uc.now == nil {
		return time.Now()
	}
	return uc.now()
}

// DraftBatchForAccount is the overnight entry point (§6.17, K2).
//
// Deliberately a thin wrapper over DraftBatch rather than a second drafting
// path: the opt-out filtering, the strategy attribution and the per-plan spend
// cap all live in there, and a parallel implementation is a second place for
// each of them to be forgotten.
//
// An empty funnelID means every active funnel, which is the common case for an
// account with one. The nightly cap is spent across them in listing order and
// stops the moment it runs out — a night that drafts forty for the first funnel
// and nothing for the second is the honest reading of "40 per night", and
// splitting the cap evenly would leave every funnel with a handful.
func (uc *ApproachUseCase) DraftBatchForAccount(
	ctx context.Context, accountID, funnelID string, limit int,
) (*domain.BatchDraftOutcome, error) {
	total := &domain.BatchDraftOutcome{}
	var err error
	funnelIDs := []string{funnelID}
	if strings.TrimSpace(funnelID) == "" {
		funnels, listErr := uc.funnelRepo.ListByAccountID(ctx, accountID)
		if listErr != nil {
			return nil, listErr
		}
		funnelIDs = funnelIDs[:0]
		for _, f := range funnels {
			if f.Active {
				funnelIDs = append(funnelIDs, f.ID)
			}
		}
	}

	for _, id := range funnelIDs {
		remaining := limit - total.Drafted
		if remaining <= 0 {
			break
		}
		res, batchErr := uc.DraftBatch(ctx, accountID, id, remaining)
		if batchErr != nil {
			// One funnel failing must not lose the work the others did. The
			// error is returned so the run is recorded as partial, but the
			// counters already collected still stand.
			err = batchErr
			if total.StoppedReason == "" {
				total.StoppedReason = batchErr.Error()
			}
			continue
		}
		total.Drafted += res.Drafted
		total.SkippedOptedOut += res.SkippedOptedOut
		total.SkippedBudget += res.SkippedBudget
		total.SkippedError += res.SkippedError
		// The FIRST reason wins: it is the one that explains the most, and
		// a later funnel failing the same way adds nothing to read.
		if total.StoppedReason == "" {
			total.StoppedReason = res.StoppedReason
		}
	}
	// A night that drafted something is not a blocked night, whatever went
	// wrong along the way — the card must not tell somebody with forty
	// offers waiting that nothing could be written.
	if total.Drafted > 0 {
		total.StoppedReason = ""
	}
	return total, err
}

// writeDifferentiators renders the account's "why ours" library.
//
// Company-wide entries first (assistance in the state, parts in stock apply to
// everything it sells), then the product-specific ones — the order a seller
// would say them in.
// SetDifferentiators wires the "why ours" library.
func (uc *ApproachUseCase) SetDifferentiators(r domain.DifferentiatorRepository) {
	uc.differentiators = r
}

func (uc *ApproachUseCase) writeDifferentiators(b *strings.Builder, accountID string) {
	if uc.differentiators == nil {
		return
	}
	items, err := uc.differentiators.List(accountID)
	if err != nil {
		slog.Warn("approach: differentiators read failed", "account_id", accountID, "error", err)
		return
	}
	if len(items) == 0 {
		return
	}
	if len(items) > maxDifferentiatorsInPrompt {
		items = items[:maxDifferentiatorsInPrompt]
	}
	b.WriteString("\nSEUS DIFERENCIAIS (palavras da própria empresa — use os que " +
		"couberem neste cliente; não invente outros):\n")
	for _, d := range items {
		line := "  - " + strings.TrimSpace(d.Label)
		if detail := strings.TrimSpace(d.Detail); detail != "" {
			line += ": " + detail
		}
		b.WriteString(line + "\n")
	}
}

// maxDifferentiatorsInPrompt bounds the block. A message that recites twelve
// advantages persuades nobody and reads as a brochure.
const maxDifferentiatorsInPrompt = 8
