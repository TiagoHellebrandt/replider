package domain

import (
	"strings"
	"testing"
)

// US-M010. The digest's whole value is that every line is something a
// person can act on, ordered by urgency, with somewhere to go. So the
// builder is what the tests pin.

func TestSDRDigest_OrdersByUrgencyAndLinksEachLine(t *testing.T) {
	d := SDRDigest{PendingApprovals: 3, NewReplies: 2, StalledLeads: 5, UnownedLeads: 1}
	lines := d.Lines()

	if len(lines) != 4 {
		t.Fatalf("expected 4 actionable lines, got %d", len(lines))
	}
	// A reply waiting beats a backlog beats a follow-up beats housekeeping:
	// somebody is sitting unanswered after WE contacted them first.
	want := []string{"new_replies", "pending_approvals", "stalled_leads", "unowned_leads"}
	for i, kind := range want {
		if lines[i].Kind != kind {
			t.Errorf("line %d: got %q, want %q", i, lines[i].Kind, kind)
		}
		if lines[i].ClickURL == "" {
			t.Errorf("line %q has no destination — the digest exists to get you one tap from the work", kind)
		}
		if !strings.Contains(lines[i].Text, "1") && !strings.ContainsAny(lines[i].Text, "0123456789") {
			t.Errorf("line %q has no count in its text", kind)
		}
	}
	// The push notification has room for one thing: the most urgent.
	if got := d.Headline(); got != lines[0].Text {
		t.Errorf("headline %q should be the most urgent line %q", got, lines[0].Text)
	}
}

// Zero items are omitted, not rendered as "0 respostas novas" — noise
// pretending to be information.
func TestSDRDigest_OmitsEmptyItems(t *testing.T) {
	d := SDRDigest{PendingApprovals: 2}
	lines := d.Lines()

	if len(lines) != 1 || lines[0].Kind != "pending_approvals" {
		t.Fatalf("expected only the pending-approvals line, got %+v", lines)
	}
	if !strings.Contains(lines[0].ClickURL, "queue") {
		t.Errorf("the approvals line must point at the queue, got %q", lines[0].ClickURL)
	}
}

// A quiet day is not worth a notification: firing anyway is how a daily
// digest becomes something people mute.
func TestSDRDigest_QuietDayHasNoWork(t *testing.T) {
	var d SDRDigest
	if d.HasWork() {
		t.Error("an empty digest must not count as work")
	}
	if len(d.Lines()) != 0 {
		t.Error("an empty digest must render no lines")
	}
	if d.Headline() != "" {
		t.Error("an empty digest must have no headline")
	}
}
