package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func atomPost(subreddit, id, title, author string) string {
	return fmt.Sprintf(`<entry xmlns="http://www.w3.org/2005/Atom">
		<title>%s</title>
		<link href="https://www.reddit.com/r/%s/comments/%s/%s/" />
		<author><name>/u/%s</name></author>
		<id>t3_%s</id>
	</entry>`, title, subreddit, id, id, author, id)
}

func atomFeedXML(subreddit string, entries ...string) string {
	body := ""
	for _, e := range entries {
		body += e
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<title>top scoring links : %s</title>
%s
</feed>`, subreddit, body)
}

func TestRedditFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "burrow/1.0 (by /u/kaktus_jack; info@burrow.janiskrasemann.com)" {
			t.Errorf("unexpected User-Agent: %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, atomFeedXML("de",
			atomPost("de", "abc", "Post 1", "user1"),
			atomPost("de", "def", "Post 2", "user2"),
		))
	}))
	defer server.Close()

	reddit := NewReddit(http.DefaultClient, []string{"de"}, "")
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
	if posts[0].Author != "user1" {
		t.Errorf("expected author 'user1', got %q", posts[0].Author)
	}
	if posts[0].FullPermalink() != "https://www.reddit.com/r/de/comments/abc/abc/" {
		t.Errorf("unexpected permalink: %q", posts[0].FullPermalink())
	}
}

func TestRedditMultiSubreddit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		switch r.URL.Path {
		case "/r/golang/top.rss":
			fmt.Fprint(w, atomFeedXML("golang",
				atomPost("golang", "aaa", "Go Post 1", "gopher1"),
				atomPost("golang", "bbb", "Go Post 2", "gopher2"),
				atomPost("golang", "ccc", "Go Post 3", "gopher3"),
			))
		case "/r/rust/top.rss":
			fmt.Fprint(w, atomFeedXML("rust",
				atomPost("rust", "ddd", "Rust Post 1", "rustacean1"),
				atomPost("rust", "eee", "Rust Post 2", "rustacean2"),
			))
		case "/r/python/top.rss":
			fmt.Fprint(w, atomFeedXML("python",
				atomPost("python", "fff", "Python Post 1", "pythonista1"),
			))
		default:
			fmt.Fprint(w, atomFeedXML("unknown"))
		}
	}))
	defer server.Close()

	reddit := NewReddit(http.DefaultClient, []string{"golang", "rust", "python"}, "")
	reddit.baseURL = server.URL

	result, err := reddit.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts, ok := result.([]RedditPost)
	if !ok {
		t.Fatal("result is not []RedditPost")
	}

	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	subredditSeen := make(map[string]bool)
	for _, p := range posts {
		subredditSeen[p.Subreddit] = true
	}
	for _, sub := range []string{"golang", "rust", "python"} {
		if !subredditSeen[sub] {
			t.Errorf("expected at least one post from r/%s", sub)
		}
	}
}

func TestRedditGuaranteeLowScoreSubreddit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		switch r.URL.Path {
		case "/r/popular/top.rss":
			fmt.Fprint(w, atomFeedXML("popular",
				atomPost("popular", "p1", "Popular 1", "u1"),
				atomPost("popular", "p2", "Popular 2", "u2"),
				atomPost("popular", "p3", "Popular 3", "u3"),
				atomPost("popular", "p4", "Popular 4", "u4"),
				atomPost("popular", "p5", "Popular 5", "u5"),
			))
		case "/r/niche/top.rss":
			fmt.Fprint(w, atomFeedXML("niche",
				atomPost("niche", "n1", "Niche 1", "u6"),
			))
		default:
			fmt.Fprint(w, atomFeedXML("unknown"))
		}
	}))
	defer server.Close()

	reddit := NewReddit(http.DefaultClient, []string{"popular", "niche"}, "")
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

	if len(posts) != 5 {
		t.Errorf("expected 5 posts, got %d", len(posts))
	}

	seen := make(map[string]bool)
	for _, p := range posts {
		seen[p.Subreddit] = true
	}
	for _, sub := range order {
		if !seen[sub] {
			t.Errorf("subreddit %q not represented in merged posts", sub)
		}
	}

	for i := 1; i < len(posts); i++ {
		if posts[i].Score > posts[i-1].Score {
			t.Errorf("posts not sorted: %d > %d", posts[i].Score, posts[i-1].Score)
		}
	}
}
