package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

func batchTestProvider(t *testing.T, handler http.HandlerFunc) *GeminiProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	old := geminiBatchBaseURL
	geminiBatchBaseURL = srv.URL
	t.Cleanup(func() { geminiBatchBaseURL = old })
	return &GeminiProvider{apiKey: "test-key", model: "gemini-2.5-flash"}
}

func TestGeminiBatch_SubmitSendsInlineRequestsWithKeys(t *testing.T) {
	var got struct {
		Batch struct {
			DisplayName string `json:"display_name"`
			InputConfig struct {
				Requests struct {
					Requests []struct {
						Request struct {
							SystemInstruction *struct {
								Parts []struct {
									Text string `json:"text"`
								} `json:"parts"`
							} `json:"system_instruction"`
							Contents []struct {
								Role  string `json:"role"`
								Parts []struct {
									Text string `json:"text"`
								} `json:"parts"`
							} `json:"contents"`
						} `json:"request"`
						Metadata map[string]string `json:"metadata"`
					} `json:"requests"`
				} `json:"requests"`
			} `json:"input_config"`
		} `json:"batch"`
	}
	p := batchTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-2.5-flash:batchGenerateContent" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("missing api key header")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "batches/abc123"})
	})

	ref, err := p.SubmitBatch(context.Background(), "overnight-acc1", []domain.AIBatchRequest{
		{Key: "l1", SystemPrompt: "seja consultivo", UserContent: "hospital X"},
		{Key: "l2", UserContent: "hospital Y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref != "batches/abc123" {
		t.Fatalf("ref = %q", ref)
	}
	reqs := got.Batch.InputConfig.Requests.Requests
	if len(reqs) != 2 || got.Batch.DisplayName != "overnight-acc1" {
		t.Fatalf("payload = %+v", got.Batch)
	}
	if reqs[0].Metadata["key"] != "l1" || reqs[1].Metadata["key"] != "l2" {
		t.Fatalf("metadata keys wrong: %+v", reqs)
	}
	if reqs[0].Request.SystemInstruction == nil || reqs[0].Request.SystemInstruction.Parts[0].Text != "seja consultivo" {
		t.Fatalf("system instruction lost")
	}
	if reqs[1].Request.SystemInstruction != nil {
		t.Fatalf("empty system prompt must omit system_instruction")
	}
	if reqs[0].Request.Contents[0].Role != "user" || reqs[0].Request.Contents[0].Parts[0].Text != "hospital X" {
		t.Fatalf("content wrong: %+v", reqs[0].Request.Contents)
	}
}

func TestGeminiBatch_FetchWhileRunningIsNotDone(t *testing.T) {
	p := batchTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":     "batches/abc123",
			"metadata": map[string]string{"state": "JOB_STATE_RUNNING"},
		})
	})
	done, results, err := p.FetchBatch(context.Background(), "batches/abc123")
	if err != nil || done || results != nil {
		t.Fatalf("done=%v results=%v err=%v, want still-running", done, results, err)
	}
}

func TestGeminiBatch_FetchSucceededParsesResultsAndUsage(t *testing.T) {
	p := batchTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/batches/abc123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]string{"state": "JOB_STATE_SUCCEEDED"},
			"dest": map[string]any{
				"inlinedResponses": []map[string]any{
					{
						"metadata": map[string]string{"key": "l1"},
						"response": map[string]any{
							"candidates": []map[string]any{{
								"content": map[string]any{
									"parts": []map[string]string{{"text": "Oi, "}, {"text": "hospital!"}},
								},
							}},
							// Reasoning model: 60 thought tokens live only in the
							// total and are billed as output (140 - 100 = 40 + 60).
							"usageMetadata": map[string]int{"promptTokenCount": 100, "candidatesTokenCount": 40, "totalTokenCount": 200},
						},
					},
					{
						"metadata": map[string]string{"key": "l2"},
						"error":    map[string]any{"message": "quota exceeded"},
					},
				},
			},
		})
	})

	done, results, err := p.FetchBatch(context.Background(), "batches/abc123")
	if err != nil || !done {
		t.Fatalf("done=%v err=%v", done, err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Key != "l1" || results[0].Text != "Oi, hospital!" || results[0].Err != "" {
		t.Fatalf("r0 = %+v", results[0])
	}
	if results[0].Usage.PromptTokens != 100 || results[0].Usage.CompletionTokens != 100 {
		t.Fatalf("usage = %+v, want thought tokens billed as output (total-prompt)", results[0].Usage)
	}
	if results[1].Key != "l2" || results[1].Err != "quota exceeded" {
		t.Fatalf("r1 = %+v", results[1])
	}
}

func TestGeminiBatch_FetchFailedJobIsDoneWithError(t *testing.T) {
	p := batchTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]string{"state": "JOB_STATE_FAILED"},
			"error":    map[string]string{"message": "internal error"},
		})
	})
	done, _, err := p.FetchBatch(context.Background(), "batches/abc123")
	if !done || err == nil {
		t.Fatalf("done=%v err=%v, want done with error", done, err)
	}
}

func TestGeminiBatch_SubmitNon200IsAnError(t *testing.T) {
	p := batchTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"key invalid"}}`))
	})
	_, err := p.SubmitBatch(context.Background(), "x", []domain.AIBatchRequest{{Key: "l1", UserContent: "y"}})
	if err == nil {
		t.Fatal("want error on 403")
	}
}
