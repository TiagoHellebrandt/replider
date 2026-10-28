package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Discovery reads somebody's mailbox to build a customer list. These pin the
// rules that keep it proportionate (envelopes only, bounded) and safe (nothing
// acts without a human, and a rescan never undoes a decision).

type stubSentScanner struct {
	pages  []domain.EmailSentScanResult
	err    error
	calls  []uint32 // the beforeUID of each call, so paging is observable
	failAt int
}

func (s *stubSentScanner) ScanSent(_ context.Context, _ domain.EmailInboxConfig, beforeUID uint32, _ int) (domain.EmailSentScanResult, error) {
	s.calls = append(s.calls, beforeUID)
	if s.failAt > 0 && len(s.calls) == s.failAt {
		return domain.EmailSentScanResult{}, errors.New("imap exploded")
	}
	if s.err != nil {
		return domain.EmailSentScanResult{}, s.err
	}
	if len(s.pages) == 0 {
		return domain.EmailSentScanResult{Done: true}, nil
	}
	page := s.pages[0]
	s.pages = s.pages[1:]
	return page, nil
}

type stubDiscoveredRepo struct {
	upserted   []domain.SentRecipient
	upsertErr  error
	candidates []*domain.DiscoveredContact
	imported   []string
	dismissed  []string
}

func (s *stubDiscoveredRepo) Upsert(_ context.Context, _, _ string, r domain.SentRecipient) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, r)
	return nil
}

func (s *stubDiscoveredRepo) ListPending(context.Context, string, int32, int32) ([]*domain.DiscoveredContact, error) {
	return s.candidates, nil
}

func (s *stubDiscoveredRepo) GetByAddresses(context.Context, string, []string) ([]*domain.DiscoveredContact, error) {
	return s.candidates, nil
}

func (s *stubDiscoveredRepo) MarkImported(_ context.Context, _ string, addresses []string, _ time.Time) error {
	s.imported = append(s.imported, addresses...)
	return nil
}

func (s *stubDiscoveredRepo) MarkDismissed(_ context.Context, _ string, addresses []string, _ time.Time) error {
	s.dismissed = append(s.dismissed, addresses...)
	return nil
}

func (s *stubDiscoveredRepo) CountPending(context.Context, string) (int64, error) {
	return int64(len(s.candidates)), nil
}

type stubDiscoveryChannelRepo struct {
	channel *domain.Channel
	domain.ChannelRepository
}

func (s *stubDiscoveryChannelRepo) GetByID(context.Context, string) (*domain.Channel, error) {
	return s.channel, nil
}

func discoveryChannel(t *testing.T) *domain.Channel {
	t.Helper()
	cfg, err := json.Marshal(map[string]string{
		"from_address":  "contato@clinica.com.br",
		"smtp_host":     "email-ssl.com.br",
		"smtp_username": "contato@clinica.com.br",
		"smtp_password": "segredo",
	})
	require.NoError(t, err)
	return &domain.Channel{
		ID: "ch-1", AccountID: "acc-1", Type: "email", IsActive: true,
		Config: json.RawMessage(cfg),
	}
}

func newDiscoveryFixture(t *testing.T, pages ...domain.EmailSentScanResult) (
	*DiscoverContactsUseCase, *stubSentScanner, *stubDiscoveredRepo,
) {
	t.Helper()
	scanner := &stubSentScanner{pages: pages}
	repo := &stubDiscoveredRepo{}
	uc := NewDiscoverContactsUseCase(scanner, repo,
		&stubDiscoveryChannelRepo{channel: discoveryChannel(t)}, nil)
	uc.now = func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) }
	return uc, scanner, repo
}

