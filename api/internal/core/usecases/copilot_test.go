package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// US-M026. The copilot can drive the pipeline from a sentence, so the
// tests are about its limits: it must not be able to send outbound, it
// must obey the same role gate as the pipeline, and it must not loop
// forever while a person waits.

func copilotUC(t *testing.T, provider *MockAIProvider) *CopilotUseCase {
	t.Helper()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	factory := new(MockAIFactory)

	planID := "plan1"
	accountRepo.On("GetByID", mock.Anything, "acc1").
		Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil).Maybe()
	planRepo.On("GetByID", mock.Anything, planID).
		Return(&domain.Plan{ID: planID, AIProvider: "google", AIModel: "gemini-3.1-flash-lite"}, nil).Maybe()
	factory.On("GetProviderForAccount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(provider, nil).Maybe()
	// The mock factory delegates to GetProvider internally.
	factory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil).Maybe()

	return NewCopilotUseCase(accountRepo, planRepo, factory)
}

func actor(role domain.Role) domain.Actor {
	return domain.Actor{UserID: "u1", AccountID: "acc1", Role: role}
}

// The product's whole posture on cold outreach is that a human reads the
// exact text and the recipient before it reaches a stranger. A chat that
// could send on "manda pra eles" would undo that in one sentence.
func TestCopilot_HasNoToolThatSends(t *testing.T) {
	uc := copilotUC(t, new(MockAIProvider))
	uc.SetTools(&FunnelUseCase{}, &LeadUseCase{}, &ProspectingUseCase{}, &ApproachUseCase{},
		NewCaseMetricsUseCase(nil))

	for _, tool := range uc.toolDefs() {
		name := strings.ToLower(tool.Name)
		assert.NotContains(t, name, "send", "the copilot must not expose a sending tool: %s", tool.Name)
		assert.NotContains(t, name, "approve", "approval stays with the human queue: %s", tool.Name)
	}

	// ...and the drafting tool says so in its own description, because the
	// model reads that, not our commit messages.
	var drafting domain.AITool
	for _, tool := range uc.toolDefs() {
		if tool.Name == "draft_approaches" {
			drafting = tool
		}
	}
	require.NotEmpty(t, drafting.Name)
	assert.Contains(t, strings.ToLower(drafting.Description), "não envia")
}

// A tool the deployment did not wire simply is not offered — the copilot
// says it cannot, instead of failing mid-call.
func TestCopilot_UnwiredToolsAreNotOffered(t *testing.T) {
	uc := copilotUC(t, new(MockAIProvider))
	uc.SetTools(&FunnelUseCase{}, nil, nil, nil, nil)

	names := []string{}
	for _, tool := range uc.toolDefs() {
		names = append(names, tool.Name)
	}
	assert.Equal(t, []string{"list_funnels"}, names)
}

// The copilot acts on the pipeline, so it answers to the pipeline's gate.
func TestCopilot_AttendantIsRefused(t *testing.T) {
	uc := copilotUC(t, new(MockAIProvider))

	_, err := uc.Ask(context.Background(), actor(domain.RoleAtendenteRole), nil, "prospecta pra mim")
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestCopilot_EmptyMessageIsRejected(t *testing.T) {
	uc := copilotUC(t, new(MockAIProvider))

	_, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "   ")
	require.Error(t, err)
}

// A plain answer with no tool call comes straight back.
func TestCopilot_AnswersWithoutTools(t *testing.T) {
	provider := new(MockAIProvider)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "Seu funil tem 12 leads ativos."}, nil).Once()

	uc := copilotUC(t, provider)
	reply, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "como está o funil?")
	require.NoError(t, err)
	assert.Equal(t, "Seu funil tem 12 leads ativos.", reply.Message)
	assert.Empty(t, reply.ToolsUsed)
}

// A model that keeps asking for tools is confused or looping; either way
// a person is waiting, so the loop is bounded and says so.
func TestCopilot_ToolLoopIsBounded(t *testing.T) {
	provider := new(MockAIProvider)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{
			ToolCalls: []domain.ToolCall{{FunctionName: "list_funnels", Arguments: "{}"}},
		}, nil)

	uc := copilotUC(t, provider)
	uc.SetTools(nil, nil, nil, nil, nil) // unknown tool → text back, loop continues

	reply, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "faz tudo")
	require.NoError(t, err)
	assert.NotEmpty(t, reply.Message, "a bounded loop still answers something")
	// The model asked for list_funnels on EVERY round and the loop stopped at the
	// bound, as before. What changed is that an identical repeat does not run
	// again: the round is spent, the side effect is not.
	assert.Len(t, reply.ToolsUsed, 1,
		"it stops after the bound, and an identical call runs once")
}

