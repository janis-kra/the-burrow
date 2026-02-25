package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type RedditPost struct {
	Title       string `json:"title"`
	Score       int    `json:"score"`
	NumComments int    `json:"num_comments"`
	Permalink   string `json:"permalink"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Selftext    string `json:"selftext"`
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
	subreddit string
	baseURL   string
}

func NewReddit(subreddit string) *Reddit {
	return &Reddit{subreddit: subreddit, baseURL: "https://www.reddit.com"}
}

func (r *Reddit) Name() string { return "Reddit r/" + r.subreddit }

func (r *Reddit) Fetch(ctx context.Context) (any, error) {
	url := fmt.Sprintf("%s/r/%s/top/.json?t=day&limit=5", r.baseURL, r.subreddit)

	cmd := exec.CommandContext(ctx, "curl", "-s",
		"--user-agent", "burrow/1.0 (by /u/kaktus_jack; info@burrow.janiskrasemann.com)",
		url,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fetching reddit posts: %w", err)
	}

	var result redditResponse
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decoding Reddit response: %w", err)
	}

	posts := make([]RedditPost, 0, len(result.Data.Children))
	for _, child := range result.Data.Children {
		posts = append(posts, child.Data)
	}

	return posts, nil
}
