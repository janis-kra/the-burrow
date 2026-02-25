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
					{"data": {"title": "Post 1", "score": 500, "num_comments": 100, "permalink": "/r/de/comments/abc/post_1/", "url": "https://example.com", "subreddit": "de"}},
					{"data": {"title": "Post 2", "score": 300, "num_comments": 50, "permalink": "/r/de/comments/def/post_2/", "url": "https://example2.com", "subreddit": "de"}}
				]
			}
		}`))
	}))
	defer server.Close()

	reddit := NewReddit([]string{"de"})
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
	if posts[0].Subreddit != "de" {
		t.Errorf("expected subreddit 'de', got %q", posts[0].Subreddit)
	}
}

func TestRedditMultiSubreddit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/r/golang/top/.json":
			w.Write([]byte(`{
				"data": {
					"children": [
						{"data": {"title": "Go Post 1", "score": 1000, "num_comments": 200, "permalink": "/r/golang/comments/aaa/go_1/", "subreddit": "golang"}},
						{"data": {"title": "Go Post 2", "score": 800, "num_comments": 150, "permalink": "/r/golang/comments/bbb/go_2/", "subreddit": "golang"}},
						{"data": {"title": "Go Post 3", "score": 600, "num_comments": 100, "permalink": "/r/golang/comments/ccc/go_3/", "subreddit": "golang"}}
					]
				}
			}`))
		case "/r/rust/top/.json":
			w.Write([]byte(`{
				"data": {
					"children": [
						{"data": {"title": "Rust Post 1", "score": 900, "num_comments": 180, "permalink": "/r/rust/comments/ddd/rust_1/", "subreddit": "rust"}},
						{"data": {"title": "Rust Post 2", "score": 700, "num_comments": 120, "permalink": "/r/rust/comments/eee/rust_2/", "subreddit": "rust"}}
					]
				}
			}`))
		case "/r/python/top/.json":
			w.Write([]byte(`{
				"data": {
					"children": [
						{"data": {"title": "Python Post 1", "score": 50, "num_comments": 10, "permalink": "/r/python/comments/fff/py_1/", "subreddit": "python"}}
					]
				}
			}`))
		default:
			w.Write([]byte(`{"data": {"children": []}}`))
		}
	}))
	defer server.Close()

	reddit := NewReddit([]string{"golang", "rust", "python"})
	reddit.baseURL = server.URL

	result, err := reddit.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts, ok := result.([]RedditPost)
	if !ok {
		t.Fatal("result is not []RedditPost")
	}

	// With 3 subreddits, total should be max(5, 3) = 5
	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	// Verify at least one post from each subreddit
	subredditSeen := make(map[string]bool)
	for _, p := range posts {
		subredditSeen[p.Subreddit] = true
	}
	for _, sub := range []string{"golang", "rust", "python"} {
		if !subredditSeen[sub] {
			t.Errorf("expected at least one post from r/%s", sub)
		}
	}

	// Verify posts are sorted by score descending
	for i := 1; i < len(posts); i++ {
		if posts[i].Score > posts[i-1].Score {
			t.Errorf("posts not sorted by score: post %d (score %d) > post %d (score %d)",
				i, posts[i].Score, i-1, posts[i-1].Score)
		}
	}
}

func TestRedditGuaranteeLowScoreSubreddit(t *testing.T) {
	// Test that a low-score subreddit still gets at least one post
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/r/popular/top/.json":
			w.Write([]byte(`{
				"data": {
					"children": [
						{"data": {"title": "Popular 1", "score": 10000, "permalink": "/r/popular/1/", "subreddit": "popular"}},
						{"data": {"title": "Popular 2", "score": 9000, "permalink": "/r/popular/2/", "subreddit": "popular"}},
						{"data": {"title": "Popular 3", "score": 8000, "permalink": "/r/popular/3/", "subreddit": "popular"}},
						{"data": {"title": "Popular 4", "score": 7000, "permalink": "/r/popular/4/", "subreddit": "popular"}},
						{"data": {"title": "Popular 5", "score": 6000, "permalink": "/r/popular/5/", "subreddit": "popular"}}
					]
				}
			}`))
		case "/r/niche/top/.json":
			w.Write([]byte(`{
				"data": {
					"children": [
						{"data": {"title": "Niche 1", "score": 5, "permalink": "/r/niche/1/", "subreddit": "niche"}}
					]
				}
			}`))
		default:
			w.Write([]byte(`{"data": {"children": []}}`))
		}
	}))
	defer server.Close()

	reddit := NewReddit([]string{"popular", "niche"})
	reddit.baseURL = server.URL

	result, err := reddit.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts := result.([]RedditPost)

	nicheFound := false
	for _, p := range posts {
		if p.Subreddit == "niche" {
			nicheFound = true
			break
		}
	}
	if !nicheFound {
		t.Error("expected at least one post from r/niche (guaranteed minimum)")
	}
}

func TestRedditPostPermalink(t *testing.T) {
	post := RedditPost{Permalink: "/r/de/comments/abc/test/"}
	expected := "https://www.reddit.com/r/de/comments/abc/test/"
	if got := post.FullPermalink(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMergePosts(t *testing.T) {
	bySubreddit := map[string][]RedditPost{
		"a": {
			{Title: "A1", Score: 100, Permalink: "/a/1", Subreddit: "a"},
			{Title: "A2", Score: 80, Permalink: "/a/2", Subreddit: "a"},
			{Title: "A3", Score: 60, Permalink: "/a/3", Subreddit: "a"},
		},
		"b": {
			{Title: "B1", Score: 90, Permalink: "/b/1", Subreddit: "b"},
			{Title: "B2", Score: 70, Permalink: "/b/2", Subreddit: "b"},
		},
		"c": {
			{Title: "C1", Score: 10, Permalink: "/c/1", Subreddit: "c"},
		},
	}
	order := []string{"a", "b", "c"}

	posts := mergePosts(bySubreddit, order)

	// max(5, 3) = 5 posts
	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	// Verify each subreddit is represented
	seen := make(map[string]bool)
	for _, p := range posts {
		seen[p.Subreddit] = true
	}
	for _, sub := range order {
		if !seen[sub] {
			t.Errorf("subreddit %q not represented in merged posts", sub)
		}
	}

	// Verify sorted by score
	for i := 1; i < len(posts); i++ {
		if posts[i].Score > posts[i-1].Score {
			t.Errorf("posts not sorted: %d > %d", posts[i].Score, posts[i-1].Score)
		}
	}
}
