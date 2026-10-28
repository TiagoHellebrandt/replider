package domain

import "testing"

// One predicate, two callers (the queue banner and the drafting gate). Two
// places deciding "can this account send?" is how a screen ends up promising
// what the engine refuses.
func TestAccountCanSend(t *testing.T) {
	cases := []struct {
		name string
		in   []*Channel
		want bool
	}{
		{"no channels at all", nil, false},
		{"an active email channel is a door", []*Channel{
			{Type: "email", IsActive: true}}, true},
		{"a paused door is a closed one", []*Channel{
			{Type: "email", IsActive: false}}, false},
		// The internal channel is the CS thread with the Replider platform,
		// not a way to reach a hospital.
		{"the internal CS thread is not a door", []*Channel{
			{Type: ChannelTypeInternal, IsActive: true}}, false},
		{"internal plus a real one still counts", []*Channel{
			{Type: ChannelTypeInternal, IsActive: true},
			{Type: "whatsapp", IsActive: true}}, true},
		{"a nil entry is not a door", []*Channel{nil}, false},
	}
	for _, c := range cases {
		if got := AccountCanSend(c.in); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// A door is not an address, and the two questions come apart in production:
// an account with a connected mailbox and 70 phone-only contacts passes
// AccountCanSend and cannot post a single letter.
func TestCanReachContact(t *testing.T) {
	mail := []*Channel{{Type: "email", IsActive: true}}
	wa := []*Channel{{Type: "whatsapp", IsActive: true}}
	both := []*Channel{{Type: "email", IsActive: true}, {Type: "whatsapp", IsActive: true}}

	email := "compras@hospital.com.br"
	phone := "+5565988887777"

	withEmail := &Contact{Email: &email}
	withPhone := &Contact{Phone: &phone}
	withNeither := &Contact{ExternalID: "pncp:03507415002864"}

	tests := []struct {
		name     string
		channels []*Channel
		contact  *Contact
		want     bool
	}{
		{"mailbox reaches an address", mail, withEmail, true},
		{"mailbox cannot reach a phone", mail, withPhone, false},
		{"whatsapp reaches a phone", wa, withPhone, true},
		{"whatsapp cannot reach an address", wa, withEmail, false},
		{"either door reaches either half", both, withPhone, true},
		{"nothing reaches a contact with no address", both, withNeither, false},
		{"a paused door reaches nobody", []*Channel{{Type: "email"}}, withEmail, false},
		{"the internal CS thread is not a door to a hospital",
			[]*Channel{{Type: ChannelTypeInternal, IsActive: true}}, withEmail, false},
		{"no channels at all", nil, withEmail, false},
		{"no contact at all", both, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanReachContact(tc.channels, tc.contact); got != tc.want {
				t.Fatalf("CanReachContact = %v, want %v", got, tc.want)
			}
		})
	}
}

// The queue's undeliverable count is the crossing of the three predicates
// with the channels that exist — never one of them alone.
func TestUndeliverableQueueCounts_UndeliverableWith(t *testing.T) {
	counts := UndeliverableQueueCounts{NoAddress: 13, NoEmail: 50, NoPhone: 18, Pending: 50}

	mail := []*Channel{{Type: "email", IsActive: true}}
	wa := []*Channel{{Type: "whatsapp", IsActive: true}}
	both := []*Channel{{Type: "email", IsActive: true}, {Type: "sms", IsActive: true}}

	if got := counts.UndeliverableWith(mail); got != 50 {
		t.Fatalf("mailbox-only: got %d, want every contact with no e-mail", got)
	}
	if got := counts.UndeliverableWith(wa); got != 18 {
		t.Fatalf("phone-only: got %d, want every contact with no phone", got)
	}
	if got := counts.UndeliverableWith(both); got != 13 {
		t.Fatalf("both doors: got %d, want only the contacts with neither", got)
	}
	// With no door at all the account-level gate already says so; repeating
	// it here would show the operator the same problem twice.
	if got := counts.UndeliverableWith(nil); got != 0 {
		t.Fatalf("no channels: got %d, want 0 so the no-channel message stands alone", got)
	}
}
