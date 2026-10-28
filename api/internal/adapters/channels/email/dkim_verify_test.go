package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A message DKIM-signed by its own domain must verify even when the
// customer's mailbox provider stamped no Authentication-Results — that gap
// is exactly why legitimate Gmail mail rendered as "remetente não
// verificado" on GoDaddy-hosted inboxes.

func signedMessage(t *testing.T, signDomain string) ([]byte, []string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	msg := strings.Join([]string{
		"From: Tiago <tiago@" + signDomain + ">",
		"To: contato@clinica.com.br",
		"Subject: testando replider",
		"Date: Mon, 25 Aug 2026 12:00:00 -0300",
		"",
		"salve!",
	}, "\r\n") + "\r\n"

	var signed bytes.Buffer
	require.NoError(t, dkim.Sign(&signed, strings.NewReader(msg), &dkim.SignOptions{
		Domain:   signDomain,
		Selector: "sel1",
		Signer:   key,
		HeaderKeys: []string{
			"From", "To", "Subject", "Date",
		},
	}))

	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	record := "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(pub)
	return signed.Bytes(), []string{record}
}

func withTestResolver(t *testing.T, answers map[string][]string) {
	t.Helper()
	prev := dkimLookupTXT
	dkimLookupTXT = func(_ context.Context, name string) ([]string, error) {
		if v, ok := answers[name]; ok {
			return v, nil
		}
		return nil, errors.New("NXDOMAIN")
	}
	t.Cleanup(func() { dkimLookupTXT = prev })
}

func TestVerifyDKIM_AlignedSignaturePasses(t *testing.T) {
	raw, record := signedMessage(t, "gmail.com")
	withTestResolver(t, map[string][]string{"sel1._domainkey.gmail.com": record})

	got := VerifyDKIM(context.Background(), raw, "gmail.com")
	assert.Equal(t, AuthPass, got)
}

// A valid signature from an UNRELATED domain (a list's own) proves nothing
// about the From identity — it must not mint a checkmark.
func TestVerifyDKIM_UnalignedSignatureIsNotAPass(t *testing.T) {
	raw, record := signedMessage(t, "lista-de-email.com")
	withTestResolver(t, map[string][]string{"sel1._domainkey.lista-de-email.com": record})

	got := VerifyDKIM(context.Background(), raw, "gmail.com")
	assert.Equal(t, AuthUnknown, got)
}

// A tampered body is the shape a forgery takes: signatures present, none
// verifying, permanent failure.
func TestVerifyDKIM_TamperedBodyFails(t *testing.T) {
	raw, record := signedMessage(t, "gmail.com")
	withTestResolver(t, map[string][]string{"sel1._domainkey.gmail.com": record})

	tampered := bytes.Replace(raw, []byte("salve!"), []byte("golpe!"), 1)
	got := VerifyDKIM(context.Background(), tampered, "gmail.com")
	assert.Equal(t, AuthFail, got)
}

// No signatures at all: unknown, never fail — most legitimate small-provider
// mail is unsigned and must not be branded a forgery.
func TestVerifyDKIM_UnsignedIsUnknown(t *testing.T) {
	raw := []byte("From: a@b.com\r\nTo: c@d.com\r\nSubject: oi\r\n\r\ncorpo\r\n")
	withTestResolver(t, nil)

	assert.Equal(t, AuthUnknown, VerifyDKIM(context.Background(), raw, "b.com"))
}

// DNS DOWN is a verdict we could not compute — unknown, not fail. (An
// NXDOMAIN is different: a signature naming a key that does not exist is a
// permanent failure, and the library already treats it so.)
func TestVerifyDKIM_DNSOutageIsUnknown(t *testing.T) {
	raw, _ := signedMessage(t, "gmail.com")
	prev := dkimLookupTXT
	dkimLookupTXT = func(context.Context, string) ([]string, error) {
		return nil, &net.DNSError{Err: "timeout", IsTemporary: true, IsTimeout: true}
	}
	t.Cleanup(func() { dkimLookupTXT = prev })

	assert.Equal(t, AuthUnknown, VerifyDKIM(context.Background(), raw, "gmail.com"))
}

func TestDKIMDomainAlignment(t *testing.T) {
	assert.True(t, dkimDomainAligned("gmail.com", "gmail.com"))
	assert.True(t, dkimDomainAligned("empresa.com.br", "mail.empresa.com.br"))
	assert.True(t, dkimDomainAligned("mail.empresa.com.br", "empresa.com.br"))
	assert.False(t, dkimDomainAligned("outra.com", "gmail.com"))
	assert.False(t, dkimDomainAligned("", "gmail.com"))
}
