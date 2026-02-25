package renderer

import (
	"bytes"
	"fmt"
	htmltpl "html/template"
	"strings"
	texttpl "text/template"
	"time"

	"github.com/janiskrasemann/burrow/internal/fetcher"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

type DigestData struct {
	Date    string
	Edition int
	Results []fetcher.Result
}

type RenderedEmail struct {
	HTML string
	Text string
}

type Renderer struct {
	htmlTpl        *htmltpl.Template
	textTpl        *texttpl.Template
	sectionCounter *int
}

func New(htmlTemplate, textTemplate string) (*Renderer, error) {
	counter := new(int)
	nextSection := func() int {
		v := *counter
		*counter++
		return v
	}
	isEven := func(n int) bool { return n%2 == 0 }

	funcMap := htmltpl.FuncMap{
		"weatherIcon": weatherIcon,
		"hasPrefix":   strings.HasPrefix,
		"hnPosts":     asHNPosts,
		"weatherData": asWeatherData,
		"highlights":  asHighlights,
		"redditPosts":   asRedditPosts,
		"redditLead":    redditLead,
		"redditSidebar": redditSidebar,
		"markdown":      renderMarkdown,
		"excerpt":     excerpt,
		"slice":       sliceFrom,
		"nextSection":   nextSection,
		"isEven":        isEven,
		"nitterPosts":   asNitterPosts,
		"nitterTimeAgo": nitterTimeAgo,
	}
	textFuncMap := texttpl.FuncMap{
		"weatherIcon": weatherIcon,
		"hasPrefix":   strings.HasPrefix,
		"hnPosts":     asHNPosts,
		"weatherData": asWeatherData,
		"highlights":  asHighlights,
		"redditPosts":   asRedditPosts,
		"redditLead":    redditLead,
		"redditSidebar": redditSidebar,
		"excerpt":       excerpt,
		"slice":       sliceFrom,
		"nextSection":   func() int { return 0 },
		"isEven":        isEven,
		"nitterPosts":   asNitterPosts,
		"nitterTimeAgo": nitterTimeAgo,
	}

	ht, err := htmltpl.New("digest.html").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML template: %w", err)
	}

	tt, err := texttpl.New("digest.txt").Funcs(textFuncMap).Parse(textTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing text template: %w", err)
	}

	return &Renderer{htmlTpl: ht, textTpl: tt, sectionCounter: counter}, nil
}

func (r *Renderer) Render(results []fetcher.Result, edition int) (*RenderedEmail, error) {
	*r.sectionCounter = 0

	data := DigestData{
		Date:    time.Now().Format("Monday, January 2, 2006"),
		Edition: edition,
		Results: results,
	}

	var htmlBuf bytes.Buffer
	if err := r.htmlTpl.Execute(&htmlBuf, data); err != nil {
		return nil, fmt.Errorf("rendering HTML: %w", err)
	}

	var textBuf bytes.Buffer
	if err := r.textTpl.Execute(&textBuf, data); err != nil {
		return nil, fmt.Errorf("rendering text: %w", err)
	}

	return &RenderedEmail{
		HTML: htmlBuf.String(),
		Text: textBuf.String(),
	}, nil
}

func asHNPosts(data any) []fetcher.HNPost {
	if posts, ok := data.([]fetcher.HNPost); ok {
		return posts
	}
	return nil
}

func asWeatherData(data any) *fetcher.WeatherData {
	if w, ok := data.(fetcher.WeatherData); ok {
		return &w
	}
	return nil
}

func asHighlights(data any) []fetcher.Highlight {
	if h, ok := data.([]fetcher.Highlight); ok {
		return h
	}
	return nil
}

func asRedditPosts(data any) []fetcher.RedditPost {
	if posts, ok := data.([]fetcher.RedditPost); ok {
		return posts
	}
	return nil
}

// redditLead returns the first post with selftext from the top 5 posts.
// Falls back to the first post if none have selftext.
func redditLead(data any) *fetcher.RedditPost {
	posts := asRedditPosts(data)
	if len(posts) == 0 {
		return nil
	}
	limit := 5
	if limit > len(posts) {
		limit = len(posts)
	}
	for i := 0; i < limit; i++ {
		if strings.TrimSpace(posts[i].Selftext) != "" {
			return &posts[i]
		}
	}
	return &posts[0]
}

// redditSidebar returns all posts except the lead post.
func redditSidebar(data any) []fetcher.RedditPost {
	posts := asRedditPosts(data)
	lead := redditLead(data)
	if lead == nil {
		return posts
	}
	var rest []fetcher.RedditPost
	for i := range posts {
		if &posts[i] != lead {
			rest = append(rest, posts[i])
		}
	}
	return rest
}

var md = goldmark.New(
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func renderMarkdown(s string) htmltpl.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return htmltpl.HTML(htmltpl.HTMLEscapeString(s))
	}
	return htmltpl.HTML(buf.String())
}

func excerpt(s string, maxSentences int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var sentences []string
	remaining := s
	for i := 0; i < maxSentences && remaining != ""; i++ {
		idx := -1
		for _, sep := range []string{". ", "! ", "? "} {
			if j := strings.Index(remaining, sep); j != -1 && (idx == -1 || j < idx) {
				idx = j + 1
			}
		}
		if idx == -1 {
			sentences = append(sentences, remaining)
			break
		}
		sentences = append(sentences, remaining[:idx])
		remaining = strings.TrimSpace(remaining[idx:])
	}
	result := strings.Join(sentences, " ")
	if len(result) > 280 {
		result = result[:277] + "..."
	}
	return result
}

func sliceFrom(start int, items any) any {
	switch v := items.(type) {
	case []fetcher.HNPost:
		if start >= len(v) {
			return []fetcher.HNPost{}
		}
		return v[start:]
	case []fetcher.RedditPost:
		if start >= len(v) {
			return []fetcher.RedditPost{}
		}
		return v[start:]
	default:
		return items
	}
}

func asNitterPosts(data any) []fetcher.NitterPost {
	if posts, ok := data.([]fetcher.NitterPost); ok {
		return posts
	}
	return nil
}

func nitterTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func weatherIcon(code int) string {
	switch {
	case code == 0:
		return "☀️"
	case code <= 3:
		return "⛅"
	case code <= 48:
		return "🌫️"
	case code <= 57:
		return "🌦️"
	case code <= 67:
		return "🌧️"
	case code <= 77:
		return "❄️"
	case code <= 82:
		return "🌧️"
	case code <= 86:
		return "🌨️"
	case code <= 99:
		return "⛈️"
	default:
		return "?"
	}
}
