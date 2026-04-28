package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"xiaohongshu-tool/internal/storage"
)

type LLMAnalyzer struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type llmAnalysisPayload struct {
	Topic            string `json:"topic"`
	AudiencePain     string `json:"audience_pain"`
	TitleHook        string `json:"title_hook"`
	OpeningHook      string `json:"opening_hook"`
	EmotionalTrigger string `json:"emotional_trigger"`
	ContentStructure string `json:"content_structure"`
	ConversionIntent string `json:"conversion_intent"`
	ReusablePattern  string `json:"reusable_pattern"`
	RiskNotes        string `json:"risk_notes"`
}

func (a LLMAnalyzer) Analyze(ctx context.Context, item storage.StoredItem) (storage.NoteAnalysis, error) {
	if a.APIKey == "" {
		return storage.NoteAnalysis{}, fmt.Errorf("llm api key is required")
	}
	if a.Model == "" {
		return storage.NoteAnalysis{}, fmt.Errorf("llm model is required")
	}
	baseURL := strings.TrimRight(a.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}

	reqBody := chatCompletionRequest{
		Model: a.Model,
		Messages: []chatMessage{
			{Role: "system", Content: analysisSystemPrompt},
			{Role: "user", Content: buildAnalysisUserPrompt(item)},
		},
		Temperature:    0.2,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return storage.NoteAnalysis{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return storage.NoteAnalysis{}, err
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return storage.NoteAnalysis{}, err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return storage.NoteAnalysis{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return storage.NoteAnalysis{}, fmt.Errorf("llm request failed: status=%d body=%s", resp.StatusCode, string(respData))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(respData, &completion); err != nil {
		return storage.NoteAnalysis{}, err
	}
	if len(completion.Choices) == 0 {
		return storage.NoteAnalysis{}, fmt.Errorf("llm response has no choices")
	}
	payload, err := parseAnalysisPayload(completion.Choices[0].Message.Content)
	if err != nil {
		return storage.NoteAnalysis{}, err
	}

	return storage.NoteAnalysis{
		ItemID:           item.ID,
		Topic:            payload.Topic,
		AudiencePain:     payload.AudiencePain,
		TitleHook:        payload.TitleHook,
		OpeningHook:      payload.OpeningHook,
		EmotionalTrigger: payload.EmotionalTrigger,
		ContentStructure: payload.ContentStructure,
		ConversionIntent: payload.ConversionIntent,
		ReusablePattern:  payload.ReusablePattern,
		RiskNotes:        payload.RiskNotes,
		ModelName:        a.Model,
		RawJSON: map[string]any{
			"source":        "llm",
			"model":         a.Model,
			"response":      completion,
			"parsed":        payload,
			"input_item_id": item.ID,
		},
	}, nil
}

func parseAnalysisPayload(content string) (llmAnalysisPayload, error) {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	var payload llmAnalysisPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return llmAnalysisPayload{}, err
	}
	if payload.Topic == "" {
		return llmAnalysisPayload{}, fmt.Errorf("llm analysis missing topic")
	}
	return payload, nil
}

func buildAnalysisUserPrompt(item storage.StoredItem) string {
	body := strings.TrimSpace(item.Body)
	if body == "" {
		body = "（采集结果没有正文，仅有标题和互动数据）"
	}
	return fmt.Sprintf(`请拆解以下小红书笔记，输出严格 JSON。

标题：%s
作者：%s
正文：%s
标签：%s
点赞数：%s
收藏数：%s
评论数：%s

只输出 JSON，不要输出解释。`, item.Title, item.AuthorName, body, strings.Join(item.Tags, ","), formatIntPtr(item.LikeCount), formatIntPtr(item.CollectCount), formatIntPtr(item.CommentCount))
}

const analysisSystemPrompt = `你是资深小红书内容运营和增长分析师。你的任务是拆解笔记，不是改写或搬运。

输出必须是 JSON object，字段如下：
- topic: 这篇笔记背后的可复用选题
- audience_pain: 目标用户痛点
- title_hook: 标题钩子类型
- opening_hook: 开头钩子，如果原文不足则根据标题推断
- emotional_trigger: 情绪触发点
- content_structure: 内容结构
- conversion_intent: 转化或互动意图
- reusable_pattern: 可复用内容模式
- risk_notes: 风险提示，包括夸大、同质化、事实核查、平台风险

要求：
- 不复制原文长句
- 不生成新文案
- 不做发布建议
- 每个字段用中文短句`

type chatCompletionRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func formatIntPtr(value *int) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}
