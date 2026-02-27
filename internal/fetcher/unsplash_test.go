package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnsplashFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"urls": {"regular": "https://images.unsplash.com/photo-123"},
			"alt_description": "A mountain landscape",
			"user": {
				"name": "Jane Doe",
				"links": {"html": "https://unsplash.com/@janedoe"}
			}
		}`))
	}))
	defer server.Close()

	u := NewUnsplash(server.Client(), "test-key", "nature")
	u.baseURL = server.URL

	result, err := u.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, ok := result.(*UnsplashImage)
	if !ok {
		t.Fatal("result is not *UnsplashImage")
	}

	if img.URL != "https://images.unsplash.com/photo-123" {
		t.Errorf("unexpected URL: %q", img.URL)
	}
	if img.AltDescription != "A mountain landscape" {
		t.Errorf("unexpected alt: %q", img.AltDescription)
	}
	if img.PhotographerName != "Jane Doe" {
		t.Errorf("unexpected photographer: %q", img.PhotographerName)
	}
	if img.PhotographerURL != "https://unsplash.com/@janedoe?utm_source=burrow&utm_medium=referral" {
		t.Errorf("unexpected photographer URL (missing UTM?): %q", img.PhotographerURL)
	}
	if img.Query != "nature" {
		t.Errorf("unexpected query: %q", img.Query)
	}
}

func TestUnsplashNoAccessKey(t *testing.T) {
	u := NewUnsplash(http.DefaultClient, "", "nature")

	_, err := u.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for empty access key")
	}
}

func TestUnsplashTopicFallback(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		query := r.URL.Query().Get("query")
		if query == "specific-topic" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Fallback query succeeds
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"urls": {"regular": "https://images.unsplash.com/fallback"},
			"alt_description": "fallback image",
			"user": {
				"name": "Fallback Photographer",
				"links": {"html": "https://unsplash.com/@fallback"}
			}
		}`))
	}))
	defer server.Close()

	u := NewUnsplash(server.Client(), "test-key", "nature")
	u.baseURL = server.URL
	u.SetTopicQuery("specific-topic")

	result, err := u.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img := result.(*UnsplashImage)
	if img.URL != "https://images.unsplash.com/fallback" {
		t.Errorf("expected fallback image, got %q", img.URL)
	}
	if calls != 2 {
		t.Errorf("expected 2 API calls (topic + fallback), got %d", calls)
	}
}

func TestUnsplashAuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"urls": {"regular": "https://images.unsplash.com/photo-1"},
			"alt_description": "test",
			"user": {"name": "Test", "links": {"html": "https://unsplash.com/@test"}}
		}`))
	}))
	defer server.Close()

	u := NewUnsplash(server.Client(), "my-secret-key", "nature")
	u.baseURL = server.URL

	_, err := u.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Client-ID my-secret-key" {
		t.Errorf("expected 'Client-ID my-secret-key', got %q", gotAuth)
	}
}
