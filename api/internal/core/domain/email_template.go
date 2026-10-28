package domain

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Email templates (§6.8.1).
//
// A library of short texts with variables, not a drag-and-drop builder. Cold
// email that works is 25 to 100 words with minimal HTML, because heavy markup
// is both worse to read and worse for deliverability — the editor everyone
// asks for produces exactly the emails that land in spam.

// EmailTemplate is one reusable message.
type EmailTemplate struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`

	// Scope follows the brand-kit cascade: global, one funnel, or one
	// salesperson. Both nil means the whole company.
	FunnelID *string `json:"funnel_id,omitempty"`
	UserID   *string `json:"user_id,omitempty"`

	// Version is bumped on every save, because "this template gets 12% reply"
	// is meaningless if the text changed twice since the number was earned.
	Version  int  `json:"version"`
	IsActive bool `json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EmailTemplateStats is one version's performance.
type EmailTemplateStats struct {
	TemplateID string `json:"template_id"`
	Version    int    `json:"version"`

	Sent int `json:"sent"`
	// Opened is a WEAK signal and labelled as such wherever it is shown:
	// Apple Mail Privacy Protection pre-fetches images, inflating opens by
	// roughly half. Kept because the trend still means something; never
	// decisive on its own.
	Opened  int `json:"opened"`
	Clicked int `json:"clicked"`
	Replied int `json:"replied"`
	Bounced int `json:"bounced"`
}

// ReplyRate is the number that actually matters. Nil when nothing was sent —
// zero replies out of zero sends is not a zero rate, it is unknown, and
// showing 0% would condemn a template nobody has used.
func (s EmailTemplateStats) ReplyRate() *float64 {
	if s.Sent == 0 {
		return nil
	}
	r := float64(s.Replied) / float64(s.Sent)
	return &r
}

// BounceRate answers "is this list dirty". Same nil semantics.
func (s EmailTemplateStats) BounceRate() *float64 {
	if s.Sent == 0 {
		return nil
	}
	r := float64(s.Bounced) / float64(s.Sent)
	return &r
}

// DirtyListThreshold is where a bounce rate stops being noise and starts
// being a deliverability problem. Five percent: mailbox providers begin
// throttling a sender well before ten, and by then the damage takes weeks to
// undo.
const DirtyListThreshold = 0.05

// NeedsListWarning reports a bounce rate high enough to hurt the sender's
// reputation. Requires a real sample — one bounce out of two sends is 50% and
// means nothing.
func (s EmailTemplateStats) NeedsListWarning() bool {
	if s.Sent < 20 {
		return false
	}
	r := s.BounceRate()
	return r != nil && *r >= DirtyListThreshold
}

var templateVarRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// TemplateVariables lists the placeholders a template uses, deduplicated and
// sorted so the editor can show a stable list.
func TemplateVariables(subject, body string) []string {
	seen := map[string]struct{}{}
	for _, text := range []string{subject, body} {
		for _, m := range templateVarRe.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RenderTemplate substitutes the variables it has values for.
//
// An UNKNOWN variable is left standing rather than blanked, and that is
// deliberate: "Olá {{nome}}" reaching a customer is embarrassing, but "Olá ,"
// reaching them is embarrassing AND invisible to whoever approved it. A
// placeholder that survives is caught by the approval queue's placeholder
// check; an empty gap is not.
func RenderTemplate(text string, values map[string]string) string {
	return templateVarRe.ReplaceAllStringFunc(text, func(match string) string {
		name := templateVarRe.FindStringSubmatch(match)[1]
		if v, ok := values[name]; ok && strings.TrimSpace(v) != "" {
			return v
		}
		return match
	})
}

// MissingVariables reports which placeholders would survive rendering.
//
// Called before a template is used so the caller can refuse rather than send
// a message with "{{empresa}}" in it.
func MissingVariables(subject, body string, values map[string]string) []string {
	var missing []string
	for _, name := range TemplateVariables(subject, body) {
		if v, ok := values[name]; !ok || strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// ColdEmailMaxWords is where a cold email stops being read.
//
// A hundred words is already generous: the messages that get replies are
// closer to fifty. This is a warning rather than a limit, because the person
// writing knows their market and the number is a rule of thumb, not a law.
const ColdEmailMaxWords = 100

// WordCount counts words in the body, for that warning.
func WordCount(body string) int {
	return len(strings.Fields(body))
}

// EmailTemplateRepository persists templates and their per-version stats.
type EmailTemplateRepository interface {
	List(ctx context.Context, accountID string) ([]*EmailTemplate, error)
	Get(ctx context.Context, id, accountID string) (*EmailTemplate, error)
	// Upsert creates or updates, bumping the version when the TEXT changed —
	// renaming a template must not orphan the numbers it earned.
	Upsert(ctx context.Context, t *EmailTemplate) error
	Delete(ctx context.Context, id, accountID string) error
	Stats(ctx context.Context, templateID string) ([]EmailTemplateStats, error)

	// The counters. One method each rather than a single Record(kind) so the
	// column name can never come from a caller — the tracking endpoints are
	// unauthenticated by nature, and "increment whatever column this token
	// names" is not a thing that should be expressible.
	RecordSend(ctx context.Context, templateID string, version int) error
	RecordOpen(ctx context.Context, templateID string, version int) error
	RecordClick(ctx context.Context, templateID string, version int) error
	RecordReply(ctx context.Context, templateID string, version int) error
	RecordBounce(ctx context.Context, templateID string, version int) error
}
