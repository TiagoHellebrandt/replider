package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// SentimentAnalyzer classifies message sentiment using an AI provider.
type SentimentAnalyzer struct {
	aiFactory domain.AIFactory
	scope     accountAIScope
	meter     UsageMeter
}

// NewSentimentAnalyzer creates a new SentimentAnalyzer.
func NewSentimentAnalyzer(aiFactory domain.AIFactory) *SentimentAnalyzer {
	return &SentimentAnalyzer{aiFactory: aiFactory}
}

// WithAccounts wires BYOK resolution: the classification runs on the
// account's own key and is skipped (ErrHelperSkipped) when it has none.
func (sa *SentimentAnalyzer) WithAccounts(accounts domain.AccountRepository, plans domain.PlanRepository) *SentimentAnalyzer {
	sa.scope = accountAIScope{accounts: accounts, plans: plans}
	return sa
}

// WithMeter lands every classification in the cost ledger ("sentiment").
func (sa *SentimentAnalyzer) WithMeter(m UsageMeter) *SentimentAnalyzer {
	sa.meter = m
	return sa
}

// sentimentResponse is the expected JSON structure from the AI provider.
type sentimentResponse struct {
	Sentiment string  `json:"sentiment"`
	Score     float64 `json:"score"`
}

// validSentiments defines the allowed sentiment labels.
var validSentiments = map[string]bool{
	"positive": true,
	"neutral":  true,
	"negative": true,
	"angry":    true,
}

// Analyze returns a sentiment label ("positive", "neutral", "negative", "angry")
// and a confidence score (0.0-1.0) for the given text.
func (sa *SentimentAnalyzer) Analyze(ctx context.Context, accountID, text string) (string, float64, error) {
	provider, plan, err := sa.scope.providerFor(ctx, sa.aiFactory, "google", accountID)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		// Best-effort helper: no key of their own means no badge, not an
		// error and never the platform's key.
		return "", 0, ErrHelperSkipped
	}
	if err != nil {
		return "", 0, fmt.Errorf("sentiment: failed to get AI provider: %w", err)
	}

	systemPrompt := `Classify the sentiment of this customer message as one of: positive, neutral, negative, angry.
Respond with ONLY a JSON object: {"sentiment": "...", "score": 0.0-1.0}`

	history := []domain.ChatMessage{
		{Role: "user", Content: "Message: " + text},
	}

	resp, err := provider.GenerateResponse(ctx, systemPrompt, history, nil)
	if err != nil {
		return "", 0, fmt.Errorf("sentiment: AI generation failed: %w", err)
	}
	recordUsage(ctx, sa.meter, accountID, "sentiment", plan, resp.Usage)

	content := strings.TrimSpace(resp.Content)

	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result sentimentResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", 0, fmt.Errorf("sentiment: failed to parse AI response %q: %w", content, err)
	}

	result.Sentiment = strings.ToLower(strings.TrimSpace(result.Sentiment))

	if !validSentiments[result.Sentiment] {
		return "", 0, fmt.Errorf("sentiment: unexpected label %q", result.Sentiment)
	}

	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 1 {
		result.Score = 1
	}

	return result.Sentiment, result.Score, nil
}
