package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Signed-URL TTL bounds (US-028). A too-short TTL breaks legitimate client
// flows (slow uploads, retries, a dashboard left open a few minutes); a
// too-long one is a security exposure — a leaked URL stays usable for days.
const (
	minSignedURLTTL = 5 * time.Minute
	maxSignedURLTTL = 24 * time.Hour
)

// GCSAdapter handles interactions with Google Cloud Storage
type GCSAdapter struct {
	bucketName        string
	credentialsBase64 string

	// clientMu guards the memoized client. A *gcs.Client is safe for
	// concurrent use and is MEANT to be long-lived: building one signs a JWT
	// and exchanges it for an OAuth token over the network.
	//
	// Every method used to build its own and close it. That put a full token
	// exchange in front of every upload, every signed URL and every health
	// probe — and the probe, which runs on a 4s readiness budget, timed out
	// often enough to log `gcs bucket "replider" unreachable: context
	// deadline exceeded` and flip the whole endpoint to 503 while uploads
	// were in fact working fine. Crying wolf about a dependency that is up
	// is worse than not checking it.
	clientMu sync.Mutex
	client   *gcs.Client
}

// validateSignedURLTTL rejects an expiry outside [5m, 24h]. Enforced at the
// single choke point every signed URL passes through, so no caller can mint
// a URL that lives too long (or expires before it's usable). (US-028)
func validateSignedURLTTL(expiry time.Duration) error {
	if expiry < minSignedURLTTL || expiry > maxSignedURLTTL {
		return fmt.Errorf("signed URL expiry %s out of allowed range [%s, %s]", expiry, minSignedURLTTL, maxSignedURLTTL)
	}
	return nil
}

// NewGCSAdapter creates a new GCS adapter
func NewGCSAdapter(bucketName, credentialsBase64 string) *GCSAdapter {
	return &GCSAdapter{
		bucketName:        bucketName,
		credentialsBase64: credentialsBase64,
	}
}

// zeroBytes overwrites b in place so the decoded service-account key does not
// linger on the heap. (US-078)
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// withDecodedCredentials decodes the Base64 service-account JSON, hands the raw
// bytes to fn, and unconditionally zeros them before returning. The GCS SDK's
// option.WithCredentialsJSON copies the slice internally and NewClient parses
// it synchronously, so wiping our copy afterwards leaves a working client while
// ensuring a later heap dump yields no usable GCS credentials. (US-078)
func (a *GCSAdapter) withDecodedCredentials(fn func(decoded []byte) (*gcs.Client, error)) (*gcs.Client, error) {
	if a.credentialsBase64 == "" {
		return nil, fmt.Errorf("GCS credentials not configured (Base64 string is empty)")
	}
	decodedJSON, err := base64.StdEncoding.DecodeString(a.credentialsBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GCS credentials Base64: %w", err)
	}
	defer zeroBytes(decodedJSON)
	return fn(decodedJSON)
}

// newClient returns the shared, authenticated GCS client, building it on
// first use.
//
// A failed build is NOT cached: a token exchange that timed out once must not
// leave the adapter permanently broken, so the next caller tries again.
//
// The context belongs to the CALLER, and the client outlives it. That is safe
// here — `gcs.NewClient` uses the context for the credential fetch only, not
// for the client's lifetime — but it does mean a caller with a very short
// deadline can fail the build for everyone that instant. They will retry.
func (a *GCSAdapter) newClient(ctx context.Context) (*gcs.Client, error) {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()
	if a.client != nil {
		return a.client, nil
	}
	client, err := a.withDecodedCredentials(func(decoded []byte) (*gcs.Client, error) {
		return gcs.NewClient(ctx, option.WithCredentialsJSON(decoded))
	})
	if err != nil {
		return nil, err
	}
	a.client = client
	return client, nil
}

// Close releases the shared client. Called once at shutdown; the adapter is
// unusable afterwards, which is the point.
func (a *GCSAdapter) Close() error {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()
	if a.client == nil {
		return nil
	}
	err := a.client.Close()
	a.client = nil
	return err
}

// GenerateSignedURL creates a V4 signed URL for uploading (PUT) a file to GCS.
// The client can PUT a file directly to the returned URL.
func (a *GCSAdapter) GenerateSignedURL(ctx context.Context, objectName, contentType string, expiry time.Duration) (string, error) {
	if err := validateSignedURLTTL(expiry); err != nil {
		return "", err
	}
	client, err := a.newClient(ctx)
	if err != nil {
		return "", err
	}

	opts := &gcs.SignedURLOptions{
		Scheme:      gcs.SigningSchemeV4,
		Method:      "PUT",
		ContentType: contentType,
		Expires:     time.Now().Add(expiry),
	}

	url, err := client.Bucket(a.bucketName).SignedURL(objectName, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate upload signed URL: %w", err)
	}
	return url, nil
}

// GenerateReadSignedURL creates a V4 signed URL for reading (GET) an object from GCS.
// The URL is time-limited; use 30 minutes for dashboard display, 1 hour for channel sends.
func (a *GCSAdapter) GenerateReadSignedURL(ctx context.Context, objectPath string, expiry time.Duration) (string, error) {
	if err := validateSignedURLTTL(expiry); err != nil {
		return "", err
	}
	client, err := a.newClient(ctx)
	if err != nil {
		return "", err
	}

	opts := &gcs.SignedURLOptions{
		Scheme:  gcs.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	}

	url, err := client.Bucket(a.bucketName).SignedURL(objectPath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate read signed URL: %w", err)
	}
	return url, nil
}

