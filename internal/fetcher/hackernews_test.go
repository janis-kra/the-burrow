package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHackerNewsFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"hits": [
				{"title": "Post A", "url": "https://a.com", "points": 100, "num_comments": 50, "objectID": "1"},
				{"title": "Post B", "url": "https://b.com", "points": 200, "num_comments": 80, "objectID": "2"},
				{"title": "Post C", "url": "https://c.com", "points": 150, "num_comments": 60, "objectID": "3"},
				{"title": "Post D", "url": "https://d.com", "points": 50, "num_comments": 20, "objectID": "4"},
				{"title": "Post E", "url": "https://e.com", "points": 300, "num_comments": 120, "objectID": "5"},
				{"title": "Post F", "url": "https://f.com", "points": 10, "num_comments": 5, "objectID": "6"}
			]
		}`))
	}))
	defer server.Close()

	hn := NewHackerNews(server.Client())
	hn.baseURL = server.URL

	result, err := hn.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts, ok := result.([]HNPost)
	if !ok {
		t.Fatal("result is not []HNPost")
	}

	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	if posts[0].Points != 300 {
		t.Errorf("expected first post to have 300 points, got %d", posts[0].Points)
	}
	if posts[0].Title != "Post E" {
		t.Errorf("expected first post title 'Post E', got %q", posts[0].Title)
	}
}

func TestHackerNewsCommentsURL(t *testing.T) {
	post := HNPost{ObjectID: "42"}
	expected := "https://news.ycombinator.com/item?id=42"
	if got := post.CommentsURL(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
