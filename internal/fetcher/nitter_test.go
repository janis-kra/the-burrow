package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNitterFetch(t *testing.T) {
	now := time.Now()
	rssXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <item>
      <title>User A tweet 1</title>
      <link>https://nitter.net/userA/status/1</link>
      <description><![CDATA[<p>User A tweet 1</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>userA</dc:creator>
    </item>
    <item>
      <title>User A tweet 2</title>
      <link>https://nitter.net/userA/status/2</link>
      <description><![CDATA[<p>User A tweet 2</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>userA</dc:creator>
    </item>
    <item>
      <title>User B tweet 1</title>
      <link>https://nitter.net/userB/status/3</link>
      <description><![CDATA[<p>User B tweet 1</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>userB</dc:creator>
    </item>
  </channel>
</rss>`,
		now.Add(-1*time.Hour).Format(time.RFC1123Z),
		now.Add(-2*time.Hour).Format(time.RFC1123Z),
		now.Add(-3*time.Hour).Format(time.RFC1123Z),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer server.Close()

	n := NewNitter(server.Client(), []string{server.URL}, []string{"userA", "userB"}, 5)

	result, err := n.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts, ok := result.([]NitterPost)
	if !ok {
		t.Fatal("result is not []NitterPost")
	}

	// Both users should be represented (per-user guarantee)
	users := make(map[string]bool)
	for _, p := range posts {
		users[p.Username] = true
	}
	if !users["userA"] {
		t.Error("expected at least one post from userA")
	}
	if !users["userB"] {
		t.Error("expected at least one post from userB")
	}

	// Posts should be sorted by recency (newest first)
	for i := 1; i < len(posts); i++ {
		if posts[i].PubDate.After(posts[i-1].PubDate) {
			t.Errorf("posts not sorted by recency: post[%d] (%v) is after post[%d] (%v)",
				i, posts[i].PubDate, i-1, posts[i-1].PubDate)
		}
	}

	// Status links should be rewritten to x.com
	for _, p := range posts {
		if !strings.HasPrefix(p.Link, "https://x.com/") {
			t.Errorf("expected x.com link, got %q", p.Link)
		}
	}
}

func TestNitterFiltersOldPosts(t *testing.T) {
	now := time.Now()
	rssXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <item>
      <title>Recent tweet</title>
      <link>https://nitter.net/testuser/status/1</link>
      <description><![CDATA[<p>recent</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>testuser</dc:creator>
    </item>
    <item>
      <title>Old tweet</title>
      <link>https://nitter.net/testuser/status/2</link>
      <description><![CDATA[<p>old</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>testuser</dc:creator>
    </item>
  </channel>
</rss>`,
		now.Add(-1*time.Hour).Format(time.RFC1123Z),
		now.Add(-48*time.Hour).Format(time.RFC1123Z),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer server.Close()

	n := NewNitter(server.Client(), []string{server.URL}, []string{"testuser"}, 5)

	result, err := n.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts := result.([]NitterPost)
	if len(posts) != 1 {
		t.Fatalf("expected 1 post (old one filtered), got %d", len(posts))
	}
	if posts[0].Text != "Recent tweet" {
		t.Errorf("expected 'Recent tweet', got %q", posts[0].Text)
	}
}

func TestNitterRetweet(t *testing.T) {
	now := time.Now()
	rssXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <item>
      <title>RT by @other: This is a retweet</title>
      <link>https://nitter.net/testuser/status/1</link>
      <description><![CDATA[<p>rt</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>testuser</dc:creator>
    </item>
  </channel>
</rss>`, now.Add(-1*time.Hour).Format(time.RFC1123Z))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(rssXML))
	}))
	defer server.Close()

	n := NewNitter(server.Client(), []string{server.URL}, []string{"testuser"}, 5)
	result, err := n.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts := result.([]NitterPost)
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if !posts[0].IsRetweet {
		t.Error("expected IsRetweet to be true")
	}
	if posts[0].Text != "This is a retweet" {
		t.Errorf("expected text stripped of RT prefix, got %q", posts[0].Text)
	}
}

func TestNitterReply(t *testing.T) {
	now := time.Now()
	rssXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <item>
      <title>R to @someone: This is a reply</title>
      <link>https://nitter.net/testuser/status/1</link>
      <description><![CDATA[<p>reply</p>]]></description>
      <pubDate>%s</pubDate>
      <dc:creator>testuser</dc:creator>
    </item>
  </channel>
</rss>`, now.Add(-1*time.Hour).Format(time.RFC1123Z))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(rssXML))
	}))
	defer server.Close()

	n := NewNitter(server.Client(), []string{server.URL}, []string{"testuser"}, 5)
	result, err := n.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	posts := result.([]NitterPost)
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if !posts[0].IsReply {
		t.Error("expected IsReply to be true")
	}
	if posts[0].Text != "This is a reply" {
		t.Errorf("expected text stripped of reply prefix, got %q", posts[0].Text)
	}
}

func TestSelectNitterPostsGuaranteesPerUser(t *testing.T) {
	now := time.Now()
	fillCutoff := now.Add(-24 * time.Hour)

	// Prolific user G dominates the last 24h; A/B only have older posts still
	// within the guarantee window. Limit 6 must still include A and B once.
	posts := []NitterPost{
		{Username: "G", Text: "g1", Link: "https://x.com/G/status/1", PubDate: now.Add(-1 * time.Hour)},
		{Username: "G", Text: "g2", Link: "https://x.com/G/status/2", PubDate: now.Add(-2 * time.Hour)},
		{Username: "G", Text: "g3", Link: "https://x.com/G/status/3", PubDate: now.Add(-3 * time.Hour)},
		{Username: "G", Text: "g4", Link: "https://x.com/G/status/4", PubDate: now.Add(-4 * time.Hour)},
		{Username: "G", Text: "g5", Link: "https://x.com/G/status/5", PubDate: now.Add(-5 * time.Hour)},
		{Username: "G", Text: "g6", Link: "https://x.com/G/status/6", PubDate: now.Add(-6 * time.Hour)},
		{Username: "G", Text: "g7", Link: "https://x.com/G/status/7", PubDate: now.Add(-7 * time.Hour)},
		{Username: "A", Text: "a1", Link: "https://x.com/A/status/1", PubDate: now.Add(-36 * time.Hour)},
		{Username: "B", Text: "b1", Link: "https://x.com/B/status/1", PubDate: now.Add(-40 * time.Hour)},
	}

	got := selectNitterPosts(posts, 6, fillCutoff)
	if len(got) != 6 {
		t.Fatalf("expected 6 posts, got %d: %+v", len(got), got)
	}

	users := map[string]int{}
	for _, p := range got {
		users[p.Username]++
	}
	if users["A"] != 1 {
		t.Errorf("expected exactly 1 post from A, got %d", users["A"])
	}
	if users["B"] != 1 {
		t.Errorf("expected exactly 1 post from B, got %d", users["B"])
	}
	if users["G"] != 4 {
		t.Errorf("expected 4 fill posts from G, got %d", users["G"])
	}

	// Newest first
	for i := 1; i < len(got); i++ {
		if got[i].PubDate.After(got[i-1].PubDate) {
			t.Fatalf("not sorted by recency at index %d", i)
		}
	}
}

func TestSelectNitterPostsMoreUsersThanLimit(t *testing.T) {
	now := time.Now()
	posts := []NitterPost{
		{Username: "a", Text: "1", Link: "l1", PubDate: now.Add(-1 * time.Hour)},
		{Username: "b", Text: "2", Link: "l2", PubDate: now.Add(-2 * time.Hour)},
		{Username: "c", Text: "3", Link: "l3", PubDate: now.Add(-3 * time.Hour)},
	}
	got := selectNitterPosts(posts, 2, now.Add(-24*time.Hour))
	// Guarantee wins over limit — every user represented.
	if len(got) != 3 {
		t.Fatalf("expected 3 posts (one per user), got %d", len(got))
	}
}

func TestNitterInstanceFallback(t *testing.T) {
	now := time.Now()
	rssXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <item>
      <title>From fallback</title>
      <link>http://nitter.example/testuser/status/42</link>
      <description><![CDATA[<p>hi</p><img src="/pic/media%%2Fabc.jpg" />]]></description>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, now.Add(-1*time.Hour).Format(time.RFC1123Z))

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	defer dead.Close()

	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer live.Close()

	n := NewNitter(http.DefaultClient, []string{dead.URL, live.URL}, []string{"testuser"}, 5)
	result, err := n.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	posts := result.([]NitterPost)
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if posts[0].Text != "From fallback" {
		t.Errorf("unexpected text %q", posts[0].Text)
	}
	if posts[0].Link != "https://x.com/testuser/status/42" {
		t.Errorf("unexpected link %q", posts[0].Link)
	}
	if len(posts[0].Images) != 1 || posts[0].Images[0] != "https://pbs.twimg.com/media/abc.jpg" {
		t.Errorf("unexpected images %#v", posts[0].Images)
	}
}

func TestNitterRejectsNonRSS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<!DOCTYPE html><html>challenge</html>"))
	}))
	defer server.Close()

	n := NewNitter(server.Client(), []string{server.URL}, []string{"testuser"}, 5)
	result, err := n.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch should not fail hard, got %v", err)
	}
	posts := result.([]NitterPost)
	if len(posts) != 0 {
		t.Fatalf("expected 0 posts from non-RSS body, got %d", len(posts))
	}
}

