package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/generative-ai-go/genai"
	"github.com/replider/api/internal/core/domain"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GeminiProvider struct {
	client *genai.Client
	apiKey string // kept for direct REST embedding calls (v1 endpoint)
	model  string
}

// NewGeminiProvider initializes a new Gemini implementation of AIProvider.
// The apiKey is stored separately for direct REST embedding calls, because
// the genai SDK hardcodes /v1beta/ internally and text-embedding-004 only
// exists on the stable v1 API.
func NewGeminiProvider(ctx context.Context, apiKey string, modelName string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	if modelName == "" {
		modelName = "gemini-3.7-flash"
	}

	return &GeminiProvider{
		client: client,
		apiKey: apiKey,
		model:  modelName,
	}, nil
}

// GenerateStream sends the history and prompt to Gemini, streaming the response chunks back.
func (p *GeminiProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	model := p.client.GenerativeModel(p.model)

	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}

	if len(tools) > 0 {
		var genaiTools []*genai.Tool
		for _, t := range tools {
			genaiTools = append(genaiTools, &genai.Tool{
				FunctionDeclarations: []*genai.FunctionDeclaration{
					{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  toGenaiSchema(t.Parameters),
					},
				},
			})
		}
		model.Tools = genaiTools
	}

	// Truncate history to prevent context window overflow
	truncated := truncateHistory(history, 24000)

	var genaiHistory []*genai.Content
	for _, msg := range truncated {
		role := "user"
		if msg.Role == "model" || msg.Role == "assistant" {
			role = "model"
		}
		genaiHistory = append(genaiHistory, &genai.Content{
			Role:  role,
			Parts: []genai.Part{genai.Text(msg.Content)},
		})
	}

	cs := model.StartChat()
	cs.History = genaiHistory

	var lastMessage string
	if len(truncated) > 0 && truncated[len(truncated)-1].Role == "user" {
		lastMessage = truncated[len(truncated)-1].Content
		cs.History = genaiHistory[:len(genaiHistory)-1]
	} else {
		lastMessage = "Please proceed."
	}

	iter := cs.SendMessageStream(ctx, genai.Text(lastMessage))

	out := make(chan string)
	var usage geminiStreamUsage

	go func() {
		defer close(out)
		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				// Surface provider failures instead of ending the
				// stream silently — the customer otherwise sees a
				// truncated reply with no trace for ops.
				slog.Error("gemini stream ended with error", "model", p.model, "error", err)
				break
			}

			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					var chunk string
					if text, ok := part.(genai.Text); ok {
						chunk = string(text)
					} else if _, ok := part.(genai.FunctionCall); ok {
						chunk = "[FUNCTION_CALL]"
					}
					if chunk == "" {
						continue
					}
					// Honour caller cancellation — without this the
					// goroutine blocks on `out` forever (leak) once the
					// consumer stops reading.
					select {
					case out <- chunk:
					case <-ctx.Done():
						slog.Warn("gemini stream abandoned (context cancelled)", "model", p.model)
						return
					}
				}
			}

			usage.observe(resp.UsageMetadata)
		}
	}()

	return domain.StreamResult{
		Chunks: out,
		// Read after the consumer drained Chunks (the close happens-before
		// the final receive), so the goroutine's last write is visible.
		GetUsage: usage.totals,
	}, nil
}

// geminiStreamUsage tracks token usage across a stream.
//
// Every chunk's UsageMetadata is CUMULATIVE — the prompt count is repeated in
// each one and the candidate count is the running total so far. Summing the
// chunks therefore multiplied the prompt by the number of chunks (a 700-token
// prompt on a 40-chunk reply was recorded as 28,000) and, because the SDK
// counter for candidates ignores reasoning, the thought tokens Google bills
// as output were lost entirely. Keeping only the LAST chunk and applying the
// same total−prompt rule as geminiUsage fixes both.
type geminiStreamUsage struct {
	last *genai.UsageMetadata
}

func (u *geminiStreamUsage) observe(m *genai.UsageMetadata) {
	if m != nil {
		u.last = m
	}
}

func (u *geminiStreamUsage) totals() (prompt, completion int64) {
	return geminiUsage(u.last)
}

