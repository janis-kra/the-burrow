package renderer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/janiskrasemann/burrow/internal/fetcher"
)

func TestRenderHTML(t *testing.T) {
	htmlTpl := `<html><body>{{.Date}}{{range .Results}}{{if .Error}}ERROR{{else}}{{if eq .Name "Hacker News"}}{{range hnPosts .Data}}<p>{{.Title}}</p>{{end}}{{end}}{{end}}{{end}}</body></html>`
	textTpl := `{{.Date}}{{range .Results}}{{.Name}}{{end}}`

	r, err := New(htmlTpl, textTpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := []fetcher.Result{
		{
			Name: "Hacker News",
			Data: []fetcher.HNPost{
				{Title: "Test Post", Points: 100, NumComments: 50, ObjectID: "1", URL: "https://example.com"},
			},
		},
	}

	email, err := r.Render(results, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(email.HTML, "Test Post") {
		t.Error("expected HTML to contain 'Test Post'")
	}
	if !strings.Contains(email.Text, "Hacker News") {
		t.Error("expected text to contain 'Hacker News'")
	}
}

func TestRenderErrorModule(t *testing.T) {
	htmlTpl := `{{range .Results}}{{if .Error}}ERROR:{{.Name}}{{end}}{{end}}`
	textTpl := `{{range .Results}}{{if .Error}}ERROR:{{.Name}}{{end}}{{end}}`

	r, err := New(htmlTpl, textTpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := []fetcher.Result{
		{Name: "Weather", Error: fmt.Errorf("network error")},
	}

	email, err := r.Render(results, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(email.HTML, "ERROR:Weather") {
		t.Error("expected HTML to show error for Weather module")
	}
}

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"empty", "", 2, ""},
		{"single sentence", "Hello world.", 2, "Hello world."},
		{"two sentences", "First sentence. Second sentence. Third one.", 2, "First sentence. Second sentence."},
		{"truncation at 280", strings.Repeat("A", 300), 5, strings.Repeat("A", 277) + "..."},
		{"no sentence boundary", "Just a long text with no periods", 2, "Just a long text with no periods"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := excerpt(tt.input, tt.max)
			if string(got) != tt.expected {
				t.Errorf("excerpt(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expected)
			}
		})
	}
}

func TestNitterTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		expected string
	}{
		{"now", 10 * time.Second, "now"},
		{"minutes", 15 * time.Minute, "15m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nitterTimeAgo(time.Now().Add(-tt.offset))
			if got != tt.expected {
				t.Errorf("nitterTimeAgo(%v ago) = %q, want %q", tt.offset, got, tt.expected)
			}
		})
	}
}

func TestWeatherIcon(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "☀️"},
		{2, "⛅"},
		{45, "🌫️"},
		{55, "🌦️"},
		{63, "🌧️"},
		{73, "❄️"},
		{80, "🌧️"},
		{85, "🌨️"},
		{95, "⛈️"},
		{100, "?"},
	}

	for _, tt := range tests {
		got := weatherIcon(tt.code)
		if got != tt.want {
			t.Errorf("weatherIcon(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}
