package xhsnative

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func TestSearchThenOpenDetailsExperiment(t *testing.T) {
	if os.Getenv("RUN_XHS_SEARCH_EXPERIMENT") != "1" {
		t.Skip("set RUN_XHS_SEARCH_EXPERIMENT=1 to run")
	}
	keyword := firstNonEmpty(os.Getenv("XHS_EXPERIMENT_KEYWORD"), "特朗普")
	limit := 3
	if value := os.Getenv("XHS_EXPERIMENT_LIMIT"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	headless := os.Getenv("XHS_EXPERIMENT_HEADLESS") == "1"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cookiePath := firstNonEmpty(os.Getenv("XHS_EXPERIMENT_COOKIES"), findExperimentCookies())
	b, err := newBrowserInstance(headless, os.Getenv("ROD_BROWSER_BIN"), cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	page := b.NewPage()
	defer page.Close()

	if err := requireLoggedIn(ctx, page); err != nil {
		t.Fatal(err)
	}
	t.Logf("experiment: keyword=%q limit=%d headless=%v", keyword, limit, headless)
	feeds, err := xiaohongshu.NewSearchAction(page).Search(ctx, keyword)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) > limit {
		feeds = feeds[:limit]
	}
	t.Logf("search_results=%d", len(feeds))

	for i, feed := range feeds {
		if i > 0 {
			delay := time.Duration(10+i*7) * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			t.Logf("sleep_before_next=%s", delay)
			time.Sleep(delay)
		}
		card := ItemFromFeed(feed)
		t.Logf("open_result index=%d id=%s title=%q author=%q url=%s", i+1, card.ExternalID, card.Title, card.AuthorName, card.URL)
		detail, err := xiaohongshu.NewFeedDetailAction(page).GetFeedDetailWithConfig(
			ctx,
			feed.ID,
			feed.XsecToken,
			false,
			xiaohongshu.DefaultCommentLoadConfig(),
		)
		if err != nil {
			t.Logf("detail_failed index=%d id=%s error=%v", i+1, feed.ID, err)
			continue
		}
		item := ItemFromDetail(detail)
		t.Logf("detail_ok index=%d id=%s body_len=%d tags=%v published_at=%s", i+1, item.ExternalID, len(item.Body), item.Tags, item.PublishedAt)
	}
}

func findExperimentCookies() string {
	for _, path := range []string{
		filepath.Join("data", "cookies.json"),
		filepath.Join("..", "..", "data", "cookies.json"),
		filepath.Join("..", "..", "..", "data", "cookies.json"),
	} {
		if _, err := os.Stat(path); err == nil {
			abs, err := filepath.Abs(path)
			if err == nil {
				return abs
			}
			return path
		}
	}
	return filepath.Join("data", "cookies.json")
}
