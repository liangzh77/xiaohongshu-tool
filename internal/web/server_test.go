package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"xiaohongshu-tool/internal/storage"
)

func TestServerStateAndWorkflowActions(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, storage.Target{Kind: "keyword", Name: "AI", Keyword: "AI工具", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveItems(ctx, targetID, []storage.Item{{ExternalID: "note-1", Title: "保姆级 AI 工具教程", Body: "正文"}}, time.Now()); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewServer(Config{DB: db, DefaultLimit: 10}).Handler())
	defer srv.Close()

	postJSON(t, srv.URL+"/api/analyze/batch", map[string]any{"engine": "rule", "limit": 10})
	postJSON(t, srv.URL+"/api/score/batch", map[string]any{"engine": "rule", "limit": 10})
	postJSON(t, srv.URL+"/api/draft/batch", map[string]any{"engine": "rule", "limit": 10})

	resp, err := http.Get(srv.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
	var state StateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.Items) != 1 || len(state.Analyses) != 1 || len(state.Candidates) != 1 || len(state.Drafts) != 1 {
		t.Fatalf("unexpected state counts: %#v", state)
	}
}

func TestServerAddTarget(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewServer(Config{DB: db}).Handler())
	defer srv.Close()

	postJSON(t, srv.URL+"/api/targets", map[string]any{
		"kind":     "keyword",
		"name":     "AI工具",
		"keyword":  "AI工具",
		"interval": 300,
	})

	targets, err := db.ListTargets(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Keyword != "AI工具" {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestServerUpdateAndDeleteTarget(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := db.AddTarget(ctx, storage.Target{Kind: "keyword", Name: "AI", Keyword: "AI工具", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(Config{DB: db}).Handler())
	defer srv.Close()

	postJSON(t, srv.URL+"/api/targets", map[string]any{
		"id":       id,
		"kind":     "keyword",
		"name":     "AI效率工具",
		"keyword":  "AI效率",
		"interval": 600,
	})
	targets, err := db.ListTargets(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Name != "AI效率工具" || targets[0].Keyword != "AI效率" {
		t.Fatalf("unexpected updated targets: %#v", targets)
	}

	postJSON(t, srv.URL+"/api/targets/delete", map[string]any{"id": id})
	targets, err = db.ListTargets(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected target to be hidden after delete, got %#v", targets)
	}
}

func TestServerKeyConfigTest(t *testing.T) {
	keyService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "token-1"})
		case "/api/keys":
			if r.Header.Get("Authorization") != "Bearer token-1" {
				t.Fatalf("unexpected auth header %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{
				{"id": 1, "keyName": "OPENAI_API_KEY"},
				{"id": 2, "keyName": "GEMINI_API_KEY"},
			}})
		case "/api/keys/OPENAI_API_KEY":
			if r.Header.Get("Authorization") != "Bearer token-1" {
				t.Fatalf("unexpected auth header %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"keyName": "OPENAI_API_KEY", "value": "sk-test"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer keyService.Close()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(Config{DB: db}).Handler())
	defer srv.Close()

	postJSON(t, srv.URL+"/api/key-config/test", map[string]any{
		"base_url": keyService.URL,
		"username": "user",
		"password": "pass",
	})

	resp, err := http.Get(srv.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var state StateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if !state.Config.KeyDistConfigured || state.Config.KeyDistBaseURL != keyService.URL {
		t.Fatalf("expected saved key config, got %#v", state.Config)
	}
	if len(state.Config.AvailableKeys) != 2 || state.Config.AvailableKeys[1] != "GEMINI_API_KEY" {
		t.Fatalf("expected available keys, got %#v", state.Config.AvailableKeys)
	}
	if state.Config.KeyName != "OPENAI_API_KEY" {
		t.Fatalf("unexpected key name %q", state.Config.KeyName)
	}
}

func postJSON(t *testing.T, url string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("unexpected status %d for %s", resp.StatusCode, url)
	}
}
