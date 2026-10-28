package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// LeadUseCase covers the account-scoped reads and mutations the dashboard
// needs for the outbound-sales lead/CRM panel (US-015): listing a funnel's
// leads, fetching a single lead, manually editing collected metadata
// (source:'human', US-014), and moving a lead through the pipeline status.
//
// Ownership is enforced on every operation: funnel-scoped lists verify the
// funnel's account_id, and per-lead operations verify the lead's account_id —
// against the caller's account_id, so one account can never read or mutate
// another's leads.
type LeadUseCase struct {
	// experiments is optional (US-F216): counts discards against the arm.
	experiments domain.ApproachExperimentRepository
	leadRepo    domain.LeadRepository
	funnelRepo  domain.SalesFunnelRepository
	// lifecycle emits lead.won (J2). Optional.
	lifecycle LifecycleEmitter
	// cases records pipeline facts for the case report (US-M011). Nil is a
	// valid state — see CaseRecorder.
	cases *CaseRecorder
	// contactRepo is optional (SetContactRepo) and only needed by Create,
	// which must upsert the contact row leads.contact_id references.
	contactRepo domain.ContactRepository
	// identityRepo indexes the contact's reachable identifiers (US-F005).
	// Optional (SetIdentityRepo) — unwired means no index write, never a
	// failed lead creation.
	identityRepo domain.ContactIdentityRepository
	// auditRepo records ownership changes (US-F010). Optional.
	auditRepo domain.AuditLogRepository
	// stageRepo backs the Kanban board (US-M008). Optional.
	stageRepo domain.FunnelStageRepository
	// userRepo confirms the assigned owner belongs to this account. Optional:
	// without it AssignOwner refuses rather than accepting unchecked — see the
	// comment there.
	userRepo domain.UserRepository
}

// SetUserRepo liga a checagem do usuário de destino em AssignOwner.
func (uc *LeadUseCase) SetUserRepo(repo domain.UserRepository) {
	uc.userRepo = repo
}

func NewLeadUseCase(leadRepo domain.LeadRepository, funnelRepo domain.SalesFunnelRepository) *LeadUseCase {
	return &LeadUseCase{leadRepo: leadRepo, funnelRepo: funnelRepo}
}

// SetContactRepo wires the optional contact repository Create depends on.
// nil leaves Create returning ErrLeadCreateUnavailable; main.go injects the
// Postgres implementation.
func (uc *LeadUseCase) SetContactRepo(repo domain.ContactRepository) {
	uc.contactRepo = repo
}

// CreateLeadDTO carries a manual lead registration: the target funnel, the
// contact's name + E.164 phone, and optional initial values for fields
// declared in the funnel's metadata schema.
type CreateLeadDTO struct {
	AccountID string
	FunnelID  string
	// Name is the PERSON (the buyer, the director); Company is the
	// establishment. In this product a lead is a company with somebody in
	// it, and a hospital's purchasing manager reached by e-mail is as real a
	// lead as a clinic owner on WhatsApp — which is why Phone is no longer
	// required when Email is given (2026-08-19).
	Name     string
	Company  string
	Role     string
	Email    string
	Phone    string
	City     string
	Metadata map[string]string
}

// Create registers a lead manually on a funnel. The phone (else the e-mail)
// doubles as the contact key (external_id, mirroring prospecting and the
// customer import) so the lead is immediately messageable through the
// approach flow; both are registered as identities so the next message from
// this person lands here. Company, role and city seed the lead's enrichment
// — the same keys the researcher fills — so the approach can cite them.
// Metadata keys must be declared in the funnel schema — unknown keys are
// rejected like UpdateMetadataField — and are stored with source:'human'.
// Idempotent: an existing (funnel, contact) lead is returned untouched with
// created=false. Ownership of the funnel is enforced against accountID.
func (uc *LeadUseCase) Create(ctx context.Context, dto CreateLeadDTO) (lead *domain.Lead, created bool, err error) {
	if uc.contactRepo == nil {
		return nil, false, domain.ErrLeadCreateUnavailable
	}

	funnel, err := uc.funnelRepo.GetByID(ctx, dto.FunnelID)
	if err != nil {
		return nil, false, err
	}
	if funnel == nil {
		return nil, false, domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != dto.AccountID {
		return nil, false, domain.ErrUnauthorized
	}

	// A human typed this into a form — accept their country's format. A
	// phone that is present but unparseable is refused (a typo, not a
	// choice); an absent phone is fine when there is an e-mail.
	var phone string
	if strings.TrimSpace(dto.Phone) != "" {
		p, ok := domain.NormalizeTypedPhone(dto.Phone)
		if !ok {
			return nil, false, domain.ErrLeadPhoneInvalid
		}
		phone = p
	}
	email := strings.ToLower(strings.TrimSpace(dto.Email))
	if email != "" {
		probe := &domain.ContactIdentity{
			AccountID: dto.AccountID, ContactID: "probe",
			Kind: domain.IdentityKindEmail, Value: email,
		}
		if probe.Validate() != nil {
			return nil, false, domain.ErrLeadEmailInvalid
		}
		email = probe.Value
	}
	if phone == "" && email == "" {
		return nil, false, domain.ErrLeadNeedsPhoneOrEmail
	}

	collected, err := uc.collectInitialMetadata(funnel, dto.Metadata)
	if err != nil {
		return nil, false, err
	}

	// US-F005: contacts carry an opaque ULID; external_id is the ONE key
	// every door uses (domain.ContactKey: phone digits, else the e-mail).
	// The resolver looks the person up by either address — under any
	// spelling an older door wrote — before creating anyone, so a customer
	// imported from a spreadsheet and typed here again is one contact. A
	// found contact is completed, never renamed: a name learned from the
	// channel outranks one typed into this form, and renaming would change
	// the confirmation of a message already waiting for approval.
	key, _ := domain.ContactKey(phone, email)
	channelType := "whatsapp"
	if phone == "" {
		channelType = "email"
	}
	contact := &domain.Contact{
		ID:          ulid.Make().String(),
		AccountID:   dto.AccountID,
		ExternalID:  key,
		ChannelType: &channelType,
	}
	if phone != "" {
		contact.Phone = &phone
	}
	if email != "" {
		contact.Email = &email
	}
	if name := displayNameFor(dto.Name, dto.Company); name != "" {
		contact.Name = &name
	}
	if company := strings.TrimSpace(dto.Company); company != "" {
		contact.CompanyName = &company
	}
	saved, _, err := newContactResolver(uc.contactRepo, uc.identityRepo).
		resolve(ctx, contact, contactResolveOpts{})
	if err != nil {
		return nil, false, err
	}

	// Dedup mirrors prospecting: a lead already tied to this (funnel, contact)
	// pair is returned as-is so re-running a setup never resets its pipeline.
	if existing, err := uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, saved.ID); err == nil && existing != nil {
		return existing, false, nil
	}

	lead = &domain.Lead{
		ID:                ulid.Make().String(),
		AccountID:         dto.AccountID,
		FunnelID:          funnel.ID,
		ContactID:         saved.ID,
		Status:            domain.LeadStatusNew,
		Source:            "manual",
		CollectedMetadata: collected,
		Enrichment:        manualEnrichment(dto),
	}
	if err := uc.leadRepo.Upsert(ctx, lead); err != nil {
		return nil, false, err
	}
	uc.cases.LeadCreated(ctx, dto.AccountID, lead.ID, lead.Source)
	return lead, true, nil
}

