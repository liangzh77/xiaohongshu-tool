package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"xiaohongshu-tool/internal/storage"
	"xiaohongshu-tool/internal/xhsnative"
)

type adapterResult struct {
	Items []storage.Item `json:"items"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	kind := flag.String("kind", os.Getenv("XHS_TARGET_KIND"), "target kind: keyword or feed")
	keyword := flag.String("keyword", os.Getenv("XHS_TARGET_KEYWORD"), "keyword for keyword targets")
	feedID := flag.String("feed-id", os.Getenv("XHS_TARGET_FEED_ID"), "feed id for feed targets")
	xsecToken := flag.String("xsec-token", os.Getenv("XHS_TARGET_XSEC_TOKEN"), "xsec token for feed detail")
	limit := flag.Int("limit", 5, "maximum items to emit")
	withDetails := flag.Bool("details", true, "fetch detail pages after search")
	loadComments := flag.Bool("load-comments", false, "load comments when fetching details")
	headless := flag.Bool("headless", true, "run Chrome in headless mode")
	binPath := flag.String("bin", os.Getenv("ROD_BROWSER_BIN"), "Chrome/Chromium binary path")
	timeout := flag.Duration("timeout", 3*time.Minute, "collection timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	collector := xhsnative.NewCollector(*headless, *binPath)
	defer collector.Close()

	var result adapterResult
	switch *kind {
	case "keyword":
		if *keyword == "" {
			return fmt.Errorf("keyword target requires --keyword or XHS_TARGET_KEYWORD")
		}
		items, err := collector.Search(ctx, xhsnative.SearchOptions{
			Keyword:      *keyword,
			Limit:        *limit,
			WithDetails:  *withDetails,
			LoadComments: *loadComments,
		})
		if err != nil {
			return err
		}
		result.Items = items
	case "feed", "note", "note_url":
		if *feedID == "" || *xsecToken == "" {
			return fmt.Errorf("feed target requires --feed-id and --xsec-token")
		}
		item, err := collector.FeedDetail(ctx, *feedID, *xsecToken, *loadComments)
		if err != nil {
			return err
		}
		result.Items = []storage.Item{item}
	default:
		return fmt.Errorf("unsupported target kind %q", *kind)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}
