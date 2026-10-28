package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"

	"github.com/replider/api/internal/core/domain"
)

// Reading the Sent folder to learn who the customer already talks to.
//
// Deliberately NOT the same operation as reading INBOX. Inbound is a stream of
// new things to answer; this is a one-time look backwards through history to
// answer one question: who are your customers? So it walks NEWEST FIRST (recent
// correspondents matter most and a scan that is interrupted has still found the
// useful ones) and it reads ENVELOPES, not bodies.
//
// The envelope is the whole privacy argument. It carries To, Cc and Date and
// nothing else — the recipients and the timing, which is exactly the question.
// Reading the text of every message she has ever sent, to build a contact list,
// would be wildly out of proportion; and ENVELOPE is not a body fetch, so it
// cannot set \Seen either.

// ScanSent walks the Sent folder downwards from beforeUID.
//
// beforeUID == 0 starts at the top. Each call returns at most `limit` messages,
// the LOWEST uid it read, and whether the bottom was reached — so a large
// mailbox is walked across several calls without holding a session open.
func (f *IMAPFetcher) ScanSent(
	ctx context.Context,
	cfg domain.EmailInboxConfig,
	beforeUID uint32,
	limit int,
) (domain.EmailSentScanResult, error) {
	var out domain.EmailSentScanResult

	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return out, fmt.Errorf("imap: no host configured")
	}
	port := strings.TrimSpace(cfg.Port)
	if port == "" {
		port = DefaultIMAPPort
	}
	if limit <= 0 {
		limit = 200
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	c, err := f.dial(ctx, net.JoinHostPort(host, port), &tls.Config{ServerName: host})
	if err != nil {
		return out, fmt.Errorf("imap dial: %w", err)
	}
	defer func() { _ = c.Logout() }()

	if err := authenticate(c, cfg); err != nil {
		return out, err
	}

	mailbox := strings.TrimSpace(cfg.Mailbox)
	if mailbox == "" {
		mailbox, err = findSentMailbox(c)
		if err != nil {
			return out, err
		}
	}
	out.Mailbox = mailbox

	status, err := c.Select(mailbox, true) // read-only, like every folder we open
	if err != nil {
		return out, fmt.Errorf("imap select %q: %w", mailbox, err)
	}
	out.UIDValidity = status.UidValidity
	if status.Messages == 0 {
		out.Done = true
		return out, nil
	}

	uids, err := sentUIDsBelow(c, beforeUID)
	if err != nil {
		return out, err
	}
	if len(uids) == 0 {
		out.Done = true
		return out, nil
	}
	out.HighestUID = uids[0] // descending, so the first is the top

	if len(uids) > limit {
		uids = uids[:limit]
	} else {
		// Nothing older than this page: the walk has reached the bottom of the
		// folder, and a caller that stops here has seen everything.
		out.Done = true
	}
	out.LowestScannedUID = uids[len(uids)-1]

	recipients, err := f.fetchEnvelopeRecipients(ctx, c, uids)
	if err != nil {
		return out, err
	}
	out.Recipients = recipients
	return out, nil
}

// findSentMailbox asks the server which folder is the Sent one.
//
// The \Sent special-use attribute (RFC 6154) is the correct answer and the one
// that works regardless of the mailbox's language. The name list is the fallback
// for servers that do not advertise it — including the Portuguese names, because
// that is what a Brazilian customer's mailbox is actually called, and the
// dotted forms because Dovecot (Locaweb, most cPanel hosts) nests under INBOX.
func findSentMailbox(c *imapclient.Client) (string, error) {
	boxes := make(chan *imap.MailboxInfo, 64)
	done := make(chan error, 1)
	go func() { done <- c.List("", "*", boxes) }()

	var (
		bySpecialUse string
		byName       = map[string]string{}
	)
	for box := range boxes {
		if box == nil {
			continue
		}
		for _, attr := range box.Attributes {
			if strings.EqualFold(attr, imap.SentAttr) {
				bySpecialUse = box.Name
			}
		}
		byName[strings.ToLower(box.Name)] = box.Name
	}
	if err := <-done; err != nil {
		return "", fmt.Errorf("imap list: %w", err)
	}
	if bySpecialUse != "" {
		return bySpecialUse, nil
	}
	for _, candidate := range domain.SentMailboxNames {
		if actual, ok := byName[strings.ToLower(candidate)]; ok {
			return actual, nil
		}
	}
	// Named rather than guessed: picking an arbitrary folder would import the
	// wrong people, and "we could not find it" is a fixable answer.
	return "", fmt.Errorf("imap: could not find a Sent folder")
}

// sentUIDsBelow returns the folder's UIDs below beforeUID, highest first.
func sentUIDsBelow(c *imapclient.Client, beforeUID uint32) ([]uint32, error) {
	criteria := imap.NewSearchCriteria()
	set := new(imap.SeqSet)
	if beforeUID > 1 {
		set.AddRange(1, beforeUID-1)
	} else if beforeUID == 1 {
		// Nothing below UID 1.
		return nil, nil
	} else {
		set.AddRange(1, 0) // everything
	}
	criteria.Uid = set

	uids, err := c.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("imap uid search: %w", err)
	}
	// Descending: newest first, so an interrupted walk has still collected the
	// correspondents that matter.
	for i := 1; i < len(uids); i++ {
		for j := i; j > 0 && uids[j-1] < uids[j]; j-- {
			uids[j-1], uids[j] = uids[j], uids[j-1]
		}
	}
	filtered := uids[:0]
	for _, uid := range uids {
		if beforeUID == 0 || uid < beforeUID {
			filtered = append(filtered, uid)
		}
	}
	return filtered, nil
}

// maxScannedSubjectRunes bounds the stored subject. Still envelope-only data;
// the cap is because a subject line abused as a body (some ERPs dump whole
// order summaries there) must not bloat a memory field.
const maxScannedSubjectRunes = 200

func trimSubject(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > maxScannedSubjectRunes {
		return strings.TrimSpace(string(runes[:maxScannedSubjectRunes])) + "…"
	}
	return s
}

// fetchEnvelopeRecipients reads To + Cc + Subject + Date for the given UIDs.
func (f *IMAPFetcher) fetchEnvelopeRecipients(ctx context.Context, c *imapclient.Client, uids []uint32) ([]domain.SentRecipient, error) {
	set := new(imap.SeqSet)
	set.AddNum(uids...)

	// ENVELOPE only. No body section, so nothing is read of what she wrote and
	// no \Seen flag can be touched.
	items := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope}

	messages := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(set, items, messages) }()

	var out []domain.SentRecipient
	for msg := range messages {
		if msg == nil || msg.Envelope == nil {
			continue
		}
		sentAt := msg.Envelope.Date
		subject := trimSubject(msg.Envelope.Subject)
		// Bcc is deliberately not read: a blind copy is the one recipient the
		// customer chose to keep private, and a contact list that surfaces it
		// reveals something she hid on purpose.
		for _, addr := range append(msg.Envelope.To, msg.Envelope.Cc...) {
			if addr == nil {
				continue
			}
			address := strings.ToLower(strings.TrimSpace(addr.Address()))
			if address == "" || !strings.Contains(address, "@") {
				continue
			}
			out = append(out, domain.SentRecipient{
				Address: address,
				Name:    strings.TrimSpace(addr.PersonalName),
				SentAt:  sentAt,
				Subject: subject,
			})
		}
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("imap uid fetch: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