// collectInitialMetadata validates the initial field values against the
// funnel schema and wraps them as human-sourced collected fields.
func (uc *LeadUseCase) collectInitialMetadata(funnel *domain.SalesFunnel, metadata map[string]string) (json.RawMessage, error) {
	if len(metadata) == 0 {
		return json.RawMessage("{}"), nil
	}
	fields, err := funnel.MetadataFields()
	if err != nil {
		return nil, err
	}
	collected := make(map[string]services.CollectedField, len(metadata))
	for k, v := range metadata {
		key := strings.TrimSpace(k)
		if !fieldInSchema(fields, key) {
			return nil, domain.ErrLeadFieldNotInSchema
		}
		collected[key] = services.CollectedField{
			Value:     strings.TrimSpace(v),
			Source:    services.CollectedSourceHuman,
			UpdatedAt: time.Now(),
		}
	}
	blob, err := json.Marshal(collected)
	if err != nil {
		return nil, err
	}
	return blob, nil
}

// ListByFunnel returns the funnel's leads, optionally filtered by status.
// The funnel is verified to belong to accountID first so a caller can't
// enumerate another account's leads by guessing funnel ids.
func (uc *LeadUseCase) ListByFunnel(ctx context.Context, accountID, funnelID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	if status != "" && !status.IsValid() {
		return nil, domain.ErrInvalidLeadStatus
	}
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
	return uc.leadRepo.ListByFunnel(ctx, funnelID, status, limit, offset)
}

// Get returns a single lead after verifying it belongs to accountID.
func (uc *LeadUseCase) Get(ctx context.Context, accountID, id string) (*domain.Lead, error) {
	lead, err := uc.leadRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, domain.ErrLeadNotFound
	}
	if lead.AccountID != accountID {
		return nil, domain.ErrUnauthorized
	}
	return lead, nil
}

// UpdateMetadataField sets a single collected-metadata field on a lead with
// source:'human' (US-014). The field key must exist in the funnel's metadata
// schema — an unknown key is rejected rather than silently creating a new
// field, mirroring the AI collection guard. Returns the updated lead.
func (uc *LeadUseCase) UpdateMetadataField(ctx context.Context, accountID, id, fieldKey, value string) (*domain.Lead, error) {
	lead, err := uc.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}

	key := strings.TrimSpace(fieldKey)
	if key == "" {
		return nil, domain.ErrLeadFieldNotInSchema
	}

	// The field must be declared on the lead's funnel. Reject unknown keys so
	// a typo (or a stale UI) can't accumulate junk fields the AI never reads.
	funnel, err := uc.funnelRepo.GetByID(ctx, lead.FunnelID)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	fields, err := funnel.MetadataFields()
	if err != nil {
		return nil, err
	}
	if !fieldInSchema(fields, key) {
		return nil, domain.ErrLeadFieldNotInSchema
	}

	collected := map[string]services.CollectedField{}
	if len(lead.CollectedMetadata) > 0 {
		// Best-effort decode: a malformed prior blob starts fresh rather than
		// blocking the human's correction.
		_ = json.Unmarshal(lead.CollectedMetadata, &collected)
	}
	collected[key] = services.CollectedField{
		Value:     strings.TrimSpace(value),
		Source:    services.CollectedSourceHuman,
		UpdatedAt: time.Now(),
	}
	blob, err := json.Marshal(collected)
	if err != nil {
		return nil, err
	}
	if err := uc.leadRepo.UpdateMetadata(ctx, lead.ID, blob, lead.Enrichment); err != nil {
		return nil, err
	}
	lead.CollectedMetadata = blob
	return lead, nil
}

