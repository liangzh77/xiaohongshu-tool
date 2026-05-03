package xhsnative

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	rodinput "github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
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
	Log          func(string)
}

type NaturalSearchResult struct {
	Items   []storage.Item
	Logs    []string
	Skipped int
	Failed  int
}

type progressLogger func(string, ...any)

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
		opts.DelayMin = 2 * time.Second
	}
	if opts.DelayMax < opts.DelayMin {
		opts.DelayMax = 5 * time.Second
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	page, err := c.getPageLocked()
	if err != nil {
		return NaturalSearchResult{}, err
	}
	if ok, _ := pageHasSearchInput(ctx, page); !ok {
		if err := requireLoggedIn(ctx, page); err != nil {
			if IsQRCodeLoginRequired(err) {
				return NaturalSearchResult{}, fmt.Errorf("%w：%v", ErrQRCodeLoginRequired, err)
			}
			return NaturalSearchResult{}, err
		}
	} else if loggedOut, reason, err := pageHasLoggedOutSignals(ctx, page); err == nil && loggedOut {
		if reason == "" {
			reason = "页面显示未登录"
		}
		return NaturalSearchResult{}, fmt.Errorf("%w：小红书登录态已失效，请在当前页面扫码验证：%s", ErrQRCodeLoginRequired, reason)
	}

	result := NaturalSearchResult{Logs: []string{}}
	logProgress := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		result.Logs = append(result.Logs, line)
		if opts.Log != nil {
			opts.Log(line)
		}
	}
	logProgress("准备搜索 keyword=%q", opts.Keyword)
	feeds, err := searchByTyping(ctx, page, opts.Keyword, logProgress)
	if err != nil {
		if IsQRCodeLoginRequired(err) {
			return result, fmt.Errorf("%w：%v", ErrQRCodeLoginRequired, err)
		}
		return result, err
	}
	if captureQRCodeDataURL(ctx, page, "") != "" {
		logProgress("搜索页出现二维码，暂停采集")
		return result, fmt.Errorf("%w：搜索页出现二维码", ErrQRCodeLoginRequired)
	}
	logProgress("搜索结果 %d 条，检查前 %d 条", len(feeds), minInt(len(feeds), opts.Limit))
	if len(feeds) > opts.Limit {
		feeds = feeds[:opts.Limit]
	}

	for index, feed := range feeds {
		if captureQRCodeDataURL(ctx, page, "") != "" {
			logProgress("第 %d 条前检测到二维码，暂停采集", index+1)
			return result, fmt.Errorf("%w：搜索页出现二维码", ErrQRCodeLoginRequired)
		}
		card := ItemFromFeed(feed)
		if !isLikelyXHSNoteID(feed.ID) {
			result.Skipped++
			logProgress("跳过第 %d 条，非标准笔记卡片：id=%s model_type=%s title=%q", index+1, feed.ID, feed.ModelType, card.Title)
			continue
		}
		if opts.Exists != nil {
			exists, err := opts.Exists(ctx, card.ExternalID)
			if err != nil {
				return result, err
			}
			if exists {
				result.Skipped++
				logProgress("跳过第 %d 条，已采集：%s %q", index+1, card.ExternalID, card.Title)
				continue
			}
		}
		if len(result.Items) > 0 || result.Skipped > 0 {
			shortHumanPause(ctx, logProgress, fmt.Sprintf("打开第 %d 条", index+1))
		} else {
			logProgress("打开第 %d 条：%s %q", index+1, card.ExternalID, card.Title)
		}
		detail, err := feedDetailByClickingSearchResult(ctx, page, opts.Keyword, feed, opts.LoadComments, logProgress)
		if err != nil {
			if IsQRCodeLoginRequired(err) {
				logProgress("第 %d 条触发扫码验证，暂停采集：%v", index+1, err)
				return result, fmt.Errorf("%w：%v", ErrQRCodeLoginRequired, err)
			}
			result.Failed++
			item := MarkDetailFailure(card, err)
			result.Items = append(result.Items, item)
			logProgress("第 %d 条详情失败：%v", index+1, err)
			continue
		}
		item := MergeItem(card, ItemFromDetail(detail))
		result.Items = append(result.Items, item)
		logProgress("第 %d 条详情成功：body_len=%d tags=%s published_at=%s", index+1, len(item.Body), strings.Join(item.Tags, "/"), item.PublishedAt)
		if err := returnToSearchResults(ctx, page, opts.Keyword, logProgress); err != nil {
			logProgress("第 %d 条后返回搜索列表失败：%v", index+1, err)
			return result, err
		}
	}
	if captureQRCodeDataURL(ctx, page, "") != "" {
		logProgress("采集结束前检测到二维码，暂停采集")
		return result, fmt.Errorf("%w：搜索页出现二维码", ErrQRCodeLoginRequired)
	}
	logProgress("完成：新增 %d 条，跳过 %d 条，详情失败 %d 条", len(result.Items), result.Skipped, result.Failed)
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

