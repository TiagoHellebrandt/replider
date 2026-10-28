package usecases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Learning the customer list from the customer's own Sent folder.
//
// This is the product's second promise — "traz os clientes que você já tem" —
// and before this there was no path for it at all: no endpoint creates a
// contact, and the only import reads prospecting results. The answer was in her
// mailbox: the people she has emailed for years ARE the list.
//
// Two rules, both structural rather than documented:
//
//   - Only envelopes are read (To/Cc/Date). Never a body. Reading the text of
//     every message she ever sent to answer "who are your customers" is out of
//     proportion to the question.
//   - Discovery produces CANDIDATES. Nothing becomes a contact, a lead, or a
//     message until a human picks it — the same gate as the approval queue, for
//     the same reason: a list a machine assembled from somebody's mailbox must
//     not act on its own.
type DiscoverContactsUseCase struct {
	scanner    domain.EmailSentScanner
	discovered domain.DiscoveredContactRepository
	channels   domain.ChannelRepository
	// cursors reuses the inbox cursor table, keyed by the Sent folder's name.
	// The mailbox column exists precisely so a second folder gets its own
	// position without a second table.
	cursors domain.EmailInboxCursorRepository

	// pageSize is how many messages one scan call reads. Bounded because this
	// walks history: a mailbox with 40k sent messages must not be one request.
	pageSize int
	// maxMessages caps the whole backfill. Ten years of mail answers the
	// question no better than the last few thousand messages do, and an
	// unbounded walk is a session held open for an hour against a provider that
	// caps sessions.
	maxMessages int

	now func() time.Time
}

func NewDiscoverContactsUseCase(
	scanner domain.EmailSentScanner,
	discovered domain.DiscoveredContactRepository,
	channels domain.ChannelRepository,
	cursors domain.EmailInboxCursorRepository,
) *DiscoverContactsUseCase {
	return &DiscoverContactsUseCase{
		scanner:     scanner,
		discovered:  discovered,
		channels:    channels,
		cursors:     cursors,
		pageSize:    200,
		maxMessages: 4000,
		now:         time.Now,
	}
}

// ScanResult is what one discovery run found.
type ScanResult struct {
	// Mailbox is the folder that was read, echoed so a customer whose Sent
	// folder has an unusual name can see we found the right one.
	Mailbox string `json:"mailbox"`
	// Scanned is how many messages were read; Found is how many DISTINCT
	// addresses came out of them. Both, because "read 4000, found 60" is the
	// sentence that explains why the list is short.
	Scanned int `json:"scanned"`
	Found   int `json:"found"`
	// Skipped counts addresses left out as automated or as the account's own.
	Skipped int `json:"skipped"`
	// Complete reports whether the walk reached the bottom of the folder. False
	// means the cap stopped it, which the UI says out loud rather than implying
	// the list is everybody.
	Complete bool `json:"complete"`
}

// ErrSentFolderUnavailable means the folder could not be found or read.
var ErrSentFolderUnavailable = &domain.AppError{
	Code:    "SENT_FOLDER_UNAVAILABLE",
	Message: "não conseguimos ler a pasta de enviados desse e-mail",
}

// ErrEmailConnectionFailed is what the client sees when the mailbox could
// not be opened at all. Deliberately generic: the underlying dial/auth
// error is logged server-side only.
var ErrEmailConnectionFailed = &domain.AppError{
	Code:    "EMAIL_CONNECTION_FAILED",
	Message: "não conseguimos conectar à caixa de e-mail. Confira o servidor, a porta e a senha e tente de novo",
}