// UpdateStatus moves a lead to a new pipeline status after verifying ownership
// and that the status is one of the allowed values.
func (uc *LeadUseCase) UpdateStatus(ctx context.Context, accountID, id string, status domain.LeadStatus) (*domain.Lead, error) {
	if !status.IsValid() {
		return nil, domain.ErrInvalidLeadStatus
	}
	lead, err := uc.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	// Leaving the approval queue WITHOUT sending is a discard, and the A/B
	// has to hear about it. Left unrecorded, a discarded draft would simply
	// drop out of the denominator — so a variant whose drafts people throw
	// away would look indistinguishable from one nobody had reached yet.
	if lead.Status == domain.LeadStatusPendingApproval && status != domain.LeadStatusContacted {
		uc.discardExperiment(ctx, lead.FunnelID, lead.ID)
	}
	previous := lead.Status
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, status); err != nil {
		return nil, err
	}
	lead.Status = status

	// J2: only on the TRANSITION into won. A board where somebody drags a card
	// out of "Ganhos" and back would otherwise fire the event twice for one deal.
	if previous != domain.LeadStatusWon && status == domain.LeadStatusWon {
		emitLifecycle(uc.lifecycle, accountID, domain.EventLeadWon, domain.LifecyclePayload{
			ID:        lead.ID,
			ContactID: lead.ContactID,
			Confirmed: true,
		})
	}
	return lead, nil
}

// SetLifecycleEmitter wires the lead.won webhook (J2). Optional.
func (uc *LeadUseCase) SetLifecycleEmitter(e LifecycleEmitter) {
	uc.lifecycle = e
}

// SetExperimentRepo wires the approach A/B so discards are counted. Optional:
// nil means the tally simply does not see them.
func (uc *LeadUseCase) SetExperimentRepo(r domain.ApproachExperimentRepository) {
	uc.experiments = r
}

func (uc *LeadUseCase) discardExperiment(ctx context.Context, funnelID, leadID string) {
	if uc.experiments == nil {
		return
	}
	exp, err := uc.experiments.GetRunningByFunnel(ctx, funnelID)
	if err != nil || exp == nil {
		return
	}
	if err := uc.experiments.SetOutcome(ctx, exp.ID, leadID, domain.ApproachOutcomeDiscarded, 0); err != nil {
		slog.WarnContext(ctx, "approach A/B: could not record discard", "error", err)
	}
}

func fieldInSchema(fields []domain.MetadataFieldDef, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

// registerPhoneIdentity records the contact's phone in contact_identities
// (US-F005). Best-effort: the identity table is a resolver index, and a
// write failure must never fail the lead creation that already committed.
// A nil repository (unwired in tests / older wiring) simply skips.
// displayNameFor is what the contact is called: the person when there is one,
// the company otherwise.
func displayNameFor(person, company string) string {
	if n := strings.TrimSpace(person); n != "" {
		return n
	}
	return strings.TrimSpace(company)
}

// manualEnrichment seeds the lead's enrichment with what the person typed,
// under the SAME keys the researcher fills (company_name, decision_maker_*,
// address) — so the kanban card, the approval screen and the draft read it
// without knowing where it came from. Empty fields are omitted.
func manualEnrichment(dto CreateLeadDTO) json.RawMessage {
	m := map[string]string{}
	if v := strings.TrimSpace(dto.Company); v != "" {
		m["company_name"] = v
	}
	if v := strings.TrimSpace(dto.Name); v != "" {
		m["decision_maker_name"] = v
	}
	if v := strings.TrimSpace(dto.Role); v != "" {
		m["decision_maker_role"] = v
	}
	if v := strings.TrimSpace(dto.City); v != "" {
		m["address"] = v
	}
	if len(m) == 0 {
		return json.RawMessage("{}")
	}
	m["source"] = "manual"
	raw, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func (uc *LeadUseCase) registerPhoneIdentity(ctx context.Context, contact *domain.Contact, phone string) {
	if uc.identityRepo == nil || contact == nil || phone == "" {
		return
	}
	if _, err := uc.identityRepo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: contact.AccountID,
		ContactID: contact.ID,
		Kind:      domain.IdentityKindPhone,
		Value:     phone,
	}); err != nil {
		slog.WarnContext(ctx, "lead: failed to register phone identity",
			"contact_id", contact.ID, "error", err)
	}
}

// SetIdentityRepo wires the contact-identity index (US-F005). Optional
// setter so the constructor and its test call sites stay untouched.
func (uc *LeadUseCase) SetIdentityRepo(r domain.ContactIdentityRepository) {
	uc.identityRepo = r
}

// ── Resource-scoped access (US-F009) ────────────────────────────────────────
//
// The account check above answers "is this lead in my company?". Inside the
// company a vendedor must only reach THEIR pipeline, and that question is
// answered in exactly one place — domain.CanAccessResource — so an MCP tool
// can never be more permissive than the HTTP endpoint it mirrors.

// ownership projects a lead onto the authorization-relevant fields.
func leadOwnership(lead *domain.Lead) domain.ResourceOwnership {
	owner := ""
	if lead.OwnerUserID != nil {
		owner = *lead.OwnerUserID
	}
	visibility := lead.Visibility
	if !visibility.Valid() {
		visibility = domain.VisibilityTeam
	}
	return domain.ResourceOwnership{
		AccountID:   lead.AccountID,
		OwnerUserID: owner,
		Visibility:  visibility,
	}
}

// GetForActor is Get plus the per-resource check. Handlers and MCP tools
// should call THIS; the bare Get stays for internal callers that already
// established authority (workers, the inbound pipeline).
func (uc *LeadUseCase) GetForActor(ctx context.Context, actor domain.Actor, id string) (*domain.Lead, error) {
	lead, err := uc.Get(ctx, actor.AccountID, id)
	if err != nil {
		return nil, err
	}
	if !domain.CanAccessResource(actor, leadOwnership(lead)) {
		return nil, domain.ErrForbidden
	}
	uc.attachContactLabels(ctx, actor.AccountID, []*domain.Lead{lead})
	return lead, nil
}

