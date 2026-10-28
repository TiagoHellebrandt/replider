// Package dnsprovider writes email-authentication records into a customer's
// own DNS zone, so the four records that gate sending do not have to be
// pasted by hand into a registrar panel.
package dnsprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// CloudflareProvisioner publishes records through the Cloudflare API.
//
// Cloudflare first because it is where a large share of domains already
// live, and because its API token model fits what we need exactly: the
// customer mints a token scoped to Zone.DNS:Edit on one zone, we use it for
// the few seconds it takes to write four records, and it is never stored.
//
// That "never stored" is the whole security posture. A token that can edit a
// company's DNS is close to the keys to their identity — anyone holding it
// can redirect their mail. Keeping one at rest to save the customer a paste
// on some future visit is a trade nobody asked for.
type CloudflareProvisioner struct {
	client  *http.Client
	baseURL string
}

func NewCloudflareProvisioner() *CloudflareProvisioner {
	return &CloudflareProvisioner{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://api.cloudflare.com/client/v4",
	}
}

// WithBaseURL points the provisioner at a test server.
func (p *CloudflareProvisioner) WithBaseURL(u string) *CloudflareProvisioner {
	p.baseURL = strings.TrimRight(u, "/")
	return p
}

func (p *CloudflareProvisioner) Name() string { return "cloudflare" }

// Apply publishes the setup's records into the customer's zone.
func (p *CloudflareProvisioner) Apply(ctx context.Context, credential string, setup domain.EmailDNSSetup) (domain.DNSProvisionReport, error) {
	token := strings.TrimSpace(credential)
	if token == "" {
		return domain.DNSProvisionReport{}, domain.ErrDNSCredentialInvalid
	}

	zoneID, zoneName, err := p.findZone(ctx, token, setup.Domain)
	if err != nil {
		return domain.DNSProvisionReport{}, err
	}

	report := domain.DNSProvisionReport{Provider: p.Name(), Zone: zoneName}
	for _, rec := range setup.Records {
		report.Results = append(report.Results, p.applyRecord(ctx, token, zoneID, rec))
	}
	return report, nil
}

func (p *CloudflareProvisioner) applyRecord(ctx context.Context, token, zoneID string, rec domain.EmailDNSRecord) domain.DNSProvisionResult {
	out := domain.DNSProvisionResult{Kind: rec.Kind, Host: rec.Host}

	existing, err := p.findRecords(ctx, token, zoneID, rec.Type, rec.Host)
	if err != nil {
		out.Action = domain.DNSActionFailed
		out.Detail = err.Error()
		return out
	}

	switch rec.Kind {
	case domain.DNSKindSPF:
		return p.applySPF(ctx, token, zoneID, rec, existing)
	case domain.DNSKindDMARC:
		// An existing DMARC policy is LEFT ALONE. A customer running
		// p=quarantine or p=reject is strictly better protected than our
		// suggested p=none, and replacing theirs with ours would quietly
		// weaken their domain to make our checklist go green.
		for _, e := range existing {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(e.Content)), "v=dmarc1") {
				out.Action = domain.DNSActionKept
				out.Detail = "você já tem uma política DMARC publicada; mantivemos a sua"
				return out
			}
		}
	}

	// Everything else is ours to own: the DKIM CNAME points at our signing
	// key and the tracking CNAME at our host, so an existing value under
	// those exact names is either ours already or stale.
	for _, e := range existing {
		if strings.EqualFold(strings.TrimSuffix(e.Content, "."), strings.TrimSuffix(rec.Value, ".")) {
			out.Action = domain.DNSActionKept
			out.Detail = "já estava publicado corretamente"
			return out
		}
	}
	if len(existing) > 0 {
		if err := p.updateRecord(ctx, token, zoneID, existing[0].ID, rec.Type, rec.Host, rec.Value); err != nil {
			out.Action, out.Detail = domain.DNSActionFailed, err.Error()
			return out
		}
		out.Action = domain.DNSActionUpdated
		return out
	}
	if err := p.createRecord(ctx, token, zoneID, rec.Type, rec.Host, rec.Value); err != nil {
		out.Action, out.Detail = domain.DNSActionFailed, err.Error()
		return out
	}
	out.Action = domain.DNSActionCreated
	return out
}

