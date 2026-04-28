package xhsnative

import (
	"net/url"
	"strconv"
	"time"

	upstream "github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"

	"xiaohongshu-tool/internal/storage"
)

func ItemFromFeed(feed upstream.Feed) storage.Item {
	return storage.Item{
		ExternalID:   feed.ID,
		URL:          feedURL(feed.ID, feed.XsecToken),
		AuthorName:   firstNonEmpty(feed.NoteCard.User.Nickname, feed.NoteCard.User.NickName),
		Title:        feed.NoteCard.DisplayTitle,
		Tags:         []string{},
		LikeCount:    intPtr(feed.NoteCard.InteractInfo.LikedCount),
		CollectCount: intPtr(feed.NoteCard.InteractInfo.CollectedCount),
		CommentCount: intPtr(feed.NoteCard.InteractInfo.CommentCount),
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
		Body:         note.Desc,
		Tags:         []string{},
		LikeCount:    intPtr(note.InteractInfo.LikedCount),
		CollectCount: intPtr(note.InteractInfo.CollectedCount),
		CommentCount: intPtr(note.InteractInfo.CommentCount),
		PublishedAt:  timeFromMillis(note.Time),
		Raw: map[string]any{
			"source": "xiaohongshu-native",
			"note":   note,
		},
	}
}

func MergeItem(base, detail storage.Item) storage.Item {
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
	base.Raw = map[string]any{
		"source": "xiaohongshu-native",
		"search": base.Raw,
		"detail": detail.Raw,
	}
	return base
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
