package xhsmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xiaohongshu-tool/internal/storage"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type SearchOptions struct {
	Keyword      string
	Limit        int
	WithDetails  bool
	LoadComments bool
}

type AdapterResult struct {
	Items []storage.Item `json:"items"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Client) Search(ctx context.Context, opts SearchOptions) (AdapterResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 5
	}
	path := "/api/v1/feeds/search?keyword=" + url.QueryEscape(opts.Keyword)
	var envelope envelope
	if err := c.get(ctx, path, &envelope); err != nil {
		return AdapterResult{}, err
	}
	if !envelope.Success {
		return AdapterResult{}, fmt.Errorf("xiaohongshu-mcp search failed: %s", envelope.Error)
	}

	feeds := asSlice(get(envelope.Data, "feeds"))
	if len(feeds) > opts.Limit {
		feeds = feeds[:opts.Limit]
	}

	result := AdapterResult{Items: make([]storage.Item, 0, len(feeds))}
	for _, feedRaw := range feeds {
		feed, ok := feedRaw.(map[string]any)
		if !ok {
			continue
		}
		item := itemFromFeed(feed)
		if opts.WithDetails {
			feedID := stringValue(get(feed, "id"))
			xsecToken := stringValue(get(feed, "xsecToken"))
			if feedID != "" && xsecToken != "" {
				detail, err := c.FeedDetail(ctx, feedID, xsecToken, opts.LoadComments)
				if err == nil && len(detail.Items) > 0 {
					item = mergeItem(item, detail.Items[0])
				}
			}
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (c *Client) FeedDetail(ctx context.Context, feedID, xsecToken string, loadComments bool) (AdapterResult, error) {
	body := map[string]any{
		"feed_id":           feedID,
		"xsec_token":        xsecToken,
		"load_all_comments": loadComments,
	}
	var envelope envelope
	if err := c.postJSON(ctx, "/api/v1/feeds/detail", body, &envelope); err != nil {
		return AdapterResult{}, err
	}
	if !envelope.Success {
		return AdapterResult{}, fmt.Errorf("xiaohongshu-mcp detail failed: %s", envelope.Error)
	}
	data := asMap(get(envelope.Data, "data"))
	note := asMap(get(data, "note"))
	if len(note) == 0 {
		return AdapterResult{}, fmt.Errorf("xiaohongshu-mcp detail response missing note")
	}
	item := itemFromNote(note)
	if comments := asMap(get(data, "comments")); len(comments) > 0 {
		item.Raw["comments"] = comments
	}
	return AdapterResult{Items: []storage.Item{item}}, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s returned %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(data))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", req.Method, req.URL.Path, err)
	}
	return nil
}

func itemFromFeed(feed map[string]any) storage.Item {
	card := asMap(get(feed, "noteCard"))
	interact := asMap(get(card, "interactInfo"))
	user := asMap(get(card, "user"))
	feedID := stringValue(get(feed, "id"))
	xsecToken := stringValue(get(feed, "xsecToken"))
	return storage.Item{
		ExternalID:   feedID,
		URL:          feedURL(feedID, xsecToken),
		AuthorName:   firstNonEmpty(stringValue(get(user, "nickname")), stringValue(get(user, "nickName"))),
		Title:        stringValue(get(card, "displayTitle")),
		Tags:         []string{},
		LikeCount:    intPtr(get(interact, "likedCount")),
		CollectCount: intPtr(get(interact, "collectedCount")),
		CommentCount: intPtr(get(interact, "commentCount")),
		Raw:          map[string]any{"source": "xiaohongshu-mcp", "feed": feed},
	}
}

func itemFromNote(note map[string]any) storage.Item {
	interact := asMap(get(note, "interactInfo"))
	user := asMap(get(note, "user"))
	feedID := firstNonEmpty(stringValue(get(note, "noteId")), stringValue(get(note, "id")))
	xsecToken := stringValue(get(note, "xsecToken"))
	return storage.Item{
		ExternalID:   feedID,
		URL:          feedURL(feedID, xsecToken),
		AuthorName:   firstNonEmpty(stringValue(get(user, "nickname")), stringValue(get(user, "nickName"))),
		Title:        stringValue(get(note, "title")),
		Body:         stringValue(get(note, "desc")),
		Tags:         tagsFromNote(note),
		LikeCount:    intPtr(get(interact, "likedCount")),
		CollectCount: intPtr(get(interact, "collectedCount")),
		CommentCount: intPtr(get(interact, "commentCount")),
		PublishedAt:  timeFromMillis(get(note, "time")),
		Raw:          map[string]any{"source": "xiaohongshu-mcp", "note": note},
	}
}

func mergeItem(base, detail storage.Item) storage.Item {
	if detail.ExternalID != "" {
		base.ExternalID = detail.ExternalID
	}
	if detail.URL != "" {
		base.URL = detail.URL
	}
	if detail.AuthorName != "" {
		base.AuthorName = detail.AuthorName
	}
	if detail.Title != "" {
		base.Title = detail.Title
	}
	if detail.Body != "" {
		base.Body = detail.Body
	}
	if len(detail.Tags) > 0 {
		base.Tags = detail.Tags
	}
	if detail.LikeCount != nil {
		base.LikeCount = detail.LikeCount
	}
	if detail.CollectCount != nil {
		base.CollectCount = detail.CollectCount
	}
	if detail.CommentCount != nil {
		base.CommentCount = detail.CommentCount
	}
	if detail.PublishedAt != "" {
		base.PublishedAt = detail.PublishedAt
	}
	base.Raw = map[string]any{"source": "xiaohongshu-mcp", "search": base.Raw, "detail": detail.Raw}
	return base
}

type envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Code    string `json:"code"`
}

func get(root any, path ...string) any {
	current := root
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return ""
	}
}

func intPtr(v any) *int {
	switch x := v.(type) {
	case float64:
		i := int(x)
		return &i
	case string:
		cleaned := strings.ReplaceAll(strings.TrimSpace(x), ",", "")
		if cleaned == "" {
			return nil
		}
		i, err := strconv.Atoi(cleaned)
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}

func tagsFromNote(note map[string]any) []string {
	tagValues := asSlice(get(note, "tagList"))
	tags := make([]string, 0, len(tagValues))
	for _, raw := range tagValues {
		switch tag := raw.(type) {
		case string:
			tags = append(tags, tag)
		case map[string]any:
			name := firstNonEmpty(stringValue(get(tag, "name")), stringValue(get(tag, "tagName")))
			if name != "" {
				tags = append(tags, name)
			}
		}
	}
	return tags
}

func timeFromMillis(v any) string {
	var millis int64
	switch x := v.(type) {
	case float64:
		millis = int64(x)
	case string:
		parsed, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return ""
		}
		millis = parsed
	default:
		return ""
	}
	if millis <= 0 {
		return ""
	}
	return time.UnixMilli(millis).UTC().Format(time.RFC3339)
}

func feedURL(feedID, xsecToken string) string {
	if feedID == "" {
		return ""
	}
	values := url.Values{}
	if xsecToken != "" {
		values.Set("xsec_token", xsecToken)
	}
	if encoded := values.Encode(); encoded != "" {
		return "https://www.xiaohongshu.com/explore/" + feedID + "?" + encoded
	}
	return "https://www.xiaohongshu.com/explore/" + feedID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
