package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RedditPost struct {
	Title       string
	Score       int
	NumComments int
	Permalink   string
	URL         string
	Author      string
	Selftext    string
	Subreddit   string
}

func (p RedditPost) FullPermalink() string {
	return "https://www.reddit.com" + p.Permalink
}

type atomFeed struct {
	Entries []atomEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

type atomEntry struct {
	Title  string     `xml:"http://www.w3.org/2005/Atom title"`
	Link   atomLink   `xml:"http://www.w3.org/2005/Atom link"`
	Author atomAuthor `xml:"http://www.w3.org/2005/Atom author"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
}

type atomAuthor struct {
	Name string `xml:"http://www.w3.org/2005/Atom name"`
}

// pacer enforces a minimum interval between requests. Reddit rate-limits
// unauthenticated RSS requests per IP, so all Reddit fetcher instances share
// one pacer to avoid bursts when multiple fetchers run concurrently.
type pacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newPacer(interval time.Duration) *pacer {
	return &pacer{interval: interval}
}

func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	now := time.Now()
	if p.next.Before(now) {
		p.next = now
	}
	delay := p.next.Sub(now)
	p.next = p.next.Add(p.interval)
	p.mu.Unlock()

	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var redditPacer = newPacer(4 * time.Second)

type Reddit struct {
	client     *http.Client
	subreddits []string
	label      string
	baseURL    string
	pacer      *pacer
}

func NewReddit(client *http.Client, subreddits []string, label string) *Reddit {
	return &Reddit{
		client:     client,
		subreddits: subreddits,
		label:      label,
		baseURL:    "https://www.reddit.com",
		pacer:      redditPacer,
	}
}

func (r *Reddit) Label() string { return r.label }

func (r *Reddit) Name() string { return "Reddit" }

func (r *Reddit) Fetch(ctx context.Context) (any, error) {
	bySubreddit := make(map[string][]RedditPost)
	var firstErr error
	for _, sub := range r.subreddits {
		posts, err := r.fetchSubreddit(ctx, sub)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		bySubreddit[sub] = posts
	}

	if len(bySubreddit) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return []RedditPost{}, nil
	}

	return mergePosts(bySubreddit, r.subreddits), nil
}

func (r *Reddit) fetchSubreddit(ctx context.Context, subreddit string) ([]RedditPost, error) {
	const maxAttempts = 3
	var feed atomFeed

	for attempt := 1; ; attempt++ {
		if err := r.pacer.wait(ctx); err != nil {
			return nil, fmt.Errorf("waiting for reddit rate limit slot for r/%s: %w", subreddit, err)
		}

		retryAfter, err := r.requestFeed(ctx, subreddit, &feed)
		if err == nil {
			break
		}
		if retryAfter < 0 || attempt == maxAttempts {
			return nil, err
		}
		timer := time.NewTimer(retryAfter)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}

	posts := make([]RedditPost, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		permalink := strings.TrimPrefix(e.Link.Href, "https://www.reddit.com")
		author := strings.TrimPrefix(e.Author.Name, "/u/")
		posts = append(posts, RedditPost{
			Title:     e.Title,
			Permalink: permalink,
			Author:    author,
			Subreddit: subreddit,
		})
	}

	return posts, nil
}

// requestFeed performs a single feed request. On a retryable failure (429 or
// 5xx) it returns the delay to wait before the next attempt; on a permanent
// failure the returned delay is negative.
func (r *Reddit) requestFeed(ctx context.Context, subreddit string, feed *atomFeed) (time.Duration, error) {
	apiURL := fmt.Sprintf("%s/r/%s/top.rss?t=day&limit=5", r.baseURL, subreddit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return -1, fmt.Errorf("creating request for r/%s: %w", subreddit, err)
	}
	req.Header.Set("User-Agent", "burrow/1.0 (by /u/kaktus_jack; info@burrow.janiskrasemann.com)")

	resp, err := r.client.Do(req)
	if err != nil {
		return 5 * time.Second, fmt.Errorf("fetching reddit posts for r/%s: %w", subreddit, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		retryAfter := 10 * time.Second
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 && secs <= 60 {
			retryAfter = time.Duration(secs) * time.Second
		}
		return retryAfter, fmt.Errorf("Reddit RSS returned status %d for r/%s", resp.StatusCode, subreddit)
	}
	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("Reddit RSS returned status %d for r/%s", resp.StatusCode, subreddit)
	}

	*feed = atomFeed{}
	if err := xml.NewDecoder(resp.Body).Decode(feed); err != nil {
		return -1, fmt.Errorf("decoding Reddit RSS for r/%s: %w", subreddit, err)
	}
	// Reddit soft-throttles by serving 200 with an empty feed instead of 429.
	if len(feed.Entries) == 0 {
		return 10 * time.Second, fmt.Errorf("Reddit RSS returned empty feed for r/%s (soft rate limit)", subreddit)
	}
	return 0, nil
}

// mergePosts combines posts from multiple subreddits, guaranteeing at least one
// post per subreddit. Remaining slots are filled by the original feed order
// (which is already ranked by score). Uses stable sort to preserve feed order
// when scores are equal.
func mergePosts(bySubreddit map[string][]RedditPost, subredditOrder []string) []RedditPost {
	total := len(bySubreddit)
	if total < 5 {
		total = 5
	}

	var guaranteed []RedditPost
	used := make(map[string]bool)

	for _, sub := range subredditOrder {
		posts := bySubreddit[sub]
		if len(posts) > 0 {
			guaranteed = append(guaranteed, posts[0])
			used[posts[0].Permalink] = true
		}
	}

	var remaining []RedditPost
	for _, sub := range subredditOrder {
		for _, p := range bySubreddit[sub] {
			if !used[p.Permalink] {
				remaining = append(remaining, p)
			}
		}
	}

	sort.SliceStable(remaining, func(i, j int) bool {
		return remaining[i].Score > remaining[j].Score
	})

	result := guaranteed
	spotsLeft := total - len(result)
	for i := 0; i < len(remaining) && i < spotsLeft; i++ {
		result = append(result, remaining[i])
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}
