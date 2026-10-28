package email

import (
	"bytes"
	"context"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
)

// Local DKIM verification for mail read off IMAP.
//
// The auth verdict normally comes from the Authentication-Results header the
// customer's own MX stamped on arrival. But plenty of hosted providers
// (GoDaddy/secureserver among them) stamp nothing — and then a message
// DKIM-signed by Gmail itself rendered as "remetente não verificado", which
// reads as an accusation against every legitimate sender the customer has.
//
// IMAP hands us the message exactly as the sending server wrote it, and a
// DKIM signature survives transport by design. So when the receiving server
// answered nothing, we run the check it skipped: verify the signatures in
// the raw bytes against the signer's published DNS key.
//
// Only DKIM. SPF cannot be evaluated after the fact — it is a statement
// about the CONNECTING IP, which the mailbox does not preserve.

// dkimVerifyTimeout bounds the DNS lookups one verification may spend. The
// poller has a batch to get through; an unresponsive DNS zone must cost one
// message a verdict, not the whole poll.
const dkimVerifyTimeout = 8 * time.Second

// VerifyDKIM checks the raw message's DKIM signatures and answers in the
// same vocabulary as ParseAuthResults.
//
//   - AuthPass: a signature verified AND its d= domain aligns with the From
//     domain (relaxed alignment, same-organization suffix). An aligned pass
//     is what "this really is who it claims" means; a pass from an unrelated
//     domain (a mailing list's own signature) proves nothing about From.
//   - AuthFail: signatures exist, none verified, and at least one failure
//     was permanent — the shape a forged or altered message takes.
//   - AuthUnknown: no signatures, or only temporary (DNS) failures. Never
//     guess in either direction on a lookup we could not complete.
func VerifyDKIM(ctx context.Context, raw []byte, fromDomain string) AuthResult {
	ctx, cancel := context.WithTimeout(ctx, dkimVerifyTimeout)
	defer cancel()

	verifications, err := dkim.VerifyWithOptions(bytes.NewReader(raw), &dkim.VerifyOptions{
		// The deadline rides on the lookup: the library itself has no ctx.
		LookupTXT: func(name string) ([]string, error) {
			return dkimLookupTXT(ctx, name)
		},
	})
	if err != nil || len(verifications) == 0 {
		return AuthUnknown
	}

	sawPermFail := false
	for _, v := range verifications {
		if v == nil {
			continue
		}
		if v.Err == nil {
			if dkimDomainAligned(v.Domain, fromDomain) {
				return AuthPass
			}
			continue
		}
		if !dkim.IsTempFail(v.Err) {
			sawPermFail = true
		}
	}
	if sawPermFail {
		return AuthFail
	}
	return AuthUnknown
}

// dkimLookupTXT resolves the signer's published key. A package var so tests
// can serve a key they minted instead of touching real DNS.
var dkimLookupTXT = func(ctx context.Context, name string) ([]string, error) {
	return (&net.Resolver{}).LookupTXT(ctx, name)
}

// dkimDomainAligned implements relaxed alignment: the signing domain and the
// From domain match when equal or when one is a parent of the other
// (mail.empresa.com.br signs for empresa.com.br and vice versa).
func dkimDomainAligned(signer, from string) bool {
	s := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(signer, ".")))
	f := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(from, ".")))
	if s == "" || f == "" {
		return false
	}
	return s == f || strings.HasSuffix(f, "."+s) || strings.HasSuffix(s, "."+f)
}

// EmailDomainOf lifts the domain out of an address, for alignment.
func EmailDomainOf(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 || at == len(address)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(address[at+1:]))
}
