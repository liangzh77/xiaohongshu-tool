package xhsnative

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	upstream "github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"

	"xiaohongshu-tool/internal/storage"
)

func ItemFromFeed(feed upstream.Feed) storage.Item {
	return storage.Item{
		ExternalID:    feed.ID,
		URL:           feedURL(feed.ID, feed.XsecToken),
		AuthorName:    firstNonEmpty(feed.NoteCard.User.Nickname, feed.NoteCard.User.NickName),
		Title:         feed.NoteCard.DisplayTitle,
		Tags:          []string{},
		DetailStatus:  "search_only",
		MissingFields: []string{"body", "tags", "published_at"},
		LikeCount:     intPtr(feed.NoteCard.InteractInfo.LikedCount),
		CollectCount:  intPtr(feed.NoteCard.InteractInfo.CollectedCount),
		CommentCount:  intPtr(feed.NoteCard.InteractInfo.CommentCount),
		Raw: map[string]any{
			"source": "xiaohongshu-native",
			"feed":   feed,
		},
	}
}

func ItemFromDetail(detail *upstream.FeedDetailResponse) storage.Item {
	note := detail.Note
	return storage.Item{
		ExternalID:   note.NoteID,
		URL:          feedURL(note.NoteID, note.XsecToken),
		AuthorName:   firstNonEmpty(note.User.Nickname, note.User.NickName),
		Title:        note.Title,
		Body:         StripTags(note.Desc),
		Tags:         extractTags(note.Desc),
		DetailStatus: "succeeded",
		LikeCount:    intPtr(note.InteractInfo.LikedCount),
		CollectCount: intPtr(note.InteractInfo.CollectedCount),
		CommentCount: intPtr(note.InteractInfo.CommentCount),
		PublishedAt:  timeFromMillis(note.Time),
		Raw: map[string]any{
			"source":   "xiaohongshu-native",
			"note":     note,
			"comments": detail.Comments,
		},
	}
}

func MergeItem(base, detail storage.Item) storage.Item {
	if base.ExternalID != "" && detail.ExternalID != "" && base.ExternalID != detail.ExternalID {
		base.DetailStatus = "failed"
		base.DetailMessage = fmt.Sprintf("detail external_id mismatch: search=%s detail=%s", base.ExternalID, detail.ExternalID)
		base.Raw = map[string]any{
			"source": "xiaohongshu-native",
			"search": base.Raw,
			"detail": detail.Raw,
		}
		base.MissingFields = MissingFields(base)
		return base
	}
	if detail.ExternalID != "" && (base.ExternalID == "" || base.ExternalID == detail.ExternalID) {
		base.ExternalID = detail.ExternalID
	}
	if detail.URL != "" && (base.ExternalID == "" || detail.ExternalID == "" || base.ExternalID == detail.ExternalID) {
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
	base.DetailStatus = detail.DetailStatus
	base.DetailMessage = detail.DetailMessage
	base.Raw = map[string]any{
		"source": "xiaohongshu-native",
		"search": base.Raw,
		"detail": detail.Raw,
	}
	base.MissingFields = MissingFields(base)
	return base
}

func MarkDetailFailure(item storage.Item, err error) storage.Item {
	item.DetailStatus = "failed"
	if err != nil {
		item.DetailMessage = err.Error()
	}
	item.MissingFields = MissingFields(item)
	if item.Raw == nil {
		item.Raw = map[string]any{}
	}
	item.Raw["detail_error"] = item.DetailMessage
	return item
}

func MissingFields(item storage.Item) []string {
	var missing []string
	if strings.TrimSpace(item.Body) == "" && len(item.Tags) == 0 {
		missing = append(missing, "body")
	}
	if len(item.Tags) == 0 {
		missing = append(missing, "tags")
	}
	if strings.TrimSpace(item.PublishedAt) == "" {
		missing = append(missing, "published_at")
	}
	return missing
}

func ExtractTags(body string) []string {
	return extractTags(body)
}

func StripTags(body string) string {
	cleaned := topicTagBlockPattern.ReplaceAllString(body, " ")
	cleaned = plainHashTagPattern.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(spacePattern.ReplaceAllString(cleaned, " "))
}

var tagPattern = regexp.MustCompile(`#\s*([\p{Han}\p{L}\p{N}_-]+)`)
var topicTagBlockPattern = regexp.MustCompile(`#\s*[^#\r\n]*?\[话题\]\s*#`)
var plainHashTagPattern = regexp.MustCompile(`#\s*[\p{Han}\p{L}\p{N}_-]+#?`)
var spacePattern = regexp.MustCompile(`[ \t]{2,}`)

func extractTags(body string) []string {
	matches := tagPattern.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	tags := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		tag := strings.TrimSpace(match[1])
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

func intPtr(value string) *int {
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func timeFromMillis(millis int64) string {
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
