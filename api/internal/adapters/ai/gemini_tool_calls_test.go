package ai

import (
	"encoding/json"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gemini is the platform's DEFAULT provider, and GenerateResponse used to
// collect only text parts. A response that was a single FunctionCall
// therefore came back empty, which silently disabled every consumer of
// ToolCalls: check_availability, the invisible lead capture (US-011),
// buy-intent flagging (US-012), the funnel webhook triggers (US-016) and
// the copilot (US-M026). No error was raised anywhere — the model simply
// appeared to have nothing to say.

func TestSplitGeminiParts_ExtractsFunctionCalls(t *testing.T) {
	text, calls := splitGeminiParts([]genai.Part{
		genai.FunctionCall{
			Name: "check_availability",
			Args: map[string]any{"date": "2026-08-01", "calendar_id": "cal_1"},
		},
	})

	assert.Empty(t, text, "a pure function-call response has no text")
	require.Len(t, calls, 1, "the call must not be dropped")
	assert.Equal(t, "check_availability", calls[0].FunctionName)
	assert.NotEmpty(t, calls[0].ID, "callers correlate on an id")

	// Arguments arrive as the JSON string the contract promises, not a map.
	var args map[string]any
	require.NoError(t, json.Unmarshal([]byte(calls[0].Arguments), &args))
	assert.Equal(t, "2026-08-01", args["date"])
	assert.Equal(t, "cal_1", args["calendar_id"])
}

// Text and a call can arrive together; neither may swallow the other.
func TestSplitGeminiParts_KeepsTextAlongsideCalls(t *testing.T) {
	text, calls := splitGeminiParts([]genai.Part{
		genai.Text("Vou verificar a agenda. "),
		genai.FunctionCall{Name: "check_availability", Args: map[string]any{"date": "2026-08-01"}},
		genai.Text("Um momento."),
	})

	assert.Equal(t, "Vou verificar a agenda. Um momento.", text)
	require.Len(t, calls, 1)
	assert.Equal(t, "check_availability", calls[0].FunctionName)
}

// A zero-argument tool still produces valid JSON, because callers
// json.Unmarshal the arguments unconditionally.
func TestSplitGeminiParts_ArgumentlessCallIsValidJSON(t *testing.T) {
	_, calls := splitGeminiParts([]genai.Part{
		genai.FunctionCall{Name: "list_funnels"},
	})

	require.Len(t, calls, 1)
	var args map[string]any
	assert.NoError(t, json.Unmarshal([]byte(calls[0].Arguments), &args))
}

func TestSplitGeminiParts_PlainTextHasNoCalls(t *testing.T) {
	text, calls := splitGeminiParts([]genai.Part{genai.Text("Olá!")})

	assert.Equal(t, "Olá!", text)
	assert.Empty(t, calls)
}

// Several calls in one turn keep distinct ids, or a caller correlating on
// id would collapse them into one.
func TestSplitGeminiParts_MultipleCallsGetDistinctIDs(t *testing.T) {
	_, calls := splitGeminiParts([]genai.Part{
		genai.FunctionCall{Name: "record_lead_data", Args: map[string]any{"field_key": "cnpj"}},
		genai.FunctionCall{Name: "record_lead_data", Args: map[string]any{"field_key": "porte"}},
	})

	require.Len(t, calls, 2)
	assert.NotEqual(t, calls[0].ID, calls[1].ID)
}
