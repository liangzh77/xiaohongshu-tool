package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"xiaohongshu-tool/internal/analyzer"
	"xiaohongshu-tool/internal/collector"
	"xiaohongshu-tool/internal/draftgen"
	"xiaohongshu-tool/internal/reviewer"
	"xiaohongshu-tool/internal/scorer"
	"xiaohongshu-tool/internal/storage"
	"xiaohongshu-tool/internal/xhsnative"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "db":
		return runDB(args[1:])
	case "target":
		return runTarget(args[1:])
	case "collect":
		return runCollect(args[1:])
	case "item":
		return runItem(args[1:])
	case "run":
		return runRun(args[1:])
	case "login":
		return runLogin(args[1:])
	case "analyze":
		return runAnalyze(args[1:])
	case "score":
		return runScore(args[1:])
	case "draft":
		return runDraft(args[1:])
	case "publish":
		return runPublish(args[1:])
	case "review":
		return runReview(args[1:])
	default:
		return usage()
	}
}

func usage() error {
	return fmt.Errorf(`usage:
  xhs-tool db init --db data/xhs.db
  xhs-tool target add --db data/xhs.db --kind keyword --name "AI工具" --keyword "AI工具" --interval 5m
  xhs-tool collect once --db data/xhs.db --command "./collector-rpa"
  xhs-tool collect daemon --db data/xhs.db --command "./collector-rpa" --every 5m
  xhs-tool item list --db data/xhs.db --limit 20
  xhs-tool item show --db data/xhs.db --id 123
  xhs-tool run list --db data/xhs.db --limit 20
  xhs-tool login status --cookies data/cookies.json
  xhs-tool login qrcode --cookies data/cookies.json --out data/login-qrcode.html
  xhs-tool analyze item --db data/xhs.db --id 123
  xhs-tool analyze batch --db data/xhs.db --limit 20
  xhs-tool score batch --db data/xhs.db --limit 20
  xhs-tool score list --db data/xhs.db --limit 20
  xhs-tool draft batch --db data/xhs.db --limit 20 --engine rule
  xhs-tool draft list --db data/xhs.db --limit 20
  xhs-tool publish add --db data/xhs.db --draft-id 1 --url "https://www.xiaohongshu.com/explore/..."
  xhs-tool publish list --db data/xhs.db --limit 20
  xhs-tool review add --db data/xhs.db --publish-id 1 --views 1000 --likes 80 --collects 20 --comments 5 --follows 3
  xhs-tool review list --db data/xhs.db --limit 20
  xhs-tool review score --db data/xhs.db --limit 20
  xhs-tool review report --db data/xhs.db --limit 20`)
}

func runDB(args []string) error {
	if len(args) == 0 || args[0] != "init" {
		return usage()
	}
	fs := flag.NewFlagSet("db init", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	db, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Migrate(context.Background())
}

func runTarget(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return usage()
	}
	fs := flag.NewFlagSet("target add", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	kind := fs.String("kind", "", "target kind: keyword, account, note_url")
	name := fs.String("name", "", "human readable target name")
	url := fs.String("url", "", "target URL when applicable")
	keyword := fs.String("keyword", "", "keyword when kind=keyword")
	interval := fs.Duration("interval", 5*time.Minute, "minimum interval between collection attempts")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	db, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		return err
	}
	id, err := db.AddTarget(context.Background(), storage.Target{
		Kind:               *kind,
		Name:               *name,
		URL:                *url,
		Keyword:            *keyword,
		MinIntervalSeconds: int(interval.Seconds()),
		Enabled:            true,
	})
	if err != nil {
		return err
	}
	fmt.Printf("target added: %d\n", id)
	return nil
}

func runCollect(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "once":
		return collectOnce(args[1:])
	case "daemon":
		return collectDaemon(args[1:])
	default:
		return usage()
	}
}

