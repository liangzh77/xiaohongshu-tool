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

	"xiaohongshu-tool/internal/collector"
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
  xhs-tool login qrcode --cookies data/cookies.json --out data/login-qrcode.html`)
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
