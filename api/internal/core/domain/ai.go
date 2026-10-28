package domain

import "context"

// ChatMessage represents a single message in the conversation history
type ChatMessage struct {
	Role    string `json:"role"` // "user" or "model"
	Content string `json:"content"`
}

// StreamResult wraps the chunk channel and a closure to fetch usage after the stream closes
type StreamResult struct {
	Chunks   <-chan string
	GetUsage func() (promptTokens int64, completionTokens int64)
}

// AITool represents a function that the AI can call
type AITool struct {
	Name        string
	Description string
	Parameters  any // JSON Schema
}

// ToolCall represents a specific tool invocation requested by the AI
type ToolCall struct {
	Index        int
	ID           string
	FunctionName string
	Arguments    string // JSON string
}

// AIUsage represents token consumption for an AI call
type AIUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	// CachedTokens is the part of PromptTokens the provider served from its
	// prefix cache — a SUBSET, not an addition (§6.16, K3). Zero means either no
	// hit or a provider that does not report one; both are priced at full rate,
	// which is the direction a margin estimate should err in.
	CachedTokens int64
}

// AICallbackResponse is the result of a non-streaming AI generation, which can be text or tool calls
type AICallbackResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     AIUsage
}

// AIProvider abstracts the underlying LLM provider implementation
type AIProvider interface {
	// GenerateStream takes a system prompt and conversation history,
	// returning a StreamResult that yields chunks and eventually token usage.
	GenerateStream(ctx context.Context, systemPrompt string, history []ChatMessage, tools []AITool) (StreamResult, error)

	// GenerateResponse behaves synchronously, returning the full response string or tool calls.
	GenerateResponse(ctx context.Context, systemPrompt string, history []ChatMessage, tools []AITool) (*AICallbackResponse, error)

	// GetEmbedding generates a vector representation of the provided text.
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// AIDocument is a file handed straight to a multimodal model.
//
// The scanned-PDF path: a contract that was signed, scanned and re-sent has no
// text layer, so extracting text first yields nothing. Sending the FILE lets the
// model do the reading — which is the same call the pipeline was going to make
// anyway, with a different input.
type AIDocument struct {
	MIMEType string
	Data     []byte
	// Filename is passed through for providers that use it as a hint. Optional.
	Filename string
}

// DocumentReader is an OPTIONAL provider capability, deliberately NOT part of
// AIProvider.
//
// Not every provider (or every model) can read a file, and widening AIProvider
// would force a dozen mocks to implement a method they cannot honour. Callers
// type-assert: a provider that cannot read documents makes the caller keep its
// honest "this PDF has no text" answer instead of silently doing nothing.
type DocumentReader interface {
	GenerateFromDocument(ctx context.Context, systemPrompt, prompt string, doc AIDocument) (*AICallbackResponse, error)
}

// AIFactory abstracts the creation of AIProviders.
// GetProviderForAccount handles BYOK: when the resolved Plan has no
// Stripe price (contract billing), the factory uses the account's own
// stored API key instead of the platform's env-level key. Returns
// ErrBYOKKeyMissing when the contract account has no key configured
// yet — the caller surfaces a "configure your API key" message.
type AIFactory interface {
	GetProvider(ctx context.Context, providerName string) (AIProvider, error)
	GetProviderForAccount(ctx context.Context, providerName string, account *Account, plan *Plan) (AIProvider, error)
	GetCostConfig(provider, model string) (float64, float64)
}

// ErrBYOKKeyMissing signals a contract-plan account that hasn't
// configured an OpenAI/Gemini key yet. Callers should save a friendly
// fallback message and surface a banner asking the admin to set the
// key in Settings.
var ErrBYOKKeyMissing = &AppError{Code: "BYOK_KEY_MISSING", Message: "API key is not configured for this account"}

// ErrBYOKNotConfigured is the outbound-sales-facing variant surfaced by
// AI-assist endpoints (e.g. funnel persona generation, US-005) when the
// account's BYOK key is absent. Distinct code so the sales UI can render
// a tailored "configure your AI key to use this feature" prompt.
var ErrBYOKNotConfigured = &AppError{Code: "BYOK_NOT_CONFIGURED", Message: "AI key is not configured for this account"}
