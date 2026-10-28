package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// Reading the conversation with one lead.
//
// The authorisation is the LEAD's, not the inbox's, and that distinction is the
// whole reason this exists. The inbox is `RequireRole("admin", "attendant")`; a
// vendedor does not satisfy attendant, because opening every conversation in the
// account is not part of the job. But reading the replies to their OWN lead is,
// and until now they could send a proposal and never see the answer.
//
// So the gate is `GetForActor` — the same ownership and visibility rules that
// decide whether this person may open the lead at all. A vendedor sees their own
// leads and the team-visible ones; a colleague's private lead answers the same
// "not found" the pipeline already answers.
type LeadTimelineUseCase struct {
	leads    *LeadUseCase
	timeline domain.LeadTimelineRepository
}

func NewLeadTimelineUseCase(
	leads *LeadUseCase, timeline domain.LeadTimelineRepository,
) *LeadTimelineUseCase {
	return &LeadTimelineUseCase{leads: leads, timeline: timeline}
}

// LeadTimeline is what the lead panel renders.
type LeadTimeline struct {
	Messages []domain.TimelineMessage `json:"messages"`
	// Truncated says the cap cut the history. Reported rather than implied: a
	// list that silently starts mid-conversation reads as "this is everything",
	// and the first line then looks like the beginning of the relationship.
	Truncated bool `json:"truncated"`
}

// Execute returns the recent exchange with the lead's contact.
func (uc *LeadTimelineUseCase) Execute(
	ctx context.Context, actor domain.Actor, leadID string,
) (*LeadTimeline, error) {
	lead, err := uc.leads.GetForActor(ctx, actor, leadID)
	if err != nil {
		return nil, err
	}
	if uc.timeline == nil {
		return nil, domain.ErrLeadTimelineUnavailable
	}
	// A lead with no contact has nothing to show and is not an error: prospected
	// rows exist before anybody has been written to.
	if lead.ContactID == "" {
		return &LeadTimeline{Messages: []domain.TimelineMessage{}}, nil
	}

	// One more than the cap, so "was there more?" is answered by the query rather
	// than guessed from a full page.
	messages, err := uc.timeline.ListForContact(
		ctx, actor.AccountID, lead.ContactID, domain.TimelineLimit+1)
	if err != nil {
		return nil, err
	}

	out := &LeadTimeline{Messages: messages}
	if len(messages) > domain.TimelineLimit {
		out.Truncated = true
		// Drop the OLDEST, because the recent stretch is the one being read.
		out.Messages = messages[len(messages)-domain.TimelineLimit:]
	}
	if out.Messages == nil {
		out.Messages = []domain.TimelineMessage{}
	}
	return out, nil
}