// MutableForActor loads a lead the actor is allowed to CHANGE. Stricter than
// reading: a vendedor may look at a teammate's team-visible lead but never
// edit it.
func (uc *LeadUseCase) MutableForActor(ctx context.Context, actor domain.Actor, id string) (*domain.Lead, error) {
	lead, err := uc.Get(ctx, actor.AccountID, id)
	if err != nil {
		return nil, err
	}
	if !domain.CanMutateResource(actor, leadOwnership(lead)) {
		return nil, domain.ErrForbidden
	}
	return lead, nil
}

// ListByFunnelForActor returns the funnel's pipeline as this actor may see
// it. For a manager or above that is everything; for a vendedor the filter
// runs IN SQL, so pagination cannot silently drop their leads past the page
// limit (the same bug the conversation `assigned` filter once had).
func (uc *LeadUseCase) ListByFunnelForActor(ctx context.Context, actor domain.Actor, funnelID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	if status != "" && !status.IsValid() {
		return nil, domain.ErrInvalidLeadStatus
	}
	funnel, err := uc.funnelRepo.GetByID(ctx, funnelID)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		return nil, domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != actor.AccountID {
		return nil, domain.ErrUnauthorized
	}
	var leads []*domain.Lead
	if domain.RoleSatisfies(actor.Role, domain.RoleGestor) {
		leads, err = uc.leadRepo.ListByFunnel(ctx, funnelID, status, limit, offset)
	} else if actor.Role == domain.RoleVendedor {
		leads, err = uc.leadRepo.ListByFunnelForOwner(ctx, actor.AccountID, funnelID, actor.UserID, status, limit, offset)
	} else {
		// An attendant has no business in the sales pipeline.
		return nil, domain.ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	uc.attachContactLabels(ctx, actor.AccountID, leads)
	return leads, nil
}

// attachContactLabels fills the display-only recipient on each lead.
//
// Best-effort on purpose: a failed label lookup degrades the pipeline to
// showing IDs, which is worse UI but still a working pipeline. Failing the
// whole list because a decorative join broke would be the wrong trade.
func (uc *LeadUseCase) attachContactLabels(ctx context.Context, accountID string, leads []*domain.Lead) {
	if uc.contactRepo == nil || len(leads) == 0 {
		return
	}
	ids := make([]string, 0, len(leads))
	seen := make(map[string]struct{}, len(leads))
	for _, l := range leads {
		if l == nil || l.ContactID == "" {
			continue
		}
		if _, dup := seen[l.ContactID]; dup {
			continue
		}
		seen[l.ContactID] = struct{}{}
		ids = append(ids, l.ContactID)
	}
	if len(ids) == 0 {
		return
	}
	contacts, err := uc.contactRepo.ListByIDs(ctx, accountID, ids)
	if err != nil {
		slog.WarnContext(ctx, "lead: failed to resolve contact labels", "error", err)
		return
	}
	byID := make(map[string]*domain.Contact, len(contacts))
	for _, c := range contacts {
		byID[c.ID] = c
	}
	for _, l := range leads {
		if c, ok := byID[l.ContactID]; ok {
			l.ContactLabel = c.DisplayLabel()
			l.ContactAddress = c.DisplayAddress()
		}
	}
}

// ApprovalQueueOptions is what the operator asked the queue to show them.
//
// The zero value is the queue as it always was: every funnel, oldest first.
type ApprovalQueueOptions struct {
	FunnelID    string
	NewestFirst bool
}

// ApprovalQueuePage is the queue plus the shape of the backlog behind it.
//
// ByFunnel and Total are counted over the WHOLE queue, not over Items. That is
// the entire point of carrying them: the page is capped, so a client that
// derived its own totals from Items would tell the operator the backlog is 50
// when it is 340 — and would give no chip at all to the funnel holding the
// other 290.
type ApprovalQueuePage struct {
	Items    []*domain.Lead
	ByFunnel []domain.QueueFunnelCount
	Total    int64
}

// ApprovalQueue lists everything waiting on a human (US-M020). It is the
// operator's whole job in one list, so it defaults to EVERY funnel: a draft
// parked in a funnel nobody opened today is exactly the one that goes stale.
// A funnel can be selected, but never as the starting point.
//
// Scoped like the pipeline: a vendedor sees their own plus unassigned and
// team-visible drafts, a gestor sees the account's — and the per-funnel counts
// are read under the same scope, so a chip never promises rows the list would
// then refuse to show.
func (uc *LeadUseCase) ApprovalQueue(ctx context.Context, actor domain.Actor, opts ApprovalQueueOptions, limit, offset int32) (*ApprovalQueuePage, error) {
	filter := domain.ApprovalQueueFilter{
		AccountID:   actor.AccountID,
		Status:      domain.LeadStatusPendingApproval,
		FunnelID:    opts.FunnelID,
		NewestFirst: opts.NewestFirst,
	}
	switch {
	case domain.RoleSatisfies(actor.Role, domain.RoleGestor):
		// Whole account: no owner narrow.
	case actor.Role == domain.RoleVendedor:
		filter.OwnerUserID = actor.UserID
	default:
		// An attendant has no business approving outbound sales.
		return nil, domain.ErrForbidden
	}

	leads, err := uc.leadRepo.ListForApprovalQueue(ctx, filter, limit, offset)
	if err != nil {
		return nil, err
	}
	uc.attachContactLabels(ctx, actor.AccountID, leads)

	page := &ApprovalQueuePage{Items: leads}

	// The counts are the screen's navigation, not its content. Failing the
	// whole request because the map could not be drawn would hide drafts that
	// were read successfully and are waiting on somebody — so this degrades to
	// no chips, loudly, and the list still renders.
	byFunnel, err := uc.leadRepo.CountApprovalQueueByFunnel(ctx, filter)
	if err != nil {
		slog.WarnContext(ctx, "lead: failed to count approval queue by funnel", "error", err)
		return page, nil
	}
	page.ByFunnel = byFunnel
	for _, f := range byFunnel {
		page.Total += f.Total
	}
	return page, nil
}

// PipelineOverview is the command centre's payload (US-M023).
//
// Every number here is COUNTED, not sampled: the screen's whole job is to
// tell a salesperson whether they are on top of their pipeline, and a tile
// that silently reports the first page instead of the whole funnel is worse
// than no tile. Anything the platform cannot measure honestly yet (revenue
// won, meetings booked) is deliberately absent rather than zero.
type PipelineOverview struct {
	// ByStatus holds every status with at least one lead. Absent statuses
	// mean zero — the client renders what it knows about.
	ByStatus map[domain.LeadStatus]int64 `json:"by_status"`
	// Total is the whole pipeline, terminal statuses included.
	Total int64 `json:"total"`
	// Active excludes won/lost: the work actually in front of the user.
	Active int64 `json:"active"`
	// PendingApproval is the human-in-the-loop backlog (US-M020).
	PendingApproval int64 `json:"pending_approval"`
	// Unowned is live leads nobody is responsible for. Managers only: a
	// vendedor cannot assign leads, so the number would be a nag they
	// cannot act on.
	Unowned *int64 `json:"unowned,omitempty"`
}

// Overview counts the caller's pipeline, scoped exactly like the board.
func (uc *LeadUseCase) Overview(ctx context.Context, actor domain.Actor) (*PipelineOverview, error) {
	isManager := domain.RoleSatisfies(actor.Role, domain.RoleGestor)
	if !isManager && actor.Role != domain.RoleVendedor {
		// An attendant has no business in the sales pipeline.
		return nil, domain.ErrForbidden
	}

	var (
		byStatus map[domain.LeadStatus]int64
		err      error
	)
	if isManager {
		byStatus, err = uc.leadRepo.CountByStatus(ctx, actor.AccountID)
	} else {
		byStatus, err = uc.leadRepo.CountByStatusForOwner(ctx, actor.AccountID, actor.UserID)
	}
	if err != nil {
		return nil, err
	}

	out := &PipelineOverview{ByStatus: byStatus}
	for status, n := range byStatus {
		out.Total += n
		if status != domain.LeadStatusWon && status != domain.LeadStatusLost {
			out.Active += n
		}
	}
	out.PendingApproval = byStatus[domain.LeadStatusPendingApproval]

	if isManager {
		unowned, err := uc.leadRepo.CountUnowned(ctx, actor.AccountID)
		if err != nil {
			return nil, err
		}
		out.Unowned = &unowned
	}
	return out, nil
}

// AssignOwner hands a lead to a salesperson (or unassigns it with nil).
// Only a manager and above may reassign — letting a vendedor claim a
// teammate's lead would make the ownership model decorative.
func (uc *LeadUseCase) AssignOwner(ctx context.Context, actor domain.Actor, leadID string, ownerUserID *string) (*domain.Lead, error) {
	if !domain.RoleSatisfies(actor.Role, domain.RoleGestor) {
		return nil, domain.ErrForbidden
	}
	if _, err := uc.Get(ctx, actor.AccountID, leadID); err != nil {
		return nil, err
	}
	if err := uc.ensureOwnerInAccount(ctx, actor.AccountID, ownerUserID); err != nil {
		return nil, err
	}
	lead, err := uc.leadRepo.AssignOwner(ctx, actor.AccountID, leadID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, domain.ErrLeadNotFound
	}
	uc.auditAssign(ctx, actor, lead, ownerUserID)
	return lead, nil
}

// ensureOwnerInAccount confirms the assigned owner belongs to the account.
//
// The UPDATE only filters on the LEAD's account_id, not the user's, so a ULID
// from another tenant was accepted and the account ended up with a lead whose
// owner sits outside it — invisible in the team panel and accruing commission
// for somebody who should not be there. A non-existent ULID became a foreign-key
// error, i.e. a 500 in place of a refusal.
//
// Without userRepo the assignment is REFUSED. That is the opposite of the other
// optionals on this type, and deliberately so: the others degrade by losing a
// nicety, this one would lose the only check that separates accounts.
func (uc *LeadUseCase) ensureOwnerInAccount(ctx context.Context, accountID string, ownerUserID *string) error {
	// Unassigning has no destination to check.
	if ownerUserID == nil || *ownerUserID == "" {
		return nil
	}
	if uc.userRepo == nil {
		return domain.ErrLeadAssignUnavailable
	}
	user, err := uc.userRepo.GetByID(ctx, *ownerUserID)
	if err != nil {
		return err
	}
	// Missing and belonging-to-another-account give the SAME answer: replying
	// "that user is not from here" to an outside id would confirm it exists
	// somewhere.
	if user == nil || user.AccountID != accountID || !user.IsActive {
		return domain.ErrLeadOwnerNotInAccount
	}
	return nil
}

// maxBulkAssign caps how many leads one call may redistribute.
//
// Not performance: it is the blast radius of one click. Redistributing touches
// commission, and a mistake across 200 is already an afternoon of repair — across
// 5000 it is a week.
const maxBulkAssign = 200

// BulkAssignResult reports what the redistribution actually did.
//
// It exists because "assigned 64" and "assigned 61, three vanished" are
// different answers, and the second is the one that shows up when somebody
// already touched the funnel while the screen was open.
type BulkAssignResult struct {
	Assigned int      `json:"assigned"`
	Skipped  []string `json:"skipped,omitempty"`
}

// AssignOwnerBulk hands several leads at once to the same person.
//
// A lead that vanished (or never belonged to this account) is SKIPPED rather than
// aborting the rest: whoever clicked is looking at a screen that may be seconds
// stale, and undoing 60 assignments because the 61st no longer exists is worse
// than reporting the three left out.
//
// The owner check runs ONCE, before any write. Per lead it would be the same
// answer repeated N times, and an error midway would leave half assigned.
func (uc *LeadUseCase) AssignOwnerBulk(ctx context.Context, actor domain.Actor, leadIDs []string, ownerUserID *string) (*BulkAssignResult, error) {
	if !domain.RoleSatisfies(actor.Role, domain.RoleGestor) {
		return nil, domain.ErrForbidden
	}
	if len(leadIDs) == 0 {
		return nil, domain.ErrBulkAssignEmpty
	}
	if len(leadIDs) > maxBulkAssign {
		return nil, domain.ErrBulkAssignTooMany
	}
	if err := uc.ensureOwnerInAccount(ctx, actor.AccountID, ownerUserID); err != nil {
		return nil, err
	}

	out := &BulkAssignResult{}
	seen := make(map[string]bool, len(leadIDs))
	for _, id := range leadIDs {
		// A mesma tela pode mandar o mesmo id duas vezes; atribuir duas
		// twice does no harm, but it would count two in the report.
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		lead, err := uc.leadRepo.AssignOwner(ctx, actor.AccountID, id, ownerUserID)
		if err != nil {
			return nil, err
		}
		if lead == nil {
			out.Skipped = append(out.Skipped, id)
			continue
		}
		uc.auditAssign(ctx, actor, lead, ownerUserID)
		out.Assigned++
	}
	return out, nil
}

// SetVisibility hides a lead from the rest of the team, or gives it back.
//
// Manager and above, for the same reason as AssignOwner: a salesperson who could
// mark leads private would take the pipeline out of the sight of whoever answers
// for it.
func (uc *LeadUseCase) SetVisibility(ctx context.Context, actor domain.Actor, leadID string, visibility domain.ResourceVisibility) (*domain.Lead, error) {
	if !domain.RoleSatisfies(actor.Role, domain.RoleGestor) {
		return nil, domain.ErrForbidden
	}
	if !visibility.Valid() {
		return nil, domain.ErrInvalidLeadVisibility
	}
	if _, err := uc.Get(ctx, actor.AccountID, leadID); err != nil {
		return nil, err
	}
	lead, err := uc.leadRepo.SetVisibility(ctx, actor.AccountID, leadID, visibility)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, domain.ErrLeadNotFound
	}
	return lead, nil
}