func TestDiscoverContacts_RecordsTheAddressesSheWritesTo(t *testing.T) {
	sent := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	uc, _, repo := newDiscoveryFixture(t, domain.EmailSentScanResult{
		Mailbox: "INBOX.Sent", UIDValidity: 42, HighestUID: 900,
		Recipients: []domain.SentRecipient{
			{Address: "ana@clinicaaurora.com.br", Name: "Ana Souza", SentAt: sent},
			{Address: "compras@hospital.com.br", SentAt: sent},
		},
		Done: true,
	})

	out, err := uc.Scan(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	assert.Equal(t, "INBOX.Sent", out.Mailbox)
	assert.Equal(t, 2, out.Found)
	assert.Equal(t, 2, out.Scanned)
	assert.True(t, out.Complete)
	assert.Len(t, repo.upserted, 2)
}

// Her own address is not a customer, and a robot never is. Both are COUNTED
// rather than silently dropped: "read 4000, found 12" needs an explanation, and
// the skipped number is it.
func TestDiscoverContacts_SkipsHerOwnAddressAndRobots(t *testing.T) {
	uc, _, repo := newDiscoveryFixture(t, domain.EmailSentScanResult{
		Recipients: []domain.SentRecipient{
			{Address: "contato@clinica.com.br"},        // her own
			{Address: "no-reply@banco.com.br"},         // robot
			{Address: "naoresponda@fornecedor.com.br"}, // robot, pt
			{Address: "mailer-daemon@servidor.com"},    // bounce
			{Address: "ana@clinicaaurora.com.br"},      // a person
		},
		Done: true,
	})

	out, err := uc.Scan(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	assert.Equal(t, 1, out.Found)
	assert.Equal(t, 4, out.Skipped)
	require.Len(t, repo.upserted, 1)
	assert.Equal(t, "ana@clinicaaurora.com.br", repo.upserted[0].Address)
}

// The same person in several messages is ONE candidate in the count, even though
// every sighting is recorded (the repo accumulates them into a sent_count).
func TestDiscoverContacts_CountsDistinctPeopleNotMessages(t *testing.T) {
	uc, _, repo := newDiscoveryFixture(t, domain.EmailSentScanResult{
		Recipients: []domain.SentRecipient{
			{Address: "ana@clinicaaurora.com.br"},
			{Address: "ana@clinicaaurora.com.br"},
			{Address: "ana@clinicaaurora.com.br"},
		},
		Done: true,
	})

	out, err := uc.Scan(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	assert.Equal(t, 1, out.Found, "one person")
	assert.Len(t, repo.upserted, 3, "three sightings, so the ranking means something")
}

// A mailbox with years of history is walked in pages, downwards, and the walk is
// capped: ten years answers the question no better than a few thousand messages,
// and an unbounded walk holds a session open against a provider that caps them.
func TestDiscoverContacts_PagesDownwardsAndStopsAtTheCap(t *testing.T) {
	uc, scanner, _ := newDiscoveryFixture(t)
	uc.maxMessages = 5
	uc.pageSize = 3
	// Every page reports more below it, so only the cap can stop the walk.
	for i := 0; i < 10; i++ {
		scanner.pages = append(scanner.pages, domain.EmailSentScanResult{
			Mailbox: "Sent", UIDValidity: 42, HighestUID: 900,
			LowestScannedUID: uint32(900 - i*3),
			Recipients: []domain.SentRecipient{
				{Address: "a@b.com"}, {Address: "c@d.com"}, {Address: "e@f.com"},
			},
		})
	}

	out, err := uc.Scan(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	assert.False(t, out.Complete, "the cap stopped it, and the caller is told")
	assert.GreaterOrEqual(t, out.Scanned, 5)
	// Each call continues BELOW the previous page: a walk that restarted at the
	// top would read the same messages forever.
	require.Greater(t, len(scanner.calls), 1)
	assert.Equal(t, uint32(0), scanner.calls[0])
	assert.Equal(t, uint32(900), scanner.calls[1])
}

// A failure on the first page is an error the customer sees: nothing was found
// and "we could not connect to the mailbox" is the actionable answer. The
// transport detail stays out of the message — echoed, it would tell a caller
// which internal hosts answer on which ports.
func TestDiscoverContacts_AFirstPageFailureIsReported(t *testing.T) {
	uc, scanner, _ := newDiscoveryFixture(t)
	scanner.err = errors.New("dial tcp 10.0.0.5:5432: connection refused")

	_, err := uc.Scan(context.Background(), "acc-1", "ch-1")

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "EMAIL_CONNECTION_FAILED", appErr.Code)
	assert.NotContains(t, appErr.Message, "10.0.0.5")
	assert.NotContains(t, appErr.Message, "refused")
}

// A failure PART WAY through keeps what it found: those candidates are already
// written, and throwing the run away would lose real work over a network blip.
func TestDiscoverContacts_AMidWalkFailureKeepsWhatItFound(t *testing.T) {
	uc, scanner, repo := newDiscoveryFixture(t, domain.EmailSentScanResult{
		Mailbox: "Sent", UIDValidity: 42, HighestUID: 900, LowestScannedUID: 800,
		Recipients: []domain.SentRecipient{{Address: "ana@clinicaaurora.com.br"}},
	})
	scanner.failAt = 2

	out, err := uc.Scan(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	assert.Equal(t, 1, out.Found)
	assert.False(t, out.Complete, "it did not finish, and does not claim to")
	assert.Len(t, repo.upserted, 1)
}

func TestDiscoverContacts_RefusesANonEmailChannel(t *testing.T) {
	uc := NewDiscoverContactsUseCase(&stubSentScanner{}, &stubDiscoveredRepo{},
		&stubDiscoveryChannelRepo{channel: &domain.Channel{
			ID: "ch-1", AccountID: "acc-1", Type: "whatsapp",
		}}, nil)

	_, err := uc.Scan(context.Background(), "acc-1", "ch-1")
	assert.ErrorIs(t, err, ErrChannelNotEmail)
}

func TestDiscoverContacts_AnotherAccountsChannelIsNotFound(t *testing.T) {
	uc, scanner, _ := newDiscoveryFixture(t)

	_, err := uc.Scan(context.Background(), "outra-acc", "ch-1")

	assert.ErrorIs(t, err, domain.ErrChannelNotFound)
	assert.Empty(t, scanner.calls, "no mailbox is opened for another tenant")
}

// --- The human gate ---------------------------------------------------------

type stubLeadCreator struct {
	created []string
	err     error
}

func (s *stubLeadCreator) CreateFromDiscovered(_ context.Context, _ ImportInput, c *domain.DiscoveredContact) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.created = append(s.created, c.Address)
	return "lead-" + c.Address, nil
}

func TestDiscoverContacts_ImportCreatesOnlyWhatWasConfirmed(t *testing.T) {
	uc, _, repo := newDiscoveryFixture(t)
	repo.candidates = []*domain.DiscoveredContact{
		{Address: "ana@clinicaaurora.com.br", SentCount: 40},
		{Address: "compras@hospital.com.br", SentCount: 12},
	}
	creator := &stubLeadCreator{}

	out, err := uc.Import(context.Background(), ImportInput{
		AccountID: "acc-1", FunnelID: "fn-1",
		Addresses: []string{"ana@clinicaaurora.com.br", "compras@hospital.com.br"},
	}, creator)

	require.NoError(t, err)
	assert.Equal(t, 2, out.Imported)
	assert.Len(t, out.LeadIDs, 2)
	assert.Len(t, creator.created, 2)
	// Stamped, so a rescan does not offer somebody already in the CRM.
	assert.Len(t, repo.imported, 2)
}

// Re-importing the same person would create a second lead and split their
// history in two.
func TestDiscoverContacts_ImportSkipsWhatIsAlreadyIn(t *testing.T) {
	already := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	uc, _, repo := newDiscoveryFixture(t)
	repo.candidates = []*domain.DiscoveredContact{
		{Address: "ana@clinicaaurora.com.br", ImportedAt: &already},
	}
	creator := &stubLeadCreator{}

	out, err := uc.Import(context.Background(), ImportInput{
		AccountID: "acc-1", FunnelID: "fn-1",
		Addresses: []string{"ana@clinicaaurora.com.br"},
	}, creator)

	require.NoError(t, err)
	assert.Zero(t, out.Imported)
	assert.Equal(t, 1, out.Skipped)
	assert.Empty(t, creator.created)
}

// A lead with no funnel is a row nobody sees.
func TestDiscoverContacts_ImportNeedsAFunnel(t *testing.T) {
	uc, _, _ := newDiscoveryFixture(t)

	_, err := uc.Import(context.Background(), ImportInput{
		AccountID: "acc-1",
		Addresses: []string{"ana@clinicaaurora.com.br"},
	}, &stubLeadCreator{})

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

// An address this account never discovered is reported as skipped, not silently
// ignored: a count that does not match the selection is a question.
func TestDiscoverContacts_ImportReportsSelectionsItCouldNotResolve(t *testing.T) {
	uc, _, repo := newDiscoveryFixture(t)
	repo.candidates = []*domain.DiscoveredContact{{Address: "ana@clinicaaurora.com.br"}}

	out, err := uc.Import(context.Background(), ImportInput{
		AccountID: "acc-1", FunnelID: "fn-1",
		Addresses: []string{"ana@clinicaaurora.com.br", "ninguem@lugar.com"},
	}, &stubLeadCreator{})

	require.NoError(t, err)
	assert.Equal(t, 1, out.Imported)
	assert.Equal(t, 1, out.Skipped)
}

func TestDiscoverContacts_ImportWithoutACreatorRefuses(t *testing.T) {
	uc, _, _ := newDiscoveryFixture(t)

	_, err := uc.Import(context.Background(), ImportInput{
		AccountID: "acc-1", FunnelID: "fn-1",
		Addresses: []string{"a@b.com"},
	}, nil)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "DISCOVERY_IMPORT_UNAVAILABLE", appErr.Code)
}

func TestDiscoverContacts_DismissRecordsTheRefusal(t *testing.T) {
	uc, _, repo := newDiscoveryFixture(t)

	require.NoError(t, uc.Dismiss(context.Background(), "acc-1",
		[]string{"no@interessa.com"}))

	assert.Equal(t, []string{"no@interessa.com"}, repo.dismissed)
}
