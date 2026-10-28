// Package httpsafe hardens every outbound HTTP call whose destination is
// chosen by a CUSTOMER (webhook subscriptions, per-funnel webhooks) rather
// than by us. Such a URL is an SSRF primitive: without a guard, a tenant can
// point it at 127.0.0.1, the Docker bridge, or the cloud metadata endpoint
// (169.254.169.254) and make the API fetch internal resources on their behalf
// — with our network identity (US-F011 / V2).
//
// Two layers, because either alone is bypassable:
//
//  1. ValidateURL at WRITE time (webhook create/update): fail fast with a
//     message the user can act on, and keep obviously-hostile rows out of
//     the DB.
//  2. Client() at DIAL time: the guard runs inside the dialer's Control hook,
//     on the address the connection is ACTUALLY going to. This is what stops
//     DNS rebinding — a hostname that resolved public at validation time and
//     flips to 127.0.0.1 seconds later never gets connected to. Redirects are
//     bound to the original host for the same reason.
package httpsafe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// ErrBlockedTarget is returned for any URL that resolves to a non-public
// address, or that isn't an http(s) URL at all. Callers map it to a 400.
var ErrBlockedTarget = errors.New("webhook target is not a public http(s) address")

// allowPrivate opens the guard for loopback/private destinations. Dev and
// tests legitimately point webhooks at localhost; production must not.
// Package-level (rather than threaded through a dozen constructors) because
// it is a single process-wide deployment fact, set once at boot.
var allowPrivate atomic.Bool

// SetAllowPrivateTargets is called once from main.go with
// `cfg.Environment != "production"`. Default (never called) is the safe
// value: private targets denied.
func SetAllowPrivateTargets(allow bool) { allowPrivate.Store(allow) }

// AllowsPrivateTargets reports the current policy (for logs/tests).
func AllowsPrivateTargets() bool { return allowPrivate.Load() }

// resolveTimeout bounds the DNS lookup done during validation so a tenant
// can't park a request goroutine on a deliberately slow resolver.
const resolveTimeout = 3 * time.Second

// ValidateURL rejects a customer-supplied webhook URL that is not a plain
// http(s) URL, or whose host resolves — even partially — to an address we
// refuse to talk to. "Even partially" matters: a hostname with one public
// and one loopback A record must be rejected, or the dialer's choice
// decides whether we get exploited.
func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBlockedTarget, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q", ErrBlockedTarget, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrBlockedTarget)
	}
	if allowPrivate.Load() {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("%w: %s", ErrBlockedTarget, ip)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve %s", ErrBlockedTarget, host)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: %s resolves to nothing", ErrBlockedTarget, host)
	}
	for _, a := range addrs {
		if !isPublicIP(a.IP) {
			return fmt.Errorf("%w: %s resolves to %s", ErrBlockedTarget, host, a.IP)
		}
	}
	return nil
}

// isPublicIP reports whether we are willing to open a connection to ip.
// Everything reserved, internal, or otherwise not globally routable is out:
// loopback, RFC1918, link-local (which covers the 169.254.169.254 cloud
// metadata endpoint), CGNAT, multicast, unspecified, IPv6 ULA.
func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.IsPrivate() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		// 100.64.0.0/10 carrier-grade NAT — reaches other tenants of the
		// same provider network, not the public internet.
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return false
		}
		// 0.0.0.0/8 "this network" and 240.0.0.0/4 reserved.
		if v4[0] == 0 || v4[0] >= 240 {
			return false
		}
		return true
	}
	// IPv6 unique-local fc00::/7 (ip.IsPrivate covers it on modern Go, but
	// be explicit — this check is the security boundary).
	if len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc {
		return false
	}
	return true
}