func searchByTyping(ctx context.Context, page *rod.Page, keyword string, log progressLogger) ([]xiaohongshu.Feed, error) {
	if _, err := findSearchInput(ctx, page); err != nil {
		humanPause(ctx, log, "当前页没有搜索框，打开小红书首页")
		if err := page.Context(ctx).Navigate("https://www.xiaohongshu.com/explore"); err != nil {
			return nil, err
		}
		_ = page.Context(ctx).Timeout(20 * time.Second).WaitStable(900 * time.Millisecond)
		humanPause(ctx, log, "等待首页加载完成")
		if _, err := findSearchInput(ctx, page); err != nil {
			return nil, fmt.Errorf("没有找到搜索输入框: %w", err)
		}
	}

	shortHumanPause(ctx, log, "点击搜索输入框")
	inputEl, err := findSearchInput(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("点击前没有找到搜索输入框: %w", err)
	}
	if err := clickElementCenter(ctx, page, inputEl); err != nil {
		return nil, fmt.Errorf("点击搜索输入框失败: %w", err)
	}
	shortHumanPause(ctx, log, "清空搜索关键词")
	if err := page.Context(ctx).KeyActions().Press(rodinput.ControlLeft).Type(rodinput.KeyA).Do(); err != nil {
		return nil, fmt.Errorf("全选搜索输入框失败: %w", err)
	}
	if err := page.Context(ctx).Keyboard.Type(rodinput.Backspace); err != nil {
		return nil, fmt.Errorf("清空搜索输入框失败: %w", err)
	}
	shortHumanPause(ctx, log, "输入搜索关键词")
	inputEl, err = findSearchInput(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("输入前没有找到搜索输入框: %w", err)
	}
	if err := clickElementCenter(ctx, page, inputEl); err != nil {
		return nil, fmt.Errorf("focus search input before typing failed: %w", err)
	}
	if err := typeTextByKeyEvents(ctx, page, keyword); err != nil {
		return nil, fmt.Errorf("输入搜索关键词失败: %w", err)
	}
	if value, ok := searchInputValue(ctx, page); !ok || strings.TrimSpace(value) != strings.TrimSpace(keyword) {
		log("搜索框输入校验失败，当前值=%q，改用受控输入补写关键词", value)
		if err := setSearchInputValue(ctx, page, keyword); err != nil {
			return nil, fmt.Errorf("补写搜索关键词失败: %w", err)
		}
		if value, ok := searchInputValue(ctx, page); !ok || strings.TrimSpace(value) != strings.TrimSpace(keyword) {
			return nil, fmt.Errorf("搜索关键词没有写入搜索框，当前值=%q，诊断=%s", value, searchInputDiagnostics(ctx, page))
		}
	}
	shortHumanPause(ctx, log, "提交搜索")
	if err := page.Context(ctx).Keyboard.Type(rodinput.Enter); err != nil {
		return nil, fmt.Errorf("提交搜索失败: %w", err)
	}
	if !isSearchResultPage(page) {
		if clicked, err := clickSearchSubmit(ctx, page); err == nil && clicked {
			log("回车后未进入搜索结果页，已点击搜索按钮提交")
		}
	}
	_ = page.Context(ctx).Timeout(30 * time.Second).WaitStable(900 * time.Millisecond)
	humanPause(ctx, log, "等待搜索结果加载")
	feeds, err := extractSearchFeedsFromPage(ctx, page)
	if err != nil {
		return nil, err
	}
	return feeds, nil
}

func findSearchInput(ctx context.Context, page *rod.Page) (*rod.Element, error) {
	const marker = "xhs-collector-search-input"
	res, err := page.Context(ctx).Timeout(5*time.Second).Eval(`(marker) => {
		document.querySelectorAll("[data-xhs-collector-search-input]").forEach((node) => {
			node.removeAttribute("data-xhs-collector-search-input");
		});
		const visible = (node) => {
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width >= 80 && rect.height >= 24 &&
				rect.bottom > 0 && rect.right > 0 &&
				rect.top < window.innerHeight && rect.left < window.innerWidth &&
				style.visibility !== "hidden" && style.display !== "none" && style.opacity !== "0";
		};
		const inputs = Array.from(document.querySelectorAll("input"));
		const scored = inputs
			.filter(visible)
			.map((node) => {
				const text = [
					node.placeholder || "",
					node.getAttribute("aria-label") || "",
					node.className || "",
					node.parentElement?.className || "",
					node.closest("header, nav, .search, [class*='search']")?.className || "",
				].join(" ");
				let score = 0;
				if (/搜索|search/i.test(text)) score += 100;
				if (node.type === "search") score += 40;
				if (node.type === "text" || node.type === "") score += 10;
				const rect = node.getBoundingClientRect();
				if (rect.top < 180) score += 25;
				if (rect.width > 180) score += 15;
				return { node, score };
			})
			.filter((item) => item.score > 0)
			.sort((a, b) => b.score - a.score);
		const topScore = scored[0]?.score;
		const topScored = scored.filter((item) => item.score === topScore);
		const target = topScored.at(-1)?.node;
		if (!target) return "";
		target.setAttribute("data-xhs-collector-search-input", marker);
		window.__xhsCollectorSearchInput = target;
		return [
			target.placeholder || "",
			target.type || "",
			target.className || "",
			Math.round(target.getBoundingClientRect().width) + "x" + Math.round(target.getBoundingClientRect().height),
		].join(" | ");
	}`, marker)
	if err != nil {
		return nil, err
	}
	if res.Value.String() == "" {
		return nil, fmt.Errorf("search input not found")
	}
	return page.Context(ctx).Timeout(2 * time.Second).Element(`[data-xhs-collector-search-input="` + marker + `"]`)
}

