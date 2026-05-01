package collector

import (
	"context"
	"strconv"
	"time"

	"xiaohongshu-tool/internal/storage"
)

type Store interface {
	DueTargets(ctx context.Context, now time.Time, limit int) ([]storage.Target, error)
	StartRun(ctx context.Context, targetID int64, mode string, startedAt time.Time) (int64, error)
	FinishRun(ctx context.Context, runID int64, status, message string, finishedAt time.Time) error
	SaveItemsForRun(ctx context.Context, runID, targetID int64, items []storage.Item, capturedAt time.Time) ([]storage.RunItem, error)
}

type Collector interface {
	Collect(ctx context.Context, target storage.Target) (Result, string, error)
}

type Runner struct {
	store     Store
	collector Collector
	now       func() time.Time
}

type RunSummary struct {
	RunID      int64    `json:"run_id"`
	TargetID   int64    `json:"target_id"`
	TargetName string   `json:"target_name"`
	ItemCount  int      `json:"item_count"`
	Titles     []string `json:"titles"`
}

type Summary struct {
	TargetCount int          `json:"target_count"`
	ItemCount   int          `json:"item_count"`
	Runs        []RunSummary `json:"runs"`
}

func NewRunner(store Store, collector Collector) *Runner {
	return &Runner{
		store:     store,
		collector: collector,
		now:       time.Now,
	}
}

func (r *Runner) RunDue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 1
	}
	targets, err := r.store.DueTargets(ctx, r.now(), limit)
	if err != nil {
		return 0, err
	}
	summary, err := r.RunTargets(ctx, targets)
	if err != nil {
		return 0, err
	}
	return summary.TargetCount, nil
}

func (r *Runner) RunTargets(ctx context.Context, targets []storage.Target) (Summary, error) {
	summary := Summary{Runs: make([]RunSummary, 0, len(targets))}
	for _, target := range targets {
		startedAt := r.now()
		runID, err := r.store.StartRun(ctx, target.ID, "external_command", startedAt)
		if err != nil {
			return summary, err
		}
		result, stderr, err := r.collector.Collect(ctx, target)
		if err != nil {
			_ = r.store.FinishRun(ctx, runID, "failed", stderr+"\n"+err.Error(), r.now())
			continue
		}
		saved, err := r.store.SaveItemsForRun(ctx, runID, target.ID, result.Items, r.now())
		if err != nil {
			_ = r.store.FinishRun(ctx, runID, "failed", err.Error(), r.now())
			continue
		}
		titles := make([]string, 0, len(saved))
		for _, item := range saved {
			titles = append(titles, item.Title)
		}
		summary.TargetCount++
		summary.ItemCount += len(saved)
		summary.Runs = append(summary.Runs, RunSummary{
			RunID:      runID,
			TargetID:   target.ID,
			TargetName: target.Name,
			ItemCount:  len(saved),
			Titles:     titles,
		})
		message := stderr
		if message != "" {
			message += "\n"
		}
		message += "collected_items=" + strconv.Itoa(len(saved))
		_ = r.store.FinishRun(ctx, runID, "succeeded", message, r.now())
	}
	return summary, nil
}
