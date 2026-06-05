package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
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

type Reddit struct {
	client     *http.Client
	subreddits []string
	label      string
	baseURL    string
}

func NewReddit(client *http.Client, subreddits []string, label string) *Reddit {
	return &Reddit{
		client:     client,
		subreddits: subreddits,
		label:      label,
		baseURL:    "https://www.reddit.com",
	}
}

func (r *Reddit) Label() string { return r.label }

func (r *Reddit) Name() string { return "Reddit" }

func (r *Reddit) Fetch(ctx context.Context) (any, error) {
	type subredditResult struct {
		subreddit string
		posts     []RedditPost
		err       error
	}

	var wg sync.WaitGroup
	results := make([]subredditResult, len(r.subreddits))

	for i, sub := range r.subreddits {
		wg.Add(1)
		go func(idx int, subreddit string) {
			defer wg.Done()
			posts, err := r.fetchSubreddit(ctx, subreddit)
			results[idx] = subredditResult{subreddit: subreddit, posts: posts, err: err}
		}(i, sub)
	}

	wg.Wait()

	bySubreddit := make(map[string][]RedditPost)
	var firstErr error
	for _, res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		bySubreddit[res.subreddit] = res.posts
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
	apiURL := fmt.Sprintf("%s/r/%s/top.rss?t=day&limit=5", r.baseURL, subreddit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for r/%s: %w", subreddit, err)
	}
	req.Header.Set("User-Agent", "burrow/1.0 (by /u/kaktus_jack; info@burrow.janiskrasemann.com)")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching reddit posts for r/%s: %w", subreddit, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Reddit RSS returned status %d for r/%s", resp.StatusCode, subreddit)
	}

	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decoding Reddit RSS for r/%s: %w", subreddit, err)
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