// A prospecting run that created 56 leads was announced in the chat as "1 of them
// was added as a new lead". The loop already said "use these numbers in the
// answer" and the model rewrote them anyway — asking a text generator for honesty
// is not a mechanism.
//
// The receipt is the mechanism: the tool's text reaches the screen without passing
// through the model, so its prose can be whatever it likes.
func TestCopilot_WriteToolResultDoesNotPassThroughTheModel(t *testing.T) {
	provider := new(MockAIProvider)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{
			ToolCalls: []domain.ToolCall{{
				FunctionName: "draft_approaches",
				Arguments:    `{"funnel_id":"fn-1"}`,
			}},
		}, nil).Once()
	// The model tells a different story about what just happened.
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "1 lead adicionado."}, nil).Once()

	uc := copilotUC(t, provider)

	reply, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "ache clínicas")
	require.NoError(t, err)
	require.Len(t, reply.Receipts, 1, "uma ferramenta que escreve deixa recibo")
	assert.Equal(t, "draft_approaches", reply.Receipts[0].Tool)
	assert.NotEmpty(t, reply.Receipts[0].Text)
	assert.NotEqual(t, reply.Message, reply.Receipts[0].Text,
		"o recibo vem do servidor, não do que o modelo escreveu")
}

// On a read, a wrong paraphrase costs one more question. On a write, the user is
// left not knowing what entered their account — which is why only the writers
// leave a receipt, and the list is a single one.
func TestCopilot_OnlyWriteToolsLeaveAReceipt(t *testing.T) {
	assert.True(t, copilotToolWrites("draft_approaches"))
	assert.True(t, copilotToolWrites("draft_lead_approach"))

	for _, readOnly := range []string{
		"list_funnels", "pipeline_overview", "list_approval_queue", "case_report",
		"get_pending_actions", "list_signals",
	} {
		assert.False(t, copilotToolWrites(readOnly),
			"%s não muda nada na conta", readOnly)
	}
}

// Seen in production: four identical prospect_businesses in one turn. The first
// created three leads, the other three created zero — dedup held the funnel, but
// they were three paid Google Places searches, and the rounds spent on them are
// the ones the model then lacked to finish (the turn ended in "I lost my way").
func TestCopilot_RepeatedToolCallDoesNotRunTwice(t *testing.T) {
	provider := new(MockAIProvider)
	call := domain.ToolCall{
		FunctionName: "draft_approaches",
		Arguments:    `{"funnel_id":"fn-1","limit":5}`,
	}
	// O modelo pede a mesma coisa de novo, com as chaves em outra ordem.
	again := domain.ToolCall{
		FunctionName: "draft_approaches",
		Arguments:    `{"limit":5,"funnel_id":"fn-1"}`,
	}
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{ToolCalls: []domain.ToolCall{call}}, nil).Once()
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{ToolCalls: []domain.ToolCall{again}}, nil).Once()
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "pronto"}, nil).Once()

	uc := copilotUC(t, provider)

	reply, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "ache clínicas")
	require.NoError(t, err)
	assert.Len(t, reply.ToolsUsed, 1,
		"a segunda chamada idêntica não roda — e não conta como trabalho feito")
	assert.Len(t, reply.Receipts, 1, "um efeito, um recibo")
}

// Key order changes between rounds; comparing the raw string would let the repeat
// through.
func TestCopilot_ToolCallKeyIgnoresArgumentOrder(t *testing.T) {
	a := domain.ToolCall{FunctionName: "x", Arguments: `{"a":1,"b":2}`}
	b := domain.ToolCall{FunctionName: "x", Arguments: `{"b":2,"a":1}`}
	assert.Equal(t, toolCallKey(a), toolCallKey(b))

	c := domain.ToolCall{FunctionName: "x", Arguments: `{"a":1,"b":3}`}
	assert.NotEqual(t, toolCallKey(a), toolCallKey(c),
		"argumentos diferentes são chamadas diferentes")
}

