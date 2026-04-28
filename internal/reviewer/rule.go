package reviewer

import "xiaohongshu-tool/internal/storage"

type RuleReviewer struct{}

func NewRuleReviewer() RuleReviewer {
	return RuleReviewer{}
}

func (r RuleReviewer) Review(snapshot storage.PerformanceSnapshot) storage.PerformanceReport {
	engagementBasis := rateBasis(snapshot.Likes+snapshot.Collects+snapshot.Comments, snapshot.Views)
	followBasis := rateBasis(snapshot.Follows, snapshot.Views)
	score := performanceScore(engagementBasis, followBasis)
	summary, adjustment := reviewText(score, engagementBasis, followBasis)

	return storage.PerformanceReport{
		PublishRecordID:     snapshot.PublishRecordID,
		SnapshotID:          snapshot.ID,
		PerformanceScore:    score,
		EngagementRateBasis: engagementBasis,
		FollowRateBasis:     followBasis,
		Summary:             summary,
		SuggestedAdjustment: adjustment,
		ReviewModel:         "rule-review-v0",
		RawJSON: map[string]any{
			"snapshot": snapshot,
			"rates": map[string]any{
				"engagement_basis": engagementBasis,
				"follow_basis":     followBasis,
			},
		},
	}
}

func rateBasis(numerator, denominator int) int {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	return numerator * 10000 / denominator
}

func performanceScore(engagementBasis, followBasis int) int {
	score := engagementBasis/20 + followBasis/5
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}

func reviewText(score, engagementBasis, followBasis int) (string, string) {
	switch {
	case score >= 80:
		return "表现强，选题和表达值得复用", "保留选题结构，继续测试相近角度，并记录标题和封面变量"
	case score >= 50:
		return "表现中等，有可复用信号", "保留核心选题，优先优化标题钩子、封面信息密度和开头承诺"
	case engagementBasis == 0 && followBasis == 0:
		return "暂无有效互动信号", "继续等待数据或检查发布时间、封面、标题是否影响冷启动"
	default:
		return "表现偏弱，需要谨慎复用", "降低同类选题权重，复查用户痛点是否具体、内容是否给出明确结果"
	}
}
