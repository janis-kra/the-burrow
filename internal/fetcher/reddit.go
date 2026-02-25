package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"sync"
)

type RedditPost struct {
	Title       string `json:"title"`
	Score       int    `json:"score"`
	NumComments int    `json:"num_comments"`
	Permalink   string `json:"permalink"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Selftext    string `json:"selftext"`
	Subreddit   string `json:"subreddit"`
}

func (p RedditPost) FullPermalink() string {
	return "https://www.reddit.com" + p.Permalink
}

type redditResponse struct {
	Data struct {
		Children []struct {
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type Reddit struct {
	subreddits []string
	baseURL    string
}

func NewReddit(subreddits []string) *Reddit {
	return &Reddit{subreddits: subreddits, baseURL: "https://www.reddit.com"}
}

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
	url := fmt.Sprintf("%s/r/%s/top/.json?t=day&limit=5", r.baseURL, subreddit)

	cmd := exec.CommandContext(ctx, "curl", "-s",
		"--user-agent", "burrow/1.0 (by /u/kaktus_jack; info@burrow.janiskrasemann.com)",
		url,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fetching reddit posts for r/%s: %w", subreddit, err)
	}

	var result redditResponse
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decoding Reddit response for r/%s: %w", subreddit, err)
	}

	posts := make([]RedditPost, 0, len(result.Data.Children))
	for _, child := range result.Data.Children {
		p := child.Data
		if p.Subreddit == "" {
			p.Subreddit = subreddit
		}
		posts = append(posts, p)
	}

	return posts, nil
}

// mergePosts combines posts from multiple subreddits, guaranteeing at least one
// post per subreddit. Remaining slots are filled with the highest-scored posts.
func mergePosts(bySubreddit map[string][]RedditPost, subredditOrder []string) []RedditPost {
	total := len(bySubreddit)
	if total < 5 {
		total = 5
	}

	var guaranteed []RedditPost
	used := make(map[string]bool)

	// Take the top post from each subreddit (guarantee)
	for _, sub := range subredditOrder {
		posts := bySubreddit[sub]
		if len(posts) > 0 {
			guaranteed = append(guaranteed, posts[0])
			used[posts[0].Permalink] = true
		}
	}

	// Collect remaining posts from all subreddits
	var remaining []RedditPost
	for _, sub := range subredditOrder {
		posts := bySubreddit[sub]
		for _, p := range posts {
			if !used[p.Permalink] {
				remaining = append(remaining, p)
			}
		}
	}

	// Sort remaining by score descending
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].Score > remaining[j].Score
	})

	// Fill up to total
	result := guaranteed
	spotsLeft := total - len(result)
	for i := 0; i < len(remaining) && i < spotsLeft; i++ {
		result = append(result, remaining[i])
	}

	// Sort final result by score descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}
