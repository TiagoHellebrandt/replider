package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The reply from the lead: the vendedor can READ the answer to her proposal
// (timeline); this is where she answers it. Gated by the lead's ownership,
// sent through the same human path the inbox uses.

type fakeReplyConvs struct {
	found    *domain.Conversation
	full     *domain.Conversation
	findErr  error
	findArgs []string
}

func (f *fakeReplyConvs) FindByAccountContact(_ context.Context, accountID, contactID string) (*domain.Conversation, error) {
	f.findArgs = []string{accountID, contactID}
	return f.found, f.findErr
}

func (f *fakeReplyConvs) GetByID(_ context.Context, id, accountID string) (*domain.Conversation, error) {
	if f.full != nil && f.full.ID == id && f.full.AccountID == accountID {
		return f.full, nil
	}
	return nil, nil
}

type fakeReplier struct {
	assumed   []string
	sent      []SendHumanMessageDTO
	assumeErr error
	sendErr   error
}

func (f *fakeReplier) Assume(_ context.Context, conversationID, _ string, userID string) (*AssumeResult, error) {
	f.assumed = append(f.assumed, conversationID+"|"+userID)
	return &AssumeResult{}, f.assumeErr
}

func (f *fakeReplier) SendHumanMessage(_ context.Context, dto SendHumanMessageDTO) (*domain.Message, error) {
	f.sent = append(f.sent, dto)
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &domain.Message{ID: "msg-1"}, nil
}

func replyLead(owner string) *domain.Lead {
	l := &domain.Lead{ID: "lead-1", AccountID: "acc-1", ContactID: "contact-1", Visibility: domain.VisibilityTeam}
	if owner != "" {
		l.OwnerUserID = &owner
	}
	return l
}

func replyActor(userID string, role domain.Role) domain.Actor {
	return domain.Actor{UserID: userID, AccountID: "acc-1", Role: role}
}

func newReplyUC(lead *domain.Lead, convs *fakeReplyConvs, sender *fakeReplier) *LeadReplyUseCase {
	repo := new(mockLeadRepo)
	repo.On("GetByID", mock.Anything, "lead-1").Return(lead, nil)
	return NewLeadReplyUseCase(NewLeadUseCase(repo, nil), convs, sender)
}

func TestLeadReply_VendedorAnswersOnHerOwnLeadAndPausesTheBotFirst(t *testing.T) {
	convs := &fakeReplyConvs{
		found: &domain.Conversation{ID: "conv-1"},
		full:  &domain.Conversation{ID: "conv-1", AccountID: "acc-1", IsBotPaused: false},
	}
	sender := &fakeReplier{}
	uc := newReplyUC(replyLead("rafa"), convs, sender)

	msg, err := uc.Execute(context.Background(), replyActor("rafa", domain.RoleVendedor),
		LeadReplyDTO{LeadID: "lead-1", Text: "  Ótimo, mando o orçamento hoje.  ", UserName: "Rafa"})
	require.NoError(t, err)
	assert.Equal(t, "msg-1", msg.ID)

	// The thread is looked up by the PERSON (contact ref), inside the account.
	assert.Equal(t, []string{"acc-1", "contact-1"}, convs.findArgs)
	// A live-bot thread is taken over by the person replying, so the human
	// send passes its gate and the AI never answers on top of her.
	assert.Equal(t, []string{"conv-1|rafa"}, sender.assumed)
	require.Len(t, sender.sent, 1)
	assert.Equal(t, "conv-1", sender.sent[0].ConversationID)
	assert.Equal(t, "acc-1", sender.sent[0].AccountID)
	assert.Equal(t, "rafa", sender.sent[0].UserID)
	assert.Equal(t, "Rafa", sender.sent[0].UserName)
	assert.Equal(t, "Ótimo, mando o orçamento hoje.", sender.sent[0].Content, "trimmed")
}

func TestLeadReply_AnAlreadyPausedThreadIsNotReassumed(t *testing.T) {
	convs := &fakeReplyConvs{
		found: &domain.Conversation{ID: "conv-1"},
		full:  &domain.Conversation{ID: "conv-1", AccountID: "acc-1", IsBotPaused: true},
	}
	sender := &fakeReplier{}
	uc := newReplyUC(replyLead(""), convs, sender)

	_, err := uc.Execute(context.Background(), replyActor("rafa", domain.RoleVendedor),
		LeadReplyDTO{LeadID: "lead-1", Text: "Oi"})
	require.NoError(t, err)
	// A colleague who assumed it keeps it.
	assert.Empty(t, sender.assumed)
	assert.Len(t, sender.sent, 1)
}

func TestLeadReply_AColleaguesLeadIsForbiddenBeforeAnyLookup(t *testing.T) {
	convs := &fakeReplyConvs{found: &domain.Conversation{ID: "conv-1"}}
	sender := &fakeReplier{}
	uc := newReplyUC(replyLead("someone-else"), convs, sender)

	_, err := uc.Execute(context.Background(), replyActor("rafa", domain.RoleVendedor),
		LeadReplyDTO{LeadID: "lead-1", Text: "Oi"})
	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.Nil(t, convs.findArgs, "the thread must not even be looked up")
	assert.Empty(t, sender.sent)
}

func TestLeadReply_NoConversationYetIs409NotASend(t *testing.T) {
	convs := &fakeReplyConvs{found: nil}
	sender := &fakeReplier{}
	uc := newReplyUC(replyLead("rafa"), convs, sender)

	_, err := uc.Execute(context.Background(), replyActor("rafa", domain.RoleVendedor),
		LeadReplyDTO{LeadID: "lead-1", Text: "Oi"})
	assert.ErrorIs(t, err, domain.ErrLeadNoConversation)
	assert.Empty(t, sender.sent)
}

func TestLeadReply_ALeadWithoutContactHasNoThread(t *testing.T) {
	lead := replyLead("rafa")
	lead.ContactID = ""
	convs := &fakeReplyConvs{found: &domain.Conversation{ID: "conv-1"}}
	uc := newReplyUC(lead, convs, &fakeReplier{})

	_, err := uc.Execute(context.Background(), replyActor("rafa", domain.RoleVendedor),
		LeadReplyDTO{LeadID: "lead-1", Text: "Oi"})
	assert.ErrorIs(t, err, domain.ErrLeadNoConversation)
	assert.Nil(t, convs.findArgs)
}

func TestLeadReply_EmptyTextIsRefusedBeforeAnything(t *testing.T) {
	convs := &fakeReplyConvs{found: &domain.Conversation{ID: "conv-1"}}
	sender := &fakeReplier{}
	uc := newReplyUC(replyLead("rafa"), convs, sender)

	_, err := uc.Execute(context.Background(), replyActor("rafa", domain.RoleVendedor),
		LeadReplyDTO{LeadID: "lead-1", Text: "   \n"})
	assert.ErrorIs(t, err, domain.ErrLeadReplyTextRequired)
	assert.Nil(t, convs.findArgs)
	assert.Empty(t, sender.sent)
}

func TestLeadReply_GestorAnswersOnAnyLeadOfTheAccount(t *testing.T) {
	convs := &fakeReplyConvs{
		found: &domain.Conversation{ID: "conv-1"},
		full:  &domain.Conversation{ID: "conv-1", AccountID: "acc-1", IsBotPaused: true},
	}
	sender := &fakeReplier{}
	uc := newReplyUC(replyLead("someone-else"), convs, sender)

	_, err := uc.Execute(context.Background(), replyActor("chefe", domain.RoleGestor),
		LeadReplyDTO{LeadID: "lead-1", Text: "Oi"})
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1)
}
