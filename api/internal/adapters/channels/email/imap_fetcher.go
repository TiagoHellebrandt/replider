package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	imapclient "github.com/emersion/go-imap/client"

	"github.com/emersion/go-imap"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
)

// IMAPFetcher reads new mail from the customer's own mailbox.
//
// THE THREE RULES, and they are the whole design:
//
//  1. BODY.PEEK, never BODY. Fetching a body without PEEK sets \Seen, so the
//     salesperson's unread badge empties itself every time we poll. Their inbox
//     is a tool they use; we are a guest in it.
//  2. The cursor is a UID, never the \Seen flag. Flags are shared mutable state
//     the human touches — filter on "unseen" and a reply they glanced at on
//     their phone is one we never ingest. A UID is the server's own monotonic
//     counter.
//  3. Nothing is moved, deleted, or expunged. Ever.
//
// Read-only is enforced structurally where the protocol allows it: the folder is
// opened with SELECT in read-only mode, so a bug that tried to write would be
// refused by the server rather than by our own discipline.
type IMAPFetcher struct {
	// timeout bounds the whole poll. A mail server that stops answering
	// mid-fetch must not hold a worker slot indefinitely — the mailbox is still
	// there next tick.
	timeout time.Duration
	// dial is swappable so tests can point at an in-process server. Production
	// always uses implicit TLS; see dialTLS for why there is no plaintext path.
	dial func(ctx context.Context, addr string, tlsConfig *tls.Config) (*imapclient.Client, error)
}

func NewIMAPFetcher() *IMAPFetcher {
	return &IMAPFetcher{timeout: 45 * time.Second, dial: dialTLS}
}

var _ domain.EmailInboxFetcher = (*IMAPFetcher)(nil)

// DefaultIMAPPort is implicit TLS. Chosen as the default because it is what
// every hosted provider documents (Locaweb, Titan, Zoho and Workspace all say
// 993) and because the alternative, 143, carries the password in the clear
// until STARTTLS succeeds.
const DefaultIMAPPort = "993"

// DefaultMailbox is the folder polled for replies.
const DefaultMailbox = "INBOX"

// maxRawMessageBytes caps one fetched message. A 40MB attachment chain read
// into memory to find a two-line reply is how a poller takes the process down;
// past this the message is skipped with a log and the cursor still advances,
// because retrying it forever would block every message behind it.
const maxRawMessageBytes = 15 << 20

func dialTLS(ctx context.Context, addr string, tlsConfig *tls.Config) (*imapclient.Client, error) {
	// The host is the customer's: httpsafe.Dialer re-checks the resolved
	// address in the socket's Control hook, so a hostname pointed at the
	// database or the metadata endpoint is refused after DNS, not before.
	dialer := httpsafe.Dialer(0)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	// The password goes over this socket, so TLS is established before the
	// protocol starts rather than negotiated inside it.
	return imapclient.New(tls.Client(conn, tlsConfig))
}

// Fetch opens the mailbox read-only and returns messages above afterUID.
func (f *IMAPFetcher) Fetch(
	ctx context.Context,
	cfg domain.EmailInboxConfig,
	knownUIDValidity, afterUID uint32,
	limit int,
) (domain.EmailFetchResult, error) {
	var out domain.EmailFetchResult

	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return out, fmt.Errorf("imap: no host configured")
	}
	port := strings.TrimSpace(cfg.Port)
	if port == "" {
		port = DefaultIMAPPort
	}
	mailbox := strings.TrimSpace(cfg.Mailbox)
	if mailbox == "" {
		mailbox = DefaultMailbox
	}
	if limit <= 0 {
		limit = 50
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	c, err := f.dial(ctx, net.JoinHostPort(host, port), &tls.Config{ServerName: host})
	if err != nil {
		return out, fmt.Errorf("imap dial: %w", err)
	}
	// Logout is the polite close and also flushes the server's session; a
	// dropped connection leaves it holding state until its own timeout, and
	// providers cap concurrent sessions per account.
	defer func() { _ = c.Logout() }()

	if err := authenticate(c, cfg); err != nil {
		return out, err
	}

	// Read-only SELECT: rule 3, enforced by the server instead of by us.
	status, err := c.Select(mailbox, true)
	if err != nil {
		return out, fmt.Errorf("imap select %q: %w", mailbox, err)
	}
	out.UIDValidity = status.UidValidity

	// The cursor's UIDs addressed a different generation of this mailbox. Report
	// the new validity and read nothing: deciding whether to re-anchor belongs
	// to the caller, which knows whether skipping mail is acceptable, and
	// guessing here would either re-import everything or silently lose it.
	if knownUIDValidity != 0 && status.UidValidity != knownUIDValidity {
		out.HighestUID = highestUID(ctx, c, status)
		return out, nil
	}

	out.HighestUID = highestUID(ctx, c, status)

	// Nothing above the cursor. Also the shape of an empty mailbox, and of a
	// first connection that just anchored to the top.
	if status.Messages == 0 || (out.HighestUID != 0 && out.HighestUID <= afterUID) {
		return out, nil
	}

	uids, err := searchNewerUIDs(c, afterUID)
	if err != nil {
		return out, err
	}
	if len(uids) == 0 {
		return out, nil
	}
	if len(uids) > limit {
		uids = uids[:limit]
		out.More = true
	}

	msgs, err := f.fetchBodies(ctx, c, uids)
	if err != nil {
		return out, err
	}
	out.Messages = msgs
	return out, nil
}