// applySPF merges rather than replaces.
//
// A domain may have only ONE SPF record, and a customer already sending
// through Google Workspace, an ERP and an NF-e provider has all of them in
// it. Replacing that record stops every one of those systems from
// delivering — silently, with no error anywhere they would look.
func (p *CloudflareProvisioner) applySPF(ctx context.Context, token, zoneID string, rec domain.EmailDNSRecord, existing []cfDNSRecord) domain.DNSProvisionResult {
	out := domain.DNSProvisionResult{Kind: rec.Kind, Host: rec.Host}

	include := spfIncludeOf(rec.Value)
	for _, e := range existing {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(e.Content)), "v=spf1") {
			continue
		}
		merged, changed := domain.MergeSPF(e.Content, include)
		if !changed {
			out.Action = domain.DNSActionKept
			out.Detail = "seu SPF já autoriza nosso envio"
			return out
		}
		if err := p.updateRecord(ctx, token, zoneID, e.ID, rec.Type, rec.Host, merged); err != nil {
			out.Action, out.Detail = domain.DNSActionFailed, err.Error()
			return out
		}
		out.Action = domain.DNSActionMerged
		out.Detail = "adicionamos nosso include ao seu SPF, sem remover o que já existia"
		if n := domain.CountSPFLookups(merged); n > domain.SPFLookupLimit {
			// A warning, never a refusal: the count cannot see inside a
			// nested include, so refusing on it would block setups that are
			// actually fine.
			out.Detail += fmt.Sprintf(" — atenção: seu SPF passou de %d consultas DNS (limite %d), o que pode invalidá-lo",
				n, domain.SPFLookupLimit)
		}
		return out
	}

	if err := p.createRecord(ctx, token, zoneID, rec.Type, rec.Host, rec.Value); err != nil {
		out.Action, out.Detail = domain.DNSActionFailed, err.Error()
		return out
	}
	out.Action = domain.DNSActionCreated
	return out
}

// spfIncludeOf pulls our include target back out of the record we generated,
// so the merge path does not need the sending config threaded into it.
func spfIncludeOf(value string) string {
	for _, f := range strings.Fields(value) {
		if strings.HasPrefix(strings.ToLower(f), "include:") {
			return f[len("include:"):]
		}
	}
	return ""
}

// ── Cloudflare wire types ─────────────────────────────────────────────────

type cfDNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type cfEnvelope struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result json.RawMessage `json:"result"`
}

// findZone walks up the domain's labels.
//
// A sending domain is often a subdomain (mail.clinica.com.br) while the zone
// is the registrable domain, so an exact-name lookup alone finds nothing and
// tells the customer their domain is not in the account — which is wrong and
// sends them hunting through Cloudflare for a zone that is right there.
func (p *CloudflareProvisioner) findZone(ctx context.Context, token, sendingDomain string) (id, name string, err error) {
	for _, candidate := range domain.ZoneCandidates(sendingDomain) {
		env, err := p.do(ctx, token, http.MethodGet, "/zones?name="+candidate, nil)
		if err != nil {
			return "", "", err
		}
		var zones []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if json.Unmarshal(env.Result, &zones) == nil && len(zones) > 0 {
			return zones[0].ID, zones[0].Name, nil
		}
	}
	return "", "", domain.ErrDNSZoneNotFound
}

func (p *CloudflareProvisioner) findRecords(ctx context.Context, token, zoneID, recType, name string) ([]cfDNSRecord, error) {
	env, err := p.do(ctx, token, http.MethodGet,
		fmt.Sprintf("/zones/%s/dns_records?type=%s&name=%s", zoneID, recType, name), nil)
	if err != nil {
		return nil, err
	}
	var out []cfDNSRecord
	if err := json.Unmarshal(env.Result, &out); err != nil {
		return nil, nil
	}
	return out, nil
}

func (p *CloudflareProvisioner) createRecord(ctx context.Context, token, zoneID, recType, name, content string) error {
	body := map[string]any{"type": recType, "name": name, "content": content, "ttl": 3600}
	// TTL 1h, not Cloudflare's "auto": during setup the customer WILL fix a
	// typo, and a day-long TTL means the fix does not take effect until
	// tomorrow — by which time they have given up.
	_, err := p.do(ctx, token, http.MethodPost, "/zones/"+zoneID+"/dns_records", body)
	return err
}

func (p *CloudflareProvisioner) updateRecord(ctx context.Context, token, zoneID, recordID, recType, name, content string) error {
	body := map[string]any{"type": recType, "name": name, "content": content, "ttl": 3600}
	_, err := p.do(ctx, token, http.MethodPut,
		fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), body)
	return err
}

func (p *CloudflareProvisioner) do(ctx context.Context, token, method, path string, body any) (cfEnvelope, error) {
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return cfEnvelope{}, err
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return cfEnvelope{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 401 and 403 are one error on purpose: "wrong token" and "token without
	// the permission" are fixed the same way, by minting a new one with the
	// documented scope, and distinguishing them would only invite guessing.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return cfEnvelope{}, domain.ErrDNSCredentialInvalid
	}

	var env cfEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare: resposta inesperada (%d)", resp.StatusCode)
	}
	if !env.Success {
		if len(env.Errors) > 0 {
			return env, fmt.Errorf("cloudflare: %s", env.Errors[0].Message)
		}
		return env, fmt.Errorf("cloudflare: falha inesperada (%d)", resp.StatusCode)
	}
	return env, nil
}

var _ domain.DNSProvisioner = (*CloudflareProvisioner)(nil)
