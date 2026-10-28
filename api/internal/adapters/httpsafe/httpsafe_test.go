package httpsafe

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The guard is process-wide; every test that needs the production policy
// must restore whatever the previous test left behind.
func withPrivateDenied(t *testing.T) {
	t.Helper()
	prev := AllowsPrivateTargets()
	SetAllowPrivateTargets(false)
	t.Cleanup(func() { SetAllowPrivateTargets(prev) })
}

func TestValidateURL_BlocksInternalTargets(t *testing.T) {
	withPrivateDenied(t)

	blocked := []string{
		"http://127.0.0.1/hook",
		"http://localhost:8080/hook",      // resolves to loopback
		"https://169.254.169.254/latest/", // cloud metadata
		"http://10.0.0.5/hook",            // RFC1918
		"http://192.168.1.10/hook",        // RFC1918
		"http://172.16.0.1/hook",          // RFC1918
		"http://100.64.0.1/hook",          // CGNAT
		"http://[::1]/hook",               // IPv6 loopback
		"http://[fd00::1]/hook",           // IPv6 ULA
		"http://0.0.0.0/hook",             // unspecified
		"file:///etc/passwd",              // not http(s)
		"gopher://evil/",                  // not http(s)
		"https://",                        // no host
		"http://[fe80::1]/hook",           // link-local
		"http://169.254.1.1/hook",         // link-local v4
		"http://224.0.0.1/hook",           // multicast
	}
	for _, raw := range blocked {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("expected %q to be blocked, got nil", raw)
		} else if !errors.Is(err, ErrBlockedTarget) {
			t.Errorf("%q: expected ErrBlockedTarget, got %v", raw, err)
		}
	}
}

func TestValidateURL_AllowsPublicTarget(t *testing.T) {
	withPrivateDenied(t)

	// A literal public IP needs no DNS, so this stays hermetic.
	if err := ValidateURL("https://93.184.216.34/hook"); err != nil {
		t.Fatalf("public IP should pass, got %v", err)
	}
}

func TestValidateURL_DevModeAllowsLocalhost(t *testing.T) {
	prev := AllowsPrivateTargets()
	SetAllowPrivateTargets(true)
	t.Cleanup(func() { SetAllowPrivateTargets(prev) })

	if err := ValidateURL("http://localhost:8080/hook"); err != nil {
		t.Fatalf("localhost must pass in dev, got %v", err)
	}
}

// The dial-time guard is what stops DNS rebinding: httptest listens on
// 127.0.0.1, so a request to it must fail even though the URL was never
// validated.
func TestClient_RefusesToDialInternalAddress(t *testing.T) {
	withPrivateDenied(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// The body is nil on the error path this test asserts, but closing it
	// when there IS one keeps the leak gate honest: a future change that
	// makes this request succeed should fail on the assertion below, not
	// leak a connection quietly.
	resp, err := Client(2 * time.Second).Get(srv.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected the dialer to refuse a loopback address")
	}
	if !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("expected ErrBlockedTarget, got %v", err)
	}
}

func TestClient_AllowsInternalAddressInDevMode(t *testing.T) {
	prev := AllowsPrivateTargets()
	SetAllowPrivateTargets(true)
	t.Cleanup(func() { SetAllowPrivateTargets(prev) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := Client(2 * time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("dev mode should reach localhost, got %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// A public endpoint that 302s to another host is the SSRF laundering path;
// the redirect policy must break the chain. Run in dev mode so the dialer
// isn't what rejects it — this asserts the CheckRedirect rule specifically.
func TestClient_BlocksCrossHostRedirect(t *testing.T) {
	prev := AllowsPrivateTargets()
	SetAllowPrivateTargets(true)
	t.Cleanup(func() { SetAllowPrivateTargets(prev) })

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer target.Close()

	// Redirect to a DIFFERENT host string (127.0.0.1 vs localhost) so the
	// hostname comparison trips even though both are the same machine.
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://localhost:1/elsewhere", http.StatusFound)
	}))
	defer redirector.Close()

	resp, err := Client(2 * time.Second).Get(redirector.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected the cross-host redirect to be refused")
	}
	if !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("expected ErrBlockedTarget, got %v", err)
	}
}

// ValidateHost is the SMTP/IMAP form's guard: bare hostnames and IP
// literals, no scheme.
func TestValidateHost_BlocksInternalTargets(t *testing.T) {
	withPrivateDenied(t)

	blocked := []string{
		"127.0.0.1",
		"localhost",
		"mail.localhost",
		"169.254.169.254", // cloud metadata
		"10.0.0.5",
		"192.168.1.10",
		"172.16.0.1",
		"100.64.0.1",
		"::1",
		"[fd00::1]",
		"db.internal",
		"postgres.local",
		"",
		"smtp://smtp.gmail.com", // a scheme is not a hostname
		"smtp.gmail.com:465",    // the port has its own field
	}
	for _, h := range blocked {
		if err := ValidateHost(h); !errors.Is(err, ErrBlockedTarget) {
			t.Errorf("ValidateHost(%q) = %v, want ErrBlockedTarget", h, err)
		}
	}
}

func TestValidateHost_AcceptsPublicMailServers(t *testing.T) {
	withPrivateDenied(t)

	// Known providers are accepted without a lookup; a public IP literal is
	// accepted on its own merits.
	for _, h := range []string{"smtp.gmail.com", "SMTP.Office365.com", "email-ssl.com.br", "8.8.8.8", "smtp.gmail.com."} {
		if err := ValidateHost(h); err != nil {
			t.Errorf("ValidateHost(%q) = %v, want nil", h, err)
		}
	}
}

func TestValidateHost_DevPolicyAllowsPrivate(t *testing.T) {
	prev := AllowsPrivateTargets()
	SetAllowPrivateTargets(true)
	t.Cleanup(func() { SetAllowPrivateTargets(prev) })

	if err := ValidateHost("localhost"); err != nil {
		t.Fatalf("dev policy must allow a local mail server: %v", err)
	}
}

// The Dialer's Control hook is the boundary for IMAP/SMTP: whatever a name
// resolved to at validation time, the socket is refused if the address it
// is about to connect to is not public.
func TestDialer_ControlRefusesPrivateAddress(t *testing.T) {
	withPrivateDenied(t)

	for _, addr := range []string{"127.0.0.1:993", "10.1.2.3:465", "169.254.169.254:80", "[::1]:587", "not-an-ip:25"} {
		if err := checkDialAddress(addr); !errors.Is(err, ErrBlockedTarget) {
			t.Errorf("checkDialAddress(%q) = %v, want ErrBlockedTarget", addr, err)
		}
	}
	if err := checkDialAddress("8.8.8.8:465"); err != nil {
		t.Errorf("public address refused: %v", err)
	}

	// End to end: a dial to loopback never opens the socket.
	d := Dialer(time.Second)
	if _, err := d.Dial("tcp", "127.0.0.1:1"); !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("Dialer connected to loopback: %v", err)
	}
}
