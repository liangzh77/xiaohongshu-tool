package analyzer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestLLMAnalyzerAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" || len(req.Messages) != 2 {
			t.Fatalf("unexpected request: %#v", req)
		}
		_ = json.NewEncoder(w).Encode(chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: `{"topic":"AI工具教程","audience_pain":"不会上手","title_hook":"保姆级教程","opening_hook":"先给结论","emotional_trigger":"学习焦虑","content_structure":"问题-步骤-结果","conversion_intent":"建立信任","reusable_pattern":"教程型选题","risk_notes":"需要事实核查"}`}},
			},
		})
	}))
	defer server.Close()

	analysis, err := (LLMAnalyzer{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}).Analyze(t.Context(), storage.StoredItem{
		ID: 1,
		Item: storage.Item{
			Title: "保姆级教程",
			Body:  "正文",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Topic != "AI工具教程" || analysis.ModelName != "test-model" {
		t.Fatalf("unexpected analysis: %#v", analysis)
	}
}

func TestParseAnalysisPayloadAcceptsJSONFence(t *testing.T) {
	payload, err := parseAnalysisPayload("```json\n{\"topic\":\"选题\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if payload.Topic != "选题" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
