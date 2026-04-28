package draftgen

import (
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestRuleGeneratorGenerate(t *testing.T) {
	draft := NewRuleGenerator().Generate(storage.TopicCandidate{
		ID:             9,
		Topic:          "AI工具教程",
		SuggestedAngle: "围绕新手痛点做步骤化教程",
		NotDoing:       "不承诺效果",
	})
	if draft.CandidateID != 9 {
		t.Fatalf("unexpected candidate id %d", draft.CandidateID)
	}
	if len(draft.TitleOptions) != 5 {
		t.Fatalf("expected title options, got %#v", draft.TitleOptions)
	}
	if draft.Opening == "" || draft.Body == "" || draft.CoverText == "" || draft.ImageBrief == "" {
		t.Fatalf("draft missing fields: %#v", draft)
	}
	if len(draft.Tags) == 0 {
		t.Fatalf("expected tags")
	}
}
