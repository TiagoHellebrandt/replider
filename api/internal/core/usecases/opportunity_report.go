package usecases

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The Opportunity Report (tese-agente-autonomo §4): what a prospect is
// leaving on the table, from public data alone — no access to anything of
// theirs. CNES says who holds the equipment they sell; PNCP says who is
// buying it RIGHT NOW. The report is deliberately incomplete: it names a
// small sample and says how much it is not showing. The rest lives inside
// the product, which is the pitch.
//
// v1 is the owner's own selling tool (super-admin endpoint); the public
// lead-magnet version comes after the landing tells this story.

// OpportunityReportInput is what the seller knows about the prospect.
type OpportunityReportInput struct {
	// UF is the prospect's territory.
	UF string
	// Categories are the equipment words the prospect sells/services
	// ("ventilador", "tomógrafo"). Resolved against the CNES dictionary.
	Categories []string
	// Sample is how many named targets to reveal. Default 3 — enough to
	// prove the data is real, few enough to leave the rest wanted.
	Sample int
	// TenderDays is the PNCP lookback window. Default 30.
	TenderDays int
}

// OpportunityCategory is one equipment word's footprint in the territory.
type OpportunityCategory struct {
	Query          string                     `json:"query"`
	MatchedTypes   []domain.CNESEquipmentType `json:"matched_types"`
	Establishments int                        `json:"establishments"`
	Units          int                        `json:"units"`
}

// OpportunitySample is one named target — proof the data is real.
type OpportunitySample struct {
	CNES    string  `json:"cnes"`
	Name    string  `json:"name"`
	Address string  `json:"address,omitempty"`
	Phone   *string `json:"phone,omitempty"`
	Units   int     `json:"units"`
}

// OpportunityTender is one open (or fresh) public purchase in the window.
type OpportunityTender struct {
	Title        string     `json:"title"`
	Organization string     `json:"organization"`
	URL          string     `json:"url,omitempty"`
	ValueCents   int64      `json:"value_cents,omitempty"`
	ClosesAt     *time.Time `json:"closes_at,omitempty"`
}

// OpportunityReport is the assembled artifact.
type OpportunityReport struct {
	UF          string    `json:"uf"`
	GeneratedAt time.Time `json:"generated_at"`
	Competencia string    `json:"competencia"`

	Categories []OpportunityCategory `json:"categories"`
	// TotalEstablishments counts DISTINCT establishments holding any of the
	// asked equipment (not the per-category sum — one hospital with two
	// categories is one door to knock on).
	TotalEstablishments int `json:"total_establishments"`
	TotalUnits          int `json:"total_units"`

	Sample []OpportunitySample `json:"sample"`
	// HiddenTargets is the teaser: how many named targets the report is NOT
	// showing. The line the pitch is built on.
	HiddenTargets int `json:"hidden_targets"`

	Tenders          []OpportunityTender `json:"tenders"`
	TendersTotal     int                 `json:"tenders_total"`
	TendersSumCents  int64               `json:"tenders_sum_cents"`
	TenderWindowDays int                 `json:"tender_window_days"`
}

// opportunityParkReader is the slice of the CNES repo the report reads.
type opportunityParkReader interface {
	ResolveEquipmentCodes(nameQuery string) ([]domain.CNESEquipmentType, error)
	CountEquipment(uf string, equipCodes []string) (establishments, units int, err error)
	EstablishmentsWithEquipment(uf string, equipCodes []string, minCount, limit int) ([]domain.CNESEquipmentHolder, error)
	NewestCompetencia(uf string) (string, error)
}

// tenderFetcher is the live PNCP read, scoped to the report's UF.
type tenderFetcher interface {
	Fetch(ctx context.Context, since time.Time, ufs []string) ([]*domain.Signal, error)
}

type OpportunityReportUseCase struct {
	park    opportunityParkReader
	tenders tenderFetcher
	now     func() time.Time
}

func NewOpportunityReportUseCase(park opportunityParkReader, tenders tenderFetcher) *OpportunityReportUseCase {
	return &OpportunityReportUseCase{park: park, tenders: tenders, now: time.Now}
}

// WithClock injects a fake clock for tests.
func (uc *OpportunityReportUseCase) WithClock(now func() time.Time) *OpportunityReportUseCase {
	uc.now = now
	return uc
}

const (
	defaultReportSample     = 3
	maxReportSample         = 10
	defaultTenderWindowDays = 30
	maxTendersListed        = 10
)

