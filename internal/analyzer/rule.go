package analyzer

import (
	"strings"

	"xiaohongshu-tool/internal/storage"
)

type RuleAnalyzer struct {
	ModelName string
}

func NewRuleAnalyzer() RuleAnalyzer {
	return RuleAnalyzer{ModelName: "rule-v0"}
}

func (a RuleAnalyzer) Analyze(item storage.StoredItem) storage.NoteAnalysis {
	text := strings.TrimSpace(item.Title + "\n" + item.Body)
	return storage.NoteAnalysis{
		ItemID:           item.ID,
		Topic:            item.Title,
		AudiencePain:     inferAudiencePain(text),
		TitleHook:        inferTitleHook(item.Title),
		OpeningHook:      firstSentence(item.Body),
		EmotionalTrigger: inferEmotionalTrigger(text),
		ContentStructure: inferStructure(text),
		ConversionIntent: inferConversionIntent(text),
		ReusablePattern:  inferReusablePattern(item.Title),
		RiskNotes:        inferRiskNotes(text),
		ModelName:        a.ModelName,
		RawJSON: map[string]any{
			"title": item.Title,
			"body":  item.Body,
			"tags":  item.Tags,
		},
	}
}

func inferAudiencePain(text string) string {
	switch {
	case containsAny(text, "教程", "保姆级", "搞懂", "怎么", "如何"):
		return "用户想快速掌握方法，但缺少清晰步骤"
	case containsAny(text, "推荐", "工具", "清单"):
		return "用户想快速筛选可用工具，避免试错"
	case containsAny(text, "避坑", "不要", "错误"):
		return "用户担心踩坑或浪费时间"
	default:
		return "用户对该主题有兴趣，但缺少明确判断依据"
	}
}

func inferTitleHook(title string) string {
	switch {
	case containsAny(title, "保姆级"):
		return "保姆级教程"
	case containsAny(title, "彻底", "搞懂"):
		return "彻底搞懂"
	case containsAny(title, "推荐", "清单"):
		return "推荐清单"
	case containsAny(title, "一分钟", "快速"):
		return "快速见效"
	default:
		return "主题直给"
	}
}

func inferEmotionalTrigger(text string) string {
	switch {
	case containsAny(text, "焦虑", "不会", "搞懂", "保姆级"):
		return "学习焦虑"
	case containsAny(text, "效率", "一分钟", "自动"):
		return "效率提升"
	case containsAny(text, "避坑", "错误"):
		return "损失规避"
	default:
		return "好奇心"
	}
}

func inferStructure(text string) string {
	switch {
	case containsAny(text, "教程", "步骤", "保姆级"):
		return "问题-步骤-结果"
	case containsAny(text, "推荐", "清单", "用法"):
		return "场景-工具-理由"
	default:
		return "观点-解释-案例"
	}
}

func inferConversionIntent(text string) string {
	if containsAny(text, "关注", "私信", "领取", "评论") {
		return "引导互动或关注"
	}
	return "建立专业信任"
}

func inferReusablePattern(title string) string {
	switch {
	case containsAny(title, "保姆级", "教程"):
		return "教程型选题"
	case containsAny(title, "推荐", "清单"):
		return "清单型选题"
	case containsAny(title, "用法"):
		return "场景用法型选题"
	default:
		return "经验分享型选题"
	}
}

func inferRiskNotes(text string) string {
	if containsAny(text, "最强", "第一", "必爆", "保证") {
		return "注意避免绝对化承诺"
	}
	return "暂无明显风险，后续仍需人工审核事实和合规表达"
}

func firstSentence(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	for _, sep := range []string{"。", "！", "？", "\n"} {
		if idx := strings.Index(body, sep); idx >= 0 {
			return strings.TrimSpace(body[:idx+len(sep)])
		}
	}
	if len([]rune(body)) > 60 {
		return string([]rune(body)[:60])
	}
	return body
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
