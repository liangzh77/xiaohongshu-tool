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
