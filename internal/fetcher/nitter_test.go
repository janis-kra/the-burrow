package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	n := NewNitter(server.Client(), server.URL, []string{"userA", "userB"}, 5)

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

	n := NewNitter(server.Client(), server.URL, []string{"testuser"}, 5)

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

	n := NewNitter(server.Client(), server.URL, []string{"testuser"}, 5)
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

	n := NewNitter(server.Client(), server.URL, []string{"testuser"}, 5)
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

func TestExtractImages(t *testing.T) {
	html := `<p>Hello</p>
		<img src="https://nitter.net/pic/1.jpg" />
		<img src="https://twemoji.maxcdn.com/v/emoji/1f600.png" />
		<img src="https://nitter.net/pic/2.jpg" />`

	images := extractImages(html)
	if len(images) != 2 {
		t.Fatalf("expected 2 images (emoji filtered), got %d", len(images))
	}
	if images[0] != "https://nitter.net/pic/1.jpg" {
		t.Errorf("expected first image URL, got %q", images[0])
	}
	if images[1] != "https://nitter.net/pic/2.jpg" {
		t.Errorf("expected second image URL, got %q", images[1])
	}
}

func TestParseRSSDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
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
