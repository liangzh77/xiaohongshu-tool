package xhsnative

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"

	"xiaohongshu-tool/internal/storage"
)

type Collector struct {
	headless bool
	binPath  string

	mu      sync.Mutex
	browser *browserInstance
	page    *rod.Page
}

type SearchOptions struct {
	Keyword      string
	Limit        int
	WithDetails  bool
	LoadComments bool
}

type NaturalSearchOptions struct {
	Keyword      string
	Limit        int
	DelayMin     time.Duration
	DelayMax     time.Duration
	LoadComments bool
	Exists       func(context.Context, string) (bool, error)
}

type NaturalSearchResult struct {
	Items   []storage.Item
	Logs    []string
	Skipped int
	Failed  int
}

var ErrQRCodeLoginRequired = errors.New("小红书需要扫码验证")

func IsQRCodeLoginRequired(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrQRCodeLoginRequired) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "Sorry, This Page Isn't Available Right Now") ||
		strings.Contains(message, "请打开小红书App扫码查看") ||
		strings.Contains(message, "小红书登录态已失效")
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
	if c.page != nil {
		_ = c.page.Close()
		c.page = nil
	}
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
	if err := requireLoggedIn(ctx, page); err != nil {
		return nil, err
	}

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
			} else {
				item = MarkDetailFailure(item, err)
			}
		} else {
			item.MissingFields = MissingFields(item)
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Collector) SearchThenOpenDetails(ctx context.Context, opts NaturalSearchOptions) (NaturalSearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 3
	}
	if opts.DelayMin <= 0 {
		opts.DelayMin = 10 * time.Second
	}
	if opts.DelayMax < opts.DelayMin {
		opts.DelayMax = opts.DelayMin
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	page, err := c.getPageLocked()
	if err != nil {
		return NaturalSearchResult{}, err
	}
	if err := requireLoggedIn(ctx, page); err != nil {
		if IsQRCodeLoginRequired(err) {
			return NaturalSearchResult{}, fmt.Errorf("%w：%v", ErrQRCodeLoginRequired, err)
		}
		return NaturalSearchResult{}, err
	}

	result := NaturalSearchResult{Logs: []string{}}
	result.Logs = append(result.Logs, fmt.Sprintf("打开搜索页 keyword=%q", opts.Keyword))
	feeds, err := xiaohongshu.NewSearchAction(page).Search(ctx, opts.Keyword)
	if err != nil {
		if IsQRCodeLoginRequired(err) {
			return result, fmt.Errorf("%w：%v", ErrQRCodeLoginRequired, err)
		}
		return result, err
	}
	if captureQRCodeDataURL(ctx, page, "") != "" {
		result.Logs = append(result.Logs, "搜索页出现二维码，暂停采集")
		return result, fmt.Errorf("%w：搜索页出现二维码", ErrQRCodeLoginRequired)
	}
	result.Logs = append(result.Logs, fmt.Sprintf("搜索结果 %d 条，检查前 %d 条", len(feeds), minInt(len(feeds), opts.Limit)))
	if len(feeds) > opts.Limit {
		feeds = feeds[:opts.Limit]
	}

	for index, feed := range feeds {
		if captureQRCodeDataURL(ctx, page, "") != "" {
			result.Logs = append(result.Logs, fmt.Sprintf("第 %d 条前检测到二维码，暂停采集", index+1))
			return result, fmt.Errorf("%w：搜索页出现二维码", ErrQRCodeLoginRequired)
		}
		card := ItemFromFeed(feed)
		if opts.Exists != nil {
			exists, err := opts.Exists(ctx, card.ExternalID)
			if err != nil {
				return result, err
			}
			if exists {
				result.Skipped++
				result.Logs = append(result.Logs, fmt.Sprintf("跳过第 %d 条，已采集：%s %q", index+1, card.ExternalID, card.Title))
				continue
			}
		}
		delay := randomDelay(opts.DelayMin, opts.DelayMax)
		if len(result.Items) > 0 || result.Skipped > 0 {
			result.Logs = append(result.Logs, fmt.Sprintf("等待 %s 后打开第 %d 条", delay.Round(time.Second), index+1))
			sleepContext(ctx, delay)
		} else {
			result.Logs = append(result.Logs, fmt.Sprintf("打开第 %d 条：%s %q", index+1, card.ExternalID, card.Title))
		}
		detail, err := feedDetailWithPageNoLogin(ctx, page, feed.ID, feed.XsecToken, opts.LoadComments)
		if err != nil {
			if IsQRCodeLoginRequired(err) {
				result.Logs = append(result.Logs, fmt.Sprintf("第 %d 条触发扫码验证，暂停采集：%v", index+1, err))
				return result, fmt.Errorf("%w：%v", ErrQRCodeLoginRequired, err)
			}
			result.Failed++
			item := MarkDetailFailure(card, err)
			result.Items = append(result.Items, item)
			result.Logs = append(result.Logs, fmt.Sprintf("第 %d 条详情失败：%v", index+1, err))
			continue
		}
		item := MergeItem(card, ItemFromDetail(detail))
		result.Items = append(result.Items, item)
		result.Logs = append(result.Logs, fmt.Sprintf("第 %d 条详情成功：body_len=%d tags=%s published_at=%s", index+1, len(item.Body), strings.Join(item.Tags, "/"), item.PublishedAt))
	}
	if captureQRCodeDataURL(ctx, page, "") != "" {
		result.Logs = append(result.Logs, "采集结束前检测到二维码，暂停采集")
		return result, fmt.Errorf("%w：搜索页出现二维码", ErrQRCodeLoginRequired)
	}
	result.Logs = append(result.Logs, fmt.Sprintf("完成：新增 %d 条，跳过 %d 条，详情失败 %d 条", len(result.Items), result.Skipped, result.Failed))
	return result, nil
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
	return feedDetailWithPage(ctx, page, feedID, xsecToken, loadComments)
}

