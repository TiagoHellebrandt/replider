package ai

import (
	"context"
	"fmt"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAIImageDescriber uses gpt-5.4-mini's vision capability to describe
// an image URL in one short paragraph. Kept separate from OpenAIProvider
// so the call site doesn't accidentally pay for a full chat completion
// round-trip just to turn a photo into text.
type OpenAIImageDescriber struct {
	client *openai.Client
	model  string
}

func NewOpenAIImageDescriber(apiKey string) *OpenAIImageDescriber {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIImageDescriber{
		client: &c,
		// gpt-5.4-mini has vision in and is ~2x faster than gpt-5-mini.
		// Overkill-cheap for a single-turn image-to-caption task.
		model: "gpt-5.4-mini",
	}
}

// Describe returns the caption. languageHint defaults to Portuguese since
// this is the primary target market.
func (d *OpenAIImageDescriber) Describe(ctx context.Context, imageURL, languageHint string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("empty image URL")
	}
	if languageHint == "" {
		languageHint = "pt"
	}
	instruction := fmt.Sprintf(
		"Descreva objetivamente em uma frase curta o que aparece nesta imagem (idioma: %s). "+
			"Foque em produtos, textos visíveis, quantidades e qualquer informação útil para atendimento ao cliente. "+
			"Não comente estilo ou composição. Retorne apenas a descrição.",
		languageHint,
	)
	resp, err := d.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(d.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				openai.TextContentPart(instruction),
				openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: imageURL}),
			}),
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from vision model")
	}
	return resp.Choices[0].Message.Content, nil
}
