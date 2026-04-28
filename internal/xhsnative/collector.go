package xhsnative

import (
	"context"
	"sync"

	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"

	"xiaohongshu-tool/internal/storage"
)

type Collector struct {
	headless bool
	binPath  string

	mu      sync.Mutex
	browser *browserInstance
}

type SearchOptions struct {
	Keyword      string
	Limit        int
	WithDetails  bool
	LoadComments bool
}

func NewCollector(headless bool, binPath string) *Collector {
	return &Collector{
		headless: headless,
		binPath:  binPath,
	}
}

func (c *Collector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.browser != nil {
		c.browser.Close()
		c.browser = nil
	}
}

func (c *Collector) Search(ctx context.Context, opts SearchOptions) ([]storage.Item, error) {
	if opts.Limit <= 0 {
		opts.Limit = 5
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b, err := c.getBrowserLocked()
	if err != nil {
		return nil, err
	}
	page := b.NewPage()
	defer page.Close()

	feeds, err := xiaohongshu.NewSearchAction(page).Search(ctx, opts.Keyword)
	if err != nil {
		return nil, err
	}
	if len(feeds) > opts.Limit {
		feeds = feeds[:opts.Limit]
	}

	items := make([]storage.Item, 0, len(feeds))
	for _, feed := range feeds {
		item := ItemFromFeed(feed)
		if opts.WithDetails && feed.ID != "" && feed.XsecToken != "" {
			detail, err := xiaohongshu.NewFeedDetailAction(page).GetFeedDetailWithConfig(
				ctx,
				feed.ID,
				feed.XsecToken,
				opts.LoadComments,
				xiaohongshu.DefaultCommentLoadConfig(),
			)
			if err == nil {
				item = MergeItem(item, ItemFromDetail(detail))
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Collector) FeedDetail(ctx context.Context, feedID, xsecToken string, loadComments bool) (storage.Item, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	b, err := c.getBrowserLocked()
	if err != nil {
		return storage.Item{}, err
	}
	page := b.NewPage()
	defer page.Close()

	detail, err := xiaohongshu.NewFeedDetailAction(page).GetFeedDetailWithConfig(
		ctx,
		feedID,
		xsecToken,
		loadComments,
		xiaohongshu.DefaultCommentLoadConfig(),
	)
	if err != nil {
		return storage.Item{}, err
	}
	return ItemFromDetail(detail), nil
}

func (c *Collector) getBrowserLocked() (*browserInstance, error) {
	if c.browser != nil {
		return c.browser, nil
	}
	b, err := newBrowserInstance(c.headless, c.binPath, "")
	if err != nil {
		return nil, err
	}
	c.browser = b
	return c.browser, nil
}
