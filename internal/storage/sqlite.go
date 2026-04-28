package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

type Target struct {
	ID                 int64
	Kind               string
	Name               string
	URL                string
	Keyword            string
	MinIntervalSeconds int
	Enabled            bool
}

type Item struct {
	ExternalID   string         `json:"external_id"`
	URL          string         `json:"url"`
	AuthorName   string         `json:"author_name"`
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	Tags         []string       `json:"tags"`
	LikeCount    *int           `json:"like_count"`
	CollectCount *int           `json:"collect_count"`
	CommentCount *int           `json:"comment_count"`
	PublishedAt  string         `json:"published_at"`
	Raw          map[string]any `json:"raw"`
}

func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, schema)
	return err
}

func (d *DB) AddTarget(ctx context.Context, t Target) (int64, error) {
	if t.Kind == "" || t.Name == "" {
		return 0, fmt.Errorf("kind and name are required")
	}
	if t.MinIntervalSeconds <= 0 {
		t.MinIntervalSeconds = 300
	}
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO collector_targets(kind, name, url, keyword, min_interval_seconds, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		t.Kind, t.Name, t.URL, t.Keyword, t.MinIntervalSeconds, t.Enabled,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DueTargets(ctx context.Context, now time.Time, limit int) ([]Target, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, kind, name, url, keyword, min_interval_seconds, enabled
		FROM collector_targets
		WHERE enabled = 1
		  AND (
		    last_attempt_at IS NULL
		    OR unixepoch(?) - unixepoch(last_attempt_at) >= min_interval_seconds
		  )
		ORDER BY COALESCE(last_attempt_at, '1970-01-01') ASC, id ASC
		LIMIT ?`, now.UTC().Format(time.RFC3339), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.Kind, &t.Name, &t.URL, &t.Keyword, &t.MinIntervalSeconds, &t.Enabled); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (d *DB) StartRun(ctx context.Context, targetID int64, mode string, startedAt time.Time) (int64, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO collection_runs(target_id, mode, status, started_at)
		VALUES (?, ?, 'running', ?)`, targetID, mode, startedAt.UTC().Format(time.RFC3339))
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE collector_targets SET last_attempt_at = ?, updated_at = datetime('now') WHERE id = ?`,
		startedAt.UTC().Format(time.RFC3339), targetID); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) FinishRun(ctx context.Context, runID int64, status, message string, finishedAt time.Time) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE collection_runs SET status = ?, message = ?, finished_at = ? WHERE id = ?`,
		status, message, finishedAt.UTC().Format(time.RFC3339), runID)
	return err
}

func (d *DB) SaveItems(ctx context.Context, targetID int64, items []Item, capturedAt time.Time) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO collected_items(
			target_id, external_id, url, author_name, title, body, tags_json,
			like_count, collect_count, comment_count, published_at, raw_json, captured_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_id, external_id) DO UPDATE SET
			url = excluded.url,
			author_name = excluded.author_name,
			title = excluded.title,
			body = excluded.body,
			tags_json = excluded.tags_json,
			like_count = excluded.like_count,
			collect_count = excluded.collect_count,
			comment_count = excluded.comment_count,
			published_at = excluded.published_at,
			raw_json = excluded.raw_json,
			captured_at = excluded.captured_at`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		tagsJSON, _ := json.Marshal(item.Tags)
		rawJSON, _ := json.Marshal(item.Raw)
		externalID := item.ExternalID
		if externalID == "" {
			externalID = item.URL
		}
		if _, err := stmt.ExecContext(ctx,
			targetID, externalID, item.URL, item.AuthorName, item.Title, item.Body, string(tagsJSON),
			item.LikeCount, item.CollectCount, item.CommentCount, item.PublishedAt, string(rawJSON),
			capturedAt.UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

const schema = `
CREATE TABLE IF NOT EXISTS collector_targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL,
	name TEXT NOT NULL,
	url TEXT NOT NULL DEFAULT '',
	keyword TEXT NOT NULL DEFAULT '',
	min_interval_seconds INTEGER NOT NULL DEFAULT 300,
	enabled INTEGER NOT NULL DEFAULT 1,
	last_attempt_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS collection_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	target_id INTEGER NOT NULL REFERENCES collector_targets(id),
	mode TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	finished_at TEXT
);

CREATE TABLE IF NOT EXISTS collected_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	target_id INTEGER NOT NULL REFERENCES collector_targets(id),
	external_id TEXT NOT NULL,
	url TEXT NOT NULL DEFAULT '',
	author_name TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL DEFAULT '',
	tags_json TEXT NOT NULL DEFAULT '[]',
	like_count INTEGER,
	collect_count INTEGER,
	comment_count INTEGER,
	published_at TEXT NOT NULL DEFAULT '',
	raw_json TEXT NOT NULL DEFAULT '{}',
	captured_at TEXT NOT NULL,
	UNIQUE(target_id, external_id)
);
`
