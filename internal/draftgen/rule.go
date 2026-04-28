package draftgen

import (
	"strings"

	"xiaohongshu-tool/internal/storage"
)

type RuleGenerator struct {
	Name string
}

func NewRuleGenerator() RuleGenerator {
	return RuleGenerator{Name: "rule-draft-v0"}
}

func (g RuleGenerator) Generate(candidate storage.TopicCandidate) storage.GeneratedDraft {
	topic := strings.TrimSpace(candidate.Topic)
	if topic == "" {
		topic = "未命名选题"
	}
	return storage.GeneratedDraft{
		CandidateID: candidate.ID,
		TitleOptions: []string{
			"保姆级拆解：" + topic,
			topic + "，新手先看这一篇",
			"我用一个真实场景讲清楚：" + topic,
			"别再乱试了，" + topic + "这样做更稳",
			"3分钟看懂：" + topic,
		},
		Opening:    buildOpening(candidate),
		Body:       buildBody(candidate),
		CoverText:  buildCoverText(topic),
		ImageBrief: buildImageBrief(candidate),
		Tags:       buildTags(candidate),
		RiskNotes:  candidate.NotDoing + "；发布前需要人工核查事实、截图素材和平台合规表达。",
		Generator:  g.Name,
		RawJSON: map[string]any{
			"candidate": candidate,
		},
	}
}

func buildOpening(candidate storage.TopicCandidate) string {
	if strings.Contains(candidate.SuggestedAngle, "新手") {
		return "如果你刚开始接触这个方向，先别急着堆工具，真正关键的是先搞清楚使用场景和边界。"
	}
	if strings.Contains(candidate.SuggestedAngle, "工具清单") {
		return "这类工具很多，但真正值得收藏的不是名字，而是它们分别适合解决什么问题。"
	}
	return "这件事看起来复杂，但拆到具体场景里，其实可以用一套更简单的方法判断。"
}

func buildBody(candidate storage.TopicCandidate) string {
	return strings.Join([]string{
		"先说结论：这个选题适合做成「场景 + 方法 + 边界」的内容，不要只做概念介绍。",
		"",
		"1. 先讲用户为什么会遇到这个问题",
		"把痛点说具体，比如不会选、不会用、怕踩坑、担心浪费时间。",
		"",
		"2. 再给一个可执行的判断框架",
		candidate.SuggestedAngle,
		"",
		"3. 最后补充不适合的情况",
		"把限制说清楚，反而更容易建立信任。",
		"",
		"这篇内容的重点不是制造焦虑，而是让用户看完之后知道下一步该怎么做。",
	}, "\n")
}

func buildCoverText(topic string) string {
	runes := []rune(topic)
	if len(runes) > 18 {
		topic = string(runes[:18])
	}
	return topic + "\n新手先看"
}

func buildImageBrief(candidate storage.TopicCandidate) string {
	return "封面突出主题和结果；正文配 3-5 张图：痛点场景、方法步骤、对比结果、注意事项、总结清单。避免使用无法核实的数据截图。"
}

func buildTags(candidate storage.TopicCandidate) []string {
	tags := []string{"小红书运营", "AI工具", "内容运营"}
	text := candidate.Topic + candidate.SuggestedAngle
	if strings.Contains(text, "OpenClaw") {
		tags = append(tags, "OpenClaw")
	}
	if strings.Contains(text, "教程") || strings.Contains(text, "新手") {
		tags = append(tags, "新手教程")
	}
	return tags
}
