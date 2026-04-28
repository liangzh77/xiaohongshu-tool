package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"xiaohongshu-tool/internal/collector"
	"xiaohongshu-tool/internal/storage"
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
	default:
		return usage()
	}
}

func usage() error {
	return fmt.Errorf(`usage:
  xhs-tool db init --db data/xhs.db
  xhs-tool target add --db data/xhs.db --kind keyword --name "AI工具" --keyword "AI工具" --interval 5m
  xhs-tool collect once --db data/xhs.db --command "./collector-rpa"
  xhs-tool collect daemon --db data/xhs.db --command "./collector-rpa" --every 5m`)
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
