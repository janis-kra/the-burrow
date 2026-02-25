package renderer

import (
	"fmt"
	"strings"
	"testing"

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

	email, err := r.Render(results)
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

	email, err := r.Render(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(email.HTML, "ERROR:Weather") {
		t.Error("expected HTML to show error for Weather module")
	}
}
