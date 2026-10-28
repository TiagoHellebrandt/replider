package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Buying signals (§2.1).
//
// A hospital that publishes a tender has told the world, in writing, that it
// intends to buy a specific thing, by a specific date, with a budget. That is
// a stronger intent signal than anything an enrichment vendor sells — and it
// is public, free, and nobody in this market uses it.
//
// The design consequence is the expiry window. A tender whose deadline has
// passed is not a weak lead, it is not a lead: approaching someone about a
// bid they already closed is worse than not writing at all, because it proves
// nobody read it. So a signal has an end date and stops being offered.

// SignalSource says where a signal came from, so its reliability can be
// judged and its origin cited in the approach.
type SignalSource string

const (
	// SignalPNCP is the Portal Nacional de Contratações Públicas — the
	// federal register every Brazilian public tender must be published in.
	SignalPNCP SignalSource = "pncp"
	// SignalNewCompany is a freshly opened CNPJ or a CNAE change.
	SignalNewCompany SignalSource = "new_company"
	// SignalJobPost is a public job opening, which says what a company is
	// building before they announce it.
	SignalJobPost SignalSource = "job_post"
)

// IsPolledSource reports whether we go and FETCH this source ourselves,
// against a standing watch the account configured.
//
// The distinction decides who owns a stored signal. A polled source only ever
// produced rows because a watch asked for them, so when that watch is gone the
// rows it collected are nobody's — migration 000234 deleted one unscoped watch
// and left eighteen national tenders about PVC reducers sitting on a
// hospital-equipment radar.
//
// A PUSHED source is the opposite: an integration the account pays for sent
// the row in, and the sending IS the claim. Applying the orphan rule to those
// would hide every signal a vendor feed ever delivered, since none of them has
// a watch and none of them ever will.
func IsPolledSource(s SignalSource) bool { return s == SignalPNCP }