// knownMailHosts are the mail servers of the hosted providers customers
// actually type into the SMTP/IMAP form. They are public by construction,
// so ValidateHost accepts them without a DNS round-trip — which also keeps
// unit tests that use them off the network.
var knownMailHosts = map[string]struct{}{
	"smtp.gmail.com": {}, "imap.gmail.com": {},
	"smtp.office365.com": {}, "outlook.office365.com": {}, "smtp-mail.outlook.com": {},
	"smtp.zoho.com": {}, "imap.zoho.com": {}, "smtp.zohomail.com": {},
	"smtp.titan.email": {}, "imap.titan.email": {},
	"email-ssl.com.br": {}, "smtplw.com.br": {}, // Locaweb
	"smtp.sendgrid.net": {}, "smtp.mailgun.org": {}, "smtp.resend.com": {},
	"smtp.postmarkapp.com": {}, "smtp.mandrillapp.com": {},
	"smtp.uol.com.br": {}, "imap.uol.com.br": {},
	"smtp.terra.com.br": {}, "imap.terra.com.br": {},
	"smtp.mail.yahoo.com": {}, "imap.mail.yahoo.com": {},
}

// ValidateHost is ValidateURL for a bare hostname or IP literal — the shape
// an SMTP/IMAP form takes. Same rules: an IP literal must be public; a name
// must not resolve, even partially, to a loopback, private, link-local or
// otherwise reserved address. Well-known hosted mail providers are accepted
// without resolving.
//
// A name that does not resolve at all is ACCEPTED here, unlike in
// ValidateURL: a mail host is dialed only through Dialer, whose Control hook
// re-checks the resolved address, so a typo surfaces as a connection error
// on the test-send rather than as a rejected form — and the guard that
// matters is the one at the socket, not this one.
func ValidateHost(raw string) error {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrBlockedTarget)
	}
	if strings.ContainsAny(host, "/:@ ") && net.ParseIP(strings.Trim(host, "[]")) == nil {
		// A scheme, a port, credentials or whitespace: not a hostname.
		return fmt.Errorf("%w: %q is not a hostname", ErrBlockedTarget, raw)
	}
	if allowPrivate.Load() {
		return nil
	}
	if _, ok := knownMailHosts[host]; ok {
		return nil
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("%w: %s", ErrBlockedTarget, ip)
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("%w: %s", ErrBlockedTarget, host)
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		if !isPublicIP(a.IP) {
			return fmt.Errorf("%w: %s resolves to %s", ErrBlockedTarget, host, a.IP)
		}
	}
	return nil
}

// Dialer returns a net.Dialer whose Control hook refuses to connect to a
// non-public address. It is the dial-time half of the guard for protocols
// that are not HTTP — the IMAP and SMTP clients that talk to a mail server
// the customer named. Client builds on the same dialer.
//
// ControlContext runs AFTER DNS resolution with the address the socket is
// about to connect to — the only place a rebinding attack is observable.
// `address` is "ip:port", never a hostname. A zero timeout leaves the
// dialer's timeout to the caller's context.
func Dialer(timeout time.Duration) *net.Dialer {
	dialer := &net.Dialer{KeepAlive: 30 * time.Second}
	if timeout > 0 {
		dialer.Timeout = timeout
	}
	dialer.ControlContext = func(_ context.Context, _ string, address string, _ syscall.RawConn) error {
		return checkDialAddress(address)
	}
	return dialer
}

// checkDialAddress is the Control-hook body, split out so a test can
// exercise the decision without opening a socket.
func checkDialAddress(address string) error {
	if allowPrivate.Load() {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlockedTarget, address)
	}
	ip := net.ParseIP(host)
	if ip == nil || !isPublicIP(ip) {
		return fmt.Errorf("%w: %s", ErrBlockedTarget, address)
	}
	return nil
}

// Client builds an http.Client that enforces the guard at DIAL time and
// refuses cross-host redirects. Use it for every request to a
// customer-supplied URL; a bare http.Client bypasses the whole package.
func Client(timeout time.Duration) *http.Client {
	dialer := Dialer(timeout)

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			// A customer endpoint is never worth a persistent pool slot.
			MaxIdleConnsPerHost: 2,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			// Cross-host redirect is the classic SSRF laundering trick:
			// a public URL 302s to 127.0.0.1. Bind the whole chain to the
			// host the customer registered.
			if !strings.EqualFold(req.URL.Hostname(), via[0].URL.Hostname()) {
				return fmt.Errorf("%w: redirect to %s", ErrBlockedTarget, req.URL.Hostname())
			}
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return ValidateURL(req.URL.String())
		},
	}
}
