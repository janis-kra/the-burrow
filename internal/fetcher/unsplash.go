package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type UnsplashImage struct {
	URL              string
	AltDescription   string
	PhotographerName string
	PhotographerURL  string
	Query            string
}

type unsplashResponse struct {
	URLs struct {
		Regular string `json:"regular"`
	} `json:"urls"`
	AltDescription string `json:"alt_description"`
	User           struct {
		Name  string `json:"name"`
		Links struct {
			HTML string `json:"html"`
		} `json:"links"`
	} `json:"user"`
}

type Unsplash struct {
	client        *http.Client
	accessKey     string
	fallbackQuery string
	topicQuery    string
}

func NewUnsplash(client *http.Client, accessKey, fallbackQuery string) *Unsplash {
	return &Unsplash{
		client:        client,
		accessKey:     accessKey,
		fallbackQuery: fallbackQuery,
	}
}

func (u *Unsplash) SetTopicQuery(topic string) {
	u.topicQuery = topic
}

func (u *Unsplash) Name() string { return "Unsplash" }

func (u *Unsplash) Fetch(ctx context.Context) (any, error) {
	if u.accessKey == "" {
		return nil, fmt.Errorf("Unsplash access key not configured")
	}

	// Try topic query first, then fall back
	if u.topicQuery != "" {
		img, err := u.fetchRandom(ctx, u.topicQuery)
		if err == nil {
			return img, nil
		}
	}

	return u.fetchRandom(ctx, u.fallbackQuery)
}

func (u *Unsplash) fetchRandom(ctx context.Context, query string) (*UnsplashImage, error) {
	endpoint := fmt.Sprintf("https://api.unsplash.com/photos/random?query=%s&orientation=landscape&content_filter=high",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Client-ID "+u.accessKey)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching photo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unsplash API returned status %d", resp.StatusCode)
	}

	var result unsplashResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	utmSuffix := "?utm_source=burrow&utm_medium=referral"

	return &UnsplashImage{
		URL:              result.URLs.Regular,
		AltDescription:   result.AltDescription,
		PhotographerName: result.User.Name,
		PhotographerURL:  result.User.Links.HTML + utmSuffix,
		Query:            query,
	}, nil
}
