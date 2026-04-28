package analyzer

import (
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestRuleAnalyzerAnalyzeTutorial(t *testing.T) {
	analysis := NewRuleAnalyzer().Analyze(storage.StoredItem{
		ID: 123,
		Item: storage.Item{
			Title: "保姆级教程！彻底搞懂Claude Code Skill！",
			Body:  "先给结论。然后分步骤讲清楚。",
		},
	})
	if analysis.ItemID != 123 {
		t.Fatalf("unexpected item id %d", analysis.ItemID)
	}
	if analysis.TitleHook != "保姆级教程" {
		t.Fatalf("unexpected title hook %q", analysis.TitleHook)
	}
	if analysis.ContentStructure != "问题-步骤-结果" {
		t.Fatalf("unexpected structure %q", analysis.ContentStructure)
	}
	if analysis.OpeningHook != "先给结论。" {
		t.Fatalf("unexpected opening hook %q", analysis.OpeningHook)
	}
}