func TestExtractImages(t *testing.T) {
	html := `<p>Hello</p>
		<img src="https://nitter.net/pic/1.jpg" />
		<img src="https://twemoji.maxcdn.com/v/emoji/1f600.png" />
		<img src="https://nitter.net/pic/media%2Fabc123.jpg" />
		<img src="/pic/https%3A%2F%2Fpbs.twimg.com%2Fmedia%2Fxyz.jpg" />`

	images := extractImages(html, "https://nitter.example")
	if len(images) != 3 {
		t.Fatalf("expected 3 images (emoji filtered), got %d: %#v", len(images), images)
	}
	if images[0] != "https://nitter.net/pic/1.jpg" {
		t.Errorf("expected first image kept absolute, got %q", images[0])
	}
	if images[1] != "https://pbs.twimg.com/media/abc123.jpg" {
		t.Errorf("expected media rewrite, got %q", images[1])
	}
	if images[2] != "https://pbs.twimg.com/media/xyz.jpg" {
		t.Errorf("expected full-url pic rewrite, got %q", images[2])
	}
}

func TestToXStatusLink(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"http://nitter.jaydenha.uk/wesbos/status/2096335785997852949#m", "https://x.com/wesbos/status/2096335785997852949"},
		{"https://nitter.net/user/status/1", "https://x.com/user/status/1"},
		{"https://example.com/not-a-status", "https://example.com/not-a-status"},
	}
	for _, tt := range tests {
		if got := toXStatusLink(tt.in); got != tt.want {
			t.Errorf("toXStatusLink(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseRSSDate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOK bool
	}{
		{"RFC1123Z", "Mon, 24 Feb 2025 10:30:00 +0100", true},
		{"RFC1123", "Mon, 24 Feb 2025 10:30:00 CET", true},
		{"format3", "Mon, 24 Feb 2025 10:30:00 -0700", true},
		{"format4", "Mon, 24 Feb 2025 10:30:00 MST", true},
		{"invalid", "not-a-date", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRSSDate(tt.input)
			if tt.wantOK && result.IsZero() {
				t.Errorf("expected non-zero time for %q", tt.input)
			}
			if !tt.wantOK && !result.IsZero() {
				t.Errorf("expected zero time for %q, got %v", tt.input, result)
			}
		})
	}
}