// auditAssign records who moved a lead to whom (US-F010). Reassignment is
// the sales-floor equivalent of moving money: commission follows ownership,
// so a disputed payout needs a trail. Best-effort — the assignment already
// committed, and losing the audit row must not undo it (it is logged loudly
// instead).
func (uc *LeadUseCase) auditAssign(ctx context.Context, actor domain.Actor, lead *domain.Lead, ownerUserID *string) {
	if uc.auditRepo == nil {
		return
	}
	to := ""
	if ownerUserID != nil {
		to = *ownerUserID
	}
	payload, err := json.Marshal(map[string]any{
		"funnel_id": lead.FunnelID,
		"to_user":   to,
		"actor":     actor.UserID,
		"role":      string(actor.Role),
	})
	if err != nil {
		return
	}
	actorID := actor.UserID
	if err := uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  actor.AccountID,
		ActorID:    &actorID,
		Action:     "lead.owner_assigned",
		EntityType: "lead",
		EntityID:   lead.ID,
		Payload:    payload,
	}); err != nil {
		slog.WarnContext(ctx, "lead: failed to write assignment audit",
			"lead_id", lead.ID, "error", err)
	}
}

// SetCaseRecorder wires case instrumentation (US-M011).
func (uc *LeadUseCase) SetCaseRecorder(r *CaseRecorder) {
	uc.cases = r
}

