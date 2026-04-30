package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDueTargetsHonorsMinimumInterval(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	id, err := db.AddTarget(ctx, Target{
		Kind:               "keyword",
		Name:               "AI tools",
		Keyword:            "AI工具",
		MinIntervalSeconds: 300,
		Enabled:            true,
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	targets, err := db.DueTargets(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].ID != id {
		t.Fatalf("expected target %d to be due, got %#v", id, targets)
	}

	runID, err := db.StartRun(ctx, id, "test", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.FinishRun(ctx, runID, "succeeded", "", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	targets, err = db.DueTargets(ctx, now.Add(299*time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected no due targets before interval, got %#v", targets)
	}

	targets, err = db.DueTargets(ctx, now.Add(300*time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected target to be due after interval, got %#v", targets)
	}
}

func TestListTargets(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", Keyword: "AI工具", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.AddTarget(ctx, Target{Kind: "keyword", Name: "运营", Keyword: "小红书运营", MinIntervalSeconds: 600, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	targets, err := db.ListTargets(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected two targets, got %d", len(targets))
	}
	if targets[0].Name != "运营" || targets[1].Name != "AI" {
		t.Fatalf("unexpected target order: %#v", targets)
	}
}

func TestUpdateAndDeleteTarget(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	id, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", Keyword: "AI工具", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTarget(ctx, Target{
		ID:                 id,
		Kind:               "keyword",
		Name:               "AI效率工具",
		Keyword:            "AI效率",
		MinIntervalSeconds: 600,
		Enabled:            true,
	}); err != nil {
		t.Fatal(err)
	}
	targets, err := db.ListTargets(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Name != "AI效率工具" || targets[0].Keyword != "AI效率" || targets[0].MinIntervalSeconds != 600 {
		t.Fatalf("unexpected updated targets: %#v", targets)
	}
	if err := db.DeleteTarget(ctx, id); err != nil {
		t.Fatal(err)
	}
	targets, err = db.ListTargets(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected deleted target to be hidden, got %#v", targets)
	}
}

func TestSaveItemsUpsertsByTargetAndExternalID(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	likes := 1
	if err := db.SaveItems(ctx, id, []Item{{ExternalID: "note-1", Title: "first", LikeCount: &likes}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	likes = 2
	if err := db.SaveItems(ctx, id, []Item{{ExternalID: "note-1", Title: "updated", LikeCount: &likes}}, time.Now()); err != nil {
		t.Fatal(err)
	}

	var count int
	var title string
	var likeCount int
	row := db.db.QueryRowContext(ctx, "SELECT COUNT(*), MAX(title), MAX(like_count) FROM collected_items WHERE target_id = ?", id)
	if err := row.Scan(&count, &title, &likeCount); err != nil {
		t.Fatal(err)
	}
	if count != 1 || title != "updated" || likeCount != 2 {
		t.Fatalf("expected one updated row, got count=%d title=%q like=%d", count, title, likeCount)
	}
}

func TestListItemsAndGetItem(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	likes := 7
	if err := db.SaveItems(ctx, targetID, []Item{{
		ExternalID: "note-1",
		URL:        "https://example.test/note-1",
		AuthorName: "作者",
		Title:      "标题",
		Body:       "正文",
		Tags:       []string{"AI", "工具"},
		LikeCount:  &likes,
		Raw:        map[string]any{"source": "test"},
	}}, time.Date(2026, 4, 28, 1, 2, 3, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	items, err := db.ListItems(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].TargetID != targetID || items[0].Title != "标题" || len(items[0].Tags) != 2 {
		t.Fatalf("unexpected listed item: %#v", items[0])
	}

	item, err := db.GetItem(ctx, items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Body != "正文" || item.Raw["source"] != "test" {
		t.Fatalf("unexpected item detail: %#v", item)
	}
}

func TestListRuns(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	startedAt := time.Date(2026, 4, 28, 1, 0, 0, 0, time.UTC)
	runID, err := db.StartRun(ctx, targetID, "test_mode", startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.FinishRun(ctx, runID, "failed", "boom", startedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	runs, err := db.ListRuns(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	if runs[0].ID != runID || runs[0].TargetName != "AI" || runs[0].Status != "failed" || runs[0].Message != "boom" {
		t.Fatalf("unexpected run: %#v", runs[0])
	}
}

func TestSaveAndGetNoteAnalysis(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveItems(ctx, targetID, []Item{{ExternalID: "note-1", Title: "标题", Body: "正文"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListItems(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	analysis := NoteAnalysis{
		ItemID:           items[0].ID,
		Topic:            "AI工具教程",
		AudiencePain:     "不知道怎么开始用 AI 工具",
		TitleHook:        "保姆级教程",
		OpeningHook:      "先给结论",
		EmotionalTrigger: "效率焦虑",
		ContentStructure: "问题-步骤-结果",
		ConversionIntent: "引导关注",
		ReusablePattern:  "教程型选题",
		RiskNotes:        "避免夸大效果",
		ModelName:        "test-model",
		RawJSON:          map[string]any{"ok": true},
	}
	id, err := db.SaveNoteAnalysis(ctx, analysis)
	if err != nil {
		t.Fatal(err)
	}
	analysis.Topic = "更新后的选题"
	id2, err := db.SaveNoteAnalysis(ctx, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id {
		t.Fatalf("expected upsert to keep id %d, got %d", id, id2)
	}

	got, err := db.GetNoteAnalysisByItemID(ctx, items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Topic != "更新后的选题" || got.RawJSON["ok"] != true {
		t.Fatalf("unexpected analysis: %#v", got)
	}

	analyses, err := db.ListNoteAnalyses(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(analyses) != 1 || analyses[0].ID != id {
		t.Fatalf("unexpected analyses: %#v", analyses)
	}
}

func TestSaveAndListTopicCandidates(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveItems(ctx, targetID, []Item{{ExternalID: "note-1", Title: "标题"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListItems(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	analysisID, err := db.SaveNoteAnalysis(ctx, NoteAnalysis{ItemID: items[0].ID, Topic: "AI工具教程"})
	if err != nil {
		t.Fatal(err)
	}

	candidate := TopicCandidate{
		AnalysisID:       analysisID,
		Topic:            "AI工具教程",
		AccountFitScore:  80,
		TrendScore:       70,
		FeasibilityScore: 90,
		GrowthScore:      85,
		Differentiation:  60,
		RiskScore:        20,
		TotalScore:       77,
		Reason:           "教程型选题，互动数据较好",
		SuggestedAngle:   "面向新手做步骤拆解",
		NotDoing:         "不承诺效果",
		ScoringModel:     "rule-score-v0",
		RawJSON:          map[string]any{"ok": true},
	}
	id, err := db.SaveTopicCandidate(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	candidate.TotalScore = 88
	id2, err := db.SaveTopicCandidate(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id {
		t.Fatalf("expected upsert id %d, got %d", id, id2)
	}

	candidates, err := db.ListTopicCandidates(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %d", len(candidates))
	}
	if candidates[0].TotalScore != 88 || candidates[0].RawJSON["ok"] != true {
		t.Fatalf("unexpected candidate: %#v", candidates[0])
	}
}

func TestSaveAndListGeneratedDrafts(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveItems(ctx, targetID, []Item{{ExternalID: "note-1", Title: "标题"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListItems(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	analysisID, err := db.SaveNoteAnalysis(ctx, NoteAnalysis{ItemID: items[0].ID, Topic: "AI工具教程"})
	if err != nil {
		t.Fatal(err)
	}
	candidateID, err := db.SaveTopicCandidate(ctx, TopicCandidate{AnalysisID: analysisID, Topic: "AI工具教程", TotalScore: 80})
	if err != nil {
		t.Fatal(err)
	}

	draft := GeneratedDraft{
		CandidateID:  candidateID,
		TitleOptions: []string{"标题1", "标题2"},
		Opening:      "开头",
		Body:         "正文",
		CoverText:    "封面",
		ImageBrief:   "图片脚本",
		Tags:         []string{"AI", "工具"},
		RiskNotes:    "需要审核",
		Generator:    "rule-draft-v0",
		RawJSON:      map[string]any{"ok": true},
	}
	id, err := db.SaveGeneratedDraft(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	draft.Body = "更新正文"
	id2, err := db.SaveGeneratedDraft(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id {
		t.Fatalf("expected upsert id %d, got %d", id, id2)
	}

	drafts, err := db.ListGeneratedDrafts(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected one draft, got %d", len(drafts))
	}
	if drafts[0].Body != "更新正文" || drafts[0].RawJSON["ok"] != true || len(drafts[0].TitleOptions) != 2 {
		t.Fatalf("unexpected draft: %#v", drafts[0])
	}
}

func TestSaveAndListPublishRecordsAndPerformanceSnapshots(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveItems(ctx, targetID, []Item{{ExternalID: "note-1", Title: "标题"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListItems(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	analysisID, err := db.SaveNoteAnalysis(ctx, NoteAnalysis{ItemID: items[0].ID, Topic: "AI工具教程"})
	if err != nil {
		t.Fatal(err)
	}
	candidateID, err := db.SaveTopicCandidate(ctx, TopicCandidate{AnalysisID: analysisID, Topic: "AI工具教程", TotalScore: 80})
	if err != nil {
		t.Fatal(err)
	}
	draftID, err := db.SaveGeneratedDraft(ctx, GeneratedDraft{CandidateID: candidateID, Body: "正文"})
	if err != nil {
		t.Fatal(err)
	}

	publishedAt := "2026-04-28T12:00:00+08:00"
	record := PublishRecord{
		DraftID:     draftID,
		Platform:    "xiaohongshu",
		NoteURL:     "https://www.xiaohongshu.com/explore/test",
		Status:      "published",
		PublishedAt: publishedAt,
		Operator:    "editor-a",
		Notes:       "首版发布",
	}
	recordID, err := db.SavePublishRecord(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	record.Status = "reviewed"
	recordID2, err := db.SavePublishRecord(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	if recordID2 != recordID {
		t.Fatalf("expected upsert id %d, got %d", recordID, recordID2)
	}

	records, err := db.ListPublishRecords(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one publish record, got %d", len(records))
	}
	if records[0].Status != "reviewed" || records[0].PublishedAt != publishedAt || records[0].Operator != "editor-a" {
		t.Fatalf("unexpected publish record: %#v", records[0])
	}

	snapshot := PerformanceSnapshot{
		PublishRecordID: recordID,
		Views:           1000,
		Likes:           88,
		Collects:        42,
		Comments:        6,
		Follows:         3,
		CapturedAt:      "2026-04-29T12:00:00+08:00",
		RawJSON:         map[string]any{"source": "manual"},
	}
	snapshotID, err := db.SavePerformanceSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotID <= 0 {
		t.Fatalf("expected snapshot id, got %d", snapshotID)
	}

	snapshots, err := db.ListPerformanceSnapshots(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	if snapshots[0].PublishRecordID != recordID || snapshots[0].Likes != 88 || snapshots[0].RawJSON["source"] != "manual" {
		t.Fatalf("unexpected snapshot: %#v", snapshots[0])
	}
}

func TestSaveAndListPerformanceReports(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	targetID, err := db.AddTarget(ctx, Target{Kind: "keyword", Name: "AI", MinIntervalSeconds: 300, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveItems(ctx, targetID, []Item{{ExternalID: "note-1", Title: "标题"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListItems(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	analysisID, err := db.SaveNoteAnalysis(ctx, NoteAnalysis{ItemID: items[0].ID, Topic: "AI工具教程"})
	if err != nil {
		t.Fatal(err)
	}
	candidateID, err := db.SaveTopicCandidate(ctx, TopicCandidate{AnalysisID: analysisID, Topic: "AI工具教程", TotalScore: 80})
	if err != nil {
		t.Fatal(err)
	}
	draftID, err := db.SaveGeneratedDraft(ctx, GeneratedDraft{CandidateID: candidateID, Body: "正文"})
	if err != nil {
		t.Fatal(err)
	}
	recordID, err := db.SavePublishRecord(ctx, PublishRecord{DraftID: draftID, Platform: "xiaohongshu"})
	if err != nil {
		t.Fatal(err)
	}
	snapshotID, err := db.SavePerformanceSnapshot(ctx, PerformanceSnapshot{PublishRecordID: recordID, Views: 1000, Likes: 80, Collects: 30, Comments: 10, Follows: 5})
	if err != nil {
		t.Fatal(err)
	}

	report := PerformanceReport{
		PublishRecordID:     recordID,
		SnapshotID:          snapshotID,
		PerformanceScore:    82,
		EngagementRateBasis: 1200,
		FollowRateBasis:     50,
		Summary:             "表现良好",
		SuggestedAdjustment: "保留选题结构",
		ReviewModel:         "rule-review-v0",
		RawJSON:             map[string]any{"ok": true},
	}
	reportID, err := db.SavePerformanceReport(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	report.PerformanceScore = 90
	reportID2, err := db.SavePerformanceReport(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	if reportID2 != reportID {
		t.Fatalf("expected upsert id %d, got %d", reportID, reportID2)
	}

	reports, err := db.ListPerformanceReports(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected one report, got %d", len(reports))
	}
	if reports[0].PerformanceScore != 90 || reports[0].RawJSON["ok"] != true {
		t.Fatalf("unexpected report: %#v", reports[0])
	}
}
