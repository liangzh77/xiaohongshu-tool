package xhsnative

import (
	"testing"

	upstream "github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func TestItemFromFeed(t *testing.T) {
	item := ItemFromFeed(upstream.Feed{
		ID:        "note-1",
		XsecToken: "token-1",
		NoteCard: upstream.NoteCard{
			DisplayTitle: "标题",
			User: upstream.User{
				Nickname: "作者",
			},
			InteractInfo: upstream.InteractInfo{
				LikedCount:     "12",
				CollectedCount: "3",
				CommentCount:   "2",
			},
		},
	})

	if item.ExternalID != "note-1" || item.Title != "标题" || item.AuthorName != "作者" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.LikeCount == nil || *item.LikeCount != 12 {
		t.Fatalf("unexpected like count: %#v", item.LikeCount)
	}
	if item.URL == "" {
		t.Fatal("expected URL to be set")
	}
}

func TestItemFromDetail(t *testing.T) {
	item := ItemFromDetail(&upstream.FeedDetailResponse{
		Note: upstream.FeedDetail{
			NoteID:    "note-2",
			XsecToken: "token-2",
			Title:     "详情标题",
			Desc:      "正文",
			Time:      1702195200000,
			User: upstream.User{
				NickName: "作者B",
			},
			InteractInfo: upstream.InteractInfo{
				LikedCount: "8",
			},
		},
	})

	if item.ExternalID != "note-2" || item.Body != "正文" || item.AuthorName != "作者B" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.PublishedAt != "2023-12-10T08:00:00Z" {
		t.Fatalf("unexpected published_at %q", item.PublishedAt)
	}
}
