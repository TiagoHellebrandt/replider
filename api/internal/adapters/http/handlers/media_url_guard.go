package handlers

import (
	"net/url"
	"strings"

	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
)

// ErrInvalidMediaURL is the 400 an attendant gets for a media_url the API
// refuses to fetch on their behalf.
var ErrInvalidMediaURL = &domain.AppError{
	Code:    "INVALID_MEDIA_URL",
	Message: "media_url must be an object uploaded via /media/upload or a public https URL",
}

// validateOutboundMediaURL is the write-time half of the SSRF guard on
// caller-supplied media. A media_url reaches three fetchers with our network
// identity — the WhatsApp and Telegram adapters (audio transcode) and the
// transcriber — so an attendant who could send `http://db:5432` or the cloud
// metadata endpoint would read the response back through the channel or the
// transcript JSON. Only two shapes are accepted:
//
//  1. a GCS object path that sits inside the caller's own account prefix
//     (minted by /media/upload; signed server-side at send time), or
//  2. an https URL whose host resolves to a public address
//     (httpsafe.ValidateURL). Plain http is refused: nothing legitimate
//     hands a customer an unencrypted media link.
//
// The dial-time half — httpsafe.Client in the fetchers — re-checks the
// resolved address, so a host that flips to 127.0.0.1 between validation and
// fetch is still refused.
func validateOutboundMediaURL(accountID, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if storage.IsGCSPath(raw) {
		if !storage.ObjectBelongsToAccount(accountID, raw) {
			return ErrInvalidMediaURL
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return ErrInvalidMediaURL
	}
	if err := httpsafe.ValidateURL(raw); err != nil {
		return ErrInvalidMediaURL
	}
	return nil
}