func collectOnce(args []string) error {
	fs := flag.NewFlagSet("collect once", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	command := fs.String("command", "", "external collector command")
	limit := fs.Int("limit", 1, "maximum due targets to collect")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return withCollector(*dbPath, *command, func(r *collector.Runner) error {
		count, err := r.RunDue(context.Background(), *limit)
		if err != nil {
			return err
		}
		fmt.Printf("collected targets: %d\n", count)
		return nil
	})
}

func collectDaemon(args []string) error {
	fs := flag.NewFlagSet("collect daemon", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	command := fs.String("command", "", "external collector command")
	every := fs.Duration("every", 5*time.Minute, "scheduler tick interval")
	limit := fs.Int("limit", 1, "maximum due targets per tick")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return withCollector(*dbPath, *command, func(r *collector.Runner) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		ticker := time.NewTicker(*every)
		defer ticker.Stop()
		for {
			count, err := r.RunDue(ctx, *limit)
			if err != nil {
				log.Printf("collect tick failed: %v", err)
			} else {
				log.Printf("collected targets: %d", count)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	})
}

func withCollector(dbPath, command string, fn func(*collector.Runner) error) error {
	if command == "" {
		return fmt.Errorf("--command is required")
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		return err
	}
	return fn(collector.NewRunner(db, collector.ExternalCommand{Command: command}))
}

func runItem(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "list":
		return itemList(args[1:])
	case "show":
		return itemShow(args[1:])
	default:
		return usage()
	}
}

func itemList(args []string) error {
	fs := flag.NewFlagSet("item list", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum items to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	items, err := db.ListItems(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(items)
	}
	for _, item := range items {
		fmt.Printf("#%d target=%s external=%s likes=%s collects=%s comments=%s captured=%s\n",
			item.ID,
			item.TargetName,
			item.ExternalID,
			formatIntPtr(item.LikeCount),
			formatIntPtr(item.CollectCount),
			formatIntPtr(item.CommentCount),
			item.CapturedAt,
		)
		fmt.Printf("  title: %s\n", item.Title)
		if item.AuthorName != "" {
			fmt.Printf("  author: %s\n", item.AuthorName)
		}
		if item.URL != "" {
			fmt.Printf("  url: %s\n", item.URL)
		}
	}
	return nil
}

func itemShow(args []string) error {
	fs := flag.NewFlagSet("item show", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	id := fs.Int64("id", 0, "collected item id")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id <= 0 {
		return fmt.Errorf("--id is required")
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	item, err := db.GetItem(context.Background(), *id)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(item)
	}
	fmt.Printf("id: %d\n", item.ID)
	fmt.Printf("target: %s (%d)\n", item.TargetName, item.TargetID)
	fmt.Printf("external_id: %s\n", item.ExternalID)
	fmt.Printf("title: %s\n", item.Title)
	fmt.Printf("author: %s\n", item.AuthorName)
	fmt.Printf("url: %s\n", item.URL)
	fmt.Printf("likes: %s\n", formatIntPtr(item.LikeCount))
	fmt.Printf("collects: %s\n", formatIntPtr(item.CollectCount))
	fmt.Printf("comments: %s\n", formatIntPtr(item.CommentCount))
	fmt.Printf("published_at: %s\n", item.PublishedAt)
	fmt.Printf("captured_at: %s\n", item.CapturedAt)
	if len(item.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(item.Tags, ", "))
	}
	if item.Body != "" {
		fmt.Printf("\n%s\n", item.Body)
	}
	return nil
}

func runRun(args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return usage()
	}
	fs := flag.NewFlagSet("run list", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum runs to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	runs, err := db.ListRuns(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(runs)
	}
	for _, run := range runs {
		fmt.Printf("#%d target=%s mode=%s status=%s started=%s finished=%s\n",
			run.ID, run.TargetName, run.Mode, run.Status, run.StartedAt, run.FinishedAt)
		if run.Message != "" {
			fmt.Printf("  message: %s\n", strings.TrimSpace(run.Message))
		}
	}
	return nil
}

func openMigratedDB(path string) (*storage.DB, error) {
	db, err := storage.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func formatIntPtr(value *int) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}

func runLogin(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "status":
		return loginStatus(args[1:])
	case "qrcode":
		return loginQRCode(args[1:])
	default:
		return usage()
	}
}

func loginStatus(args []string) error {
	fs := flag.NewFlagSet("login status", flag.ExitOnError)
	cookies := fs.String("cookies", "data/cookies.json", "cookies file path")
	binPath := fs.String("bin", os.Getenv("ROD_BROWSER_BIN"), "Chrome/Chromium binary path")
	headless := fs.Bool("headless", true, "run Chrome in headless mode")
	timeout := fs.Duration("timeout", 30*time.Second, "login status timeout")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	manager := xhsnative.NewSessionManager(*headless, *binPath, *cookies)
	loggedIn, err := manager.CheckLoginStatus(ctx)
	if err != nil {
		return err
	}
	result := map[string]any{
		"logged_in": loggedIn,
		"cookies":   *cookies,
	}
	if *asJSON {
		return printJSON(result)
	}
	fmt.Printf("logged_in: %v\n", loggedIn)
	fmt.Printf("cookies: %s\n", *cookies)
	return nil
}

func loginQRCode(args []string) error {
	fs := flag.NewFlagSet("login qrcode", flag.ExitOnError)
	cookies := fs.String("cookies", "data/cookies.json", "cookies file path")
	out := fs.String("out", "data/login-qrcode.html", "HTML file to write QR code into")
	binPath := fs.String("bin", os.Getenv("ROD_BROWSER_BIN"), "Chrome/Chromium binary path")
	headless := fs.Bool("headless", true, "run Chrome in headless mode")
	wait := fs.Duration("wait", 4*time.Minute, "how long to wait for QR scan")
	timeout := fs.Duration("timeout", 5*time.Minute, "overall command timeout")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	manager := xhsnative.NewSessionManager(*headless, *binPath, *cookies)
	result, err := manager.LoginWithQRCode(ctx, *out, *wait, func(path string) {
		if !*asJSON {
			fmt.Printf("qrcode: %s\n", path)
			fmt.Println("请打开这个 HTML 文件，用小红书 App 扫码。命令会继续等待扫码完成。")
		}
	})
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(result)
	}
	if result.AlreadyLoggedIn {
		fmt.Println("already logged in")
	}
	fmt.Printf("saved_cookies: %v\n", result.SavedCookies)
	fmt.Printf("cookies: %s\n", result.CookiePath)
	return nil
}

func runAnalyze(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "item":
		return analyzeItem(args[1:])
	case "batch":
		return analyzeBatch(args[1:])
	default:
		return usage()
	}
}

