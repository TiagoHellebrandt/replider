package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
)

// Conversational onboarding against a real database (§6.20, K5).
//
// The two things a real request proves: the conversation RESUMES (the realistic
// user closes the tab halfway), and it finishes with no AI at all — the audience
// step is the only one with a model behind it, and a provider outage must cost a
// suggestion, not a signup.

func onboardingChat(t *testing.T, token string) map[string]any {
	t.Helper()
	w := doRequest("GET", "/v1/me/onboarding-chat", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func answerOnboarding(t *testing.T, token string, body map[string]any) map[string]any {
	t.Helper()
	w := doRequest("POST", "/v1/me/onboarding-chat", body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func TestOnboardingChat_StartsAtTheCompanyAndOpensWithAQuestion(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	view := onboardingChat(t, token)

	assert.Equal(t, "company", view["step"])
	prompt, _ := view["prompt"].(map[string]any)
	require.NotNil(t, prompt)
	assert.NotEmpty(t, prompt["text"])
	// The site widget, because one URL is the highest-leverage single input.
	assert.Equal(t, "site", prompt["widget"])

	// The opening question is already in the transcript, so a resume shows the
	// conversation rather than an empty box.
	messages, _ := view["messages"].([]any)
	require.Len(t, messages, 1)
}

func TestOnboardingChat_WalksTheLadderAndFinishesWithoutAI(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	onboardingChat(t, token)

	// One sentence with the name and the site, the way people actually type it.
	afterCompany := answerOnboarding(t, token, map[string]any{
		"text": "Clínica Bem Viver, clinicabemviver.com.br",
	})
	assert.Equal(t, "product", afterCompany["step"])
	answers, _ := afterCompany["answers"].(map[string]any)
	assert.Equal(t, "Clínica Bem Viver", answers["company_name"])
	// Split without a model call: a comma is not worth a provider round trip.
	assert.Equal(t, "clinicabemviver.com.br", answers["website"])

	afterProduct := answerOnboarding(t, token, map[string]any{
		"text": "Equipamentos de fisioterapia para clínicas",
	})
	assert.Equal(t, "audiences", afterProduct["step"])

	// The audience step is the ONLY one with a model behind it. With none wired
	// it offers nothing and is skipped — a worse onboarding, not a broken one.
	afterAudiences := answerOnboarding(t, token, map[string]any{"skip": true})
	assert.Equal(t, "channel", afterAudiences["step"])

	done := answerOnboarding(t, token, map[string]any{"choices": []string{"whatsapp"}})
	assert.Equal(t, "done", done["step"])
	assert.Equal(t, true, done["done"])
	assert.Equal(t, float64(1), done["progress"])
}

func TestOnboardingChat_TheCompanyStepCannotBeSkipped(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	onboardingChat(t, token)

	// Everything downstream is written in this company's voice.
	skipped := doRequest("POST", "/v1/me/onboarding-chat",
		map[string]any{"skip": true}, token)
	assert.Equal(t, http.StatusBadRequest, skipped.Code, skipped.Body.String())

	// An answer with no name in it is the same refusal, not a silent advance.
	empty := doRequest("POST", "/v1/me/onboarding-chat",
		map[string]any{"text": "   "}, token)
	assert.Equal(t, http.StatusBadRequest, empty.Code)
}

func TestOnboardingChat_ResumesWhereItWasLeft(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	onboardingChat(t, token)
	answerOnboarding(t, token, map[string]any{"text": "Clínica Bem Viver"})

	// The realistic user: closes the tab, comes back after lunch.
	resumed := onboardingChat(t, token)

	assert.Equal(t, "product", resumed["step"])
	answers, _ := resumed["answers"].(map[string]any)
	assert.Equal(t, "Clínica Bem Viver", answers["company_name"])
	// And the transcript is there, so they see what they already answered
	// instead of being asked again.
	messages, _ := resumed["messages"].([]any)
	assert.GreaterOrEqual(t, len(messages), 3)
}

func TestOnboardingChat_TheCompanyNameReachesTheAccount(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	onboardingChat(t, token)

	answerOnboarding(t, token, map[string]any{"text": "Clínica Bem Viver"})

	// Written straight through, not left in a draft: otherwise the first
	// preview chat introduces the agent as "o assistente virtual de ".
	var name string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT name FROM accounts WHERE id = $1`, accountID).Scan(&name))
	assert.Equal(t, "Clínica Bem Viver", name)
}

func TestOnboardingChat_TheProductStepRecordsAProduct(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	onboardingChat(t, token)
	answerOnboarding(t, token, map[string]any{"text": "Clínica Bem Viver"})

	answerOnboarding(t, token, map[string]any{
		"text": "Equipamentos de fisioterapia para clínicas",
	})

	// With no extractor wired the first line becomes the name — a real product
	// they can rename, rather than an empty row or nothing at all.
	var count int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM products WHERE account_id = $1`, accountID).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestOnboardingChat_FirstLeadIsStampedOnce(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	onboardingChat(t, token)

	repo := postgres.NewOnboardingChatRepository(testPool)
	first := time.Now()
	require.NoError(t, repo.StampFirstLead(ctx, accountID, first))
	// An established account creating another lead must not restart the clock,
	// or the activation metric improves every time somebody sells something.
	require.NoError(t, repo.StampFirstLead(ctx, accountID, first.Add(time.Hour)))

	chat, err := repo.Get(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, chat.FirstLeadAt)
	assert.WithinDuration(t, first, *chat.FirstLeadAt, 2*time.Second)
}

func TestOnboardingChat_IsPerAccount(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")
	theirs := seedIdentityAccount(t)
	theirToken := tokenForSeededAccount(t, theirs, "admin")

	onboardingChat(t, myToken)
	answerOnboarding(t, myToken, map[string]any{"text": "Clínica Bem Viver"})

	// Another account starts at the beginning, with none of my answers.
	theirView := onboardingChat(t, theirToken)
	assert.Equal(t, "company", theirView["step"])
	answers, _ := theirView["answers"].(map[string]any)
	assert.Empty(t, answers["company_name"])
}
