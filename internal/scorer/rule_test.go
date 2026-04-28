package scorer

import (
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestRuleScorerScore(t *testing.T) {
	candidate := NewRuleScorer().Score(storage.NoteAnalysis{
		ID:               10,
		Topic:            "AI工具教程",
		AudiencePain:     "用户想快速掌握方法，但缺少清晰步骤",
		TitleHook:        "保姆级教程",
		EmotionalTrigger: "学习焦虑",
		ContentStructure: "问题-步骤-结果",
		ConversionIntent: "建立专业信任",
		ReusablePattern:  "教程型选题",
		RiskNotes:        "暂无明显风险，后续仍需人工审核事实和合规表达",
	})
	if candidate.AnalysisID != 10 {
		t.Fatalf("unexpected analysis id %d", candidate.AnalysisID)
	}
	if candidate.TotalScore <= 0 {
		t.Fatalf("expected positive total score: %#v", candidate)
	}
	if candidate.RiskScore >= 25 {
		t.Fatalf("expected low risk score: %#v", candidate)
	}
	if candidate.SuggestedAngle == "" || candidate.NotDoing == "" || candidate.Reason == "" {
		t.Fatalf("expected explanations: %#v", candidate)
	}
}