func analyzeItem(args []string) error {
	fs := flag.NewFlagSet("analyze item", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	id := fs.Int64("id", 0, "collected item id")
	engine := fs.String("engine", "rule", "analysis engine: rule or llm")
	llmBaseURL := fs.String("llm-base-url", getenvDefault("XHS_LLM_BASE_URL", "https://api.openai.com/v1"), "OpenAI-compatible base URL")
	llmAPIKey := fs.String("llm-api-key", os.Getenv("XHS_LLM_API_KEY"), "LLM API key")
	llmModel := fs.String("llm-model", os.Getenv("XHS_LLM_MODEL"), "LLM model")
	timeout := fs.Duration("timeout", 90*time.Second, "analysis timeout")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id <= 0 {
		return fmt.Errorf("--id is required")
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	item, err := db.GetItem(context.Background(), *id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	analysis, err := analyzeStoredItem(ctx, item, analysisConfig{
		Engine:     *engine,
		LLMBaseURL: *llmBaseURL,
		LLMAPIKey:  *llmAPIKey,
		LLMModel:   *llmModel,
	})
	if err != nil {
		return err
	}
	analysisID, err := db.SaveNoteAnalysis(context.Background(), analysis)
	if err != nil {
		return err
	}
	analysis.ID = analysisID
	if *asJSON {
		return printJSON(analysis)
	}
	printAnalysis(analysis)
	return nil
}

func analyzeBatch(args []string) error {
	fs := flag.NewFlagSet("analyze batch", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum items to analyze")
	engine := fs.String("engine", "rule", "analysis engine: rule or llm")
	llmBaseURL := fs.String("llm-base-url", getenvDefault("XHS_LLM_BASE_URL", "https://api.openai.com/v1"), "OpenAI-compatible base URL")
	llmAPIKey := fs.String("llm-api-key", os.Getenv("XHS_LLM_API_KEY"), "LLM API key")
	llmModel := fs.String("llm-model", os.Getenv("XHS_LLM_MODEL"), "LLM model")
	timeout := fs.Duration("timeout", 5*time.Minute, "batch analysis timeout")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	items, err := db.ListItems(context.Background(), *limit)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	config := analysisConfig{
		Engine:     *engine,
		LLMBaseURL: *llmBaseURL,
		LLMAPIKey:  *llmAPIKey,
		LLMModel:   *llmModel,
	}
	analyses := make([]storage.NoteAnalysis, 0, len(items))
	for _, item := range items {
		analysis, err := analyzeStoredItem(ctx, item, config)
		if err != nil {
			return err
		}
		analysisID, err := db.SaveNoteAnalysis(context.Background(), analysis)
		if err != nil {
			return err
		}
		analysis.ID = analysisID
		analyses = append(analyses, analysis)
	}
	if *asJSON {
		return printJSON(analyses)
	}
	for _, analysis := range analyses {
		printAnalysis(analysis)
		fmt.Println()
	}
	return nil
}

func printAnalysis(analysis storage.NoteAnalysis) {
	fmt.Printf("analysis_id: %d\n", analysis.ID)
	fmt.Printf("item_id: %d\n", analysis.ItemID)
	fmt.Printf("topic: %s\n", analysis.Topic)
	fmt.Printf("audience_pain: %s\n", analysis.AudiencePain)
	fmt.Printf("title_hook: %s\n", analysis.TitleHook)
	fmt.Printf("opening_hook: %s\n", analysis.OpeningHook)
	fmt.Printf("emotional_trigger: %s\n", analysis.EmotionalTrigger)
	fmt.Printf("content_structure: %s\n", analysis.ContentStructure)
	fmt.Printf("conversion_intent: %s\n", analysis.ConversionIntent)
	fmt.Printf("reusable_pattern: %s\n", analysis.ReusablePattern)
	fmt.Printf("risk_notes: %s\n", analysis.RiskNotes)
}

type analysisConfig struct {
	Engine     string
	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string
}

func analyzeStoredItem(ctx context.Context, item storage.StoredItem, config analysisConfig) (storage.NoteAnalysis, error) {
	switch config.Engine {
	case "rule":
		return analyzer.NewRuleAnalyzer().Analyze(item), nil
	case "llm":
		return analyzer.LLMAnalyzer{
			BaseURL: config.LLMBaseURL,
			APIKey:  config.LLMAPIKey,
			Model:   config.LLMModel,
		}.Analyze(ctx, item)
	default:
		return storage.NoteAnalysis{}, fmt.Errorf("unsupported analyzer engine %q", config.Engine)
	}
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func runScore(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "batch":
		return scoreBatch(args[1:])
	case "list":
		return scoreList(args[1:])
	default:
		return usage()
	}
}

func scoreBatch(args []string) error {
	fs := flag.NewFlagSet("score batch", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum analyses to score")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	analyses, err := db.ListNoteAnalyses(context.Background(), *limit)
	if err != nil {
		return err
	}
	ruleScorer := scorer.NewRuleScorer()
	candidates := make([]storage.TopicCandidate, 0, len(analyses))
	for _, analysis := range analyses {
		candidate := ruleScorer.Score(analysis)
		id, err := db.SaveTopicCandidate(context.Background(), candidate)
		if err != nil {
			return err
		}
		candidate.ID = id
		candidates = append(candidates, candidate)
	}
	if *asJSON {
		return printJSON(candidates)
	}
	for _, candidate := range candidates {
		printCandidate(candidate)
		fmt.Println()
	}
	return nil
}

func scoreList(args []string) error {
	fs := flag.NewFlagSet("score list", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum candidates to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	candidates, err := db.ListTopicCandidates(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(candidates)
	}
	for _, candidate := range candidates {
		printCandidate(candidate)
		fmt.Println()
	}
	return nil
}

func printCandidate(candidate storage.TopicCandidate) {
	fmt.Printf("candidate_id: %d\n", candidate.ID)
	fmt.Printf("analysis_id: %d\n", candidate.AnalysisID)
	fmt.Printf("topic: %s\n", candidate.Topic)
	fmt.Printf("total_score: %d\n", candidate.TotalScore)
	fmt.Printf("account_fit: %d trend: %d feasibility: %d growth: %d differentiation: %d risk: %d\n",
		candidate.AccountFitScore,
		candidate.TrendScore,
		candidate.FeasibilityScore,
		candidate.GrowthScore,
		candidate.Differentiation,
		candidate.RiskScore,
	)
	fmt.Printf("reason: %s\n", candidate.Reason)
	fmt.Printf("suggested_angle: %s\n", candidate.SuggestedAngle)
	fmt.Printf("not_doing: %s\n", candidate.NotDoing)
}

func runDraft(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "batch":
		return draftBatch(args[1:])
	case "list":
		return draftList(args[1:])
	default:
		return usage()
	}
}

func draftBatch(args []string) error {
	fs := flag.NewFlagSet("draft batch", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum candidates to draft")
	engine := fs.String("engine", "rule", "draft engine: rule or llm")
	llmBaseURL := fs.String("llm-base-url", getenvDefault("XHS_LLM_BASE_URL", "https://api.openai.com/v1"), "OpenAI-compatible base URL")
	llmAPIKey := fs.String("llm-api-key", os.Getenv("XHS_LLM_API_KEY"), "LLM API key")
	llmModel := fs.String("llm-model", os.Getenv("XHS_LLM_MODEL"), "LLM model")
	timeout := fs.Duration("timeout", 5*time.Minute, "batch draft timeout")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	candidates, err := db.ListTopicCandidates(context.Background(), *limit)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	config := draftConfig{
		Engine:     *engine,
		LLMBaseURL: *llmBaseURL,
		LLMAPIKey:  *llmAPIKey,
		LLMModel:   *llmModel,
	}
	drafts := make([]storage.GeneratedDraft, 0, len(candidates))
	for _, candidate := range candidates {
		draft, err := generateDraft(ctx, candidate, config)
		if err != nil {
			return err
		}
		id, err := db.SaveGeneratedDraft(context.Background(), draft)
		if err != nil {
			return err
		}
		draft.ID = id
		drafts = append(drafts, draft)
	}
	if *asJSON {
		return printJSON(drafts)
	}
	for _, draft := range drafts {
		printDraft(draft)
		fmt.Println()
	}
	return nil
}

type draftConfig struct {
	Engine     string
	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string
}

func generateDraft(ctx context.Context, candidate storage.TopicCandidate, config draftConfig) (storage.GeneratedDraft, error) {
	switch config.Engine {
	case "rule":
		return draftgen.NewRuleGenerator().Generate(candidate), nil
	case "llm":
		return draftgen.LLMGenerator{
			BaseURL: config.LLMBaseURL,
			APIKey:  config.LLMAPIKey,
			Model:   config.LLMModel,
		}.Generate(ctx, candidate)
	default:
		return storage.GeneratedDraft{}, fmt.Errorf("unsupported draft engine %q", config.Engine)
	}
}

func draftList(args []string) error {
	fs := flag.NewFlagSet("draft list", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum drafts to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	drafts, err := db.ListGeneratedDrafts(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(drafts)
	}
	for _, draft := range drafts {
		printDraft(draft)
		fmt.Println()
	}
	return nil
}

func printDraft(draft storage.GeneratedDraft) {
	fmt.Printf("draft_id: %d\n", draft.ID)
	fmt.Printf("candidate_id: %d\n", draft.CandidateID)
	fmt.Println("title_options:")
	for _, title := range draft.TitleOptions {
		fmt.Printf("- %s\n", title)
	}
	fmt.Printf("opening: %s\n", draft.Opening)
	fmt.Printf("cover_text: %s\n", draft.CoverText)
	fmt.Printf("image_brief: %s\n", draft.ImageBrief)
	fmt.Printf("tags: %s\n", strings.Join(draft.Tags, ", "))
	fmt.Printf("risk_notes: %s\n", draft.RiskNotes)
	if draft.Body != "" {
		fmt.Printf("body:\n%s\n", draft.Body)
	}
}

func runPublish(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "add":
		return publishAdd(args[1:])
	case "list":
		return publishList(args[1:])
	default:
		return usage()
	}
}

func publishAdd(args []string) error {
	fs := flag.NewFlagSet("publish add", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	draftID := fs.Int64("draft-id", 0, "generated draft id")
	platform := fs.String("platform", "xiaohongshu", "publish platform")
	url := fs.String("url", "", "published note URL")
	status := fs.String("status", "published", "publish status")
	publishedAt := fs.String("published-at", time.Now().Format(time.RFC3339), "published time")
	operator := fs.String("operator", "", "operator name")
	notes := fs.String("notes", "", "review notes")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *draftID <= 0 {
		return fmt.Errorf("--draft-id is required")
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	record := storage.PublishRecord{
		DraftID:     *draftID,
		Platform:    *platform,
		NoteURL:     *url,
		Status:      *status,
		PublishedAt: *publishedAt,
		Operator:    *operator,
		Notes:       *notes,
	}
	id, err := db.SavePublishRecord(context.Background(), record)
	if err != nil {
		return err
	}
	record.ID = id
	if *asJSON {
		return printJSON(record)
	}
	printPublishRecord(record)
	return nil
}

func publishList(args []string) error {
	fs := flag.NewFlagSet("publish list", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum publish records to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	records, err := db.ListPublishRecords(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(records)
	}
	for _, record := range records {
		printPublishRecord(record)
		fmt.Println()
	}
	return nil
}

func printPublishRecord(record storage.PublishRecord) {
	fmt.Printf("publish_id: %d\n", record.ID)
	fmt.Printf("draft_id: %d\n", record.DraftID)
	fmt.Printf("platform: %s\n", record.Platform)
	fmt.Printf("status: %s\n", record.Status)
	fmt.Printf("published_at: %s\n", record.PublishedAt)
	if record.NoteURL != "" {
		fmt.Printf("url: %s\n", record.NoteURL)
	}
	if record.Operator != "" {
		fmt.Printf("operator: %s\n", record.Operator)
	}
	if record.Notes != "" {
		fmt.Printf("notes: %s\n", record.Notes)
	}
}

func runReview(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "add":
		return reviewAdd(args[1:])
	case "list":
		return reviewList(args[1:])
	case "score":
		return reviewScore(args[1:])
	case "report":
		return reviewReport(args[1:])
	default:
		return usage()
	}
}

func reviewAdd(args []string) error {
	fs := flag.NewFlagSet("review add", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	publishID := fs.Int64("publish-id", 0, "publish record id")
	views := fs.Int("views", 0, "view count")
	likes := fs.Int("likes", 0, "like count")
	collects := fs.Int("collects", 0, "collect count")
	comments := fs.Int("comments", 0, "comment count")
	follows := fs.Int("follows", 0, "follow count")
	capturedAt := fs.String("captured-at", time.Now().Format(time.RFC3339), "snapshot time")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *publishID <= 0 {
		return fmt.Errorf("--publish-id is required")
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	snapshot := storage.PerformanceSnapshot{
		PublishRecordID: *publishID,
		Views:           *views,
		Likes:           *likes,
		Collects:        *collects,
		Comments:        *comments,
		Follows:         *follows,
		CapturedAt:      *capturedAt,
	}
	id, err := db.SavePerformanceSnapshot(context.Background(), snapshot)
	if err != nil {
		return err
	}
	snapshot.ID = id
	if *asJSON {
		return printJSON(snapshot)
	}
	printPerformanceSnapshot(snapshot)
	return nil
}

func reviewList(args []string) error {
	fs := flag.NewFlagSet("review list", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum performance snapshots to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	snapshots, err := db.ListPerformanceSnapshots(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(snapshots)
	}
	for _, snapshot := range snapshots {
		printPerformanceSnapshot(snapshot)
		fmt.Println()
	}
	return nil
}

func printPerformanceSnapshot(snapshot storage.PerformanceSnapshot) {
	fmt.Printf("snapshot_id: %d\n", snapshot.ID)
	fmt.Printf("publish_id: %d\n", snapshot.PublishRecordID)
	fmt.Printf("views: %d likes: %d collects: %d comments: %d follows: %d\n",
		snapshot.Views,
		snapshot.Likes,
		snapshot.Collects,
		snapshot.Comments,
		snapshot.Follows,
	)
	fmt.Printf("captured_at: %s\n", snapshot.CapturedAt)
}

func reviewScore(args []string) error {
	fs := flag.NewFlagSet("review score", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum performance snapshots to score")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	snapshots, err := db.ListPerformanceSnapshots(context.Background(), *limit)
	if err != nil {
		return err
	}
	ruleReviewer := reviewer.NewRuleReviewer()
	reports := make([]storage.PerformanceReport, 0, len(snapshots))
	for _, snapshot := range snapshots {
		report := ruleReviewer.Review(snapshot)
		id, err := db.SavePerformanceReport(context.Background(), report)
		if err != nil {
			return err
		}
		report.ID = id
		reports = append(reports, report)
	}
	if *asJSON {
		return printJSON(reports)
	}
	for _, report := range reports {
		printPerformanceReport(report)
		fmt.Println()
	}
	return nil
}

func reviewReport(args []string) error {
	fs := flag.NewFlagSet("review report", flag.ExitOnError)
	dbPath := fs.String("db", "data/xhs.db", "SQLite database path")
	limit := fs.Int("limit", 20, "maximum performance reports to list")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openMigratedDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	reports, err := db.ListPerformanceReports(context.Background(), *limit)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(reports)
	}
	for _, report := range reports {
		printPerformanceReport(report)
		fmt.Println()
	}
	return nil
}

func printPerformanceReport(report storage.PerformanceReport) {
	fmt.Printf("report_id: %d\n", report.ID)
	fmt.Printf("publish_id: %d\n", report.PublishRecordID)
	fmt.Printf("snapshot_id: %d\n", report.SnapshotID)
	fmt.Printf("performance_score: %d\n", report.PerformanceScore)
	fmt.Printf("engagement_rate_basis: %d\n", report.EngagementRateBasis)
	fmt.Printf("follow_rate_basis: %d\n", report.FollowRateBasis)
	fmt.Printf("summary: %s\n", report.Summary)
	fmt.Printf("suggested_adjustment: %s\n", report.SuggestedAdjustment)
}
