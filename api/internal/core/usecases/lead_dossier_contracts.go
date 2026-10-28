package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Who this hospital buys from today — from the public record, or not at all.
//
// The park says HOW MANY machines an establishment declares and never whose
// they are: CNES publishes no brand. So the only machine-readable answer to
// "who is the incumbent here" is the procurement record, where a signed
// contract names the supplier, the object, the value and the date it runs out.
//
// It enters as a REGISTRY fact — checkable, so the draft may state it plainly
// — and it carries the PNCP link, because a claim about somebody's supplier is
// exactly the kind a hospital will check.
//
// When PNCP has nothing, nothing is added. An account with no incumbent found
// gets a proposal that argues its own case without comparing, which is the
// honest shape of "we do not know".

const (
	// Contracts shown. The newest few say who is there now; a list of twenty
	// is a research report nobody reads before a call.
	contractsInDossier = 3
	// How long a buyer's contracts stay fresh. PNCP publishes continuously,
	// but a hospital's supplier does not change weekly, and a lead drafted
	// twice in a week must not cost two round trips.
	contractScanTTL = 14 * 24 * time.Hour
	// A contract this old says little about who serves them now.
	contractRelevance = 3 * 365 * 24 * time.Hour
)

// contractFetcher is the slice of the PNCP adapter this needs.
type contractFetcher interface {
	ContractsForBuyer(ctx context.Context, cnpj string) ([]domain.PublicContract, error)
}

// SetContracts wires the incumbent-supplier lookup. Optional: without it the
// dossier simply carries no supplier, and the draft compares with nobody.
func (b *LeadDossierBuilder) SetContracts(
	repo domain.PublicContractRepository, fetcher contractFetcher,
) *LeadDossierBuilder {
	b.contracts = repo
	b.contractFetch = fetcher
	return b
}

// addContracts adds the incumbent, when the record names one.
func (b *LeadDossierBuilder) addContracts(
	ctx context.Context, d *domain.LeadDossier, lead *domain.Lead, contact *domain.Contact,
) {
	// Both seams: matchEstablishment reaches into the park reader, so contracts
	// wired without it would panic on the first lead that has a phone.
	if b.contracts == nil || b.cnesPark == nil {
		return
	}
	est := b.matchEstablishment(lead, contact)
	if est == nil || est.CNPJ == nil {
		return
	}
	cnpj := onlyDigitsOf(*est.CNPJ)
	if len(cnpj) != 14 {
		return
	}

	// Fetch only when the cache is cold. Everything here is best-effort: a
	// PNCP outage must cost the supplier line, never the draft.
	if b.contractFetch != nil {
		if last, err := b.contracts.LastScan(ctx, cnpj); err == nil &&
			time.Since(last) > contractScanTTL {
			if found, err := b.contractFetch.ContractsForBuyer(ctx, cnpj); err != nil {
				slog.WarnContext(ctx, "dossier: pncp contracts read failed",
					"lead_id", lead.ID, "cnpj", cnpj, "error", err)
			} else {
				if err := b.contracts.Save(ctx, found); err != nil {
					slog.WarnContext(ctx, "dossier: contracts save failed",
						"lead_id", lead.ID, "error", err)
				}
				_ = b.contracts.MarkScanned(ctx, cnpj, len(found))
			}
		}
	}

	all, err := b.contracts.ListByBuyer(ctx, cnpj, contractsInDossier*4)
	if err != nil {
		slog.WarnContext(ctx, "dossier: contracts read failed", "lead_id", lead.ID, "error", err)
		return
	}
	recent := make([]domain.PublicContract, 0, contractsInDossier)
	for _, c := range all {
		if strings.TrimSpace(c.SupplierName) == "" {
			continue
		}
		if c.SignedAt != nil && time.Since(*c.SignedAt) > contractRelevance {
			continue
		}
		recent = append(recent, c)
		if len(recent) >= contractsInDossier {
			break
		}
	}
	if len(recent) == 0 {
		return
	}

	parts := make([]string, 0, len(recent))
	for _, c := range recent {
		line := c.SupplierName
		if obj := trimTo(c.Object, 80); obj != "" {
			line += " — " + obj
		}
		// The expiry is the part a seller acts on: it is the window.
		if c.ExpiresAt != nil {
			line += fmt.Sprintf(" (vigência até %s)", c.ExpiresAt.Format("02/01/2006"))
		}
		parts = append(parts, line)
	}

	d.Add(domain.DossierFact{
		Kind:   domain.DossierRegistry,
		Label:  "Fornecedores contratados (registro público)",
		Value:  strings.Join(parts, "; "),
		Source: "PNCP — " + recent[0].URL(),
	})
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

func onlyDigitsOf(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}
