package draftgen

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

type LLMGenerator struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type llmDraftPayload struct {
	TitleOptions []string `json:"title_options"`
	Opening      string   `json:"opening"`
	Body         string   `json:"body"`
	CoverText    string   `json:"cover_text"`
	ImageBrief   string   `json:"image_brief"`
	Tags         []string `json:"tags"`
	RiskNotes    string   `json:"risk_notes"`
}

func (g LLMGenerator) Generate(ctx context.Context, candidate storage.TopicCandidate) (storage.GeneratedDraft, error) {
	if g.APIKey == "" {
		return storage.GeneratedDraft{}, fmt.Errorf("llm api key is required")
	}
	if g.Model == "" {
		return storage.GeneratedDraft{}, fmt.Errorf("llm model is required")
	}
	baseURL := strings.TrimRight(g.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	client := g.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}

	reqBody := llmDraftRequest{
		Model: g.Model,
		Messages: []llmDraftMessage{
			{Role: "system", Content: draftSystemPrompt},
			{Role: "user", Content: buildDraftUserPrompt(candidate)},
		},
		Temperature:    0.7,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return storage.GeneratedDraft{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return storage.GeneratedDraft{}, err
	}
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return storage.GeneratedDraft{}, err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return storage.GeneratedDraft{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return storage.GeneratedDraft{}, fmt.Errorf("llm request failed: status=%d body=%s", resp.StatusCode, string(respData))
	}

	var completion llmDraftResponse
	if err := json.Unmarshal(respData, &completion); err != nil {
		return storage.GeneratedDraft{}, err
	}
	if len(completion.Choices) == 0 {
		return storage.GeneratedDraft{}, fmt.Errorf("llm response has no choices")
	}
	payload, err := parseDraftPayload(completion.Choices[0].Message.Content)
	if err != nil {
		return storage.GeneratedDraft{}, err
	}

	return storage.GeneratedDraft{
		CandidateID:  candidate.ID,
		TitleOptions: payload.TitleOptions,
		Opening:      payload.Opening,
		Body:         payload.Body,
		CoverText:    payload.CoverText,
		ImageBrief:   payload.ImageBrief,
		Tags:         payload.Tags,
		RiskNotes:    payload.RiskNotes,
		Generator:    g.Model,
		RawJSON: map[string]any{
			"source":    "llm",
			"model":     g.Model,
			"response":  completion,
			"parsed":    payload,
			"candidate": candidate,
		},
	}, nil
}

func parseDraftPayload(content string) (llmDraftPayload, error) {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	var payload llmDraftPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return llmDraftPayload{}, err
	}
	if len(payload.TitleOptions) == 0 {
		return llmDraftPayload{}, fmt.Errorf("llm draft missing title_options")
	}
	if strings.TrimSpace(payload.Opening) == "" {
		return llmDraftPayload{}, fmt.Errorf("llm draft missing opening")
	}
	if strings.TrimSpace(payload.Body) == "" {
		return llmDraftPayload{}, fmt.Errorf("llm draft missing body")
	}
	return payload, nil
}

func buildDraftUserPrompt(candidate storage.TopicCandidate) string {
	return fmt.Sprintf(`请基于以下候选选题生成小红书内容草稿，输出严格 JSON。

选题：%s
总分：%d
推荐角度：%s
不做什么：%s
评分理由：%s

只输出 JSON，不要解释。`, candidate.Topic, candidate.TotalScore, candidate.SuggestedAngle, candidate.NotDoing, candidate.Reason)
}

const draftSystemPrompt = `你是资深小红书内容运营和中文文案编辑。你的任务是生成可供人工审核的原创草稿，不是复制竞品笔记。

输出必须是 JSON object，字段如下：
- title_options: 5 个中文标题备选
- opening: 1 段开头，直接进入用户痛点或结论
- body: 正文草稿，结构清晰，可以包含编号
- cover_text: 封面短文案，最多两行
- image_brief: 图片/截图脚本建议
- tags: 3-6 个标签，不带 # 号
- risk_notes: 发布前需要人工核查的事实、合规和表达风险

要求：
- 不承诺无法验证的收益
- 不使用“全网最”“必爆”等夸大表达
- 不照搬原笔记句子
- 保留人工编辑空间`

type llmDraftRequest struct {
	Model          string            `json:"model"`
	Messages       []llmDraftMessage `json:"messages"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type llmDraftMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmDraftResponse struct {
	Choices []struct {
		Message llmDraftMessage `json:"message"`
	} `json:"choices"`
}
