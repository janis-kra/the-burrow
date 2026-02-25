package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadwiseFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results": [
				{"text": "The only way to do great work is to love what you do.", "book": {"title": "Steve Jobs", "author": "Walter Isaacson"}},
				{"text": "Stay hungry, stay foolish.", "book": {"title": "Whole Earth Catalog", "author": ""}}
			]
		}`))
	}))
	defer server.Close()

	rw := NewReadwise(server.Client(), "test-token")
	rw.baseURL = server.URL

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
