package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Gemini Batch API (custo-BYOK): 50% of the standard price, target
// turnaround inside 24h — free money for flows that already run at 3am and
// are read at 8am. Direct REST like the embedding call (the genai SDK pins
// its host, so it cannot be pointed at an httptest server); wire format per
// ai.google.dev/gemini-api/docs/batch-api, inline requests (20MB cap —
// a night of drafts is a few hundred KB).

// geminiBatchBaseURL is a package var so same-package tests re-point it.
var geminiBatchBaseURL = "https://generativelanguage.googleapis.com/v1beta"

var geminiBatchHTTP = &http.Client{Timeout: 60 * time.Second}

// SubmitBatch implements domain.AIBatchProvider.
func (p *GeminiProvider) SubmitBatch(ctx context.Context, displayName string, reqs []domain.AIBatchRequest) (string, error) {
	if len(reqs) == 0 {
		return "", fmt.Errorf("gemini batch: empty request set")
	}
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role,omitempty"`
		Parts []part `json:"parts"`
	}
	type inlineReq struct {
		Request struct {
			SystemInstruction *content  `json:"system_instruction,omitempty"`
			Contents          []content `json:"contents"`
		} `json:"request"`
		Metadata map[string]string `json:"metadata"`
	}

	inline := make([]inlineReq, 0, len(reqs))
	for _, r := range reqs {
		var ir inlineReq
		if s := strings.TrimSpace(r.SystemPrompt); s != "" {
			ir.Request.SystemInstruction = &content{Parts: []part{{Text: s}}}
		}
		ir.Request.Contents = []content{{Role: "user", Parts: []part{{Text: r.UserContent}}}}
		ir.Metadata = map[string]string{"key": r.Key}
		inline = append(inline, ir)
	}

	body := map[string]any{
		"batch": map[string]any{
			"display_name": displayName,
			"input_config": map[string]any{
				"requests": map[string]any{"requests": inline},
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/models/%s:batchGenerateContent", geminiBatchBaseURL, p.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := geminiBatchHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini batch submit: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini batch submit: status %d: %s", resp.StatusCode, truncateForLog(payload))
	}
	var out struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || out.Name == "" {
		return "", fmt.Errorf("gemini batch submit: no job name in answer")
	}
	return out.Name, nil
}

// FetchBatch implements domain.AIBatchProvider.
func (p *GeminiProvider) FetchBatch(ctx context.Context, jobRef string) (bool, []domain.AIBatchResult, error) {
	url := fmt.Sprintf("%s/%s", geminiBatchBaseURL, jobRef)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, nil, err
	}
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := geminiBatchHTTP.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("gemini batch fetch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("gemini batch fetch: status %d: %s", resp.StatusCode, truncateForLog(payload))
	}

	var out struct {
		// The job resource nests state under metadata on the operation
		// shape and at the top on the batch shape — read both.
		State    string `json:"state"`
		Metadata struct {
			State string `json:"state"`
		} `json:"metadata"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Dest struct {
			InlinedResponses []geminiInlined `json:"inlinedResponses"`
		} `json:"dest"`
		Response struct {
			InlinedResponses struct {
				InlinedResponses []geminiInlined `json:"inlinedResponses"`
			} `json:"inlinedResponses"`
		} `json:"response"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return false, nil, fmt.Errorf("gemini batch fetch: %w", err)
	}
	state := out.State
	if state == "" {
		state = out.Metadata.State
	}
	switch state {
	case "JOB_STATE_PENDING", "JOB_STATE_RUNNING", "JOB_STATE_QUEUED", "":
		return false, nil, nil
	case "JOB_STATE_FAILED", "JOB_STATE_CANCELLED", "JOB_STATE_EXPIRED":
		msg := state
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		return true, nil, fmt.Errorf("gemini batch job failed: %s", msg)
	}

	inlined := out.Dest.InlinedResponses
	if len(inlined) == 0 {
		inlined = out.Response.InlinedResponses.InlinedResponses
	}
	results := make([]domain.AIBatchResult, 0, len(inlined))
	for _, item := range inlined {
		results = append(results, item.toResult())
	}
	return true, results, nil
}

// geminiInlined is one answer inside a finished job.
type geminiInlined struct {
	Metadata map[string]string `json:"metadata"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error"`
	Response *struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	} `json:"response"`
}

func (g geminiInlined) toResult() domain.AIBatchResult {
	res := domain.AIBatchResult{Key: g.Metadata["key"]}
	if g.Error != nil {
		res.Err = g.Error.Message
		return res
	}
	if g.Response == nil {
		res.Err = "empty response"
		return res
	}
	var sb strings.Builder
	for _, c := range g.Response.Candidates {
		for _, p := range c.Content.Parts {
			sb.WriteString(p.Text)
		}
		break // first candidate only, same as the sync path
	}
	res.Text = strings.TrimSpace(sb.String())
	// Same billing rule as geminiUsage: reasoning models spend thought
	// tokens that land only in the total, and Google bills them as output.
	prompt := int64(g.Response.UsageMetadata.PromptTokenCount)
	completion := int64(g.Response.UsageMetadata.CandidatesTokenCount)
	if billed := int64(g.Response.UsageMetadata.TotalTokenCount) - prompt; billed > completion {
		completion = billed
	}
	res.Usage = domain.AIUsage{PromptTokens: prompt, CompletionTokens: completion}
	if res.Text == "" {
		res.Err = "empty response"
	}
	return res
}

func truncateForLog(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
