package burrow_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/janiskrasemann/burrow/internal/fetcher"
	"github.com/janiskrasemann/burrow/internal/renderer"
)

func loadTemplates(t *testing.T) (string, string) {
	t.Helper()
	htmlBytes, err := os.ReadFile("templates/digest.html")
	if err != nil {
		t.Fatalf("failed to read HTML template: %v", err)
	}
	textBytes, err := os.ReadFile("templates/digest.txt")
	if err != nil {
		t.Fatalf("failed to read text template: %v", err)
	}
	return string(htmlBytes), string(textBytes)
}

func TestFullPipelineRender(t *testing.T) {
	htmlTpl, textTpl := loadTemplates(t)

	r, err := renderer.New(htmlTpl, textTpl)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	results := []fetcher.Result{
		{
			Name: "Weather",
			Data: fetcher.WeatherData{
				Temperature:   18.5,
				HighTemp:      22.0,
				LowTemp:       14.0,
				Precipitation: 10,
				WeatherCode:   0,
				Description:   "Clear sky",
				Location:      "Berlin",
			},
		},
		{
			Name: "Readwise",
			Data: []fetcher.Highlight{{
				Text:       "The only way to do great work is to love what you do.",
				BookTitle:  "Steve Jobs",
				BookAuthor: "Walter Isaacson",
				SourceURL:  "https://example.com/book",
			}},
		},
		{
			Name: "Hacker News",
			Data: []fetcher.HNPost{
				{Title: "Go 1.25 Released", Points: 500, NumComments: 200, ObjectID: "1", URL: "https://go.dev/blog", Author: "golang"},
				{Title: "Rust vs Go in 2025", Points: 300, NumComments: 150, ObjectID: "2", URL: "https://example.com/rust-go", Author: "dev123"},
				{Title: "Third HN Post", Points: 100, NumComments: 50, ObjectID: "3", URL: "https://example.com/3"},
			},
		},
		{
			Name: "Opinion",
			Data: []fetcher.NitterPost{
				{Username: "techguru", Text: "Interesting take on AI progress", Link: "https://nitter.net/techguru/1", PubDate: time.Now().Add(-2 * time.Hour), AvatarURL: "https://unavatar.io/twitter/techguru"},
				{Username: "devops", Text: "Kubernetes is overrated", Link: "https://nitter.net/devops/1", PubDate: time.Now().Add(-4 * time.Hour), IsRetweet: true, AvatarURL: "https://unavatar.io/twitter/devops"},
			},
		},
	}

	email, err := r.Render(results, 42)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Verify HTML contains content from each source
	htmlChecks := []struct {
		label string
		want  string
	}{
		{"edition", "#42"},
		{"weather location", "Berlin"},
		{"weather temp", "18"},
		{"readwise quote", "great work"},
		{"readwise author", "Walter Isaacson"},
		{"hn title", "Go 1.25 Released"},
		{"hn points", "500"},
		{"nitter username", "@techguru"},
		{"nitter text", "Interesting take on AI progress"},
		{"footer", "Burrow"},
	}

	for _, check := range htmlChecks {
		if !strings.Contains(email.HTML, check.want) {
			t.Errorf("HTML missing %s: expected to contain %q", check.label, check.want)
		}
	}

	// Verify text version contains key content
	textChecks := []string{"Hacker News", "Go 1.25 Released", "Weather", "Readwise", "Opinion", "@techguru"}
	for _, want := range textChecks {
		if !strings.Contains(email.Text, want) {
			t.Errorf("text missing: expected to contain %q", want)
		}
	}
}

func TestFullPipelineWithErrors(t *testing.T) {
	htmlTpl, textTpl := loadTemplates(t)

	r, err := renderer.New(htmlTpl, textTpl)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	results := []fetcher.Result{
		{
			Name: "Weather",
			Data: fetcher.WeatherData{
				Temperature: 20.0,
				HighTemp:    25.0,
				LowTemp:     15.0,
				WeatherCode: 0,
				Description: "Clear sky",
				Location:    "Berlin",
			},
		},
		{
			Name:  "Hacker News",
			Error: &testError{"HN API timeout"},
		},
	}

	email, err := r.Render(results, 1)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Error module should be rendered for Hacker News
	if !strings.Contains(email.HTML, "Could not load this module") {
		t.Error("expected error module in HTML")
	}
	// Working sources should still render
	if !strings.Contains(email.HTML, "Berlin") {
		t.Error("expected working weather module in HTML")
	}
}

func TestFullPipelineEmptyResults(t *testing.T) {
	htmlTpl, textTpl := loadTemplates(t)

	r, err := renderer.New(htmlTpl, textTpl)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	email, err := r.Render([]fetcher.Result{}, 1)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Should at least contain the masthead
	if !strings.Contains(email.HTML, "The Burrow") {
		t.Error("expected masthead in HTML")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }
