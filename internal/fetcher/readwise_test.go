package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, highlightsBody string, books map[string]string, archivedDocIDs []string) *httptest.Server {
	t.Helper()

	archivedJSON := `{"results": [`
	for i, id := range archivedDocIDs {
		if i > 0 {
			archivedJSON += ","
		}
		archivedJSON += `{"id": "` + id + `"}`
	}
	archivedJSON += `], "nextPageCursor": ""}`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "v3"):
			w.Write([]byte(archivedJSON))
		case strings.Contains(r.URL.Path, "/books/"):
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			id := parts[len(parts)-1]
			body, ok := books[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write([]byte(body))
		default:
			w.Write([]byte(highlightsBody))
		}
	}))
}

func configureReadwise(rw *Readwise, serverURL string) {
	rw.baseURL = serverURL + "/v2/highlights/"
	rw.readerURL = serverURL + "/v3/"
	rw.bookURL = serverURL + "/v2/books/"
}

func TestReadwiseFetch(t *testing.T) {
	highlights := `{
		"results": [
			{"text": "The only way to do great work is to love what you do.", "book_id": 1},
			{"text": "Stay hungry, stay foolish.", "book_id": 2}
		]
	}`
	books := map[string]string{
		"1": `{"title": "Steve Jobs", "author": "Walter Isaacson", "source_url": "https://example.com/steve-jobs"}`,
		"2": `{"title": "Whole Earth Catalog", "author": "", "source_url": ""}`,
	}
	server := newTestServer(t, highlights, books, nil)
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	configureReadwise(rw, server.URL)

	result, err := rw.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := result.([]Highlight)
	if !ok {
		t.Fatal("result is not []Highlight")
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 random highlight, got %d", len(got))
	}
	if got[0].BookTitle == "" {
		t.Error("expected BookTitle to be populated from /books/ lookup, got empty")
	}
}

func TestReadwisePopulatesBookMetadata(t *testing.T) {
	// With only one highlight, the pick is deterministic and we can assert exact metadata.
	highlights := `{
		"results": [
			{"text": "Stay hungry, stay foolish.", "book_id": 42}
		]
	}`
	books := map[string]string{
		"42": `{"title": "Whole Earth Catalog", "author": "Stewart Brand", "source_url": "https://example.com/wec"}`,
	}
	server := newTestServer(t, highlights, books, nil)
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	configureReadwise(rw, server.URL)

	result, err := rw.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := result.([]Highlight)
	if len(got) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(got))
	}
	h := got[0]
	if h.BookTitle != "Whole Earth Catalog" {
		t.Errorf("BookTitle = %q, want %q", h.BookTitle, "Whole Earth Catalog")
	}
	if h.BookAuthor != "Stewart Brand" {
		t.Errorf("BookAuthor = %q, want %q", h.BookAuthor, "Stewart Brand")
	}
	if h.SourceURL != "https://example.com/wec" {
		t.Errorf("SourceURL = %q, want %q", h.SourceURL, "https://example.com/wec")
	}
}

func TestReadwiseBookLookupFailureStillReturnsQuote(t *testing.T) {
	// If /books/{id}/ fails (e.g. 404), the highlight still renders — just without attribution.
	highlights := `{
		"results": [
			{"text": "An orphaned quote.", "book_id": 999}
		]
	}`
	server := newTestServer(t, highlights, map[string]string{}, nil)
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	configureReadwise(rw, server.URL)

	result, err := rw.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := result.([]Highlight)
	if len(got) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(got))
	}
	if got[0].Text != "An orphaned quote." {
		t.Errorf("Text = %q, want %q", got[0].Text, "An orphaned quote.")
	}
	if got[0].BookTitle != "" {
		t.Errorf("expected empty BookTitle on lookup failure, got %q", got[0].BookTitle)
	}
}

func TestReadwiseNoToken(t *testing.T) {
	rw := NewReadwise(http.DefaultClient, "")
	_, err := rw.Fetch(context.Background())
	if err == nil {
		t.Error("expected error when API token is empty")
	}
}

func TestReadwiseFiltersUnreadReaderBooks(t *testing.T) {
	// Two highlights: one from an archived (read) Reader doc, one from an unread Reader doc.
	highlights := `{
		"results": [
			{"text": "Quote from a read book.", "external_id": "archived-doc-id", "book_id": 1},
			{"text": "Quote from an unread book.", "external_id": "unread-doc-id", "book_id": 2}
		]
	}`
	books := map[string]string{
		"1": `{"title": "Read Book", "author": "Author A"}`,
		"2": `{"title": "Unread Book", "author": "Author B"}`,
	}
	server := newTestServer(t, highlights, books, []string{"archived-doc-id"})
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	configureReadwise(rw, server.URL)

	result, err := rw.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := result.([]Highlight)
	if !ok {
		t.Fatal("result is not []Highlight")
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 highlight after filtering, got %d", len(got))
	}
	if got[0].BookTitle != "Read Book" {
		t.Errorf("expected highlight from Read Book, got %q", got[0].BookTitle)
	}
}

func TestReadwiseIncludesNonReaderHighlights(t *testing.T) {
	// A highlight without external_id (e.g. from Kindle) should always be included
	// even if the Reader archive is empty.
	highlights := `{
		"results": [
			{"text": "Kindle highlight.", "book_id": 7}
		]
	}`
	books := map[string]string{
		"7": `{"title": "Kindle Book", "author": "Author K"}`,
	}
	server := newTestServer(t, highlights, books, nil)
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	configureReadwise(rw, server.URL)

	result, err := rw.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := result.([]Highlight)
	if !ok {
		t.Fatal("result is not []Highlight")
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(got))
	}
	if got[0].BookTitle != "Kindle Book" {
		t.Errorf("expected Kindle Book, got %q", got[0].BookTitle)
	}
}