// searchNewerUIDs asks the server which UIDs are above the cursor.
//
// A UID SEARCH rather than a blind range fetch: the range `afterUID+1:*` is
// legal but a server answers it with the LAST message when nothing is above the
// cursor — a documented IMAP quirk that would re-ingest the newest message on
// every single poll. Asking which UIDs exist gets an empty answer instead of a
// wrong one.
func searchNewerUIDs(c *imapclient.Client, afterUID uint32) ([]uint32, error) {
	criteria := imap.NewSearchCriteria()
	set := new(imap.SeqSet)
	set.AddRange(afterUID+1, 0) // 0 is "*", the top of the mailbox
	criteria.Uid = set

	uids, err := c.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("imap uid search: %w", err)
	}
	// Ascending, so a partial ingest can advance the cursor to the last message
	// it actually handled rather than skipping the ones it did not.
	for i := 1; i < len(uids); i++ {
		for j := i; j > 0 && uids[j-1] > uids[j]; j-- {
			uids[j-1], uids[j] = uids[j], uids[j-1]
		}
	}
	filtered := uids[:0]
	for _, uid := range uids {
		if uid > afterUID {
			filtered = append(filtered, uid)
		}
	}
	return filtered, nil
}

// fetchBodies reads the raw messages for the given UIDs.
func (f *IMAPFetcher) fetchBodies(ctx context.Context, c *imapclient.Client, uids []uint32) ([]domain.FetchedEmail, error) {
	set := new(imap.SeqSet)
	set.AddNum(uids...)

	// Rule 1: PEEK. `imap.FetchRFC822` would set \Seen on every message we
	// read, emptying the salesperson's unread badge as a side effect of us
	// working. The section below is the same bytes without the flag change.
	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{imap.FetchUid, imap.FetchInternalDate, section.FetchItem()}

	messages := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(set, items, messages) }()

	out := make([]domain.FetchedEmail, 0, len(uids))
	for msg := range messages {
		if msg == nil {
			continue
		}
		body := msg.GetBody(section)
		if body == nil {
			continue
		}
		raw, err := readAtMost(body, maxRawMessageBytes)
		if err != nil {
			// Skipped, not retried forever: one oversized message must not park
			// every reply behind it. The caller still advances past this UID.
			continue
		}
		out = append(out, domain.FetchedEmail{
			UID:          msg.Uid,
			RawMessage:   raw,
			InternalDate: msg.InternalDate,
		})
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("imap uid fetch: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// highestUID reports the top of the mailbox, which is what a first connection
// anchors to. Failing to read it returns 0, which the caller reads as "cannot
// anchor yet" and retries — safer than anchoring to a wrong number, because an
// anchor that is too high skips real mail forever.
func highestUID(ctx context.Context, c *imapclient.Client, status *imap.MailboxStatus) uint32 {
	if status.Messages == 0 {
		return 0
	}
	if err := ctx.Err(); err != nil {
		return 0
	}
	set := new(imap.SeqSet)
	set.AddNum(status.Messages) // the last message by sequence number

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- c.Fetch(set, []imap.FetchItem{imap.FetchUid}, messages) }()

	var uid uint32
	for msg := range messages {
		if msg != nil && msg.Uid > uid {
			uid = msg.Uid
		}
	}
	if err := <-done; err != nil {
		return 0
	}
	return uid
}

// authenticate proves who we are, by password or by consent.
//
// One function for both because the alternative is two call sites (Fetch and
// ScanSent) each deciding, and a mailbox that reads on one path and refuses on
// the other is the kind of half-connected state that looks like a server fault.
func authenticate(c *imapclient.Client, cfg domain.EmailInboxConfig) error {
	if cfg.AccessToken != "" {
		if err := c.Authenticate(newIMAPXOAuth2(cfg.Username, cfg.AccessToken)); err != nil {
			// Named, not passed through: a revoked grant would otherwise be
			// logged as an authentication failure every poll, forever, with
			// nothing telling anybody to reconnect.
			return fmt.Errorf("%w: %w", domain.ErrEmailOAuthNotConnected, err)
		}
		return nil
	}
	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		return fmt.Errorf("imap login: %w", err)
	}
	return nil
}
