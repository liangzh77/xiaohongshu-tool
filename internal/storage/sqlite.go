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
	ID                 int64  `json:"id"`
	Kind               string `json:"kind"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	Keyword            string `json:"keyword"`
	MinIntervalSeconds int    `json:"min_interval_seconds"`
	Enabled            bool   `json:"enabled"`
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

type StoredItem struct {
	ID         int64  `json:"id"`
	TargetID   int64  `json:"target_id"`
	TargetName string `json:"target_name"`
	Item
	CapturedAt string `json:"captured_at"`
}

type Run struct {
	ID         int64  `json:"id"`
	TargetID   int64  `json:"target_id"`
	TargetName string `json:"target_name"`
	Mode       string `json:"mode"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

type NoteAnalysis struct {
	ID               int64          `json:"id"`
	ItemID           int64          `json:"item_id"`
	Topic            string         `json:"topic"`
	AudiencePain     string         `json:"audience_pain"`
	TitleHook        string         `json:"title_hook"`
	OpeningHook      string         `json:"opening_hook"`
	EmotionalTrigger string         `json:"emotional_trigger"`
	ContentStructure string         `json:"content_structure"`
	ConversionIntent string         `json:"conversion_intent"`
	ReusablePattern  string         `json:"reusable_pattern"`
	RiskNotes        string         `json:"risk_notes"`
	ModelName        string         `json:"model_name"`
	RawJSON          map[string]any `json:"raw_json"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

type TopicCandidate struct {
	ID               int64          `json:"id"`
	AnalysisID       int64          `json:"analysis_id"`
	Topic            string         `json:"topic"`
	AccountFitScore  int            `json:"account_fit_score"`
	TrendScore       int            `json:"trend_score"`
	FeasibilityScore int            `json:"feasibility_score"`
	GrowthScore      int            `json:"growth_score"`
	Differentiation  int            `json:"differentiation_score"`
	RiskScore        int            `json:"risk_score"`
	TotalScore       int            `json:"total_score"`
	Reason           string         `json:"reason"`
	SuggestedAngle   string         `json:"suggested_angle"`
	NotDoing         string         `json:"not_doing"`
	ScoringModel     string         `json:"scoring_model"`
	RawJSON          map[string]any `json:"raw_json"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

type GeneratedDraft struct {
	ID           int64          `json:"id"`
	CandidateID  int64          `json:"candidate_id"`
	TitleOptions []string       `json:"title_options"`
	Opening      string         `json:"opening"`
	Body         string         `json:"body"`
	CoverText    string         `json:"cover_text"`
	ImageBrief   string         `json:"image_brief"`
	Tags         []string       `json:"tags"`
	RiskNotes    string         `json:"risk_notes"`
	Generator    string         `json:"generator"`
	RawJSON      map[string]any `json:"raw_json"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
}

type PublishRecord struct {
	ID          int64  `json:"id"`
	DraftID     int64  `json:"draft_id"`
	Platform    string `json:"platform"`
	NoteURL     string `json:"note_url"`
	Status      string `json:"status"`
	PublishedAt string `json:"published_at"`
	Operator    string `json:"operator"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type PerformanceSnapshot struct {
	ID              int64          `json:"id"`
	PublishRecordID int64          `json:"publish_record_id"`
	Views           int            `json:"views"`
	Likes           int            `json:"likes"`
	Collects        int            `json:"collects"`
	Comments        int            `json:"comments"`
	Follows         int            `json:"follows"`
	CapturedAt      string         `json:"captured_at"`
	RawJSON         map[string]any `json:"raw_json"`
	CreatedAt       string         `json:"created_at"`
}

type PerformanceReport struct {
	ID                  int64          `json:"id"`
	PublishRecordID     int64          `json:"publish_record_id"`
	SnapshotID          int64          `json:"snapshot_id"`
	PerformanceScore    int            `json:"performance_score"`
	EngagementRateBasis int            `json:"engagement_rate_basis"`
	FollowRateBasis     int            `json:"follow_rate_basis"`
	Summary             string         `json:"summary"`
	SuggestedAdjustment string         `json:"suggested_adjustment"`
	ReviewModel         string         `json:"review_model"`
	RawJSON             map[string]any `json:"raw_json"`
	CreatedAt           string         `json:"created_at"`
	UpdatedAt           string         `json:"updated_at"`
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

func (d *DB) ListTargets(ctx context.Context, limit int) ([]Target, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, kind, name, url, keyword, min_interval_seconds, enabled
		FROM collector_targets
		WHERE enabled = 1
		ORDER BY id DESC
		LIMIT ?`, limit)
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

func (d *DB) UpdateTarget(ctx context.Context, t Target) error {
	if t.ID <= 0 {
		return fmt.Errorf("target id is required")
	}
	if t.Kind == "" || t.Name == "" {
		return fmt.Errorf("kind and name are required")
	}
	if t.MinIntervalSeconds <= 0 {
		t.MinIntervalSeconds = 300
	}
	_, err := d.db.ExecContext(ctx, `
		UPDATE collector_targets
		SET kind = ?, name = ?, url = ?, keyword = ?, min_interval_seconds = ?, enabled = ?, updated_at = datetime('now')
		WHERE id = ?`,
		t.Kind, t.Name, t.URL, t.Keyword, t.MinIntervalSeconds, t.Enabled, t.ID,
	)
	return err
}

func (d *DB) DeleteTarget(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("target id is required")
	}
	_, err := d.db.ExecContext(ctx, `
		UPDATE collector_targets
		SET enabled = 0, updated_at = datetime('now')
		WHERE id = ?`, id)
	return err
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

func (d *DB) ListItems(ctx context.Context, limit int) ([]StoredItem, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT i.id, i.target_id, t.name, i.external_id, i.url, i.author_name, i.title, i.body,
		       i.tags_json, i.like_count, i.collect_count, i.comment_count, i.published_at, i.raw_json, i.captured_at
		FROM collected_items i
		JOIN collector_targets t ON t.id = i.target_id
		ORDER BY i.captured_at DESC, i.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StoredItem
	for rows.Next() {
		item, err := scanStoredItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DB) GetItem(ctx context.Context, id int64) (StoredItem, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT i.id, i.target_id, t.name, i.external_id, i.url, i.author_name, i.title, i.body,
		       i.tags_json, i.like_count, i.collect_count, i.comment_count, i.published_at, i.raw_json, i.captured_at
		FROM collected_items i
		JOIN collector_targets t ON t.id = i.target_id
		WHERE i.id = ?`, id)
	return scanStoredItem(row)
}

func (d *DB) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT r.id, r.target_id, t.name, r.mode, r.status, r.message, r.started_at, COALESCE(r.finished_at, '')
		FROM collection_runs r
		JOIN collector_targets t ON t.id = r.target_id
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []Run
	for rows.Next() {
		var run Run
		if err := rows.Scan(&run.ID, &run.TargetID, &run.TargetName, &run.Mode, &run.Status, &run.Message, &run.StartedAt, &run.FinishedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type itemScanner interface {
	Scan(dest ...any) error
}

func scanStoredItem(scanner itemScanner) (StoredItem, error) {
	var item StoredItem
	var tagsJSON string
	var rawJSON string
	if err := scanner.Scan(
		&item.ID,
		&item.TargetID,
		&item.TargetName,
		&item.ExternalID,
		&item.URL,
		&item.AuthorName,
		&item.Title,
		&item.Body,
		&tagsJSON,
		&item.LikeCount,
		&item.CollectCount,
		&item.CommentCount,
		&item.PublishedAt,
		&rawJSON,
		&item.CapturedAt,
	); err != nil {
		return StoredItem{}, err
	}
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &item.Tags)
	}
	if rawJSON != "" {
		_ = json.Unmarshal([]byte(rawJSON), &item.Raw)
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if item.Raw == nil {
		item.Raw = map[string]any{}
	}
	return item, nil
}

func (d *DB) SaveNoteAnalysis(ctx context.Context, analysis NoteAnalysis) (int64, error) {
	if analysis.ItemID <= 0 {
		return 0, fmt.Errorf("item_id is required")
	}
	rawJSON, _ := json.Marshal(analysis.RawJSON)
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO note_analyses(
			item_id, topic, audience_pain, title_hook, opening_hook, emotional_trigger,
			content_structure, conversion_intent, reusable_pattern, risk_notes, model_name,
			raw_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(item_id) DO UPDATE SET
			topic = excluded.topic,
			audience_pain = excluded.audience_pain,
			title_hook = excluded.title_hook,
			opening_hook = excluded.opening_hook,
			emotional_trigger = excluded.emotional_trigger,
			content_structure = excluded.content_structure,
			conversion_intent = excluded.conversion_intent,
			reusable_pattern = excluded.reusable_pattern,
			risk_notes = excluded.risk_notes,
			model_name = excluded.model_name,
			raw_json = excluded.raw_json,
			updated_at = datetime('now')`,
		analysis.ItemID,
		analysis.Topic,
		analysis.AudiencePain,
		analysis.TitleHook,
		analysis.OpeningHook,
		analysis.EmotionalTrigger,
		analysis.ContentStructure,
		analysis.ConversionIntent,
		analysis.ReusablePattern,
		analysis.RiskNotes,
		analysis.ModelName,
		string(rawJSON),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}
	var existingID int64
	err = d.db.QueryRowContext(ctx, `SELECT id FROM note_analyses WHERE item_id = ?`, analysis.ItemID).Scan(&existingID)
	return existingID, err
}

func (d *DB) GetNoteAnalysisByItemID(ctx context.Context, itemID int64) (NoteAnalysis, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT id, item_id, topic, audience_pain, title_hook, opening_hook, emotional_trigger,
		       content_structure, conversion_intent, reusable_pattern, risk_notes, model_name,
		       raw_json, created_at, updated_at
		FROM note_analyses
		WHERE item_id = ?`, itemID)
	return scanNoteAnalysis(row)
}

func (d *DB) ListNoteAnalyses(ctx context.Context, limit int) ([]NoteAnalysis, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, item_id, topic, audience_pain, title_hook, opening_hook, emotional_trigger,
		       content_structure, conversion_intent, reusable_pattern, risk_notes, model_name,
		       raw_json, created_at, updated_at
		FROM note_analyses
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var analyses []NoteAnalysis
	for rows.Next() {
		analysis, err := scanNoteAnalysis(rows)
		if err != nil {
			return nil, err
		}
		analyses = append(analyses, analysis)
	}
	return analyses, rows.Err()
}

type analysisScanner interface {
	Scan(dest ...any) error
}

func scanNoteAnalysis(scanner analysisScanner) (NoteAnalysis, error) {
	var analysis NoteAnalysis
	var rawJSON string
	if err := scanner.Scan(
		&analysis.ID,
		&analysis.ItemID,
		&analysis.Topic,
		&analysis.AudiencePain,
		&analysis.TitleHook,
		&analysis.OpeningHook,
		&analysis.EmotionalTrigger,
		&analysis.ContentStructure,
		&analysis.ConversionIntent,
		&analysis.ReusablePattern,
		&analysis.RiskNotes,
		&analysis.ModelName,
		&rawJSON,
		&analysis.CreatedAt,
		&analysis.UpdatedAt,
	); err != nil {
		return NoteAnalysis{}, err
	}
	if rawJSON != "" {
		_ = json.Unmarshal([]byte(rawJSON), &analysis.RawJSON)
	}
	if analysis.RawJSON == nil {
		analysis.RawJSON = map[string]any{}
	}
	return analysis, nil
}

func (d *DB) SaveTopicCandidate(ctx context.Context, candidate TopicCandidate) (int64, error) {
	if candidate.AnalysisID <= 0 {
		return 0, fmt.Errorf("analysis_id is required")
	}
	rawJSON, _ := json.Marshal(candidate.RawJSON)
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO topic_candidates(
			analysis_id, topic, account_fit_score, trend_score, feasibility_score,
			growth_score, differentiation_score, risk_score, total_score, reason,
			suggested_angle, not_doing, scoring_model, raw_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(analysis_id) DO UPDATE SET
			topic = excluded.topic,
			account_fit_score = excluded.account_fit_score,
			trend_score = excluded.trend_score,
			feasibility_score = excluded.feasibility_score,
			growth_score = excluded.growth_score,
			differentiation_score = excluded.differentiation_score,
			risk_score = excluded.risk_score,
			total_score = excluded.total_score,
			reason = excluded.reason,
			suggested_angle = excluded.suggested_angle,
			not_doing = excluded.not_doing,
			scoring_model = excluded.scoring_model,
			raw_json = excluded.raw_json,
			updated_at = datetime('now')`,
		candidate.AnalysisID,
		candidate.Topic,
		candidate.AccountFitScore,
		candidate.TrendScore,
		candidate.FeasibilityScore,
		candidate.GrowthScore,
		candidate.Differentiation,
		candidate.RiskScore,
		candidate.TotalScore,
		candidate.Reason,
		candidate.SuggestedAngle,
		candidate.NotDoing,
		candidate.ScoringModel,
		string(rawJSON),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}
	var existingID int64
	err = d.db.QueryRowContext(ctx, `SELECT id FROM topic_candidates WHERE analysis_id = ?`, candidate.AnalysisID).Scan(&existingID)
	return existingID, err
}

func (d *DB) ListTopicCandidates(ctx context.Context, limit int) ([]TopicCandidate, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, analysis_id, topic, account_fit_score, trend_score, feasibility_score,
		       growth_score, differentiation_score, risk_score, total_score, reason,
		       suggested_angle, not_doing, scoring_model, raw_json, created_at, updated_at
		FROM topic_candidates
		ORDER BY total_score DESC, updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []TopicCandidate
	for rows.Next() {
		candidate, err := scanTopicCandidate(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

type candidateScanner interface {
	Scan(dest ...any) error
}

func scanTopicCandidate(scanner candidateScanner) (TopicCandidate, error) {
	var candidate TopicCandidate
	var rawJSON string
	if err := scanner.Scan(
		&candidate.ID,
		&candidate.AnalysisID,
		&candidate.Topic,
		&candidate.AccountFitScore,
		&candidate.TrendScore,
		&candidate.FeasibilityScore,
		&candidate.GrowthScore,
		&candidate.Differentiation,
		&candidate.RiskScore,
		&candidate.TotalScore,
		&candidate.Reason,
		&candidate.SuggestedAngle,
		&candidate.NotDoing,
		&candidate.ScoringModel,
		&rawJSON,
		&candidate.CreatedAt,
		&candidate.UpdatedAt,
	); err != nil {
		return TopicCandidate{}, err
	}
	if rawJSON != "" {
		_ = json.Unmarshal([]byte(rawJSON), &candidate.RawJSON)
	}
	if candidate.RawJSON == nil {
		candidate.RawJSON = map[string]any{}
	}
	return candidate, nil
}

func (d *DB) SaveGeneratedDraft(ctx context.Context, draft GeneratedDraft) (int64, error) {
	if draft.CandidateID <= 0 {
		return 0, fmt.Errorf("candidate_id is required")
	}
	titleOptionsJSON, _ := json.Marshal(draft.TitleOptions)
	tagsJSON, _ := json.Marshal(draft.Tags)
	rawJSON, _ := json.Marshal(draft.RawJSON)
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO generated_drafts(
			candidate_id, title_options_json, opening, body, cover_text, image_brief,
			tags_json, risk_notes, generator, raw_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(candidate_id) DO UPDATE SET
			title_options_json = excluded.title_options_json,
			opening = excluded.opening,
			body = excluded.body,
			cover_text = excluded.cover_text,
			image_brief = excluded.image_brief,
			tags_json = excluded.tags_json,
			risk_notes = excluded.risk_notes,
			generator = excluded.generator,
			raw_json = excluded.raw_json,
			updated_at = datetime('now')`,
		draft.CandidateID,
		string(titleOptionsJSON),
		draft.Opening,
		draft.Body,
		draft.CoverText,
		draft.ImageBrief,
		string(tagsJSON),
		draft.RiskNotes,
		draft.Generator,
		string(rawJSON),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}
	var existingID int64
	err = d.db.QueryRowContext(ctx, `SELECT id FROM generated_drafts WHERE candidate_id = ?`, draft.CandidateID).Scan(&existingID)
	return existingID, err
}

func (d *DB) ListGeneratedDrafts(ctx context.Context, limit int) ([]GeneratedDraft, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, candidate_id, title_options_json, opening, body, cover_text, image_brief,
		       tags_json, risk_notes, generator, raw_json, created_at, updated_at
		FROM generated_drafts
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drafts []GeneratedDraft
	for rows.Next() {
		draft, err := scanGeneratedDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, draft)
	}
	return drafts, rows.Err()
}

type draftScanner interface {
	Scan(dest ...any) error
}

func scanGeneratedDraft(scanner draftScanner) (GeneratedDraft, error) {
	var draft GeneratedDraft
	var titleOptionsJSON string
	var tagsJSON string
	var rawJSON string
	if err := scanner.Scan(
		&draft.ID,
		&draft.CandidateID,
		&titleOptionsJSON,
		&draft.Opening,
		&draft.Body,
		&draft.CoverText,
		&draft.ImageBrief,
		&tagsJSON,
		&draft.RiskNotes,
		&draft.Generator,
		&rawJSON,
		&draft.CreatedAt,
		&draft.UpdatedAt,
	); err != nil {
		return GeneratedDraft{}, err
	}
	if titleOptionsJSON != "" {
		_ = json.Unmarshal([]byte(titleOptionsJSON), &draft.TitleOptions)
	}
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &draft.Tags)
	}
	if rawJSON != "" {
		_ = json.Unmarshal([]byte(rawJSON), &draft.RawJSON)
	}
	if draft.TitleOptions == nil {
		draft.TitleOptions = []string{}
	}
	if draft.Tags == nil {
		draft.Tags = []string{}
	}
	if draft.RawJSON == nil {
		draft.RawJSON = map[string]any{}
	}
	return draft, nil
}

