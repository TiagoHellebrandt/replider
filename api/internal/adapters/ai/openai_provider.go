package ai

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/replider/api/internal/core/domain"
)

type OpenAIProvider struct {
	client *openai.Client
	apiKey string
	model  string
}

func NewOpenAIProvider(apiKey string, modelName string) *OpenAIProvider {
	if modelName == "" {
		modelName = "gpt-5.4-mini"
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &OpenAIProvider{
		client: &client,
		apiKey: apiKey,
		model:  modelName,
	}
}

func (p *OpenAIProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	if p.apiKey == "" {
		return domain.StreamResult{}, errors.New("OpenAI API key is missing")
	}

	messages := p.buildMessages(systemPrompt, history)

	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: messages,
	}

	if len(tools) > 0 {
		params.Tools = p.buildTools(tools)
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	out := make(chan string)
	var pTokens, cTokens int64

	go func() {
		defer close(out)
		for stream.Next() {
			chunk := stream.Current()

			if chunk.Usage.JSON.PromptTokens.Valid() {
				pTokens = chunk.Usage.PromptTokens
				cTokens = chunk.Usage.CompletionTokens
			}

			for _, choice := range chunk.Choices {
				pieces := make([]string, 0, 2)
				if choice.Delta.Content != "" {
					pieces = append(pieces, choice.Delta.Content)
				}
				for _, tc := range choice.Delta.ToolCalls {
					if tc.Function.Name != "" {
						pieces = append(pieces, "[FUNCTION_CALL]")
					}
				}
				for _, piece := range pieces {
					// Honour caller cancellation — without this the
					// goroutine blocks on `out` forever (leak) once the
					// consumer stops reading.
					select {
					case out <- piece:
					case <-ctx.Done():
						slog.Warn("openai stream abandoned (context cancelled)", "model", p.model)
						return
					}
				}
			}
		}
		// stream.Next() returns false on EOF *and* on error — surface
		// the error so a provider failure mid-stream is not silent.
		if err := stream.Err(); err != nil {
			slog.Error("openai stream ended with error", "model", p.model, "error", err)
		}
	}()

	return domain.StreamResult{
		Chunks: out,
		GetUsage: func() (int64, int64) {
			return pTokens, cTokens
		},
	}, nil
}

func (p *OpenAIProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	if p.apiKey == "" {
		return nil, errors.New("OpenAI API key is missing")
	}

	messages := p.buildMessages(systemPrompt, history)

	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: messages,
	}

	if len(tools) > 0 {
		params.Tools = p.buildTools(tools)
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}

	var content string
	var toolCalls []domain.ToolCall

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		content = choice.Message.Content

		for i, tc := range choice.Message.ToolCalls {
			toolCalls = append(toolCalls, domain.ToolCall{
				Index:        i,
				ID:           tc.ID,
				FunctionName: tc.Function.Name,
				Arguments:    tc.Function.Arguments,
			})
		}
	}

	return &domain.AICallbackResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Usage: domain.AIUsage{
			PromptTokens:     int64(resp.Usage.PromptTokens),
			CompletionTokens: int64(resp.Usage.CompletionTokens),
		},
	}, nil
}

func (p *OpenAIProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if p.apiKey == "" {
		return nil, errors.New("OpenAI API key is missing")
	}

	resp, err := p.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: "text-embedding-3-small",
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: param.NewOpt(text),
		},
		Dimensions: param.NewOpt[int64](768),
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("no embedding returned from OpenAI")
	}

	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// maxHistoryTokens is a rough estimate of max tokens for conversation history.
// Reserves space for system prompt (~2K tokens) and response (~2K tokens).
const maxHistoryChars = 24000 // ~6K tokens at 4 chars/token, safe for most models

func (p *OpenAIProvider) buildMessages(systemPrompt string, history []domain.ChatMessage) []openai.ChatCompletionMessageParamUnion {
	var messages []openai.ChatCompletionMessageParamUnion

	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}

	// Truncate history from the beginning to fit context window, keeping latest messages
	truncated := truncateHistory(history, maxHistoryChars)

	for _, msg := range truncated {
		switch msg.Role {
		case "user":
			messages = append(messages, openai.UserMessage(msg.Content))
		case "model", "assistant":
			messages = append(messages, openai.AssistantMessage(msg.Content))
		}
	}

	return messages
}

// truncateHistory keeps the most recent messages that fit within maxChars.
func truncateHistory(history []domain.ChatMessage, maxChars int) []domain.ChatMessage {
	totalChars := 0
	for _, msg := range history {
		totalChars += len(msg.Content)
	}
	if totalChars <= maxChars {
		return history
	}
	// Drop oldest messages until we fit
	start := 0
	for start < len(history)-1 {
		totalChars -= len(history[start].Content)
		start++
		if totalChars <= maxChars {
			break
		}
	}
	return history[start:]
}

func (p *OpenAIProvider) buildTools(tools []domain.AITool) []openai.ChatCompletionToolParam {
	var result []openai.ChatCompletionToolParam

	for _, t := range tools {
		var schema shared.FunctionParameters
		if t.Parameters != nil {
			// Parameters can be a *genai.Schema or a JSON-serializable struct.
			// Marshal then unmarshal to get a generic map for OpenAI's format.
			raw, err := json.Marshal(t.Parameters)
			if err == nil {
				_ = json.Unmarshal(raw, &schema)
			}
		}

		result = append(result, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: param.NewOpt(t.Description),
				Parameters:  schema,
			},
		})
	}

	return result
}