func (p *GeminiProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	model := p.client.GenerativeModel(p.model)

	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}

	if len(tools) > 0 {
		var genaiTools []*genai.Tool
		for _, t := range tools {
			genaiTools = append(genaiTools, &genai.Tool{
				FunctionDeclarations: []*genai.FunctionDeclaration{
					{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  toGenaiSchema(t.Parameters),
					},
				},
			})
		}
		model.Tools = genaiTools
	}

	// Truncate history to prevent context window overflow
	truncatedResp := truncateHistory(history, 24000)

	var genaiHistory2 []*genai.Content
	for _, msg := range truncatedResp {
		role := "user"
		if msg.Role == "model" || msg.Role == "assistant" {
			role = "model"
		}
		genaiHistory2 = append(genaiHistory2, &genai.Content{
			Role:  role,
			Parts: []genai.Part{genai.Text(msg.Content)},
		})
	}

	cs := model.StartChat()
	cs.History = genaiHistory2

	var lastMessage string
	if len(truncatedResp) > 0 && truncatedResp[len(truncatedResp)-1].Role == "user" {
		lastMessage = truncatedResp[len(truncatedResp)-1].Content
		cs.History = genaiHistory2[:len(genaiHistory2)-1]
	} else {
		lastMessage = "Please proceed."
	}

	resp, err := cs.SendMessage(ctx, genai.Text(lastMessage))
	if err != nil {
		return nil, err
	}

	// Text AND function calls. Only text was collected before, so a caller
	// that passed tools got an empty answer whenever the model chose to
	// call one: the FunctionCall part was dropped on the floor and there
	// was no text alongside it. Silent, and indistinguishable from "the
	// model had nothing to say".
	var totalResponse string
	var toolCalls []domain.ToolCall
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		totalResponse, toolCalls = splitGeminiParts(resp.Candidates[0].Content.Parts)
	}

	var usage domain.AIUsage
	if resp.UsageMetadata != nil {
		usage.PromptTokens, usage.CompletionTokens = geminiUsage(resp.UsageMetadata)
		usage.CachedTokens = geminiCachedTokens(resp.UsageMetadata)
	}

	return &domain.AICallbackResponse{
		Content:   totalResponse,
		ToolCalls: toolCalls,
		Usage:     usage,
	}, nil
}

// GenerateFromDocument reads a FILE and answers about it (domain.DocumentReader).
//
// This is what makes a scanned contract readable: no text layer to extract, so
// the bytes go to the model and the model does the reading. Native SDK Blob
// rather than the REST path GetEmbedding uses — GenerateContent takes parts, and
// a blob is just another part.
//
// Inline, not the Files API: the caller caps the size (see the contract use case)
// and an inline request needs no upload, no handle to track and no cleanup. A
// document too big for one request is the caller's problem to refuse, not
// something to paper over with a second protocol.
func (p *GeminiProvider) GenerateFromDocument(
	ctx context.Context, systemPrompt, prompt string, doc domain.AIDocument,
) (*domain.AICallbackResponse, error) {
	if len(doc.Data) == 0 {
		return nil, fmt.Errorf("gemini: empty document")
	}
	mime := doc.MIMEType
	if mime == "" {
		mime = "application/pdf"
	}

	model := p.client.GenerativeModel(p.model)
	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}

	// The document FIRST, then the instruction: the model reads the file and then
	// is told what to pull out of it.
	resp, err := model.GenerateContent(ctx,
		genai.Blob{MIMEType: mime, Data: doc.Data},
		genai.Text(prompt),
	)
	if err != nil {
		return nil, err
	}

	var content string
	var toolCalls []domain.ToolCall
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		content, toolCalls = splitGeminiParts(resp.Candidates[0].Content.Parts)
	}
	var usage domain.AIUsage
	if resp.UsageMetadata != nil {
		usage.PromptTokens, usage.CompletionTokens = geminiUsage(resp.UsageMetadata)
		usage.CachedTokens = geminiCachedTokens(resp.UsageMetadata)
	}
	return &domain.AICallbackResponse{
		Content: content, ToolCalls: toolCalls, Usage: usage,
	}, nil
}

var _ domain.DocumentReader = (*GeminiProvider)(nil)

// GetEmbedding generates a vector embedding via direct HTTP call to the v1beta REST API.
// The genai SDK does not expose embedContent directly, so we call the REST endpoint.
func (p *GeminiProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Parts []part `json:"parts"`
	}
	type reqBody struct {
		Content              content `json:"content"`
		OutputDimensionality int     `json:"outputDimensionality"`
	}
	type embedding struct {
		Values []float32 `json:"values"`
	}
	type respBody struct {
		Embedding embedding `json:"embedding"`
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	body, _ := json.Marshal(reqBody{
		Content:              content{Parts: []part{{Text: text}}},
		OutputDimensionality: 768,
	})

	if p.apiKey == "" {
		return nil, errors.New("Gemini API key is missing")
	}

	// gemini-embedding-2-preview stopped serving traffic in 2026-Q1 —
	// requests return 404, cascading to silent RAG misses in production.
	// gemini-embedding-001 is the GA replacement with the same 768-dim
	// output, so it slots in without re-indexing.
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=%s",
		p.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result respBody
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("embedding: failed to parse response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s", result.Error.Message)
	}

	return result.Embedding.Values, nil
}

// Close gracefully closes the genai client connection.
func (p *GeminiProvider) Close() error {
	return p.client.Close()
}

