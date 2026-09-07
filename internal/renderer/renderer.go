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
	Date      string
	DayOfWeek string
	ShortDate string
	DateSeed  string
	Edition   int
	Results   []fetcher.Result

	// Immo section (optional "page 2")
	ImmoHTML htmltpl.HTML
	ImmoText string
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
		"weatherIcon":    weatherIcon,
		"hasPrefix":      strings.HasPrefix,
		"hnPosts":        asHNPosts,
		"weatherData":    asWeatherData,
		"highlights":     asHighlights,
		"markdown":       renderMarkdown,
		"excerpt":        excerpt,
		"slice":          sliceFrom,
		"nextSection":    nextSection,
		"isEven":         isEven,
		"nitterPosts":    asNitterPosts,
		"nitterLeftCol":  nitterLeftCol,
		"nitterRightCol": nitterRightCol,
		"nitterTimeAgo":  nitterTimeAgo,
	}
	textFuncMap := texttpl.FuncMap{
		"weatherIcon":    weatherIcon,
		"hasPrefix":      strings.HasPrefix,
		"hnPosts":        asHNPosts,
		"weatherData":    asWeatherData,
		"highlights":     asHighlights,
		"excerpt":        excerpt,
		"slice":          sliceFrom,
		"nextSection":    func() int { return 0 },
		"isEven":         isEven,
		"nitterPosts":    asNitterPosts,
		"nitterLeftCol":  nitterLeftCol,
		"nitterRightCol": nitterRightCol,
		"nitterTimeAgo":  nitterTimeAgo,
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

func (r *Renderer) Render(results []fetcher.Result, edition int, immoHTML htmltpl.HTML, immoText string) (*RenderedEmail, error) {
	*r.sectionCounter = 0

	now := time.Now()
	data := DigestData{
		Date:      now.Format("Monday, January 2, 2006"),
		DayOfWeek: now.Format("Monday"),
		ShortDate: now.Format("January 2, 2006"),
		DateSeed:  now.Format("2006-01-02"),
		Edition:   edition,
		Results:   results,
		ImmoHTML:  immoHTML,
		ImmoText:  immoText,
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

var md = goldmark.New(
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func renderMarkdown(s string) htmltpl.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return htmltpl.HTML(htmltpl.HTMLEscapeString(s))
	}
	out := strings.TrimSpace(buf.String())
	// Unwrap single <p>...</p> to avoid extra block nesting in inline contexts.
	if strings.HasPrefix(out, "<p>") && strings.HasSuffix(out, "</p>") && strings.Count(out, "<p>") == 1 {
		out = out[3 : len(out)-4]
	}
	return htmltpl.HTML(out)
}

func excerpt(s string, maxSentences int) htmltpl.HTML {
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
	return htmltpl.HTML(result)
}

func sliceFrom(start int, items any) any {
	switch v := items.(type) {
	case []fetcher.HNPost:
		if start >= len(v) {
			return []fetcher.HNPost{}
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

func nitterLeftCol(data any) []fetcher.NitterPost {
	posts := asNitterPosts(data)
	if len(posts) == 0 {
		return nil
	}
	half := (len(posts) + 1) / 2
	return posts[:half]
}

func nitterRightCol(data any) []fetcher.NitterPost {
	posts := asNitterPosts(data)
	if len(posts) == 0 {
		return nil
	}
	half := (len(posts) + 1) / 2
	if half >= len(posts) {
		return nil
	}
	return posts[half:]
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
