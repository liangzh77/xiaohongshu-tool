package keydist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type keyResponse struct {
	KeyName string `json:"keyName"`
	Value   string `json:"value"`
}

type KeyInfo struct {
	ID      int    `json:"id"`
	KeyName string `json:"keyName"`
}

type listKeysResponse struct {
	Keys []KeyInfo `json:"keys"`
}

func (c Client) Login(ctx context.Context, username, password string) (string, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return "", fmt.Errorf("key distribution base URL is required")
	}
	body, err := json.Marshal(loginRequest{Username: username, Password: password})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	respData, err := c.do(req)
	if err != nil {
		return "", err
	}
	var parsed loginResponse
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", err
	}
	if parsed.Token == "" {
		return "", fmt.Errorf("key distribution login returned empty token")
	}
	return parsed.Token, nil
}

func (c Client) GetKey(ctx context.Context, token, keyName string) (string, error) {
	if strings.TrimSpace(keyName) == "" {
		return "", fmt.Errorf("key name is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/api/keys/"+keyName, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	respData, err := c.do(req)
	if err != nil {
		return "", err
	}
	var parsed keyResponse
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return "", err
	}
	if parsed.Value == "" {
		return "", fmt.Errorf("key distribution returned empty value for %s", keyName)
	}
	return parsed.Value, nil
}

func (c Client) ListKeys(ctx context.Context, token string) ([]KeyInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/api/keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	respData, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var parsed listKeysResponse
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return nil, err
	}
	return parsed.Keys, nil
}

func (c Client) do(req *http.Request) ([]byte, error) {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("key distribution request failed: status=%d body=%s", resp.StatusCode, string(data))
	}
	return data, nil
}