// toGenaiSchema converts a domain.AITool.Parameters value into the
// *genai.Schema the SDK expects. AITool.Parameters is typed as `any`
// so callers can emit a JSON-Schema-shaped map (the shape OpenAI
// expects) without importing the genai package — converting here keeps
// the agent_runner provider-agnostic. Accepts either a ready-made
// *genai.Schema (no-op) or a map[string]interface{} representing a
// JSON Schema object/array/primitive node.
func toGenaiSchema(v interface{}) *genai.Schema {
	if v == nil {
		return nil
	}
	if s, ok := v.(*genai.Schema); ok {
		return s
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return mapToGenaiSchema(m)
}

func mapToGenaiSchema(m map[string]interface{}) *genai.Schema {
	s := &genai.Schema{}

	if t, ok := m["type"].(string); ok {
		s.Type = jsonTypeToGenai(t)
	}
	if f, ok := m["format"].(string); ok {
		s.Format = f
	}
	if d, ok := m["description"].(string); ok {
		s.Description = d
	}
	if n, ok := m["nullable"].(bool); ok {
		s.Nullable = n
	}

	if enum, ok := m["enum"].([]interface{}); ok {
		for _, e := range enum {
			if str, ok := e.(string); ok {
				s.Enum = append(s.Enum, str)
			}
		}
	} else if enum, ok := m["enum"].([]string); ok {
		s.Enum = append(s.Enum, enum...)
	}

	if req, ok := m["required"].([]interface{}); ok {
		for _, r := range req {
			if str, ok := r.(string); ok {
				s.Required = append(s.Required, str)
			}
		}
	} else if req, ok := m["required"].([]string); ok {
		s.Required = append(s.Required, req...)
	}

	if items, ok := m["items"].(map[string]interface{}); ok {
		s.Items = mapToGenaiSchema(items)
	}

	if props, ok := m["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*genai.Schema, len(props))
		for name, raw := range props {
			if child, ok := raw.(map[string]interface{}); ok {
				s.Properties[name] = mapToGenaiSchema(child)
			}
		}
	}

	return s
}

func jsonTypeToGenai(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeUnspecified
	}
}

// splitGeminiParts separates a candidate's parts into text and tool calls.
//
// Extracted so it can be tested without the network, because the bug it
// fixes was invisible: only text was collected, so a response consisting
// of a FunctionCall came back EMPTY — and every consumer of ToolCalls
// (booking, invisible lead capture, buy-intent, funnel webhooks, the
// copilot) silently did nothing on the platform's default provider.
func splitGeminiParts(parts []genai.Part) (string, []domain.ToolCall) {
	var text string
	var calls []domain.ToolCall
	for i, part := range parts {
		switch p := part.(type) {
		case genai.Text:
			text += string(p)
		case genai.FunctionCall:
			args := "{}"
			if len(p.Args) > 0 {
				if raw, err := json.Marshal(p.Args); err == nil {
					args = string(raw)
				}
			}
			calls = append(calls, domain.ToolCall{
				Index: i,
				// Gemini issues no call ids; callers only need something
				// stable within the turn to correlate on.
				ID:           fmt.Sprintf("%s-%d", p.Name, i),
				FunctionName: p.Name,
				Arguments:    args,
			})
		}
	}
	return text, calls
}

// geminiUsage converts the SDK's counters into (prompt, completion) tokens
// as the PROVIDER BILLS THEM.
//
// CandidatesTokenCount is only the text handed back. Reasoning models spend
// tokens thinking, Google charges for those as output, and they land in
// neither counter — only in the difference against TotalTokenCount. Measured
// on a real approach draft: prompt 729, candidates 46, total 1875. Recording
// 46 misses 1100 tokens, 59% of that call.
//
// The gap is not an accounting curiosity: token allowances, plan limits and
// margin are all computed from this number, so under-counting it prices the
// product against spend that does not exist. When the total is present and
// larger than the parts, the remainder IS billed output.
func geminiUsage(m *genai.UsageMetadata) (prompt, completion int64) {
	if m == nil {
		return 0, 0
	}
	prompt = int64(m.PromptTokenCount)
	completion = int64(m.CandidatesTokenCount)
	if billed := int64(m.TotalTokenCount) - prompt; billed > completion {
		completion = billed
	}
	return prompt, completion
}

// geminiCachedTokens is how much of the prompt came from the provider's prefix
// cache (K3).
//
// Reported so the saving can be MEASURED instead of assumed: the whole of K3 is
// a claim about ordering, and a claim about cost with no number behind it is the
// kind this codebase refuses everywhere else.
func geminiCachedTokens(m *genai.UsageMetadata) int64 {
	if m == nil {
		return 0
	}
	return int64(m.CachedContentTokenCount)
}
