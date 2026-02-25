package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
)

type Highlight struct {
	Text       string `json:"text"`
	BookTitle  string `json:"title"`
	BookAuthor string `json:"author"`
	SourceURL  string `json:"source_url"`
}

type readwiseResponse struct {
	Results []struct {
		Text string `json:"text"`
		Book struct {
			Title     string `json:"title"`
			Author    string `json:"author"`
			SourceURL string `json:"source_url"`
		} `json:"book"`
	} `json:"results"`
}

type Readwise struct {
	client   *http.Client
	apiToken string
	baseURL  string
}

func NewReadwise(client *http.Client, apiToken string) *Readwise {
	return &Readwise{client: client, apiToken: apiToken, baseURL: "https://readwise.io/api/v2/highlights/"}
}

func (r *Readwise) Name() string { return "Readwise" }

func (r *Readwise) Fetch(ctx context.Context) (any, error) {
	if r.apiToken == "" {
		return nil, fmt.Errorf("Readwise API token not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		r.baseURL+"?page_size=100", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+r.apiToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching highlights: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Readwise API returned status %d", resp.StatusCode)
	}

	var result readwiseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding Readwise response: %w", err)
	}

	if len(result.Results) == 0 {
		return []Highlight{}, nil
	}

	pick := result.Results[rand.IntN(len(result.Results))]
	return []Highlight{{
		Text:       pick.Text,
		BookTitle:  pick.Book.Title,
		BookAuthor: pick.Book.Author,
		SourceURL:  pick.Book.SourceURL,
	}}, nil
}