// The copilot wrote 'O único funil disponível é "TESTE QA — pode apagar"
// (id: 01KYW9HYFXQK3Y41HAJFTG9CSS)'. The id is for the next tool call, not for a
// person to read.
func TestCopilot_MessageNeverCarriesAnID(t *testing.T) {
	assert.Equal(t,
		`O único funil disponível é "TESTE QA — pode apagar".`,
		stripIdentifiers(
			`O único funil disponível é "TESTE QA — pode apagar" (id: 01KYW9HYFXQK3Y41HAJFTG9CSS).`))

	// Bare in the sentence, with no surrounding "(id: ...)": the gap becomes a
	// space, or the two words collide ("Movi o leadpara Ganhos").
	assert.Equal(t, "Movi o lead para Ganhos.",
		stripIdentifiers("Movi o lead 01KYW9HYFXQK3Y41HAJFTG9CSS para Ganhos."))

	// ...and with the id INSIDE the quotes, the space does NOT come back — otherwise
	// you get the `apagar "` production showed.
	assert.Equal(t, `use o funil "TESTE QA — pode apagar"`,
		stripIdentifiers(`use o funil "TESTE QA — pode apagar (id: 01KYW9HYFXQK3Y41HAJFTG9CSS)"`))

	// Text with no id passes through intact — long upper-case words included.
	const clean = "Achei 3 clínicas de fisioterapia em CAMPINAS e importei todas."
	assert.Equal(t, clean, stripIdentifiers(clean))
}

// Taking the ULID out of the message took it out of the conversation too: the
// model asked "confirm which funnel", the user answered "yes, use that one", and
// the id was nowhere any more — the next call went out with a guessed id and came
// back SALES_FUNNEL_NOT_FOUND. Seen in production right after the fix.
//
// Whoever reads and whatever continues the conversation are different consumers of
// the same text.
func TestCopilot_ContextKeepsTheIDTheScreenHides(t *testing.T) {
	provider := new(MockAIProvider)
	const raw = `O único funil é "TESTE QA" (id: 01KYW9HYFXQK3Y41HAJFTG9CSS). Confirma?`
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: raw}, nil).Once()

	uc := copilotUC(t, provider)
	reply, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "quais funis?")
	require.NoError(t, err)

	assert.NotContains(t, reply.Message, "01KYW9HYFXQK3Y41HAJFTG9CSS",
		"a tela não mostra o id")
	assert.Equal(t, raw, reply.ContextMessage,
		"a conversa continua sabendo dele")
}

// With nothing to strip there is no second copy of the message for the client to
// carry back and forth.
func TestCopilot_NoContextCopyWhenNothingWasStripped(t *testing.T) {
	provider := new(MockAIProvider)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "Você tem 3 funis."}, nil).Once()

	uc := copilotUC(t, provider)
	reply, err := uc.Ask(context.Background(), actor(domain.RoleGestor), nil, "quais funis?")
	require.NoError(t, err)

	assert.Equal(t, "Você tem 3 funis.", reply.Message)
	assert.Empty(t, reply.ContextMessage)
}

// Two tools that both end at the approval queue must not stack two
// identical buttons — seen live before the dedup.
func TestCopilot_ActionsAreDedupedByDestination(t *testing.T) {
	reply := &CopilotReply{}
	route := "/prospect?tab=queue"

	assert.False(t, reply.hasAction(route))
	reply.Actions = append(reply.Actions, CopilotAction{Label: "Ver fila", Route: route})
	assert.True(t, reply.hasAction(route))
	assert.False(t, reply.hasAction("/funnel"))
}

// An answer that ends in "1 aguardando aprovação" and offers nothing to
// click leaves the reader to go find the queue themselves — and that queue
// is the one step in the loop that cannot happen without them.
func TestCopilot_OverviewPointsAtWhateverIsBlockedOnTheReader(t *testing.T) {
	// Drafts waiting win, even when the pipeline is mostly elsewhere.
	action := pipelineOverviewAction(1, 40)
	require.NotNil(t, action)
	assert.Equal(t, "/prospect?tab=queue", action.Route)

	// Nothing waiting on a human: the board is the useful destination.
	action = pipelineOverviewAction(0, 40)
	require.NotNil(t, action)
	assert.Equal(t, "/funnel", action.Route)

	// An empty pipeline has nothing to go and look at, so no button —
	// pointing at an empty board is worse than saying so in words.
	assert.Nil(t, pipelineOverviewAction(0, 0))
}