// Scan walks the channel's Sent folder and records the candidates.
//
// Synchronous and bounded rather than a background job: the customer clicks
// "buscar meus clientes" and watches, and a run that reports what it found is
// worth more than one that finishes invisibly. The cap is what keeps it bounded.
func (uc *DiscoverContactsUseCase) Scan(ctx context.Context, accountID, channelID string) (*ScanResult, error) {
	channel, err := uc.channels.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if channel == nil || channel.AccountID != accountID {
		return nil, domain.ErrChannelNotFound
	}
	if channel.Type != "email" {
		return nil, ErrChannelNotEmail
	}

	cfg, ok := sentScanConfig(channel)
	if !ok {
		return nil, &domain.AppError{
			Code:    "CHANNEL_MISSING_CONFIG",
			Message: "canal de e-mail sem credenciais para ler a caixa",
		}
	}
	own := strings.ToLower(strings.TrimSpace(domain.ChannelConfigString(channel, "from_address")))

	result := &ScanResult{}
	seen := map[string]struct{}{}
	before := uint32(0)

	for result.Scanned < uc.maxMessages {
		page, err := uc.scanner.ScanSent(ctx, cfg, before, uc.pageSize)
		if err != nil {
			// Nothing recorded so far is lost — the candidates are already
			// written — but the caller is told the run did not complete.
			if result.Scanned == 0 {
				// The transport error stays in the log. Echoed to the
				// client it doubles as a port-scan oracle: "connection
				// refused" versus "timeout" from a host the caller chose
				// maps the backend network one channel save at a time.
				slog.WarnContext(ctx, "sent scan could not open the mailbox",
					"channel_id", channelID, "error", err)
				return nil, ErrEmailConnectionFailed
			}
			slog.WarnContext(ctx, "sent scan stopped early",
				"channel_id", channelID, "scanned", result.Scanned, "error", err)
			break
		}
		if result.Mailbox == "" {
			result.Mailbox = page.Mailbox
			// Remember which folder name worked, so the next run does not
			// rediscover it and a customer's odd naming is answered once.
			cfg.Mailbox = page.Mailbox
		}

		for _, rec := range page.Recipients {
			address := strings.ToLower(strings.TrimSpace(rec.Address))
			// Her own address is not a customer, and neither is a robot. Both
			// are counted rather than silently dropped, so a list of 12 out of
			// 4000 messages has an explanation.
			if address == "" || address == own || domain.IsAutomatedAddress(address) {
				result.Skipped++
				continue
			}
			if err := uc.discovered.Upsert(ctx, accountID, channelID, rec); err != nil {
				slog.WarnContext(ctx, "could not record discovered contact",
					"channel_id", channelID, "error", err)
				continue
			}
			if _, already := seen[address]; !already {
				seen[address] = struct{}{}
				result.Found++
			}
		}

		result.Scanned += len(page.Recipients)
		if page.Done || page.LowestScannedUID == 0 {
			result.Complete = true
			break
		}
		before = page.LowestScannedUID

		// Record the position so a re-run resumes rather than re-reading
		// everything. Best-effort: the candidates are what matters, and a lost
		// cursor costs a slower second scan, not a wrong answer.
		if uc.cursors != nil {
			if _, err := uc.cursors.EnsureForChannel(ctx, accountID, channelID, page.Mailbox); err == nil {
				_ = uc.cursors.Advance(ctx, channelID, page.Mailbox,
					page.UIDValidity, page.HighestUID, uc.now())
			}
		}
	}
	return result, nil
}

// ImportInput is a human's selection.
type ImportInput struct {
	AccountID string
	FunnelID  string
	Addresses []string
	// OwnerUserID assigns the created leads. Empty leaves them unassigned,
	// which stays visible to the whole team.
	OwnerUserID string
}

// ImportResult reports what the confirmation produced.
type ImportResult struct {
	Imported int `json:"imported"`
	// Skipped counts selections that were already imported or that name an
	// address this account never discovered — reported rather than silently
	// ignored, because a count that does not match the selection is a question.
	Skipped int      `json:"skipped"`
	LeadIDs []string `json:"lead_ids"`
}

