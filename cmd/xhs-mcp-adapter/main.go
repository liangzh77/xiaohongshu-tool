package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"xiaohongshu-tool/internal/xhsmcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	baseURL := flag.String("base-url", "http://localhost:18060", "xiaohongshu-mcp HTTP base URL")
	kind := flag.String("kind", os.Getenv("XHS_TARGET_KIND"), "target kind: keyword or feed")
	keyword := flag.String("keyword", os.Getenv("XHS_TARGET_KEYWORD"), "keyword for keyword targets")
	feedID := flag.String("feed-id", "", "feed id for feed targets")
	xsecToken := flag.String("xsec-token", "", "xsec token for feed detail")
	limit := flag.Int("limit", 5, "maximum items to emit")
	withDetails := flag.Bool("details", true, "call feed detail for search results")
	loadComments := flag.Bool("load-comments", false, "ask xiaohongshu-mcp to load comments when fetching detail")
	timeout := flag.Duration("timeout", 90*time.Second, "HTTP request timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := xhsmcp.NewClient(*baseURL)
	var result xhsmcp.AdapterResult
	var err error

	switch *kind {
	case "keyword":
		if *keyword == "" {
			return fmt.Errorf("keyword target requires --keyword or XHS_TARGET_KEYWORD")
		}
		result, err = client.Search(ctx, xhsmcp.SearchOptions{
			Keyword:      *keyword,
			Limit:        *limit,
			WithDetails:  *withDetails,
			LoadComments: *loadComments,
		})
	case "feed", "note", "note_url":
		if *feedID == "" {
			*feedID = os.Getenv("XHS_TARGET_FEED_ID")
		}
		if *xsecToken == "" {
			*xsecToken = os.Getenv("XHS_TARGET_XSEC_TOKEN")
		}
		if *feedID == "" || *xsecToken == "" {
			return fmt.Errorf("feed target requires --feed-id and --xsec-token")
		}
		result, err = client.FeedDetail(ctx, *feedID, *xsecToken, *loadComments)
	default:
		return fmt.Errorf("unsupported target kind %q", *kind)
	}
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}
