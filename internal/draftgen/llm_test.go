package draftgen

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestLLMGeneratorGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		var req llmDraftRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" || len(req.Messages) != 2 {
			t.Fatalf("unexpected request: %#v", req)
		}
		_ = json.NewEncoder(w).Encode(llmDraftResponse{
			Choices: []struct {
				Message llmDraftMessage `json:"message"`
			}{
				{Message: llmDraftMessage{Role: "assistant", Content: `{"title_options":["标题1","标题2"],"opening":"开头","body":"正文","cover_text":"封面","image_brief":"图片脚本","tags":["AI","工具"],"risk_notes":"需要审核"}`}},
			},
		})
	}))
	defer server.Close()

	draft, err := (LLMGenerator{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}).Generate(t.Context(), storage.TopicCandidate{
		ID:             7,
		Topic:          "AI工具教程",
		SuggestedAngle: "新手教程",
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.CandidateID != 7 || draft.Generator != "test-model" || draft.Opening != "开头" || len(draft.TitleOptions) != 2 {
		t.Fatalf("unexpected draft: %#v", draft)
	}
}

func TestParseDraftPayloadAcceptsJSONFence(t *testing.T) {
	payload, err := parseDraftPayload("```json\n{\"title_options\":[\"标题\"],\"opening\":\"开头\",\"body\":\"正文\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if payload.Opening != "开头" || len(payload.TitleOptions) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
