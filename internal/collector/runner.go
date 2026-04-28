package collector

import (
	"context"
	"time"

	"xiaohongshu-tool/internal/storage"
)

type Store interface {
	DueTargets(ctx context.Context, now time.Time, limit int) ([]storage.Target, error)
	StartRun(ctx context.Context, targetID int64, mode string, startedAt time.Time) (int64, error)
	FinishRun(ctx context.Context, runID int64, status, message string, finishedAt time.Time) error
	SaveItems(ctx context.Context, targetID int64, items []storage.Item, capturedAt time.Time) error
}

type Collector interface {
	Collect(ctx context.Context, target storage.Target) (Result, string, error)
}

type Runner struct {
	store     Store
	collector Collector
	now       func() time.Time
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
	for _, target := range targets {
		startedAt := r.now()
		runID, err := r.store.StartRun(ctx, target.ID, "external_command", startedAt)
		if err != nil {
			return 0, err
		}
		result, stderr, err := r.collector.Collect(ctx, target)
		if err != nil {
			_ = r.store.FinishRun(ctx, runID, "failed", stderr+"\n"+err.Error(), r.now())
			continue
		}
		if err := r.store.SaveItems(ctx, target.ID, result.Items, r.now()); err != nil {
			_ = r.store.FinishRun(ctx, runID, "failed", err.Error(), r.now())
			continue
		}
		_ = r.store.FinishRun(ctx, runID, "succeeded", stderr, r.now())
	}
	return len(targets), nil
}
