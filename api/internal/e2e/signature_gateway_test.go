package e2e

import (
	"context"
	"sync"

	"github.com/replider/api/internal/core/domain"
)

// e2eSignatureGateway stands in for the signature provider so the suite reaches
// no network and needs no account.
//
// It is a package-level singleton because the router is built once per suite. The
// field that matters is `truth`: what the provider's API answers when asked. Tests
// set it independently of whatever a webhook payload claims, which is the only way
// to prove the callback is verified rather than trusted.
var e2eSignatureGateway = &fakeSignatureGateway{}

type fakeSignatureGateway struct {
	mu sync.Mutex

	// truth is the provider's authoritative answer to Get.
	truth *domain.SignatureResult
	// createErr makes the send fail, for the failure-recording path.
	createErr error

	getCalls    int
	docSequence int
}

// reset clears state between tests. Called at the top of each signature test,
// because the singleton outlives them.
func (f *fakeSignatureGateway) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.truth = nil
	f.createErr = nil
	f.getCalls = 0
}

// setTruth is what the provider's API will say.
func (f *fakeSignatureGateway) setTruth(status domain.SignatureStatus, signedFileURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.truth = &domain.SignatureResult{
		ProviderDocToken: f.lastToken(), Status: status,
		SignedFileURL: signedFileURL,
	}
}

func (f *fakeSignatureGateway) lastToken() string {
	return "doc-" + itoa(f.docSequence)
}

func (f *fakeSignatureGateway) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCalls
}

func (f *fakeSignatureGateway) Name() domain.SignatureProvider {
	return domain.SignatureZapSign
}

func (f *fakeSignatureGateway) Create(
	_ context.Context, apiToken string, doc domain.SignatureDocument,
) (*domain.SignatureResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	if apiToken == "" {
		return nil, domain.ErrSignatureNotConfigured
	}
	f.docSequence++
	token := "doc-" + itoa(f.docSequence)

	signers := make([]domain.SignatureSigner, 0, len(doc.Signers))
	for i, s := range doc.Signers {
		signers = append(signers, domain.SignatureSigner{
			Name: s.Name, Email: s.Email,
			ProviderToken: token + "-s" + itoa(i+1),
			SignURL:       "https://sign.test/" + token + "/" + itoa(i+1),
			Status:        "new",
		})
	}
	return &domain.SignatureResult{
		ProviderDocToken: token, Status: domain.SignatureSent, Signers: signers,
	}, nil
}

func (f *fakeSignatureGateway) Get(
	_ context.Context, apiToken, docToken string,
) (*domain.SignatureResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if apiToken == "" {
		return nil, domain.ErrSignatureNotConfigured
	}
	if f.truth != nil {
		result := *f.truth
		result.ProviderDocToken = docToken
		return &result, nil
	}
	// Default: still out for signature. Deliberately NOT "signed" — a test that
	// forgets to set the truth must not accidentally pass a forged-callback check.
	return &domain.SignatureResult{
		ProviderDocToken: docToken, Status: domain.SignatureSent,
	}, nil
}

func (f *fakeSignatureGateway) DownloadSigned(
	context.Context, string, int64,
) ([]byte, error) {
	return []byte("%PDF-1.7 signed copy"), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

var _ domain.SignatureGateway = (*fakeSignatureGateway)(nil)