func (c *Collector) FeedDetailPersistent(ctx context.Context, feedID, xsecToken string, loadComments bool) (storage.Item, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	page, err := c.getPageLocked()
	if err != nil {
		return storage.Item{}, err
	}
	return feedDetailWithPage(ctx, page, feedID, xsecToken, loadComments)
}

func (c *Collector) CurrentQRCodeDataURL(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.page == nil {
		return "", fmt.Errorf("当前采集浏览器页不存在")
	}
	qrcode := captureQRCodeDataURL(ctx, c.page, "")
	if qrcode == "" {
		return "", fmt.Errorf("当前采集浏览器页没有找到可截图的二维码")
	}
	return qrcode, nil
}

func (c *Collector) OpenLoginQRCodeDataURL(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	page, err := c.getPageLocked()
	if err != nil {
		return "", err
	}
	pp := page.Context(ctx)
	if err := pp.Navigate("https://www.xiaohongshu.com/explore"); err != nil {
		return "", err
	}
	_ = pp.WaitLoad()
	time.Sleep(2 * time.Second)

	qrcode := captureQRCodeDataURL(ctx, page, "")
	if qrcode == "" {
		_, _ = pp.Eval(`() => {
			const nodes = Array.from(document.querySelectorAll('button, a, div, span'));
			const target = nodes.find((node) => /登录|登入/.test((node.textContent || '').trim()));
			if (target) {
				target.click();
				return true;
			}
			return false;
		}`)
		time.Sleep(2 * time.Second)
		qrcode = captureQRCodeDataURL(ctx, page, "")
	}
	if qrcode == "" {
		return "", fmt.Errorf("主采集浏览器页面没有找到登录二维码")
	}
	return qrcode, nil
}

func (c *Collector) CheckPersistentLoginStatus(ctx context.Context) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	page, err := c.getPageLocked()
	if err != nil {
		return false, err
	}
	return xiaohongshu.NewLogin(page).CheckLoginStatus(ctx)
}

func (c *Collector) WaitForCurrentQRCodeResolved(ctx context.Context, cookiePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.page == nil {
		return fmt.Errorf("当前采集浏览器页不存在")
	}
	if !waitForQRCodeResolved(ctx, c.page) {
		return fmt.Errorf("扫码验证等待超时")
	}
	return savePageCookies(c.page, cookiePath)
}

func feedDetailWithPage(ctx context.Context, page *rod.Page, feedID, xsecToken string, loadComments bool) (storage.Item, error) {
	if err := requireLoggedIn(ctx, page); err != nil {
		return storage.Item{}, err
	}

	detail, err := feedDetailWithPageNoLogin(ctx, page, feedID, xsecToken, loadComments)
	if err != nil {
		return storage.Item{}, err
	}
	item := ItemFromDetail(detail)
	item.MissingFields = MissingFields(item)
	return item, nil
}

func feedDetailWithPageNoLogin(ctx context.Context, page *rod.Page, feedID, xsecToken string, loadComments bool) (*xiaohongshu.FeedDetailResponse, error) {
	detail, err := xiaohongshu.NewFeedDetailAction(page).GetFeedDetailWithConfig(
		ctx,
		feedID,
		xsecToken,
		loadComments,
		xiaohongshu.DefaultCommentLoadConfig(),
	)
	if err != nil {
		return nil, err
	}
	return detail, nil
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

func (c *Collector) getPageLocked() (*rod.Page, error) {
	if c.page != nil {
		return c.page, nil
	}
	b, err := c.getBrowserLocked()
	if err != nil {
		return nil, err
	}
	c.page = b.NewPage()
	return c.page, nil
}

func requireLoggedIn(ctx context.Context, page *rod.Page) error {
	loggedIn, err := xiaohongshu.NewLogin(page).CheckLoginStatus(ctx)
	if err != nil {
		return err
	}
	if !loggedIn {
		return fmt.Errorf("小红书登录态已失效，请先运行：go run ./cmd/xhs-tool login qrcode --cookies data/cookies.json --out data/login-qrcode.html --wait 4m")
	}
	return nil
}

func randomDelay(minDelay, maxDelay time.Duration) time.Duration {
	if maxDelay <= minDelay {
		return minDelay
	}
	delta := maxDelay - minDelay
	return minDelay + time.Duration(rand.Int63n(int64(delta)))
}

func sleepContext(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