func pageHasSearchInput(ctx context.Context, page *rod.Page) (bool, error) {
	res, err := page.Context(ctx).Timeout(2 * time.Second).Eval(`() => {
		const visible = (node) => {
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width >= 80 && rect.height >= 24 &&
				rect.bottom > 0 && rect.right > 0 &&
				rect.top < window.innerHeight && rect.left < window.innerWidth &&
				style.visibility !== "hidden" && style.display !== "none" && style.opacity !== "0";
		};
		return Array.from(document.querySelectorAll("input")).some((node) => {
			const text = [
				node.placeholder || "",
				node.getAttribute("aria-label") || "",
				node.className || "",
				node.parentElement?.className || "",
				node.closest("header, nav, .search, [class*='search']")?.className || "",
			].join(" ");
			return visible(node) && /搜索|search/i.test(text);
		});
	}`)
	if err != nil {
		return false, err
	}
	return res.Value.Bool(), nil
}

func searchInputValue(ctx context.Context, page *rod.Page) (string, bool) {
	res, err := page.Context(ctx).Timeout(2 * time.Second).Eval(`() => {
		const input = document.querySelector("[data-xhs-collector-search-input]") || window.__xhsCollectorSearchInput || document.activeElement;
		return input ? input.value : null;
	}`)
	if err != nil || res.Value.Nil() {
		return "", false
	}
	return res.Value.String(), true
}

func setSearchInputValue(ctx context.Context, page *rod.Page, keyword string) error {
	res, err := page.Context(ctx).Timeout(3*time.Second).Eval(`(keyword) => {
		const input = document.querySelector("[data-xhs-collector-search-input]") || window.__xhsCollectorSearchInput || document.activeElement;
		if (!input) return false;
		input.focus();
		const oldValue = input.value || "";
		const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
		if (setter) setter.call(input, keyword);
		else input.value = keyword;
		if (input._valueTracker) input._valueTracker.setValue(oldValue);
		input.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: keyword }));
		input.dispatchEvent(new Event("change", { bubbles: true }));
		window.__xhsCollectorSearchInput = input;
		return true;
	}`, keyword)
	if err != nil {
		return err
	}
	if !res.Value.Bool() {
		return fmt.Errorf("没有找到已标记的搜索输入框")
	}
	return nil
}

func typeTextByKeyEvents(ctx context.Context, page *rod.Page, text string) error {
	for _, ch := range text {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		s := string(ch)
		event := proto.InputDispatchKeyEvent{
			Type:                  proto.InputDispatchKeyEventTypeKeyDown,
			Key:                   s,
			WindowsVirtualKeyCode: int(ch),
			NativeVirtualKeyCode:  int(ch),
		}
		if err := event.Call(page); err != nil {
			return err
		}
		event.Type = proto.InputDispatchKeyEventTypeChar
		event.Text = s
		event.UnmodifiedText = s
		if err := event.Call(page); err != nil {
			return err
		}
		event.Type = proto.InputDispatchKeyEventTypeKeyUp
		event.Text = ""
		event.UnmodifiedText = ""
		if err := event.Call(page); err != nil {
			return err
		}
		sleepContext(ctx, randomDelay(80*time.Millisecond, 260*time.Millisecond))
	}
	return nil
}

func searchInputDiagnostics(ctx context.Context, page *rod.Page) string {
	res, err := page.Context(ctx).Timeout(3 * time.Second).Eval(`() => {
		const nodes = Array.from(document.querySelectorAll("input, textarea, [contenteditable='true'], [role='textbox']"));
		const active = document.activeElement;
		const summarize = (node, idx) => {
			const rect = node.getBoundingClientRect();
			return {
				idx,
				tag: node.tagName,
				type: node.getAttribute("type") || "",
				placeholder: node.getAttribute("placeholder") || "",
				aria: node.getAttribute("aria-label") || "",
				role: node.getAttribute("role") || "",
				className: String(node.className || "").slice(0, 80),
				value: "value" in node ? node.value : node.textContent,
				active: node === active,
				marked: node.hasAttribute("data-xhs-collector-search-input"),
				rect: [Math.round(rect.left), Math.round(rect.top), Math.round(rect.width), Math.round(rect.height)].join(","),
			};
		};
		return JSON.stringify(nodes.slice(0, 12).map(summarize));
	}`)
	if err != nil {
		return err.Error()
	}
	return res.Value.String()
}

func clickSearchSubmit(ctx context.Context, page *rod.Page) (bool, error) {
	const marker = "xhs-collector-search-submit"
	res, err := page.Context(ctx).Timeout(3*time.Second).Eval(`(marker) => {
		document.querySelectorAll("[data-xhs-collector-search-submit]").forEach((node) => {
			node.removeAttribute("data-xhs-collector-search-submit");
		});
		const input = document.querySelector("[data-xhs-collector-search-input]");
		if (!input) return false;
		const root = input.closest("header, nav, .search, [class*='search']") || document;
		const candidates = Array.from(root.querySelectorAll("button, a, [role='button'], svg, [class*='search']"));
		const visible = (node) => {
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width > 8 && rect.height > 8 && style.visibility !== "hidden" && style.display !== "none";
		};
		const inputRect = input.getBoundingClientRect();
		const scored = candidates
			.filter(visible)
			.map((node) => {
				const rect = node.getBoundingClientRect();
				const text = [node.textContent || "", node.getAttribute("aria-label") || "", node.className || ""].join(" ");
				let score = /搜索|search/i.test(text) ? 100 : 0;
				if (rect.left > inputRect.left) score += 20;
				score -= Math.abs((rect.top + rect.height / 2) - (inputRect.top + inputRect.height / 2));
				return { node, score };
			})
			.filter((item) => item.score > 0)
			.sort((a, b) => b.score - a.score);
		const target = scored[0]?.node;
		if (!target) return false;
		target.setAttribute("data-xhs-collector-search-submit", marker);
		return true;
	}`, marker)
	if err != nil {
		return false, err
	}
	if !res.Value.Bool() {
		return false, nil
	}
	el, err := page.Context(ctx).Timeout(2 * time.Second).Element(`[data-xhs-collector-search-submit="` + marker + `"]`)
	if err != nil {
		return false, err
	}
	if err := clickElementCenter(ctx, page, el); err != nil {
		return false, err
	}
	return true, nil
}

