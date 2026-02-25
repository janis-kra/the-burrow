package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedditFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "burrow/1.0 (by /u/kaktus_jack; info@burrow.janiskrasemann.com)" {
			t.Errorf("unexpected User-Agent: %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"children": [
					{"data": {"title": "Post 1", "score": 500, "num_comments": 100, "permalink": "/r/de/comments/abc/post_1/", "url": "https://example.com"}},
					{"data": {"title": "Post 2", "score": 300, "num_comments": 50, "permalink": "/r/de/comments/def/post_2/", "url": "https://example2.com"}}
				]
			}
		}`))
	}))
	defer server.Close()

	reddit := NewReddit("de")
	reddit.baseURL = server.URL

	result, err := reddit.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts, ok := result.([]RedditPost)
	if !ok {
		t.Fatal("result is not []RedditPost")
	}

	if len(posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].Title != "Post 1" {
		t.Errorf("expected 'Post 1', got %q", posts[0].Title)
	}
}

func TestRedditPostPermalink(t *testing.T) {
	post := RedditPost{Permalink: "/r/de/comments/abc/test/"}
	expected := "https://www.reddit.com/r/de/comments/abc/test/"
	if got := post.FullPermalink(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
