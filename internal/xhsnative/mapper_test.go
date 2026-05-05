package xhsnative

import (
	"strings"
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
			Desc:      "正文 #AI工具 #小红书运营",
			Time:      1702195200000,
			User: upstream.User{
				NickName: "作者B",
			},
			InteractInfo: upstream.InteractInfo{
				LikedCount: "8",
			},
		},
		Comments: upstream.CommentList{
			List: []upstream.Comment{
				{
					ID:         "comment-1",
					Content:    "有用",
					LikeCount:  "6",
					CreateTime: 1702195200000,
					IPLocation: "广东",
					UserInfo: upstream.User{
						Nickname: "评论者",
					},
				},
			},
		},
	})

	if item.ExternalID != "note-2" || item.Body != "正文" || item.AuthorName != "作者B" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "AI工具" || item.Tags[1] != "小红书运营" {
		t.Fatalf("unexpected tags: %#v", item.Tags)
	}
	if item.DetailStatus != "succeeded" {
		t.Fatalf("unexpected detail status %q", item.DetailStatus)
	}
	if item.PublishedAt != "2023-12-10T08:00:00Z" {
		t.Fatalf("unexpected published_at %q", item.PublishedAt)
	}
	if len(item.Comments) != 1 || item.Comments[0].AuthorName != "评论者" || item.Comments[0].CreatedAt != "2023-12-10 16:00:00" {
		t.Fatalf("unexpected comments: %#v", item.Comments)
	}
}

func TestMergeItemRejectsMismatchedDetailID(t *testing.T) {
	base := ItemFromFeed(upstream.Feed{
		ID: "search-note",
		NoteCard: upstream.NoteCard{
			DisplayTitle: "search title",
		},
	})
	detail := ItemFromDetail(&upstream.FeedDetailResponse{
		Note: upstream.FeedDetail{
			NoteID: "detail-note",
			Title:  "detail title",
			Desc:   "detail body #tag",
		},
	})

	item := MergeItem(base, detail)
	if item.ExternalID != "search-note" {
		t.Fatalf("external id was overwritten: %#v", item)
	}
	if item.Title != "search title" || item.Body != "" || len(item.Tags) != 0 {
		t.Fatalf("mismatched detail fields were merged: %#v", item)
	}
	if item.DetailStatus != "failed" || !strings.Contains(item.DetailMessage, "mismatch") {
		t.Fatalf("expected mismatch failure: %#v", item)
	}
}

func TestStripTagsRemovesXiaohongshuTopicBlocks(t *testing.T) {
	body := "今天分享一个工具 #AIChannel[话题]# #养虾高手大赏[话题]#  #openclaw[话题]# #AI工具[话题]#"
	if got := StripTags(body); got != "今天分享一个工具" {
		t.Fatalf("unexpected stripped body %q", got)
	}
	tags := ExtractTags(body)
	if len(tags) != 4 || tags[0] != "AIChannel" || tags[3] != "AI工具" {
		t.Fatalf("unexpected tags %#v", tags)
	}
}

func TestMarkDetailFailure(t *testing.T) {
	item := MarkDetailFailure(ItemFromFeed(upstream.Feed{
		ID:        "note-1",
		XsecToken: "token-1",
		NoteCard: upstream.NoteCard{
			DisplayTitle: "标题",
		},
	}), assertErr("pc detail unavailable"))

	if item.DetailStatus != "failed" {
		t.Fatalf("unexpected detail status %q", item.DetailStatus)
	}
	if item.DetailMessage != "pc detail unavailable" {
		t.Fatalf("unexpected detail message %q", item.DetailMessage)
	}
	if len(item.MissingFields) == 0 {
		t.Fatalf("expected missing fields")
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