func extractSearchFeedsFromPage(ctx context.Context, page *rod.Page) ([]xiaohongshu.Feed, error) {
	res, err := page.Context(ctx).Timeout(8 * time.Second).Eval(`() => {
		const feeds = window.__INITIAL_STATE__?.search?.feeds;
		const feedsData = feeds?.value !== undefined ? feeds.value : feeds?._value;
		if (!feedsData) return "";
		const noteIDFromHref = (href) => {
			const match = String(href || "").match(/\/explore\/([0-9a-fA-F]{24})/);
			return match ? match[1] : "";
		};
		const visible = (node) => {
			if (!node) return false;
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width > 8 && rect.height > 8 &&
				rect.bottom > 70 && rect.top < window.innerHeight &&
				style.visibility !== "hidden" && style.display !== "none";
		};
		const stateFeeds = Array.from(feedsData);
		const byID = new Map(stateFeeds.map((feed) => [feed.id, feed]));
		const seen = new Set();
		const cards = [];
		for (const link of Array.from(document.querySelectorAll("a[href*='/explore/']"))) {
			const id = noteIDFromHref(link.href);
			if (!id || seen.has(id) || !byID.has(id)) continue;
			const card = link.closest(".note-item, .feed-card, section, [class*='note'], [class*='card']") || link;
			const target = visible(card) ? card : link;
			if (!visible(target)) continue;
			seen.add(id);
			const rect = target.getBoundingClientRect();
			cards.push({ id, top: rect.top, left: rect.left });
		}
		cards.sort((a, b) => (a.top - b.top) || (a.left - b.left));
		const rows = [];
		for (const card of cards) {
			const row = rows.find((item) => Math.abs(item.top - card.top) <= 80);
			if (row) {
				row.top = Math.min(row.top, card.top);
				row.cards.push(card);
			} else {
				rows.push({ top: card.top, cards: [card] });
			}
		}
		rows.sort((a, b) => a.top - b.top);
		const visualIDs = rows.flatMap((row) => row.cards.sort((a, b) => a.left - b.left).map((card) => card.id));
		const ordered = [];
		const used = new Set();
		for (const id of visualIDs) {
			const feed = byID.get(id);
			if (!feed || used.has(id)) continue;
			feed.index = ordered.length;
			ordered.push(feed);
			used.add(id);
		}
		for (const feed of stateFeeds) {
			if (used.has(feed.id)) continue;
			feed.index = ordered.length;
			ordered.push(feed);
			used.add(feed.id);
		}
		return JSON.stringify(ordered);
	}`)
	if err != nil {
		return nil, err
	}
	payload := res.Value.String()
	if payload == "" {
		return nil, fmt.Errorf("没有捕获到搜索结果数据")
	}
	var feeds []xiaohongshu.Feed
	if err := json.Unmarshal([]byte(payload), &feeds); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}
	return feeds, nil
}

func feedDetailByClickingSearchResult(ctx context.Context, page *rod.Page, keyword string, feed xiaohongshu.Feed, _ bool, log progressLogger) (*xiaohongshu.FeedDetailResponse, error) {
	if err := returnToSearchResults(ctx, page, keyword, log); err != nil {
		return nil, err
	}
	el, err := findSearchResultElement(ctx, page, feed, log)
	if err != nil {
		return nil, err
	}
	shortHumanPause(ctx, log, "滚动到目标卡片")
	if err := scrollElementIntoViewByMouse(ctx, page, el); err != nil {
		return nil, fmt.Errorf("滚动到搜索卡片失败: %w", err)
	}
	el, err = findSearchResultElement(ctx, page, feed, log)
	if err != nil {
		return nil, err
	}
	shortHumanPause(ctx, log, "悬停目标卡片")
	if err := moveMouseToElementCenter(ctx, page, el); err != nil {
		return nil, fmt.Errorf("悬停搜索卡片失败: %w", err)
	}
	el, err = findSearchResultElement(ctx, page, feed, log)
	if err != nil {
		return nil, err
	}
	shortHumanPause(ctx, log, "点击目标卡片")
	clickedID := feed.ID
	if id, ok := searchResultElementFeedID(ctx, el); ok && id != "" {
		clickedID = id
		if clickedID != feed.ID {
			log("第 %d 条未按搜索数据 ID 匹配，改点当前列表第 %d 张可见卡片：%s", feed.Index+1, feed.Index+1, clickedID)
		}
	}
	if err := clickElementCenter(ctx, page, el); err != nil {
		return nil, fmt.Errorf("点击搜索卡片失败: %w", err)
	}
	detailID, err := waitForFeedDetailReady(ctx, page, clickedID)
	if err != nil {
		return nil, err
	}
	if detailID != clickedID {
		return nil, fmt.Errorf("点击后打开了错误的卡片：目标=%s 实际=%s", clickedID, detailID)
	}
	return extractFeedDetailFromPage(ctx, page, detailID)
}

