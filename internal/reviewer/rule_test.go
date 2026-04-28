package reviewer

import (
	"testing"

	"xiaohongshu-tool/internal/storage"
)

func TestRuleReviewerScoresSnapshot(t *testing.T) {
	reviewer := NewRuleReviewer()
	report := reviewer.Review(storage.PerformanceSnapshot{
		ID:              10,
		PublishRecordID: 20,
		Views:           1000,
		Likes:           80,
		Collects:        30,
		Comments:        10,
		Follows:         5,
	})

	if report.SnapshotID != 10 || report.PublishRecordID != 20 {
		t.Fatalf("unexpected identity fields: %#v", report)
	}
	if report.EngagementRateBasis != 1200 {
		t.Fatalf("expected engagement basis 1200, got %d", report.EngagementRateBasis)
	}
	if report.FollowRateBasis != 50 {
		t.Fatalf("expected follow basis 50, got %d", report.FollowRateBasis)
	}
	if report.PerformanceScore <= 0 || report.PerformanceScore > 100 {
		t.Fatalf("expected score in 1..100, got %d", report.PerformanceScore)
	}
	if report.Summary == "" || report.SuggestedAdjustment == "" || report.ReviewModel != "rule-review-v0" {
		t.Fatalf("unexpected report text: %#v", report)
	}
}

func TestRuleReviewerHandlesZeroViews(t *testing.T) {
	reviewer := NewRuleReviewer()
	report := reviewer.Review(storage.PerformanceSnapshot{
		ID:              1,
		PublishRecordID: 2,
		Likes:           10,
	})

	if report.EngagementRateBasis != 0 || report.FollowRateBasis != 0 || report.PerformanceScore != 0 {
		t.Fatalf("expected zero rates and score, got %#v", report)
	}
	if report.Summary == "" {
		t.Fatalf("expected summary for zero-view snapshot")
	}
}
