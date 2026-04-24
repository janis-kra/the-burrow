package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, highlights string, archivedDocIDs []string) *httptest.Server {
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

		if strings.Contains(r.URL.Path, "v3") {
			w.Write([]byte(archivedJSON))
		} else {
			w.Write([]byte(highlights))
		}
	}))
}

func TestReadwiseFetch(t *testing.T) {
	server := newTestServer(t, `{
		"results": [
			{"text": "The only way to do great work is to love what you do.", "book": {"title": "Steve Jobs", "author": "Walter Isaacson"}},
			{"text": "Stay hungry, stay foolish.", "book": {"title": "Whole Earth Catalog", "author": ""}}
		]
	}`, nil)
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	rw.baseURL = server.URL + "/v2/"
	rw.readerURL = server.URL + "/v3/"

	result, err := rw.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	highlights, ok := result.([]Highlight)
	if !ok {
		t.Fatal("result is not []Highlight")
	}

	if len(highlights) != 1 {
		t.Errorf("expected 1 random highlight, got %d", len(highlights))
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
			{"text": "Quote from a read book.", "external_id": "archived-doc-id", "book": {"title": "Read Book", "author": "Author A"}},
			{"text": "Quote from an unread book.", "external_id": "unread-doc-id", "book": {"title": "Unread Book", "author": "Author B"}}
		]
	}`
	server := newTestServer(t, highlights, []string{"archived-doc-id"})
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	rw.baseURL = server.URL + "/v2/"
	rw.readerURL = server.URL + "/v3/"

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
			{"text": "Kindle highlight.", "book": {"title": "Kindle Book", "author": "Author K"}}
		]
	}`
	server := newTestServer(t, highlights, nil)
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	rw.baseURL = server.URL + "/v2/"
	rw.readerURL = server.URL + "/v3/"

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
