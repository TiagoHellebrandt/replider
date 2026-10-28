package domain

import (
	"context"
	"strings"
	"time"
)

// Sales strategies (§4, E2/E3).
//
// A strategy is the written-down version of "como a gente vende": who we go
// after, through which channels in which order, in what tone, with which
// arguments, and what to say when they push back.
//
// It exists as an ENTITY rather than as prose in a document for one reason:
// attribution. Without a strategy id on every approach and every lead,
// nothing downstream measures anything — an A/B is comparing two unnamed
// things, and "essa abordagem funciona" is a feeling.
type Strategy struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	Name string `json:"name"`
	// Version is bumped when the CONTENT changes. "Essa estratégia converte
	// 14%" is meaningless if the arguments were rewritten twice since the
	// number was earned — the same reason email templates version.
	Version int `json:"version"`

	Status StrategyStatus `json:"status"`

	// ICP is who this is for, in the seller's own words.
	ICP string `json:"icp"`
	// ChannelMix is the plan: which channels in which order. Free text
	// rather than a structured cadence, because a strategy describes an
	// intent and a Sequence implements it — conflating them would make
	// every wording change a migration.
	ChannelMix string `json:"channel_mix,omitempty"`
	// SequenceID links the cadence that implements this strategy, when one
	// exists. Optional: a strategy is worth writing down before anyone has
	// built the automation for it.
	SequenceID string `json:"sequence_id,omitempty"`

	Tone string `json:"tone,omitempty"`
	// Arguments are what to lead with. Ordered by the author, because the
	// order IS the argument.
	Arguments []string `json:"arguments,omitempty"`
	// Objections map what they say to what to answer. The part of a
	// strategy that a new salesperson actually reads.
	Objections []StrategyObjection `json:"objections,omitempty"`
	// Triggers are the moments this strategy applies: "publicou edital",
	// "trocou de diretor clínico".
	Triggers []string `json:"triggers,omitempty"`

	// AuthorUserID is recognition, and recognition is what makes a
	// salesperson contribute one. A strategy with no visible author is a
	// company document; one with a name on it is somebody's method.
	AuthorUserID string `json:"author_user_id,omitempty"`
	AuthorName   string `json:"author_name,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StrategyObjection is one push-back and its answer.
type StrategyObjection struct {
	Objection string `json:"objection"`
	Response  string `json:"response"`
}

// StrategyStatus is where a strategy sits in its life.
type StrategyStatus string

const (
	// StrategyDraft is being written. Not offered for use: half a strategy
	// applied to a real prospect is worse than none.
	StrategyDraft StrategyStatus = "draft"
	// StrategyTesting is in an A/B. Usable, and its numbers count.
	StrategyTesting StrategyStatus = "testing"
	// StrategyPromoted won. The default for new leads that match it.
	StrategyPromoted StrategyStatus = "promoted"
	// StrategyRetired lost, or stopped applying. Kept rather than deleted:
	// every lead it ever touched still points at it, and the numbers it
	// earned are the evidence for retiring it.
	StrategyRetired StrategyStatus = "retired"
)

// Usable reports a strategy that may be attached to a new approach.
func (s *Strategy) Usable() bool {
	return s != nil && (s.Status == StrategyTesting || s.Status == StrategyPromoted)
}

// ValidStrategyStatus guards the value before it reaches SQL.
func ValidStrategyStatus(v string) bool {
	switch StrategyStatus(strings.ToLower(strings.TrimSpace(v))) {
	case StrategyDraft, StrategyTesting, StrategyPromoted, StrategyRetired:
		return true
	}
	return false
}

// ContentChanged reports whether a save should bump the version.
//
// The name, the status and the linked sequence are NOT content: renaming a
// strategy or promoting it must not orphan the numbers it earned, exactly as
// renaming an email template does not.
func (s *Strategy) ContentChanged(other *Strategy) bool {
	if s == nil || other == nil {
		return true
	}
	if s.ICP != other.ICP || s.Tone != other.Tone || s.ChannelMix != other.ChannelMix {
		return true
	}
	if !equalStrings(s.Arguments, other.Arguments) ||
		!equalStrings(s.Triggers, other.Triggers) {
		return true
	}
	if len(s.Objections) != len(other.Objections) {
		return true
	}
	for i := range s.Objections {
		if s.Objections[i] != other.Objections[i] {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// StrategyAttribution is the stamp every approach and lead carries.
//
// Both the id AND the version: an approach sent under version 2 must not be
// counted toward version 3's numbers, which is the whole reason versions
// exist.
type StrategyAttribution struct {
	StrategyID      string `json:"strategy_id,omitempty"`
	StrategyVersion int    `json:"strategy_version,omitempty"`
}

// Attributed reports whether anything is stamped.
func (a StrategyAttribution) Attributed() bool {
	return strings.TrimSpace(a.StrategyID) != ""
}

// StrategyRepository persists strategies.
type StrategyRepository interface {
	List(ctx context.Context, accountID string) ([]*Strategy, error)
	Get(ctx context.Context, id, accountID string) (*Strategy, error)
	// Upsert bumps the version only when the CONTENT changed — see
	// ContentChanged.
	Upsert(ctx context.Context, s *Strategy) error
	SetStatus(ctx context.Context, id, accountID string, status StrategyStatus) error
	// Stats tallies what each strategy VERSION produced.
	Stats(ctx context.Context, accountID string) ([]StrategyStat, error)
}

// StrategyStat is one version's results.
type StrategyStat struct {
	StrategyID string `json:"strategy_id"`
	Version    int    `json:"version"`
	// Approaches counts leads that were written to under this version.
	Approaches int `json:"approaches"`
	Replied    int `json:"replied"`
	Won        int `json:"won"`
}

// ReplyRate and WinRate are nil when nothing was sent: zero out of zero is
// unknown, not zero, and rendering 0% would condemn a strategy nobody used.
func (s StrategyStat) ReplyRate() *float64 {
	if s.Approaches == 0 {
		return nil
	}
	r := float64(s.Replied) / float64(s.Approaches)
	return &r
}

func (s StrategyStat) WinRate() *float64 {
	if s.Approaches == 0 {
		return nil
	}
	r := float64(s.Won) / float64(s.Approaches)
	return &r
}