func (d *DB) SavePublishRecord(ctx context.Context, record PublishRecord) (int64, error) {
	if record.DraftID <= 0 {
		return 0, fmt.Errorf("draft_id is required")
	}
	if record.Platform == "" {
		record.Platform = "xiaohongshu"
	}
	if record.Status == "" {
		record.Status = "published"
	}
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO publish_records(
			draft_id, platform, note_url, status, published_at, operator, notes, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(draft_id, platform) DO UPDATE SET
			note_url = excluded.note_url,
			status = excluded.status,
			published_at = excluded.published_at,
			operator = excluded.operator,
			notes = excluded.notes,
			updated_at = datetime('now')`,
		record.DraftID,
		record.Platform,
		record.NoteURL,
		record.Status,
		record.PublishedAt,
		record.Operator,
		record.Notes,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}
	var existingID int64
	err = d.db.QueryRowContext(ctx, `SELECT id FROM publish_records WHERE draft_id = ? AND platform = ?`, record.DraftID, record.Platform).Scan(&existingID)
	return existingID, err
}

func (d *DB) ListPublishRecords(ctx context.Context, limit int) ([]PublishRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, draft_id, platform, note_url, status, published_at, operator, notes, created_at, updated_at
		FROM publish_records
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []PublishRecord
	for rows.Next() {
		var record PublishRecord
		if err := rows.Scan(
			&record.ID,
			&record.DraftID,
			&record.Platform,
			&record.NoteURL,
			&record.Status,
			&record.PublishedAt,
			&record.Operator,
			&record.Notes,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (d *DB) SavePerformanceSnapshot(ctx context.Context, snapshot PerformanceSnapshot) (int64, error) {
	if snapshot.PublishRecordID <= 0 {
		return 0, fmt.Errorf("publish_record_id is required")
	}
	if snapshot.CapturedAt == "" {
		snapshot.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	rawJSON, _ := json.Marshal(snapshot.RawJSON)
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO performance_snapshots(
			publish_record_id, views, likes, collects, comments, follows, captured_at, raw_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		snapshot.PublishRecordID,
		snapshot.Views,
		snapshot.Likes,
		snapshot.Collects,
		snapshot.Comments,
		snapshot.Follows,
		snapshot.CapturedAt,
		string(rawJSON),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) ListPerformanceSnapshots(ctx context.Context, limit int) ([]PerformanceSnapshot, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, publish_record_id, views, likes, collects, comments, follows, captured_at, raw_json, created_at
		FROM performance_snapshots
		ORDER BY captured_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snapshots []PerformanceSnapshot
	for rows.Next() {
		var snapshot PerformanceSnapshot
		var rawJSON string
		if err := rows.Scan(
			&snapshot.ID,
			&snapshot.PublishRecordID,
			&snapshot.Views,
			&snapshot.Likes,
			&snapshot.Collects,
			&snapshot.Comments,
			&snapshot.Follows,
			&snapshot.CapturedAt,
			&rawJSON,
			&snapshot.CreatedAt,
		); err != nil {
			return nil, err
		}
		if rawJSON != "" {
			_ = json.Unmarshal([]byte(rawJSON), &snapshot.RawJSON)
		}
		if snapshot.RawJSON == nil {
			snapshot.RawJSON = map[string]any{}
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (d *DB) SavePerformanceReport(ctx context.Context, report PerformanceReport) (int64, error) {
	if report.PublishRecordID <= 0 {
		return 0, fmt.Errorf("publish_record_id is required")
	}
	if report.SnapshotID <= 0 {
		return 0, fmt.Errorf("snapshot_id is required")
	}
	rawJSON, _ := json.Marshal(report.RawJSON)
	res, err := d.db.ExecContext(ctx, `
		INSERT INTO performance_reports(
			publish_record_id, snapshot_id, performance_score, engagement_rate_basis,
			follow_rate_basis, summary, suggested_adjustment, review_model,
			raw_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(snapshot_id) DO UPDATE SET
			publish_record_id = excluded.publish_record_id,
			performance_score = excluded.performance_score,
			engagement_rate_basis = excluded.engagement_rate_basis,
			follow_rate_basis = excluded.follow_rate_basis,
			summary = excluded.summary,
			suggested_adjustment = excluded.suggested_adjustment,
			review_model = excluded.review_model,
			raw_json = excluded.raw_json,
			updated_at = datetime('now')`,
		report.PublishRecordID,
		report.SnapshotID,
		report.PerformanceScore,
		report.EngagementRateBasis,
		report.FollowRateBasis,
		report.Summary,
		report.SuggestedAdjustment,
		report.ReviewModel,
		string(rawJSON),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}
	var existingID int64
	err = d.db.QueryRowContext(ctx, `SELECT id FROM performance_reports WHERE snapshot_id = ?`, report.SnapshotID).Scan(&existingID)
	return existingID, err
}

func (d *DB) ListPerformanceReports(ctx context.Context, limit int) ([]PerformanceReport, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, publish_record_id, snapshot_id, performance_score, engagement_rate_basis,
		       follow_rate_basis, summary, suggested_adjustment, review_model, raw_json,
		       created_at, updated_at
		FROM performance_reports
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []PerformanceReport
	for rows.Next() {
		var report PerformanceReport
		var rawJSON string
		if err := rows.Scan(
			&report.ID,
			&report.PublishRecordID,
			&report.SnapshotID,
			&report.PerformanceScore,
			&report.EngagementRateBasis,
			&report.FollowRateBasis,
			&report.Summary,
			&report.SuggestedAdjustment,
			&report.ReviewModel,
			&rawJSON,
			&report.CreatedAt,
			&report.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if rawJSON != "" {
			_ = json.Unmarshal([]byte(rawJSON), &report.RawJSON)
		}
		if report.RawJSON == nil {
			report.RawJSON = map[string]any{}
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
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

CREATE TABLE IF NOT EXISTS note_analyses (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id INTEGER NOT NULL REFERENCES collected_items(id),
	topic TEXT NOT NULL DEFAULT '',
	audience_pain TEXT NOT NULL DEFAULT '',
	title_hook TEXT NOT NULL DEFAULT '',
	opening_hook TEXT NOT NULL DEFAULT '',
	emotional_trigger TEXT NOT NULL DEFAULT '',
	content_structure TEXT NOT NULL DEFAULT '',
	conversion_intent TEXT NOT NULL DEFAULT '',
	reusable_pattern TEXT NOT NULL DEFAULT '',
	risk_notes TEXT NOT NULL DEFAULT '',
	model_name TEXT NOT NULL DEFAULT '',
	raw_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(item_id)
);

CREATE TABLE IF NOT EXISTS topic_candidates (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	analysis_id INTEGER NOT NULL REFERENCES note_analyses(id),
	topic TEXT NOT NULL DEFAULT '',
	account_fit_score INTEGER NOT NULL DEFAULT 0,
	trend_score INTEGER NOT NULL DEFAULT 0,
	feasibility_score INTEGER NOT NULL DEFAULT 0,
	growth_score INTEGER NOT NULL DEFAULT 0,
	differentiation_score INTEGER NOT NULL DEFAULT 0,
	risk_score INTEGER NOT NULL DEFAULT 0,
	total_score INTEGER NOT NULL DEFAULT 0,
	reason TEXT NOT NULL DEFAULT '',
	suggested_angle TEXT NOT NULL DEFAULT '',
	not_doing TEXT NOT NULL DEFAULT '',
	scoring_model TEXT NOT NULL DEFAULT '',
	raw_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(analysis_id)
);

CREATE TABLE IF NOT EXISTS generated_drafts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_id INTEGER NOT NULL REFERENCES topic_candidates(id),
	title_options_json TEXT NOT NULL DEFAULT '[]',
	opening TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL DEFAULT '',
	cover_text TEXT NOT NULL DEFAULT '',
	image_brief TEXT NOT NULL DEFAULT '',
	tags_json TEXT NOT NULL DEFAULT '[]',
	risk_notes TEXT NOT NULL DEFAULT '',
	generator TEXT NOT NULL DEFAULT '',
	raw_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(candidate_id)
);

CREATE TABLE IF NOT EXISTS publish_records (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	draft_id INTEGER NOT NULL REFERENCES generated_drafts(id),
	platform TEXT NOT NULL DEFAULT 'xiaohongshu',
	note_url TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'published',
	published_at TEXT NOT NULL DEFAULT '',
	operator TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(draft_id, platform)
);

CREATE TABLE IF NOT EXISTS performance_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	publish_record_id INTEGER NOT NULL REFERENCES publish_records(id),
	views INTEGER NOT NULL DEFAULT 0,
	likes INTEGER NOT NULL DEFAULT 0,
	collects INTEGER NOT NULL DEFAULT 0,
	comments INTEGER NOT NULL DEFAULT 0,
	follows INTEGER NOT NULL DEFAULT 0,
	captured_at TEXT NOT NULL,
	raw_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS performance_reports (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	publish_record_id INTEGER NOT NULL REFERENCES publish_records(id),
	snapshot_id INTEGER NOT NULL REFERENCES performance_snapshots(id),
	performance_score INTEGER NOT NULL DEFAULT 0,
	engagement_rate_basis INTEGER NOT NULL DEFAULT 0,
	follow_rate_basis INTEGER NOT NULL DEFAULT 0,
	summary TEXT NOT NULL DEFAULT '',
	suggested_adjustment TEXT NOT NULL DEFAULT '',
	review_model TEXT NOT NULL DEFAULT '',
	raw_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(snapshot_id)
);
`
