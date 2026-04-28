package scorer

import (
	"strings"

	"xiaohongshu-tool/internal/storage"
)

type RuleScorer struct {
	ModelName string
}

func NewRuleScorer() RuleScorer {
	return RuleScorer{ModelName: "rule-score-v0"}
}

func (s RuleScorer) Score(analysis storage.NoteAnalysis) storage.TopicCandidate {
	accountFit := scoreAccountFit(analysis)
	trend := scoreTrend(analysis)
	feasibility := scoreFeasibility(analysis)
	growth := scoreGrowth(analysis)
	differentiation := scoreDifferentiation(analysis)
	risk := scoreRisk(analysis)
	total := weightedTotal(accountFit, trend, feasibility, growth, differentiation, risk)

	return storage.TopicCandidate{
		AnalysisID:       analysis.ID,
		Topic:            analysis.Topic,
		AccountFitScore:  accountFit,
		TrendScore:       trend,
		FeasibilityScore: feasibility,
		GrowthScore:      growth,
		Differentiation:  differentiation,
		RiskScore:        risk,
		TotalScore:       total,
		Reason:           buildReason(analysis, accountFit, trend, feasibility, growth, differentiation, risk),
		SuggestedAngle:   buildSuggestedAngle(analysis),
		NotDoing:         buildNotDoing(analysis),
		ScoringModel:     s.ModelName,
		RawJSON: map[string]any{
			"analysis": analysis,
			"scores": map[string]int{
				"account_fit":     accountFit,
				"trend":           trend,
				"feasibility":     feasibility,
				"growth":          growth,
				"differentiation": differentiation,
				"risk":            risk,
				"total":           total,
			},
		},
	}
}

func scoreAccountFit(analysis storage.NoteAnalysis) int {
	text := joined(analysis.Topic, analysis.AudiencePain, analysis.ReusablePattern)
	score := 60
	if containsAny(text, "AI", "工具", "教程", "效率", "OpenClaw", "Claude") {
		score += 25
	}
	if containsAny(text, "美妆", "穿搭", "母婴") {
		score -= 20
	}
	return clamp(score)
}

func scoreTrend(analysis storage.NoteAnalysis) int {
	text := joined(analysis.Topic, analysis.TitleHook, analysis.ReusablePattern)
	score := 55
	if containsAny(text, "AI", "OpenClaw", "Claude", "工具") {
		score += 25
	}
	if containsAny(text, "教程", "用法", "清单") {
		score += 10
	}
	return clamp(score)
}

func scoreFeasibility(analysis storage.NoteAnalysis) int {
	score := 60
	if containsAny(analysis.ContentStructure, "步骤", "工具", "案例", "场景") {
		score += 20
	}
	if containsAny(analysis.RiskNotes, "事实核查", "夸大", "承诺") {
		score -= 10
	}
	return clamp(score)
}

func scoreGrowth(analysis storage.NoteAnalysis) int {
	text := joined(analysis.AudiencePain, analysis.EmotionalTrigger, analysis.ConversionIntent)
	score := 55
	if containsAny(text, "焦虑", "效率", "不会", "筛选", "信任") {
		score += 25
	}
	if containsAny(text, "引导互动", "关注") {
		score += 10
	}
	return clamp(score)
}

func scoreDifferentiation(analysis storage.NoteAnalysis) int {
	score := 55
	if containsAny(joined(analysis.Topic, analysis.TitleHook, analysis.OpeningHook), "顶级", "彻底", "保姆级", "一分钟") {
		score += 10
	}
	if containsAny(analysis.ReusablePattern, "经验", "教程", "场景") {
		score += 10
	}
	return clamp(score)
}

func scoreRisk(analysis storage.NoteAnalysis) int {
	score := 25
	if containsAny(analysis.RiskNotes, "暂无明显风险") {
		score -= 10
	}
	if containsAny(analysis.RiskNotes, "夸大", "承诺", "绝对化", "平台风险") {
		score += 35
	}
	return clamp(score)
}

func weightedTotal(accountFit, trend, feasibility, growth, differentiation, risk int) int {
	score := accountFit*20 + trend*20 + feasibility*20 + growth*25 + differentiation*15 - risk*15
	return clamp(score / 85)
}

func buildReason(analysis storage.NoteAnalysis, accountFit, trend, feasibility, growth, differentiation, risk int) string {
	return "账号匹配 " + itoa(accountFit) +
		"，趋势 " + itoa(trend) +
		"，可制作 " + itoa(feasibility) +
		"，涨粉潜力 " + itoa(growth) +
		"，差异化 " + itoa(differentiation) +
		"，风险 " + itoa(risk) +
		"。核心依据：" + analysis.AudiencePain + "；模式：" + analysis.ReusablePattern + "。"
}

func buildSuggestedAngle(analysis storage.NoteAnalysis) string {
	if containsAny(analysis.ReusablePattern, "教程") {
		return "围绕新手痛点做步骤化教程，强调真实上手过程和结果边界"
	}
	if containsAny(analysis.ReusablePattern, "清单") {
		return "按使用场景拆成工具清单，说明每个工具适合谁和不适合谁"
	}
	return "从具体场景切入，用案例证明观点，避免泛泛而谈"
}

func buildNotDoing(analysis storage.NoteAnalysis) string {
	if containsAny(analysis.RiskNotes, "夸大", "承诺", "绝对化") {
		return "不使用绝对化承诺，不暗示必然涨粉或必然变现"
	}
	return "不照搬原笔记表达，不堆砌泛泛的 AI 热词"
}

func clamp(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func joined(values ...string) string {
	return strings.Join(values, "\n")
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = digits[value%10]
		value /= 10
	}
	return string(buf[i:])
}