// Signal is one public event worth writing to somebody about.
type Signal struct {
	ID        string       `json:"id"`
	AccountID string       `json:"-"`
	Source    SignalSource `json:"source"`

	// ExternalID is the source's own identifier, and the dedup key. PNCP
	// republishes the same tender as it is amended, and a salesperson who
	// gets the same "novo edital" three times stops reading them.
	ExternalID string `json:"external_id"`

	// Organization is who published it, as a human recognises them.
	Organization string `json:"organization"`
	// CNPJ, when the source gives one — the join key to everything else we
	// know about this company.
	CNPJ string `json:"cnpj,omitempty"`

	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// URL is the source page, so the salesperson can read the actual notice
	// rather than trusting our summary of it.
	URL string `json:"url,omitempty"`

	// ValueCents is the estimated amount, when published. Zero means "not
	// disclosed", which is common and is NOT the same as zero reais.
	ValueCents int64  `json:"value_cents,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`

	PublishedAt time.Time `json:"published_at"`
	// ClosesAt is the deadline. After it the signal is dead — see Expired.
	ClosesAt *time.Time `json:"closes_at,omitempty"`

	// LeadID is set once somebody acts on this signal, so the same tender
	// is not worked twice by two people.
	LeadID string `json:"lead_id,omitempty"`

	// DismissedAt records a human saying "not for us". Kept rather than
	// deleted so the same tender does not come back tomorrow.
	DismissedAt *time.Time `json:"dismissed_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// Expired reports a signal whose deadline has passed.
//
// A tender with no published deadline never expires here: guessing one and
// hiding a live opportunity is a worse error than showing an old one, which
// the salesperson can see the date of.
func (s *Signal) Expired(now time.Time) bool {
	return s.ClosesAt != nil && now.After(*s.ClosesAt)
}

// Actionable reports a signal still worth showing.
func (s *Signal) Actionable(now time.Time) bool {
	return s != nil && s.DismissedAt == nil && s.LeadID == "" && !s.Expired(now)
}

// DaysLeft is what the card shows. Nil when there is no deadline.
//
// Urgency is the whole value of a tender signal: three days left is a reason
// to write today, and thirty is a reason to prepare properly.
func (s *Signal) DaysLeft(now time.Time) *int {
	if s.ClosesAt == nil {
		return nil
	}
	d := int(s.ClosesAt.Sub(now).Hours() / 24)
	return &d
}

// SignalWatch is one account's standing interest: what to look for.
type SignalWatch struct {
	ID        string       `json:"id"`
	AccountID string       `json:"-"`
	Source    SignalSource `json:"source"`

	// Keywords match the tender's title and description. Any match counts —
	// a supplier of monitors sells to "monitor multiparâmetro" and to
	// "monitorização", and demanding all terms would find nothing.
	Keywords []string `json:"keywords"`
	// CNAEs narrow by the buyer's economic activity, when the source
	// publishes it.
	CNAEs []string `json:"cnaes,omitempty"`
	// States narrows by region, because a supplier who cannot deliver to
	// Acre does not want to read about Acre.
	States []string `json:"states,omitempty"`

	// DerivedFromCatalog marks keywords the seeder derived from the
	// account's own products, as opposed to typed by a person. The signals
	// screen phrases itself as "matches what you sell" — that claim is only
	// honest for derived watches, so the UI shows the origin.
	DerivedFromCatalog bool `json:"derived_from_catalog"`

	// MinValueCents filters out the noise floor. A tender for R$ 800 of
	// office supplies is not worth a salesperson's morning.
	MinValueCents int64 `json:"min_value_cents,omitempty"`

	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NormalizeAndValidate trims a watch into the only shape that can ever fire,
// and refuses the rest at the moment somebody can still fix it.
//
// It is the single rule for all three writers — the HTTP route, the MCP tool
// and the seeder — because the two conditions below are cheap to state and
// expensive to discover in production.
//
// KEYWORDS: a watch with none matches nothing (see Matches), so saving one is
// saving a watch that silently never fires.
//
// STATES: a watch with none used to mean "anywhere", and anywhere is not a
// territory — it is the whole country. Worse, the scope of a poll is the
// UNION of the watches in it (watchUFs), so a single unscoped row dragged
// every other account's fetch national with it. One such row existed: a
// dogfood watch for the word "luva", no UF, created 2026-08-01. Every tick
// after that walked 3 modalities × up to 40 pages of the national feed — some
// 120 requests against a free, unauthenticated government API — instead of
// the one page that MT actually publishes in 48 hours. PNCP answered exactly
// as anyone would expect: 429, then 502, 503 and timeouts, and the radar sat
// empty while the source was blamed.
//
// So an unscoped watch is not a watch. Say where, or do not ask.
func (w *SignalWatch) NormalizeAndValidate() error {
	keywords := make([]string, 0, len(w.Keywords))
	for _, kw := range w.Keywords {
		if kw = strings.TrimSpace(kw); kw != "" {
			keywords = append(keywords, kw)
		}
	}
	if len(keywords) == 0 {
		return &AppError{
			Code:    "VALIDATION_ERROR",
			Message: "informe ao menos uma palavra-chave",
		}
	}
	w.Keywords = keywords

	states := NormalizeUFs(w.States)
	if len(states) == 0 {
		return &AppError{
			Code:    "VALIDATION_ERROR",
			Message: "informe ao menos um estado (UF) para o monitoramento",
		}
	}
	w.States = states
	return nil
}

// Matches reports whether a signal is worth showing to this watch.
//
// Runs in Go rather than SQL because the sources are polled once and fanned
// out to every interested account — a query per watch would hit the external
// API once per account, which is both slow and a good way to get rate
// limited.
func (w *SignalWatch) Matches(s *Signal) bool {
	if w == nil || s == nil || !w.IsActive {
		return false
	}
	if w.MinValueCents > 0 && s.ValueCents > 0 && s.ValueCents < w.MinValueCents {
		return false
	}
	// A signal with no published value passes the floor. "Not disclosed" is
	// common in public tenders and is not the same as cheap.
	if len(w.States) > 0 && !containsFold(w.States, s.State) {
		return false
	}
	// A watch with no keywords matches NOTHING, not everything. "Every
	// tender in the state" is thousands of rows a month and, with the
	// auto-offer behind it, a draft budget burnt on office supplies. A watch
	// that wants to say "everything" has to say it with words.
	// Lowercase AFTER folding: FoldAccents upper-cases as it strips, so
	// folding last would undo the lowering and every case test below
	// (including the plural suffix) would compare against the wrong case.
	haystack := strings.ToLower(FoldAccents(
		s.Title + " " + s.Description + " " + s.Organization))
	// A DERIVED watch must also see the tender in a health context.
	//
	// The keywords come from the account's own catalogue, and half of them
	// are ordinary Portuguese words that mean something else outside a
	// hospital. Measured over 25 stored tenders: "ventilador" matched
	// "televisões e ventiladores", "bomba" matched a fuel-supply pump,
	// "termômetro" matched room thermometers for air conditioning, and
	// "cilindro" matched a printer drum cartridge. One row in twenty-five
	// was actually for this account — and the loudest two, by value and by
	// deadline, were a locksmith contract and inflatable playground toys.
	//
	// The damage is not wasted time. R$757k of bouncy castles at the top of
	// a salesperson's morning teaches them not to trust the feed, including
	// the half of it that is good.
	//
	// A HAND-WRITTEN watch is exempt: somebody typed those words on purpose
	// and the product has no business second-guessing them.
	if w.DerivedFromCatalog && !looksLikeHealthTender(s) {
		return false
	}

	for _, kw := range w.Keywords {
		k := strings.ToLower(FoldAccents(strings.TrimSpace(kw)))
		if k != "" && containsWord(haystack, k) {
			return true
		}
	}
	return false
}

// containsWord is a WHOLE-WORD substring test over already-folded text,
// tolerant of the Portuguese plural.
//
// Plain strings.Contains was matching inside longer words, and against real
// tenders that is most of the noise: "monitor" hit "serviços de
// monitoramento", "leito" hit the middle of an unrelated word. Measured over
// 417 published MT tenders (30 days to 2026-09-08) with one account's derived
// watch, whole-word matching cut 57 hits to 3 while keeping the only genuine
// one ("aquisição de gás oxigênio medicinal"). Every hit it removed was a
// tender the account would have spent a model call drafting an offer for.
//
// The plural tolerance is not a nicety: a catalogue names a product
// ("monitor") and a tender buys several ("aquisição de monitores"), so a
// strict boundary would have thrown away the ordinary case. Only "s" and
// "es" are allowed to follow — enough for monitores/capuzes/luvas, and
// still nothing like "monitoramento".
//
// Accents are folded on BOTH sides by the caller, so a watch word written
// "oxigênio" still finds a tender that wrote "oxigenio".
// ContainsWord is containsWord for callers outside this package: whole-word,
// plural-tolerant, on text already folded and lowercased.
//
// Exported for the offer matcher's catalogue ranking, which asks the same
// question this file asks of a tender — "does this text actually say this
// word?" — and must give the same answer. A second, looser matcher there
// would rank a product in because the dossier merely contains its letters.
func ContainsWord(haystack, word string) bool { return containsWord(haystack, word) }

func containsWord(haystack, word string) bool {
	if word == "" {
		return false
	}
	// A watch word that is itself plural must find the singular too.
	stems := []string{word}
	if s := strings.TrimSuffix(word, "es"); len(s) >= 4 && s != word {
		stems = append(stems, s)
	} else if s := strings.TrimSuffix(word, "s"); len(s) >= 4 && s != word {
		stems = append(stems, s)
	}
	for _, stem := range stems {
		if containsStem(haystack, stem) {
			return true
		}
	}
	return false
}

func containsStem(haystack, stem string) bool {
	for i := 0; i < len(haystack); {
		j := strings.Index(haystack[i:], stem)
		if j < 0 {
			return false
		}
		start := i + j
		if !isWordByte(haystack, start-1) && endsWord(haystack, start+len(stem)) {
			return true
		}
		// Advance one byte, not one word: an occurrence can overlap the
		// one that just failed the boundary test.
		i = start + 1
	}
	return false
}

// endsWord reports whether position i ends the word, allowing a Portuguese
// plural suffix to follow.
func endsWord(s string, i int) bool {
	if !isWordByte(s, i) {
		return true
	}
	if strings.HasPrefix(s[i:], "es") && !isWordByte(s, i+2) {
		return true
	}
	return s[i] == 's' && !isWordByte(s, i+1)
}

// isWordByte reports whether the byte at i continues a word. Out of range is
// a boundary. Folded text is ASCII letters and digits, so a byte test is
// enough and no rune decoding is needed.
func isWordByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func containsFold(list []string, v string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

// SignalRepository persists signals and the watches that select them.
type SignalRepository interface {
	// Upsert is keyed on (account_id, source, external_id). A source that
	// republishes an amended tender must update it, not duplicate it — a
	// salesperson who sees the same notice three times stops reading them.
	Upsert(ctx context.Context, s *Signal) error
	// List is the /signals feed: open, not dismissed, NOT yet worked.
	List(ctx context.Context, accountID string, limit, offset int) ([]*Signal, error)
	// ListForRadar is the same feed INCLUDING signals already linked to a
	// lead. The Radar must keep showing a tender after the auto-offer
	// turned it into a draft — that is the moment it became work, not the
	// moment it stopped mattering.
	ListForRadar(ctx context.Context, accountID string, limit, offset int) ([]*Signal, error)
	// CountByAccount says whether an account's watches have EVER matched
	// anything (dismissed, worked and expired rows included). The poller
	// uses zero as "fresh watch: backfill".
	CountByAccount(ctx context.Context, accountID string) (int, error)
	Get(ctx context.Context, id, accountID string) (*Signal, error)
	Dismiss(ctx context.Context, id, accountID string, at time.Time) error
	// LinkLead marks a signal as worked, so two people do not chase the same
	// tender.
	LinkLead(ctx context.Context, id, accountID, leadID string) error

	ListWatches(ctx context.Context, accountID string) ([]*SignalWatch, error)
	// ListActiveWatches spans every account: the poller fetches once and
	// fans out, rather than hitting the external API per tenant.
	ListActiveWatches(ctx context.Context) ([]*SignalWatch, error)
	UpsertWatch(ctx context.Context, w *SignalWatch) error
	DeleteWatch(ctx context.Context, id, accountID string) error
}

// SignalSourceHealth is whether a source answered the last time it was asked.
//
// It exists because the radar could not tell an outage from a quiet week. The
// screen renders what is stored, so when PNCP refused every request from
// 06:20 on 2026-09-10 it went on showing eight-day-old tenders as if they
// were today's — the same failure this codebase already found in the nightly
// briefing, the ranking and the mini-funnel: a view filtered to "what is
// worth showing" erases its own failure and presents the last good state as
// the current one. A failure state needs a column and a predicate of its own.
type SignalSourceHealth struct {
	Source        SignalSource `json:"source"`
	LastAttemptAt time.Time    `json:"last_attempt_at"`
	// LastSuccessAt is nil when the source has never answered.
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	// ConsecutiveFailures separates one bad tick from "down since
	// yesterday" without keeping a history nobody reads.
	ConsecutiveFailures int `json:"consecutive_failures"`
	// LastError is the source's own words, for the operator. Deliberately
	// NOT in any customer-facing payload: a seller needs to know the feed is
	// stale, not that page 20 of modality 8 returned 429.
	LastError string `json:"-"`
}

// Unreachable reports a source that did not answer its last attempt.
//
// One failed tick counts. The poll runs every six hours, so waiting for a
// second one would leave the screen lying for half a day — and the honest
// sentence ("we could not read the source at 12:13; what follows is from the
// last collection") is true after the first.
func (h *SignalSourceHealth) Unreachable() bool {
	return h != nil && h.ConsecutiveFailures > 0
}

// SignalSourceHealthRepository records how each poll went.
//
// A NARROW port, wired by optional setter, deliberately not part of
// SignalRepository: that interface has a dozen mocks, and none of them cares
// whether a portal was up.
type SignalSourceHealthRepository interface {
	// RecordAttempt stamps one pass. A nil err is a success and resets the
	// failure count; anything else increments it and keeps the message.
	RecordAttempt(ctx context.Context, source SignalSource, at time.Time, err error) error
	// ListHealth returns every source ever polled.
	ListHealth(ctx context.Context) ([]*SignalSourceHealth, error)
}

// SignalWatchCandidate is one account the derived-watch seeder should
// equip: it sells things, said where, and either never configured a watch
// or still carries the untouched one the seeder wrote.
type SignalWatchCandidate struct {
	AccountID string
	UF        string
	// DerivedWatchID is the existing seeder-written watch to REFRESH in
	// place (empty = create a new one). Refreshing rather than inserting
	// keeps the row's identity, so the customer's screen does not gain a
	// second watch every time the catalogue changes.
	DerivedWatchID string
}

// ErrSignalSourcePartial means the source answered, but not all of it was
// read: some walk, modality or page refused and the rest came through.
//
// A fetcher returns it ALONGSIDE what it managed to collect — the caller
// keeps the rows and records that the pass was incomplete. It exists because
// "we read PNCP" and "we read part of PNCP" were the same answer, and the
// second one was being filed as the first: on 2026-09-17 two of three
// modality walks for MT returned nothing and the poll still stamped a
// success, so the radar showed one tender and claimed the feed was current.
// A feed missing two thirds of its source is not current, and the screen has
// to be allowed to say so.
var ErrSignalSourcePartial = errors.New("signal source read only in part")

// SignalFetcher pulls fresh events from one public source.
type SignalFetcher interface {
	Source() SignalSource
	// Fetch returns everything published in the window for the given UFs
	// (two-letter state codes). An empty ufs means the whole country.
	// Filtering per account happens afterwards, in SignalWatch.Matches.
	//
	// The UF is part of the request rather than a post-filter because the
	// source pages nationally with no ordering guarantee: a bounded walk of
	// the national feed reads a few percent of it, and a small state's
	// tenders are simply never on those pages.
	Fetch(ctx context.Context, since time.Time, ufs []string) ([]*Signal, error)
}