// MarkMeetingBooked records that a human confirmed a meeting with this lead
// (US-M011).
//
// It is an explicit human action rather than a derived signal because the
// platform does not book anything itself — the AI only checks availability,
// and inferring "meeting" from a status change would be inventing the case
// report's most flattering number.
func (uc *LeadUseCase) MarkMeetingBooked(ctx context.Context, actor domain.Actor, leadID string) error {
	lead, err := uc.MutableForActor(ctx, actor, leadID)
	if err != nil {
		return err
	}
	uc.cases.MeetingBooked(ctx, actor.AccountID, lead.ID)
	return nil
}

// SetAuditRepo wires the audit trail (US-F010). Optional setter — an
// unwired repo skips the trail rather than blocking the operation.
func (uc *LeadUseCase) SetAuditRepo(r domain.AuditLogRepository) {
	uc.auditRepo = r
}

// ── Kanban board (US-M008) ──────────────────────────────────────────────────

// SetStageRepo wires the board columns. Optional: without it the pipeline
// works exactly as before, just with no Kanban.
func (uc *LeadUseCase) SetStageRepo(r domain.FunnelStageRepository) {
	uc.stageRepo = r
}

// BoardCard is a lead placed on the board. The stage is RESOLVED, not
// assumed: leads created before US-M008 (and by paths that know nothing
// about stages — inbound, prospecting) have no stage_id and must still
// appear, or the salesperson's board silently loses deals.
type BoardCard struct {
	Lead    *domain.Lead `json:"lead"`
	StageID string       `json:"stage_id"`
}

