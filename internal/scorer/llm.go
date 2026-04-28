package scorer

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

type LLMScorer struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type llmScorePayload struct {
	Topic            string `json:"topic"`
	AccountFitScore  int    `json:"account_fit_score"`
	TrendScore       int    `json:"trend_score"`
	FeasibilityScore int    `json:"feasibility_score"`
	GrowthScore      int    `json:"growth_score"`
	Differentiation  int    `json:"differentiation_score"`
	RiskScore        int    `json:"risk_score"`
	TotalScore       int    `json:"total_score"`
	Reason           string `json:"reason"`
	SuggestedAngle   string `json:"suggested_angle"`
	NotDoing         string `json:"not_doing"`
}

func (s LLMScorer) Score(ctx context.Context, analysis storage.NoteAnalysis) (storage.TopicCandidate, error) {
	if s.APIKey == "" {
		return storage.TopicCandidate{}, fmt.Errorf("llm api key is required")
	}
	if s.Model == "" {
		return storage.TopicCandidate{}, fmt.Errorf("llm model is required")
	}
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}

	reqBody := llmScoreRequest{
		Model: s.Model,
		Messages: []llmScoreMessage{
			{Role: "system", Content: scoreSystemPrompt},
			{Role: "user", Content: buildScoreUserPrompt(analysis)},
		},
		Temperature:    0.2,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return storage.TopicCandidate{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return storage.TopicCandidate{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return storage.TopicCandidate{}, err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return storage.TopicCandidate{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return storage.TopicCandidate{}, fmt.Errorf("llm request failed: status=%d body=%s", resp.StatusCode, string(respData))
	}

	var completion llmScoreResponse
	if err := json.Unmarshal(respData, &completion); err != nil {
		return storage.TopicCandidate{}, err
	}
	if len(completion.Choices) == 0 {
		return storage.TopicCandidate{}, fmt.Errorf("llm response has no choices")
	}
	payload, err := parseScorePayload(completion.Choices[0].Message.Content)
	if err != nil {
		return storage.TopicCandidate{}, err
	}

	return storage.TopicCandidate{
		AnalysisID:       analysis.ID,
		Topic:            payload.Topic,
		AccountFitScore:  clamp(payload.AccountFitScore),
		TrendScore:       clamp(payload.TrendScore),
		FeasibilityScore: clamp(payload.FeasibilityScore),
		GrowthScore:      clamp(payload.GrowthScore),
		Differentiation:  clamp(payload.Differentiation),
		RiskScore:        clamp(payload.RiskScore),
		TotalScore:       clamp(payload.TotalScore),
		Reason:           payload.Reason,
		SuggestedAngle:   payload.SuggestedAngle,
		NotDoing:         payload.NotDoing,
		ScoringModel:     s.Model,
		RawJSON: map[string]any{
			"source":   "llm",
			"model":    s.Model,
			"response": completion,
			"parsed":   payload,
			"analysis": analysis,
		},
	}, nil
}

func parseScorePayload(content string) (llmScorePayload, error) {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	var payload llmScorePayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return llmScorePayload{}, err
	}
	if payload.Topic == "" {
		return llmScorePayload{}, fmt.Errorf("llm score missing topic")
	}
	return payload, nil
}

func buildScoreUserPrompt(analysis storage.NoteAnalysis) string {
	return fmt.Sprintf(`请基于以下笔记拆解结果做小红书选题评分，输出严格 JSON。

选题：%s
用户痛点：%s
标题钩子：%s
开头钩子：%s
情绪触发：%s
内容结构：%s
转化意图：%s
可复用模式：%s
风险提示：%s

只输出 JSON，不要解释。`, analysis.Topic, analysis.AudiencePain, analysis.TitleHook, analysis.OpeningHook, analysis.EmotionalTrigger, analysis.ContentStructure, analysis.ConversionIntent, analysis.ReusablePattern, analysis.RiskNotes)
}

const scoreSystemPrompt = `你是小红书内容增长负责人。你的任务是把拆解结果转成可执行选题评分，不是生成文案。

输出必须是 JSON object，字段如下：
- topic: 候选选题
- account_fit_score: 账号匹配度，0-100
- trend_score: 趋势信号，0-100
- feasibility_score: 内容生产可行性，0-100
- growth_score: 涨粉潜力，0-100
- differentiation_score: 差异化空间，0-100
- risk_score: 合规和同质化风险，0-100，越高风险越大
- total_score: 综合分，0-100
- reason: 评分理由，说明主要依据
- suggested_angle: 推荐切入角度
- not_doing: 明确不要做什么

要求：
- 风险高时必须降低 total_score
- 不因标题夸张就给高分
- 重点考虑能否帮助目标用户快速判断、收藏或关注`

type llmScoreRequest struct {
	Model          string            `json:"model"`
	Messages       []llmScoreMessage `json:"messages"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type llmScoreMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmScoreResponse struct {
	Choices []struct {
		Message llmScoreMessage `json:"message"`
	} `json:"choices"`
}
