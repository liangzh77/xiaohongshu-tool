package scorer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestLLMScorerScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		var req llmScoreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" || len(req.Messages) != 2 {
			t.Fatalf("unexpected request: %#v", req)
		}
		_ = json.NewEncoder(w).Encode(llmScoreResponse{
			Choices: []struct {
				Message llmScoreMessage `json:"message"`
			}{
				{Message: llmScoreMessage{Role: "assistant", Content: `{"topic":"AI工具教程","account_fit_score":85,"trend_score":80,"feasibility_score":90,"growth_score":88,"differentiation_score":70,"risk_score":20,"total_score":86,"reason":"适合账号定位","suggested_angle":"做新手教程","not_doing":"不夸大效果"}`}},
			},
		})
	}))
	defer server.Close()

	candidate, err := (LLMScorer{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}).Score(t.Context(), storage.NoteAnalysis{
		ID:              9,
		Topic:           "AI工具教程",
		AudiencePain:    "不会上手",
		ReusablePattern: "教程型选题",
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidate.AnalysisID != 9 || candidate.ScoringModel != "test-model" || candidate.TotalScore != 86 {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
}

func TestParseScorePayloadAcceptsJSONFence(t *testing.T) {
	payload, err := parseScorePayload("```json\n{\"topic\":\"选题\",\"total_score\":70}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if payload.Topic != "选题" || payload.TotalScore != 70 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