func findSearchResultElement(ctx context.Context, page *rod.Page, feed xiaohongshu.Feed, log progressLogger) (*rod.Element, error) {
	marker := "xhs-collector-" + strings.ReplaceAll(feed.ID, "#", "-")
	scrollSearchListTowardTop(ctx, page)
	sleepContext(ctx, randomDelay(300*time.Millisecond, 700*time.Millisecond))
	for attempt := 0; attempt < 8; attempt++ {
		marked, err := markSearchResultElement(ctx, page, feed.ID, feed.Index, marker)
		if err == nil && marked {
			el, err := page.Context(ctx).Timeout(2 * time.Second).Element(`[data-xhs-collector-target="` + marker + `"]`)
			if err == nil {
				return el, nil
			}
		}
		if captureQRCodeDataURL(ctx, page, "") != "" {
			return nil, fmt.Errorf("%w：搜索列表出现二维码", ErrQRCodeLoginRequired)
		}
		if attempt == 0 && feed.Index == 0 {
			log("第一页未匹配到第 1 条，候选诊断：%s", searchResultCandidateDiagnostics(ctx, page))
		}
		scrollSearchList(ctx, page, 420+rand.Intn(220))
		humanPause(ctx, log, fmt.Sprintf("查找第 %d 条卡片", feed.Index+1))
	}
	return nil, fmt.Errorf("搜索列表中未找到第 %d 条卡片：%s", feed.Index+1, feed.ID)
}

func markSearchResultElement(ctx context.Context, page *rod.Page, feedID string, feedIndex int, marker string) (bool, error) {
	res, err := page.Context(ctx).Timeout(3*time.Second).Eval(`(feedID, feedIndex, marker) => {
		document.querySelectorAll("[data-xhs-collector-target]").forEach((node) => {
			node.removeAttribute("data-xhs-collector-target");
			node.removeAttribute("data-xhs-collector-feed-id");
		});
		const visible = (node) => {
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width > 8 && rect.height > 8 &&
				rect.bottom > 70 && rect.top < window.innerHeight &&
				style.visibility !== "hidden" && style.display !== "none";
		};
		const noteIDFromHref = (href) => {
			const match = String(href || "").match(/\/explore\/([0-9a-fA-F]{24})/);
			return match ? match[1] : "";
		};
		const seen = new Set();
		const candidates = [];
		const links = Array.from(document.querySelectorAll("a[href*='/explore/']"));
		for (const link of links) {
			const id = noteIDFromHref(link.href);
			if (!id || seen.has(id)) continue;
			const card = link.closest(".note-item, .feed-card, section, [class*='note'], [class*='card']");
			const target = visible(card) ? card : link;
			if (!visible(target)) continue;
			seen.add(id);
			const rect = target.getBoundingClientRect();
			candidates.push({ node: target, link, id, area: rect.width * rect.height, top: rect.top, left: rect.left });
		}
		candidates.sort((a, b) => (a.top - b.top) || (a.left - b.left) || (b.area - a.area));
		const item = candidates.find((item) => item.id === feedID);
		const target = item?.node;
		if (!target || !item?.id) return false;
		if (item.link) item.link.removeAttribute("target");
		target.setAttribute("data-xhs-collector-target", marker);
		target.setAttribute("data-xhs-collector-feed-id", item.id);
		return true;
	}`, feedID, feedIndex, marker)
	if err != nil {
		return false, err
	}
	return res.Value.Bool(), nil
}

func searchResultElementFeedID(ctx context.Context, el *rod.Element) (string, bool) {
	res, err := el.Context(ctx).Timeout(2 * time.Second).Eval(`() => {
		const attr = this.getAttribute("data-xhs-collector-feed-id");
		if (attr) return attr;
		const href = this.href || this.getAttribute("href") || "";
		const match = String(href).match(/\/explore\/([0-9a-fA-F]{24})/);
		return match ? match[1] : "";
	}`)
	if err != nil || res.Value.Nil() {
		return "", false
	}
	return res.Value.String(), true
}

func searchResultCandidateDiagnostics(ctx context.Context, page *rod.Page) string {
	res, err := page.Context(ctx).Timeout(2 * time.Second).Eval(`() => {
		const visible = (node) => {
			if (!node) return false;
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width > 8 && rect.height > 8 &&
				rect.bottom > 70 && rect.top < window.innerHeight &&
				style.visibility !== "hidden" && style.display !== "none";
		};
		const noteIDFromHref = (href) => {
			const match = String(href || "").match(/\/explore\/([0-9a-fA-F]{24})/);
			return match ? match[1] : "";
		};
		const items = Array.from(document.querySelectorAll("a[href*='/explore/']")).slice(0, 20).map((link) => {
			const card = link.closest(".note-item, .feed-card, section, [class*='note'], [class*='card']");
			const node = visible(card) ? card : link;
			const rect = node.getBoundingClientRect();
			return {
				id: noteIDFromHref(link.href),
				linkVisible: visible(link),
				cardVisible: visible(card),
				rect: [Math.round(rect.left), Math.round(rect.top), Math.round(rect.width), Math.round(rect.height)].join(","),
				text: (node.textContent || "").trim().slice(0, 24)
			};
		});
		return JSON.stringify(items.slice(0, 6));
	}`)
	if err != nil {
		return err.Error()
	}
	return res.Value.String()
}

func clickElementCenter(ctx context.Context, page *rod.Page, el *rod.Element) error {
	if err := moveMouseToElementCenter(ctx, page, el); err != nil {
		return err
	}
	sleepContext(ctx, randomDelay(250*time.Millisecond, 900*time.Millisecond))
	if err := page.Context(ctx).Mouse.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("鼠标点击失败: %w", err)
	}
	return nil
}