// Board is a funnel's Kanban: its columns plus the caller's visible cards.
type Board struct {
	Stages []*domain.FunnelStage `json:"stages"`
	Cards  []BoardCard           `json:"cards"`
}

// Board returns the funnel's columns and the cards this actor may see. The
// card list reuses ListByFunnelForActor, so the vendedor scoping from
// US-F009 applies here too — a board that showed everything would be a
// backdoor around the ownership rules.
func (uc *LeadUseCase) Board(ctx context.Context, actor domain.Actor, funnelID string, limit, offset int32) (*Board, error) {
	if uc.stageRepo == nil {
		return nil, &domain.AppError{Code: "KANBAN_UNAVAILABLE", Message: "board não configurado"}
	}
	leads, err := uc.ListByFunnelForActor(ctx, actor, funnelID, "", limit, offset)
	if err != nil {
		return nil, err
	}
	stages, err := uc.stageRepo.EnsureDefaults(ctx, actor.AccountID, funnelID)
	if err != nil {
		return nil, err
	}

	cards := make([]BoardCard, 0, len(leads))
	for _, lead := range leads {
		stageID := ""
		if lead.StageID != nil && *lead.StageID != "" {
			stageID = *lead.StageID
		} else if resolved := domain.ResolveStageForStatus(stages, lead.Status); resolved != nil {
			stageID = resolved.ID
		}
		cards = append(cards, BoardCard{Lead: lead, StageID: stageID})
	}
	return &Board{Stages: stages, Cards: cards}, nil
}

// MoveLead drags a card to another column — or reorders it inside its own.
//
// Ownership is checked with the MUTATION rule, not the read rule: a vendedor
// may look at a teammate's team-visible lead but must not move it through
// their pipeline.
//
// beforeLeadID/afterLeadID are the neighbors of the drop slot as the client
// rendered them (before = the card ABOVE, after = the one BELOW; either may
// be empty at an edge). When at least one is given the card gets a manual
// board_pos (fractional ranking, domain.ComputeBoardPos); with neither, the
// manual rank is CLEARED and the card returns to the automatic order.
//
// A same-stage move only changes the position: it must not re-fire status
// transitions, the backward-move guard or the terminal-move audit — a
// reorder inside "Ganhos" is not winning the deal twice.
func (uc *LeadUseCase) MoveLead(ctx context.Context, actor domain.Actor, leadID, stageID, beforeLeadID, afterLeadID string) (*domain.Lead, error) {
	if uc.stageRepo == nil {
		return nil, &domain.AppError{Code: "KANBAN_UNAVAILABLE", Message: "board não configurado"}
	}
	lead, err := uc.MutableForActor(ctx, actor, leadID)
	if err != nil {
		return nil, err
	}
	stage, err := uc.stageRepo.GetByID(ctx, actor.AccountID, stageID)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, domain.ErrStageNotFound
	}
	// A card can only be dropped on a column of ITS OWN funnel — otherwise a
	// lead silently leaves the pipeline it belongs to.
	if stage.FunnelID != lead.FunnelID {
		return nil, domain.ErrStageWrongFunnel
	}
	// Same stage = same pinned column, or (for a stage-less lead sitting in
	// its resolved column) the column its own status already maps to. Both
	// mean "reorder", never "advance the pipeline".
	sameStage := (lead.StageID != nil && *lead.StageID == stage.ID) ||
		(lead.StageID == nil && stage.MapsToStatus == lead.Status)
	if !sameStage && domain.IsBackwardMove(lead.Status, stage.MapsToStatus) {
		return nil, domain.ErrStageBackward
	}

	var boardPos *float64
	if beforeLeadID != "" || afterLeadID != "" {
		before, after, maxPos, err := uc.stageRepo.BoardNeighborPositions(
			ctx, actor.AccountID, stage.ID, beforeLeadID, afterLeadID)
		if err != nil {
			return nil, err
		}
		// A neighbor the client NAMED but that has no position yet means the
		// column is still (partly) automatic. Treating it as "no neighbor"
		// made the dropped card the column's only placed one — and the
		// placed-first rule threw it to the top, wherever the slot was.
		// Materialise the column's current order, then resolve again.
		if (beforeLeadID != "" && before == nil) || (afterLeadID != "" && after == nil) {
			if err := uc.stageRepo.SeedBoardOrder(ctx, actor.AccountID, stage.ID); err != nil {
				return nil, err
			}
			before, after, maxPos, err = uc.stageRepo.BoardNeighborPositions(
				ctx, actor.AccountID, stage.ID, beforeLeadID, afterLeadID)
			if err != nil {
				return nil, err
			}
		}
		pos := domain.ComputeBoardPos(before, after, maxPos)
		boardPos = &pos
	}

	status := stage.MapsToStatus
	if sameStage {
		// A reorder keeps the canonical status — including a transient one
		// (enriching, pending_approval) the column merely hosts.
		status = lead.Status
	}
	moved, err := uc.stageRepo.MoveLead(ctx, actor.AccountID, leadID, stage.ID, status, boardPos, !sameStage)
	if err != nil {
		return nil, err
	}
	if !sameStage {
		uc.auditMove(ctx, actor, moved, stage)
	}
	return moved, nil
}

