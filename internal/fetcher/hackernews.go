package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

type HNPost struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	ObjectID    string `json:"objectID"`
	Author      string `json:"author"`
	StoryText   string `json:"story_text"`
}

func (p HNPost) CommentsURL() string {
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%s", p.ObjectID)
}

type hnResponse struct {
	Hits []HNPost `json:"hits"`
}

type HackerNews struct {
	client  *http.Client
	baseURL string
}

func NewHackerNews(client *http.Client) *HackerNews {
	return &HackerNews{client: client, baseURL: "https://hn.algolia.com/api/v1/search"}
}

func (h *HackerNews) Name() string { return "Hacker News" }

func (h *HackerNews) Fetch(ctx context.Context) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		h.baseURL+"?tags=front_page&hitsPerPage=30", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching HN posts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN API returned status %d", resp.StatusCode)
	}

	var result hnResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding HN response: %w", err)
	}

	posts := result.Hits
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Points > posts[j].Points
	})

	if len(posts) > 5 {
		posts = posts[:5]
	}

	return posts, nil
}
