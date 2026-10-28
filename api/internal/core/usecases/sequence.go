package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Cadences (§1.5, E1).
//
// The use case owns two things the repository cannot: what counts as a valid
// cadence (which needs the account's approved WhatsApp templates) and what
// happens when a lead replies (which is the rule the whole feature rests on).
type SequenceUseCase struct {
	repo domain.SequenceRepository
	// waTemplates resolves which WhatsApp templates Meta has approved for
	// this account. Optional: unwired, a WhatsApp step simply cannot
	// validate, which is the safe direction — an unapproved template is not
	// delivered and puts the number at risk.
	waTemplates domain.WhatsAppTemplateRepository
	channelRepo domain.ChannelRepository
	// accounts resolves the account's own timezone, so "business hours"
	// means the customer's business hours. Optional: unwired falls back to
	// UTC (domain.LoadLocation).
	accounts AccountTimezoneLookup
	now      func() time.Time
	// enrollmentEnabled mirrors config.LegacyWorkers. DueEnrollments has no
	// runner, so an enrollment made while it is off is a promise nobody
	// keeps; Enroll refuses with SEQUENCES_UNAVAILABLE instead.
	enrollmentEnabled bool
}

// AccountTimezoneLookup is the one account field a schedule needs.
//
// A narrow port rather than domain.AccountRepository: dozens of test mocks
// implement that interface, and this needs one field.
type AccountTimezoneLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
}

func NewSequenceUseCase(repo domain.SequenceRepository) *SequenceUseCase {
	return &SequenceUseCase{repo: repo, now: time.Now, enrollmentEnabled: true}
}

// SetEnrollmentEnabled gates Enroll (LEGACY_WORKERS).
func (uc *SequenceUseCase) SetEnrollmentEnabled(on bool) { uc.enrollmentEnabled = on }

// SetAccounts wires the per-account timezone lookup.
func (uc *SequenceUseCase) SetAccounts(a AccountTimezoneLookup) { uc.accounts = a }

// SetWhatsAppTemplates wires the approved-template lookup.
func (uc *SequenceUseCase) SetWhatsAppTemplates(r domain.WhatsAppTemplateRepository, channels domain.ChannelRepository) {
	uc.waTemplates, uc.channelRepo = r, channels
}

func (uc *SequenceUseCase) WithClock(now func() time.Time) *SequenceUseCase {
	uc.now = now
	return uc
}

func (uc *SequenceUseCase) List(ctx context.Context, accountID string) ([]*domain.Sequence, error) {
	list, err := uc.repo.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Sequence{}
	}
	return list, nil
}

func (uc *SequenceUseCase) Get(ctx context.Context, id, accountID string) (*domain.Sequence, error) {
	seq, err := uc.repo.Get(ctx, id, accountID)
	if err != nil {
		return nil, err
	}
	if seq == nil {
		return nil, &domain.AppError{
			Code: "SEQUENCE_NOT_FOUND", Message: "cadência não encontrada"}
	}
	return seq, nil
}

// ErrSequenceInvalid carries every violation at once.
//
// One error with a list rather than a first-failure: an editor that reports
// one problem per save turns a five-step cadence into five round trips.
type ErrSequenceInvalid struct {
	Violations []domain.SequenceViolation
}

func (e *ErrSequenceInvalid) Error() string { return "sequence has violations" }

// Save validates and persists.
//
// Validation runs on SAVE, not only on activation. A cadence saved broken and
// discovered broken a week later — when it silently did nothing — is worse
// than one that refuses to save.
func (uc *SequenceUseCase) Save(ctx context.Context, accountID string, seq *domain.Sequence) (*domain.Sequence, error) {
	if strings.TrimSpace(seq.Name) == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "dê um nome à cadência"}
	}
	// Renumbered from the list order rather than trusted: a client that
	// reorders by drag sends whatever order its widgets happened to hold.
	for i := range seq.Steps {
		seq.Steps[i].Order = i + 1
		if seq.Steps[i].ID == "" {
			seq.Steps[i].ID = ulid.Make().String()
		}
	}

	if v := seq.Validate(uc.approvedTemplates(ctx, accountID)); len(v) > 0 {
		return nil, &ErrSequenceInvalid{Violations: v}
	}

	if seq.ID == "" {
		seq.ID = ulid.Make().String()
	}
	seq.AccountID = accountID
	seq.StopOnReply = true
	if err := uc.repo.Upsert(ctx, seq); err != nil {
		return nil, err
	}
	return seq, nil
}

func (uc *SequenceUseCase) Delete(ctx context.Context, id, accountID string) error {
	return uc.repo.Delete(ctx, id, accountID)
}