func moveMouseToElementCenter(ctx context.Context, page *rod.Page, el *rod.Element) error {
	if err := scrollElementIntoViewByMouse(ctx, page, el); err != nil {
		return fmt.Errorf("滚动到元素失败: %w", err)
	}
	rect, err := elementRect(ctx, el)
	if err != nil {
		return fmt.Errorf("获取元素位置失败: %w", err)
	}
	x := rect.Left + rect.Width/2
	y := rect.Top + rect.Height/2
	mouse := page.Context(ctx).Mouse
	if err := mouse.MoveTo(proto.NewPoint(x, y)); err != nil {
		return fmt.Errorf("移动鼠标失败: %w", err)
	}
	return nil
}

type elementRectInfo struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Height float64 `json:"height"`
	Width  float64 `json:"width"`
	VH     float64 `json:"vh"`
	VW     float64 `json:"vw"`
}

func elementRect(ctx context.Context, el *rod.Element) (elementRectInfo, error) {
	res, err := el.Context(ctx).Timeout(2 * time.Second).Eval(`() => {
		const rect = this.getBoundingClientRect();
		return JSON.stringify({
			top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right,
			height: rect.height, width: rect.width,
			vh: window.innerHeight, vw: window.innerWidth
		});
	}`)
	if err != nil {
		return elementRectInfo{}, err
	}
	var rect elementRectInfo
	if err := json.Unmarshal([]byte(res.Value.String()), &rect); err != nil {
		return elementRectInfo{}, err
	}
	return rect, nil
}

func scrollElementIntoViewByMouse(ctx context.Context, page *rod.Page, el *rod.Element) error {
	for attempt := 0; attempt < 16; attempt++ {
		rect, err := elementRect(ctx, el)
		if err != nil {
			return err
		}
		if rect.Width > 1 && rect.Height > 1 &&
			rect.Top >= 0 && rect.Bottom <= rect.VH-20 &&
			rect.Left >= 0 && rect.Right <= rect.VW {
			return nil
		}
		centerY := rect.Top + rect.Height/2
		targetY := rect.VH * 0.48
		delta := centerY - targetY
		if delta < 0 && delta > -180 {
			delta = -180
		}
		if delta > 0 && delta < 180 {
			delta = 180
		}
		if delta > 900 {
			delta = 900
		}
		if delta < -900 {
			delta = -900
		}
		if err := page.Context(ctx).Mouse.Scroll(0, delta, 5); err != nil {
			return err
		}
		sleepContext(ctx, randomDelay(180*time.Millisecond, 420*time.Millisecond))
	}
	return fmt.Errorf("元素没有滚动到可点击区域")
}

func waitForFeedDetailReady(ctx context.Context, page *rod.Page, feedID string) (string, error) {
	deadline := time.Now().Add(20 * time.Second)
	sawQRCode := false
	for time.Now().Before(deadline) {
		if captureQRCodeDataURL(ctx, page, "") != "" {
			sawQRCode = true
		}
		if detailID, ok, _ := currentFeedDetailStateID(ctx, page, feedID); ok {
			return detailID, nil
		}
		sleepContext(ctx, 500*time.Millisecond)
	}
	if sawQRCode {
		return "", fmt.Errorf("%w：详情页二维码未自动消失", ErrQRCodeLoginRequired)
	}
	return "", fmt.Errorf("等待详情页数据超时：%s", feedID)
}

func currentFeedDetailStateID(ctx context.Context, page *rod.Page, feedID string) (string, bool, error) {
	res, err := page.Context(ctx).Timeout(2*time.Second).Eval(`(feedID) => {
		const map = window.__INITIAL_STATE__?.note?.noteDetailMap || {};
		const ready = (id) => {
			const item = map[id];
			const note = item?.note;
			if (!note || !note.noteId) return false;
			return Boolean(
				note.desc ||
				(note.tagList && note.tagList.length > 0) ||
				note.time ||
				(item.comments?.list && item.comments.list.length > 0)
			);
		};
		const noteID = (id) => map[id]?.note?.noteId || id || "";
		const current = location.pathname.match(/\/explore\/([0-9a-fA-F]{24})/)?.[1] || "";
		if (current && ready(current)) return noteID(current);
		return "";
	}`, feedID)
	if err != nil {
		return "", false, err
	}
	id := res.Value.String()
	return id, id != "", nil
}

func extractFeedDetailFromPage(ctx context.Context, page *rod.Page, feedID string) (*xiaohongshu.FeedDetailResponse, error) {
	res, err := page.Context(ctx).Timeout(5 * time.Second).Eval(`() => {
		const map = window.__INITIAL_STATE__?.note?.noteDetailMap;
		return map ? JSON.stringify(map) : "";
	}`)
	if err != nil {
		return nil, err
	}
	payload := res.Value.String()
	if payload == "" {
		return nil, fmt.Errorf("无法获取详情页初始数据")
	}
	var noteDetailMap map[string]noteDetailState
	if err := json.Unmarshal([]byte(payload), &noteDetailMap); err != nil {
		return nil, fmt.Errorf("解析详情页初始数据失败: %w", err)
	}
	noteDetail, exists := noteDetailMap[feedID]
	if !exists || !noteDetail.Ready() {
		return nil, fmt.Errorf("详情页数据中没有找到笔记：%s", feedID)
	}
	note := noteDetail.Note.FeedDetail
	if len(noteDetail.Note.TagList) > 0 {
		note.Desc = appendMissingTagsToDesc(note.Desc, noteDetail.Note.TagNames())
	}
	return &xiaohongshu.FeedDetailResponse{
		Note:     note,
		Comments: noteDetail.Comments,
	}, nil
}

