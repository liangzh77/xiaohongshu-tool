package xhsmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchWithDetailsMapsItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/feeds/search":
			if r.URL.Query().Get("keyword") != "AI工具" {
				t.Fatalf("unexpected keyword %q", r.URL.Query().Get("keyword"))
			}
			writeJSON(t, w, map[string]any{
				"success": true,
				"data": map[string]any{
					"feeds": []any{
						map[string]any{
							"id":        "note-1",
							"xsecToken": "token-1",
							"noteCard": map[string]any{
								"displayTitle": "搜索标题",
								"user": map[string]any{
									"nickname": "作者A",
								},
								"interactInfo": map[string]any{
									"likedCount":     "12",
									"collectedCount": "3",
									"commentCount":   "2",
								},
							},
						},
					},
				},
			})
		case "/api/v1/feeds/detail":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req["feed_id"] != "note-1" || req["xsec_token"] != "token-1" {
				t.Fatalf("unexpected detail request: %#v", req)
			}
			writeJSON(t, w, map[string]any{
				"success": true,
				"data": map[string]any{
					"feed_id": "note-1",
					"data": map[string]any{
						"note": map[string]any{
							"noteId":    "note-1",
							"xsecToken": "token-1",
							"title":     "详情标题",
							"desc":      "完整正文",
							"time":      float64(1702195200000),
							"user": map[string]any{
								"nickname": "作者B",
							},
							"interactInfo": map[string]any{
								"likedCount":     "15",
								"collectedCount": "5",
								"commentCount":   "4",
							},
							"tagList": []any{
								map[string]any{"name": "AI"},
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Search(context.Background(), SearchOptions{Keyword: "AI工具", Limit: 5, WithDetails: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.ExternalID != "note-1" {
		t.Fatalf("unexpected external id %q", item.ExternalID)
	}
	if item.Title != "详情标题" || item.Body != "完整正文" || item.AuthorName != "作者B" {
		t.Fatalf("detail was not merged: %#v", item)
	}
	if item.PublishedAt != "2023-12-10T08:00:00Z" {
		t.Fatalf("unexpected published_at %q", item.PublishedAt)
	}
	if item.LikeCount == nil || *item.LikeCount != 15 {
		t.Fatalf("unexpected like count: %#v", item.LikeCount)
	}
	if len(item.Tags) != 1 || item.Tags[0] != "AI" {
		t.Fatalf("unexpected tags: %#v", item.Tags)
	}
}

func TestFeedDetailReturnsItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/feeds/detail" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, map[string]any{
			"success": true,
			"data": map[string]any{
				"data": map[string]any{
					"note": map[string]any{
						"noteId": "note-2",
						"title":  "标题",
						"desc":   "正文",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.FeedDetail(context.Background(), "note-2", "token-2", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].ExternalID != "note-2" || result.Items[0].Body != "正文" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
