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

type readwiseHighlight struct {
	Text       string `json:"text"`
	ExternalID string `json:"external_id"`
	Book       struct {
		Title     string `json:"title"`
		Author    string `json:"author"`
		SourceURL string `json:"source_url"`
	} `json:"book"`
}

type readwiseResponse struct {
	Results []readwiseHighlight `json:"results"`
}

type readerDocument struct {
	ID string `json:"id"`
}

type readerListResponse struct {
	Results        []readerDocument `json:"results"`
	NextPageCursor string           `json:"nextPageCursor"`
}

type Readwise struct {
	client    *http.Client
	apiToken  string
	baseURL   string
	readerURL string
}

func NewReadwise(client *http.Client, apiToken string) *Readwise {
	return &Readwise{
		client:    client,
		apiToken:  apiToken,
		baseURL:   "https://readwise.io/api/v2/highlights/",
		readerURL: "https://readwise.io/api/v3/list/",
	}
}

func (r *Readwise) Name() string { return "Readwise" }

// fetchArchivedReaderDocIDs returns the set of Reader document IDs that the user has archived (finished reading).
func (r *Readwise) fetchArchivedReaderDocIDs(ctx context.Context) (map[string]bool, error) {
	archived := make(map[string]bool)
	cursor := ""

	for {
		url := r.readerURL + "?location=archive"
		if cursor != "" {
			url += "&pageCursor=" + cursor
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating reader request: %w", err)
		}
		req.Header.Set("Authorization", "Token "+r.apiToken)

		resp, err := r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching reader documents: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Reader API not available or user doesn't use Reader — skip filtering
			return archived, nil
		}

		var result readerListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decoding reader response: %w", err)
		}

		for _, doc := range result.Results {
			archived[doc.ID] = true
		}

		if result.NextPageCursor == "" {
			break
		}
		cursor = result.NextPageCursor
	}

	return archived, nil
}

func (r *Readwise) Fetch(ctx context.Context) (any, error) {
	if r.apiToken == "" {
		return nil, fmt.Errorf("Readwise API token not configured")
	}

	// Fetch archived Reader document IDs to identify which books have been read.
	// Highlights whose external_id is not in this set came from unread Reader documents
	// and should be excluded. Highlights without an external_id are from non-Reader sources
	// (e.g. Kindle) and are always included.
	archivedDocIDs, err := r.fetchArchivedReaderDocIDs(ctx)
	if err != nil {
		// If the Reader API fails, proceed without filtering rather than failing entirely.
		archivedDocIDs = make(map[string]bool)
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

	// Filter out highlights from unread Reader documents.
	// A highlight is excluded only when it has an external_id (i.e. it came from Reader)
	// AND that document is not in the user's archive (not finished reading).
	var readHighlights []readwiseHighlight
	for _, h := range result.Results {
		if h.ExternalID != "" && !archivedDocIDs[h.ExternalID] {
			continue
		}
		readHighlights = append(readHighlights, h)
	}

	if len(readHighlights) == 0 {
		return []Highlight{}, nil
	}

	pick := readHighlights[rand.IntN(len(readHighlights))]
	return []Highlight{{
		Text:       pick.Text,
		BookTitle:  pick.Book.Title,
		BookAuthor: pick.Book.Author,
		SourceURL:  pick.Book.SourceURL,
	}}, nil
}
