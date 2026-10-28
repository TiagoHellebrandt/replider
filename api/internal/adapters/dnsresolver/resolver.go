// Package dnsresolver is the net-backed implementation of domain.DNSResolver
// (US-M006). Kept behind the port so the verifier's tests never touch the
// network — DNS tests that hit real resolvers are flaky by construction.
package dnsresolver

import (
	"context"
	"net"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

type Resolver struct {
	r *net.Resolver
}

var _ domain.DNSResolver = (*Resolver)(nil)

func New() *Resolver {
	// The system resolver is the right one here: it is the same recursive
	// resolver the mail providers will effectively consult, so what it sees
	// is what matters for delivery.
	return &Resolver{r: net.DefaultResolver}
}

func (d *Resolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return d.r.LookupTXT(ctx, strings.TrimSuffix(name, "."))
}

func (d *Resolver) LookupCNAME(ctx context.Context, name string) (string, error) {
	return d.r.LookupCNAME(ctx, strings.TrimSuffix(name, "."))
}

func (d *Resolver) LookupNS(ctx context.Context, name string) ([]string, error) {
	records, err := d.r.LookupNS(ctx, strings.TrimSuffix(name, "."))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(records))
	for _, ns := range records {
		out = append(out, ns.Host)
	}
	return out, nil
}