// Import turns confirmed candidates into contacts and leads.
//
// The gate: this runs only from an explicit selection. There is deliberately no
// "import everything" path — the whole value of the review step is that a human
// looked at the list, and a bulk button would make the review optional.
func (uc *DiscoverContactsUseCase) Import(ctx context.Context, in ImportInput, creator DiscoveredLeadCreator) (*ImportResult, error) {
	if creator == nil {
		return nil, &domain.AppError{
			Code:    "DISCOVERY_IMPORT_UNAVAILABLE",
			Message: "importação de contatos não configurada",
		}
	}
	if strings.TrimSpace(in.FunnelID) == "" {
		return nil, &domain.AppError{
			Code:    "VALIDATION_ERROR",
			Message: "escolha o funil onde os clientes vão entrar",
		}
	}
	candidates, err := uc.discovered.GetByAddresses(ctx, in.AccountID, in.Addresses)
	if err != nil {
		return nil, err
	}

	out := &ImportResult{}
	var imported []string
	for _, c := range candidates {
		if c.ImportedAt != nil {
			// Already in the CRM. Re-importing would create a second lead for
			// the same person and split their history in two.
			out.Skipped++
			continue
		}
		leadID, err := creator.CreateFromDiscovered(ctx, in, c)
		if err != nil {
			slog.WarnContext(ctx, "could not import discovered contact",
				"address", c.Address, "error", err)
			out.Skipped++
			continue
		}
		out.LeadIDs = append(out.LeadIDs, leadID)
		imported = append(imported, c.Address)
		out.Imported++
	}
	// Anything the selection named that this account never discovered.
	out.Skipped += len(in.Addresses) - len(candidates)

	if len(imported) > 0 {
		if err := uc.discovered.MarkImported(ctx, in.AccountID, imported, uc.now()); err != nil {
			// The leads exist; the stamp does not. Logged rather than returned:
			// the customer's work succeeded, and the worst case is the list
			// offering a name she has already brought in.
			slog.ErrorContext(ctx, "could not stamp imported contacts", "error", err)
		}
	}
	return out, nil
}

// Dismiss records "not a customer", so a rescan stops offering them.
func (uc *DiscoverContactsUseCase) Dismiss(ctx context.Context, accountID string, addresses []string) error {
	return uc.discovered.MarkDismissed(ctx, accountID, addresses, uc.now())
}

// ListPending is the review list.
func (uc *DiscoverContactsUseCase) ListPending(ctx context.Context, accountID string, limit, offset int32) ([]*domain.DiscoveredContact, error) {
	return uc.discovered.ListPending(ctx, accountID, limit, offset)
}

// CountPending feeds the nudge that tells her the list exists.
func (uc *DiscoverContactsUseCase) CountPending(ctx context.Context, accountID string) (int64, error) {
	return uc.discovered.CountPending(ctx, accountID)
}

// DiscoveredLeadCreator turns one confirmed candidate into a contact + lead.
//
// A port rather than a direct dependency because creating a lead already means
// contacts, funnels, plan limits and ownership rules, and this use case has no
// business re-deciding any of that.
type DiscoveredLeadCreator interface {
	CreateFromDiscovered(ctx context.Context, in ImportInput, c *domain.DiscoveredContact) (string, error)
}

// sentScanConfig builds the connection, reusing the sending credentials exactly
// as the inbox poller does. Mailbox is left empty so the scanner discovers the
// Sent folder by its \Sent attribute.
func sentScanConfig(channel *domain.Channel) (domain.EmailInboxConfig, bool) {
	get := func(key string) string {
		return strings.TrimSpace(domain.ChannelConfigString(channel, key))
	}
	cfg := domain.EmailInboxConfig{
		Host:     get("imap_host"),
		Port:     get("imap_port"),
		Username: get("smtp_username"),
		Password: get("smtp_password"),
	}
	if cfg.Host == "" {
		cfg.Host = get("smtp_host")
	}
	if cfg.Username == "" {
		cfg.Username = get("from_address")
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		return cfg, false
	}
	return cfg, true
}
