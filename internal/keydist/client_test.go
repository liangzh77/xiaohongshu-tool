package keydist

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientLoginAndGetKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			var req loginRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Username != "user" || req.Password != "pass" {
				t.Fatalf("unexpected login request: %#v", req)
			}
			_ = json.NewEncoder(w).Encode(loginResponse{Token: "token-1"})
		case "/api/keys":
			if r.Header.Get("Authorization") != "Bearer token-1" {
				t.Fatalf("unexpected auth header %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(listKeysResponse{Keys: []KeyInfo{
				{ID: 1, KeyName: "OPENAI_API_KEY"},
				{ID: 2, KeyName: "GEMINI_API_KEY"},
			}})
		case "/api/keys/OPENAI_API_KEY":
			if r.Header.Get("Authorization") != "Bearer token-1" {
				t.Fatalf("unexpected auth header %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(keyResponse{KeyName: "OPENAI_API_KEY", Value: "sk-test"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	token, err := client.Login(t.Context(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	key, err := client.GetKey(t.Context(), token, "OPENAI_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-test" {
		t.Fatalf("unexpected key %q", key)
	}
	keys, err := client.ListKeys(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[1].KeyName != "GEMINI_API_KEY" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}