type noteDetailState struct {
	Note     noteWithTags            `json:"note"`
	Comments xiaohongshu.CommentList `json:"comments"`
}

func (s noteDetailState) Ready() bool {
	return s.Note.NoteID != "" &&
		(s.Note.Desc != "" || len(s.Note.TagList) > 0 || s.Note.Time > 0 || len(s.Comments.List) > 0)
}

type noteWithTags struct {
	xiaohongshu.FeedDetail
	TagList []struct {
		Name string `json:"name"`
	} `json:"tagList"`
}

func (n noteWithTags) TagNames() []string {
	names := make([]string, 0, len(n.TagList))
	for _, tag := range n.TagList {
		name := strings.TrimSpace(tag.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func appendMissingTagsToDesc(desc string, tags []string) string {
	seen := map[string]bool{}
	for _, tag := range extractTags(desc) {
		seen[tag] = true
	}
	var missing []string
	for _, tag := range tags {
		if !seen[tag] {
			missing = append(missing, "#"+tag)
		}
	}
	if len(missing) == 0 {
		return desc
	}
	return strings.TrimSpace(desc + " " + strings.Join(missing, " "))
}

func returnToSearchResults(ctx context.Context, page *rod.Page, keyword string, log progressLogger) error {
	if isSearchResultPage(page) {
		return nil
	}
	humanPause(ctx, log, "点击详情关闭按钮")
	if clicked, err := clickDetailCloseButton(ctx, page); err == nil && clicked {
		_ = page.Context(ctx).Timeout(10 * time.Second).WaitStable(700 * time.Millisecond)
		if isSearchResultPage(page) {
			humanPause(ctx, log, "等待搜索列表恢复")
			return nil
		}
	}
	humanPause(ctx, log, "点击详情外部")
	if err := clickViewport(ctx, page, 0.08, 0.50); err == nil {
		_ = page.Context(ctx).Timeout(10 * time.Second).WaitStable(700 * time.Millisecond)
		if isSearchResultPage(page) {
			humanPause(ctx, log, "等待搜索列表恢复")
			return nil
		}
	}
	humanPause(ctx, log, "使用浏览器后退")
	if err := page.Context(ctx).KeyActions().Press(rodinput.AltLeft).Type(rodinput.ArrowLeft).Do(); err == nil {
		_ = page.Context(ctx).Timeout(10 * time.Second).WaitStable(700 * time.Millisecond)
		if isSearchResultPage(page) {
			humanPause(ctx, log, "等待搜索列表恢复")
			return nil
		}
	}
	if keyword == "" {
		return fmt.Errorf("关闭详情后没有回到搜索列表")
	}
	return fmt.Errorf("关闭详情后没有回到搜索列表：%s", keyword)
}

func clickDetailCloseButton(ctx context.Context, page *rod.Page) (bool, error) {
	const marker = "xhs-collector-detail-close"
	res, err := page.Context(ctx).Timeout(3*time.Second).Eval(`(marker) => {
		document.querySelectorAll("[data-xhs-collector-detail-close]").forEach((node) => {
			node.removeAttribute("data-xhs-collector-detail-close");
		});
		const visible = (node) => {
			if (!node) return false;
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width >= 12 && rect.height >= 12 &&
				rect.bottom > 0 && rect.right > 0 &&
				rect.top < window.innerHeight && rect.left < window.innerWidth &&
				style.visibility !== "hidden" && style.display !== "none" && style.opacity !== "0";
		};
		const textOf = (node) => [
			node.textContent || "",
			node.getAttribute("aria-label") || "",
			node.getAttribute("title") || "",
			node.className || "",
			node.id || ""
		].join(" ");
		const candidates = Array.from(document.querySelectorAll("button, a, [role='button'], div, span"))
			.filter(visible)
			.map((node) => {
				const rect = node.getBoundingClientRect();
				const text = textOf(node);
				let score = 0;
				if (/关闭|close|cancel|dismiss|返回|back/i.test(text)) score += 120;
				if (/(^|\s)(x|×)(\s|$)/i.test((node.textContent || "").trim())) score += 80;
				if (node.querySelector("svg,path")) score += 15;
				if (rect.top < 150 && rect.left < 160) score += 45;
				if (rect.top < 150 && rect.right > window.innerWidth - 180) score += 35;
				if (rect.width <= 96 && rect.height <= 96) score += 20;
				const style = getComputedStyle(node);
				if (style.position === "fixed" || style.position === "sticky") score += 20;
				return { node, score, top: rect.top, left: rect.left };
			})
			.filter((item) => item.score >= 90)
			.sort((a, b) => (b.score - a.score) || (a.top - b.top) || (a.left - b.left));
		const target = candidates[0]?.node;
		if (!target) return false;
		target.setAttribute("data-xhs-collector-detail-close", marker);
		return true;
	}`, marker)
	if err != nil {
		return false, err
	}
	if !res.Value.Bool() {
		return false, nil
	}
	el, err := page.Context(ctx).Timeout(2 * time.Second).Element(`[data-xhs-collector-detail-close="` + marker + `"]`)
	if err != nil {
		return false, err
	}
	if err := clickElementCenter(ctx, page, el); err != nil {
		return false, err
	}
	return true, nil
}

func isSearchResultPage(page *rod.Page) bool {
	info, err := page.Info()
	return err == nil && strings.Contains(info.URL, "/search_result")
}

func scrollSearchListTowardTop(ctx context.Context, page *rod.Page) {
	_ = page.Context(ctx).Keyboard.Type(rodinput.Escape)
	moveMouseToViewport(ctx, page, 0.55, 0.48)
	for i := 0; i < 10; i++ {
		_ = page.Context(ctx).Mouse.Scroll(0, -1200, 6)
		sleepContext(ctx, randomDelay(120*time.Millisecond, 260*time.Millisecond))
	}
	_ = page.Context(ctx).Keyboard.Type(rodinput.Home)
	sleepContext(ctx, randomDelay(180*time.Millisecond, 360*time.Millisecond))
	moveMouseToViewport(ctx, page, 0.55, 0.48)
}

func scrollSearchList(ctx context.Context, page *rod.Page, delta int) {
	_ = page.Context(ctx).Mouse.Scroll(0, float64(delta), 4)
}

func moveMouseToViewport(ctx context.Context, page *rod.Page, xRatio, yRatio float64) {
	res, err := page.Context(ctx).Timeout(2 * time.Second).Eval(`() => JSON.stringify({
		width: window.innerWidth,
		height: window.innerHeight
	})`)
	if err != nil {
		return
	}
	var viewport struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	if err := json.Unmarshal([]byte(res.Value.String()), &viewport); err != nil {
		return
	}
	if viewport.Width <= 0 || viewport.Height <= 0 {
		return
	}
	_ = page.Context(ctx).Mouse.MoveTo(proto.NewPoint(viewport.Width*xRatio, viewport.Height*yRatio))
}

func clickViewport(ctx context.Context, page *rod.Page, xRatio, yRatio float64) error {
	moveMouseToViewport(ctx, page, xRatio, yRatio)
	sleepContext(ctx, randomDelay(250*time.Millisecond, 700*time.Millisecond))
	return page.Context(ctx).Mouse.Click(proto.InputMouseButtonLeft, 1)
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
	loggedIn, err := xiaohongshu.NewLogin(page).CheckLoginStatus(ctx)
	if err != nil || !loggedIn {
		return loggedIn, err
	}
	if loggedOut, _, err := pageHasLoggedOutSignals(ctx, page); err == nil && loggedOut {
		return false, nil
	}
	return true, nil
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
		return fmt.Errorf("小红书登录态已失效，请在当前页面扫码验证")
	}
	if loggedOut, reason, err := pageHasLoggedOutSignals(ctx, page); err == nil && loggedOut {
		if reason == "" {
			reason = "页面显示未登录"
		}
		return fmt.Errorf("小红书登录态已失效，请在当前页面扫码验证：%s", reason)
	}
	return nil
}

func pageHasLoggedOutSignals(ctx context.Context, page *rod.Page) (bool, string, error) {
	res, err := page.Context(ctx).Timeout(3 * time.Second).Eval(`() => {
		const visible = (node) => {
			if (!node) return false;
			const rect = node.getBoundingClientRect();
			const style = getComputedStyle(node);
			return rect.width > 8 && rect.height > 8 &&
				style.visibility !== "hidden" && style.display !== "none" && style.opacity !== "0";
		};
		const hasVisible = (selector) => Array.from(document.querySelectorAll(selector)).some(visible);
		const bodyText = document.body ? document.body.innerText || "" : "";
		const loginInput = Array.from(document.querySelectorAll("input")).find((node) => {
			const placeholder = node.getAttribute("placeholder") || "";
			return visible(node) && /登录探索更多内容/.test(placeholder);
		});
		const loginButton = Array.from(document.querySelectorAll("button, a, div, span")).some((node) => {
			const text = (node.textContent || "").trim();
			return visible(node) && text === "登录";
		});
		const qr = hasVisible(".login-container .qrcode-img, .login-container img, [class*='qrcode'] img, [class*='qr-code'] img, [class*='qrCode'] img, [class*='qrcode'] canvas, [class*='qr-code'] canvas, [class*='qrCode'] canvas, .qrcode-img");
		if (qr) return "二维码登录弹窗";
		if (loginInput) return "未登录搜索框";
		if (/登录后推荐更懂你的笔记|手机号登录|马上登录即可|登录探索更多内容/.test(bodyText)) return "登录弹窗";
		if (loginButton) return "登录按钮";
		return "";
	}`)
	if err != nil {
		return false, "", err
	}
	reason := res.Value.String()
	return reason != "", reason, nil
}

func randomDelay(minDelay, maxDelay time.Duration) time.Duration {
	if maxDelay <= minDelay {
		return minDelay
	}
	delta := maxDelay - minDelay
	return minDelay + time.Duration(rand.Int63n(int64(delta)))
}

func humanPause(ctx context.Context, log progressLogger, reason string) time.Duration {
	delay := 2*time.Second + time.Duration(rand.Int63n(int64(3*time.Second)))
	if log != nil {
		log("等待 %.1fs：%s", delay.Seconds(), reason)
	}
	sleepContext(ctx, delay)
	return delay
}

func shortHumanPause(ctx context.Context, log progressLogger, reason string) time.Duration {
	delay := 500*time.Millisecond + time.Duration(rand.Int63n(int64(1500*time.Millisecond)))
	if log != nil {
		log("等待 %.1fs：%s", delay.Seconds(), reason)
	}
	sleepContext(ctx, delay)
	return delay
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

func isLikelyXHSNoteID(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, ch := range id {
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}
		return false
	}
	return true
}