// approvedTemplates lists the WhatsApp template names Meta has approved.
//
// An empty map means "nothing is approved", which makes every WhatsApp step
// fail validation. That is the safe direction: an unapproved template is not
// delivered, and sending against one is what gets a number banned.
func (uc *SequenceUseCase) approvedTemplates(ctx context.Context, accountID string) map[string]bool {
	out := map[string]bool{}
	if uc.waTemplates == nil || uc.channelRepo == nil {
		return out
	}
	channels, err := uc.channelRepo.ListByAccountID(ctx, accountID)
	if err != nil {
		return out
	}
	for _, ch := range channels {
		if ch == nil || ch.Type != "whatsapp" {
			continue
		}
		templates, err := uc.waTemplates.ListByChannel(ctx, ch.ID)
		if err != nil {
			continue
		}
		for _, tpl := range templates {
			if strings.EqualFold(tpl.MetaStatus, "APPROVED") {
				out[tpl.Name] = true
			}
		}
	}
	return out
}

// Enroll starts a lead on a cadence.
func (uc *SequenceUseCase) Enroll(ctx context.Context, accountID, sequenceID, leadID string) (*domain.SequenceEnrollment, error) {
	if !uc.enrollmentEnabled {
		return nil, &domain.AppError{
			Code: "SEQUENCES_UNAVAILABLE", Message: "cadências automáticas desativadas neste plano"}
	}
	seq, err := uc.Get(ctx, sequenceID, accountID)
	if err != nil {
		return nil, err
	}
	if !seq.IsActive {
		return nil, &domain.AppError{
			Code: "SEQUENCE_INACTIVE", Message: "ative a cadência antes de inscrever leads"}
	}

	// One at a time. A lead in two cadences gets two follow-ups a week from
	// the same company, which reads as disorganised at best.
	if existing, err := uc.repo.GetEnrollment(ctx, accountID, leadID); err == nil && existing != nil {
		return nil, &domain.AppError{
			Code:    "LEAD_ALREADY_ENROLLED",
			Message: "este lead já está numa cadência",
		}
	}

	next := uc.scheduleFor(ctx, accountID, seq, 1, uc.now())
	enrollment := &domain.SequenceEnrollment{
		ID: ulid.Make().String(), AccountID: accountID,
		SequenceID: seq.ID, LeadID: leadID,
		CurrentStep: 1, Status: domain.EnrollmentActive,
		NextRunAt: &next, EnrolledAt: uc.now(),
	}
	if err := uc.repo.Enroll(ctx, enrollment); err != nil {
		return nil, err
	}
	return enrollment, nil
}

// StopOnReply ends every active enrollment for a lead.
//
// The rule the whole feature rests on. Called from the inbound pipeline, so
// it is best effort by nature — but it is the one place where "best effort"
// would be wrong to swallow, because a cadence that keeps firing after
// somebody answered is the worst thing this product can do to a live deal.
func (uc *SequenceUseCase) StopOnReply(ctx context.Context, accountID, leadID string) error {
	return uc.repo.StopForLead(ctx, accountID, leadID, domain.EnrollmentReplied, uc.now())
}

// Pause stops a cadence without marking it a success.
func (uc *SequenceUseCase) Pause(ctx context.Context, accountID, leadID string) error {
	return uc.repo.StopForLead(ctx, accountID, leadID, domain.EnrollmentPaused, uc.now())
}

// scheduleFor computes when a step should fire.
func (uc *SequenceUseCase) scheduleFor(ctx context.Context, accountID string, seq *domain.Sequence, stepOrder int, from time.Time) time.Time {
	at := from
	if idx := stepOrder - 1; idx >= 0 && idx < len(seq.Steps) {
		at = at.Add(time.Duration(seq.Steps[idx].DelayHours) * time.Hour)
	}
	if !seq.BusinessHoursOnly {
		return at
	}
	return domain.NextSendWindow(at, uc.sequenceLocation(ctx, accountID))
}

// sequenceLocation is the clock a cadence runs on: the ACCOUNT's own.
//
// "Business hours" is a claim about the recipient's day. A cadence pinned to
// one country's clock emails a customer in Tokyo at four in the morning, and
// business-hours-only is precisely the setting somebody turns on to stop that
// happening.
func (uc *SequenceUseCase) sequenceLocation(ctx context.Context, accountID string) *time.Location {
	if uc.accounts == nil {
		return domain.LoadLocation("")
	}
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return domain.LoadLocation("")
	}
	return domain.LoadLocation(account.Timezone)
}