// ResetBoardOrder clears one column's manual ordering, restoring the
// automatic most-stalled-first sort ("ordenar por parados").
//
// Scoped by the stage's own account/funnel rather than per lead: the order
// is a view preference of the column, and clearing it moves nothing through
// the pipeline.
func (uc *LeadUseCase) ResetBoardOrder(ctx context.Context, actor domain.Actor, funnelID, stageID string) error {
	if uc.stageRepo == nil {
		return &domain.AppError{Code: "KANBAN_UNAVAILABLE", Message: "board não configurado"}
	}
	stage, err := uc.stageRepo.GetByID(ctx, actor.AccountID, stageID)
	if err != nil {
		return err
	}
	if stage == nil {
		return domain.ErrStageNotFound
	}
	if stage.FunnelID != funnelID {
		return domain.ErrStageWrongFunnel
	}
	return uc.stageRepo.ResetBoardOrder(ctx, actor.AccountID, funnelID, stage.ID)
}

// auditMove records board movements (US-F010 trail). A deal moving to "won"
// is the event commission and forecasting hang off, so who moved it and when
// has to survive an argument.
func (uc *LeadUseCase) auditMove(ctx context.Context, actor domain.Actor, lead *domain.Lead, stage *domain.FunnelStage) {
	if uc.auditRepo == nil || !stage.MapsToStatus.IsTerminal() {
		// Only terminal moves are audited: auditing every drag would bury
		// the events that matter in noise.
		return
	}
	payload, err := json.Marshal(map[string]any{
		"stage":  stage.Name,
		"status": string(stage.MapsToStatus),
		"actor":  actor.UserID,
	})
	if err != nil {
		return
	}
	actorID := actor.UserID
	if err := uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  actor.AccountID,
		ActorID:    &actorID,
		Action:     "lead.stage_moved",
		EntityType: "lead",
		EntityID:   lead.ID,
		Payload:    payload,
	}); err != nil {
		slog.WarnContext(ctx, "lead: failed to write stage-move audit", "lead_id", lead.ID, "error", err)
	}
}

// CreateFromDiscovered turns one confirmed Sent-folder candidate into a contact
// and a lead.
//
// Lives here rather than in the discovery use case because creating a lead
// already means funnel ownership, contact dedupe, the identity index and the
// pipeline's starting status — and a second implementation of those rules is a
// second set of rules that drift.
//
// Keyed by EMAIL, which is the difference from Create: that one takes a typed
// phone and normalizes it, while everything discovered in a mailbox is an
// address. The address becomes the contact's external_id, exactly as the email
// channel's own inbound path keys it, so a person who later writes in lands on
// this same contact instead of becoming a stranger.
func (uc *LeadUseCase) CreateFromDiscovered(ctx context.Context, in ImportInput, c *domain.DiscoveredContact) (string, error) {
	if uc.contactRepo == nil {
		return "", domain.ErrLeadCreateUnavailable
	}
	if c == nil {
		return "", domain.ErrLeadNotFound
	}

	funnel, err := uc.funnelRepo.GetByID(ctx, in.FunnelID)
	if err != nil {
		return "", err
	}
	if funnel == nil {
		return "", domain.ErrSalesFunnelNotFound
	}
	if funnel.AccountID != in.AccountID {
		return "", domain.ErrUnauthorized
	}

	address := strings.ToLower(strings.TrimSpace(c.Address))
	if !domain.IsEmailAddress(address) {
		return "", &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "endereço de e-mail inválido: " + c.Address,
		}
	}

	contact := &domain.Contact{
		ID:         ulid.Make().String(),
		AccountID:  in.AccountID,
		ExternalID: address,
		Email:      &address,
	}
	channelType := "email"
	contact.ChannelType = &channelType
	// Same rule as Create: only name a contact that has no name. A name learned
	// from a live channel outranks one read off an old email header, and
	// overwriting it would rename the person everywhere they appear — including
	// in the confirmation of a message already waiting for approval.
	if name := strings.TrimSpace(c.Name); name != "" {
		existing, _ := uc.contactRepo.GetByExternalID(ctx, in.AccountID, address)
		if existing == nil || existing.Name == nil || strings.TrimSpace(*existing.Name) == "" {
			contact.Name = &name
		}
	}
	saved, err := uc.contactRepo.Upsert(ctx, contact)
	if err != nil {
		return "", err
	}
	uc.registerEmailIdentity(ctx, saved, address)

	// The lead itself goes through ensureLeadForContact, shared with the
	// existing-customer import: the dedupe (a lead already tied to this
	// (funnel, contact) is returned untouched, so importing the same person
	// twice never resets a pipeline that has moved), the starting status, the
	// default visibility and the instrumentation are one rule, not two copies.
	leadID, _, err := uc.ensureLeadForContact(ctx, ensureLeadInput{
		AccountID:   in.AccountID,
		FunnelID:    funnel.ID,
		ContactID:   saved.ID,
		OwnerUserID: in.OwnerUserID,
		// Named so the funnel can tell these apart from a cold prospecting
		// import: somebody she has emailed 40 times is not a stranger, and an
		// approach written as if they were would read as amnesia.
		Source: "sent_folder",
	})
	if err != nil {
		return "", err
	}
	return leadID, nil
}

// registerEmailIdentity indexes the address so the same person found later by
// phone or on another channel resolves to this contact (US-F005).
func (uc *LeadUseCase) registerEmailIdentity(ctx context.Context, contact *domain.Contact, address string) {
	if uc.identityRepo == nil || contact == nil || address == "" {
		return
	}
	if _, err := uc.identityRepo.Upsert(ctx, &domain.ContactIdentity{
		AccountID: contact.AccountID,
		ContactID: contact.ID,
		Kind:      domain.IdentityKindEmail,
		Value:     address,
	}); err != nil {
		slog.WarnContext(ctx, "lead: failed to register email identity",
			"contact_id", contact.ID, "error", err)
	}
}