// Build assembles the report. The park half is required (no data for the
// UF = an honest empty report); the tender half is best-effort — PNCP
// being down must not kill the meeting the report was generated for.
func (uc *OpportunityReportUseCase) Build(ctx context.Context, in OpportunityReportInput) (*OpportunityReport, error) {
	uf := strings.ToUpper(strings.TrimSpace(in.UF))
	if len(uf) != 2 {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "uf must be two letters"}
	}
	sample := in.Sample
	if sample <= 0 {
		sample = defaultReportSample
	}
	if sample > maxReportSample {
		sample = maxReportSample
	}
	window := in.TenderDays
	if window <= 0 {
		window = defaultTenderWindowDays
	}

	report := &OpportunityReport{
		UF:               uf,
		GeneratedAt:      uc.now(),
		TenderWindowDays: window,
	}
	if comp, err := uc.park.NewestCompetencia(uf); err == nil {
		report.Competencia = comp
	}

	// Per-category footprint + the union code set.
	var allCodes []string
	seenCode := map[string]bool{}
	var keywords []string
	for _, q := range in.Categories {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		keywords = append(keywords, strings.ToLower(q))
		types, err := uc.park.ResolveEquipmentCodes(q)
		if err != nil {
			return nil, err
		}
		codes := make([]string, 0, len(types))
		for _, t := range types {
			codes = append(codes, t.Code)
			if !seenCode[t.Code] {
				seenCode[t.Code] = true
				allCodes = append(allCodes, t.Code)
			}
		}
		estabs, units, err := uc.park.CountEquipment(uf, codes)
		if err != nil {
			return nil, err
		}
		report.Categories = append(report.Categories, OpportunityCategory{
			Query: q, MatchedTypes: types, Establishments: estabs, Units: units,
		})
	}

	if len(allCodes) > 0 {
		estabs, units, err := uc.park.CountEquipment(uf, allCodes)
		if err != nil {
			return nil, err
		}
		report.TotalEstablishments, report.TotalUnits = estabs, units

		holders, err := uc.park.EstablishmentsWithEquipment(uf, allCodes, 1, 50)
		if err != nil {
			return nil, err
		}
		for _, h := range holders {
			// Only a target the report can NAME proves anything: an
			// uncached CNES id would read as a made-up number.
			if h.Establishment == nil || strings.TrimSpace(h.Establishment.TradeName) == "" {
				continue
			}
			report.Sample = append(report.Sample, OpportunitySample{
				CNES:    strings.TrimSpace(h.CNES),
				Name:    h.Establishment.TradeName,
				Address: h.Establishment.Address,
				Phone:   h.Establishment.Phone,
				Units:   h.Total,
			})
			if len(report.Sample) == sample {
				break
			}
		}
		report.HiddenTargets = report.TotalEstablishments - len(report.Sample)
		if report.HiddenTargets < 0 {
			report.HiddenTargets = 0
		}
	}

	uc.addTenders(ctx, report, keywords, window)
	return report, nil
}

// addTenders reads PNCP live and keeps the UF's keyword-matching tenders.
// Best-effort: a PNCP outage leaves the section empty, never errors the
// report.
func (uc *OpportunityReportUseCase) addTenders(ctx context.Context, r *OpportunityReport, keywords []string, windowDays int) {
	if uc.tenders == nil || len(keywords) == 0 {
		return
	}
	found, err := uc.tenders.Fetch(ctx, uc.now().AddDate(0, 0, -windowDays), []string{r.UF})
	if err != nil {
		return
	}
	var matched []OpportunityTender
	for _, s := range found {
		if s == nil || !strings.EqualFold(strings.TrimSpace(s.State), r.UF) {
			continue
		}
		hay := strings.ToLower(s.Title + " " + s.Description)
		hit := false
		for _, kw := range keywords {
			if strings.Contains(hay, kw) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		matched = append(matched, OpportunityTender{
			Title: s.Title, Organization: s.Organization, URL: s.URL,
			ValueCents: s.ValueCents, ClosesAt: s.ClosesAt,
		})
		r.TendersSumCents += s.ValueCents
	}
	r.TendersTotal = len(matched)
	sort.Slice(matched, func(i, j int) bool { return matched[i].ValueCents > matched[j].ValueCents })
	if len(matched) > maxTendersListed {
		matched = matched[:maxTendersListed]
	}
	r.Tenders = matched
}