// DownloadObject fetches the full object bytes via the GCS SDK (not a
// signed URL). Used by the ingestion worker to extract text from an
// uploaded PDF — going through a signed GET would add a roundtrip
// and an extra auth hop for no gain since the worker already holds
// the service-account credentials.
//
// maxBytes caps how much we'll read. A PDF larger than the cap
// returns an error so a misbehaving upload can't OOM the worker.
func (a *GCSAdapter) DownloadObject(ctx context.Context, objectPath string, maxBytes int64) ([]byte, error) {
	client, err := a.newClient(ctx)
	if err != nil {
		return nil, err
	}

	r, err := client.Bucket(a.bucketName).Object(objectPath).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs: open object: %w", err)
	}
	defer r.Close()

	// +1 byte past the cap so we can distinguish "exactly max" from
	// "overflow". io.ReadAll on a LimitReader never errors on EOF,
	// so we check the returned length.
	limited := io.LimitReader(r, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("gcs: read object: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("gcs: object exceeds %d byte cap", maxBytes)
	}
	return body, nil
}

// objectStream used to own the GCS client and close it with the reader, back
// when every call built its own. The client is shared now, so closing it here
// would tear down the connection for every other caller — the reader is all
// this owns.
type objectStream struct {
	io.ReadCloser
}

// OpenObject streams an object instead of buffering it.
//
// DownloadObject reads the whole body into memory under a cap, which is right
// for a document the worker parses. This is for the media proxy, where the
// bytes are only being copied to an HTTP response: buffering a 16MB video per
// concurrent viewer is memory the process never needed to hold.
//
// The caller MUST Close the returned reader.
// Returns the object's stored content type and size so the handler can set the
// response headers without a second round trip.
func (a *GCSAdapter) OpenObject(ctx context.Context, objectPath string) (io.ReadCloser, string, int64, error) {
	client, err := a.newClient(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	r, err := client.Bucket(a.bucketName).Object(objectPath).NewReader(ctx)
	if err != nil {
		return nil, "", 0, fmt.Errorf("gcs: open object: %w", err)
	}
	return &objectStream{ReadCloser: r}, r.Attrs.ContentType, r.Attrs.Size, nil
}

// IsGCSPath reports whether s is a GCS object path (not an external HTTP URL).
// External URLs (WhatsApp/Telegram CDNs) start with "http".
func IsGCSPath(s string) bool {
	return s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")
}

// DeleteObject removes objectPath from the bucket. Returns nil when
// the object is already gone (NotFound is treated as success so the
// caller does not need to special-case "deleted twice").
func (a *GCSAdapter) DeleteObject(ctx context.Context, objectPath string) error {
	client, err := a.newClient(ctx)
	if err != nil {
		return err
	}

	if err := client.Bucket(a.bucketName).Object(objectPath).Delete(ctx); err != nil {
		if err == gcs.ErrObjectNotExist {
			return nil
		}
		return fmt.Errorf("gcs: delete object: %w", err)
	}
	return nil
}

// DeletePrefix removes every object whose name starts with prefix.
// Returns the number of objects deleted. Used by account-delete flows
// to wipe `accounts/{id}/...` so GDPR right-to-be-forgotten is honored
// end-to-end (database cascade alone leaves the GCS bytes intact).
//
// Best-effort per object: a single delete failure aborts the loop and
// surfaces the error so the caller can retry. NotFound on an object
// returned by List is rare but treated as success.
func (a *GCSAdapter) DeletePrefix(ctx context.Context, prefix string) (int, error) {
	if prefix == "" {
		return 0, fmt.Errorf("gcs: refusing to delete empty prefix")
	}
	client, err := a.newClient(ctx)
	if err != nil {
		return 0, err
	}

	bucket := client.Bucket(a.bucketName)
	it := bucket.Objects(ctx, &gcs.Query{Prefix: prefix})
	deleted := 0
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return deleted, fmt.Errorf("gcs: list under prefix %q: %w", prefix, err)
		}
		if err := bucket.Object(attrs.Name).Delete(ctx); err != nil {
			if err == gcs.ErrObjectNotExist {
				continue
			}
			return deleted, fmt.Errorf("gcs: delete %q: %w", attrs.Name, err)
		}
		deleted++
	}
	return deleted, nil
}

// UploadBytes writes data to objectPath and sets contentType. Used by
// flows where the API itself holds the bytes (proxy uploads, cached
// copies of provider-side artifacts) instead of issuing a signed URL
// for the client to PUT directly.
func (a *GCSAdapter) UploadBytes(ctx context.Context, objectPath string, data []byte, contentType string) error {
	client, err := a.newClient(ctx)
	if err != nil {
		return err
	}

	w := client.Bucket(a.bucketName).Object(objectPath).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		_ = w.Close()
		return fmt.Errorf("gcs: upload bytes: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs: finalize upload: %w", err)
	}
	return nil
}

// Ping verifies the bucket is actually reachable and readable with our
// credentials (US-F015). Media is the one dependency whose failure is
// invisible in normal traffic — signed URLs are minted locally and only fail
// when the CUSTOMER clicks them, so a broken bucket surfaces as "images don't
// load" hours later. A real Attrs round-trip catches revoked credentials, a
// deleted bucket and a network partition; a config check catches none of them.
func (a *GCSAdapter) Ping(ctx context.Context) error {
	if a.bucketName == "" || a.credentialsBase64 == "" {
		return fmt.Errorf("GCS bucket or credentials not configured")
	}
	client, err := a.newClient(ctx)
	if err != nil {
		return fmt.Errorf("gcs client: %w", err)
	}

	if _, err := client.Bucket(a.bucketName).Attrs(ctx); err != nil {
		return fmt.Errorf("gcs bucket %q unreachable: %w", a.bucketName, err)
	}
	return nil
}
