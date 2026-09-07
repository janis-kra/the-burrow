package immo

import (
	"bytes"
	"fmt"
	htmltpl "html/template"
	"strings"
	texttpl "text/template"
)

type EmailData struct {
	Date       string
	PriceDrops []PriceDrop
	New        []Listing
}

func tplFuncs() map[string]any {
	return map[string]any{
		"detailLine": detailLine,
	}
}

// detailLine joins location, type and details into one muted line:
// "31785 Hameln · Mehrfamilienhaus · Vermietbare Fläche: 315 m²".
func detailLine(l Listing) string {
	var parts []string
	if l.Location != "" {
		parts = append(parts, l.Location)
	}
	if l.Type != "" {
		parts = append(parts, l.Type)
	}
	parts = append(parts, l.Details...)
	return strings.Join(parts, " · ")
}

// RenderEmail renders the immo digest. Price drops come first, then new
// listings — flat list, no per-broker sections.
func RenderEmail(htmlTemplate, textTemplate string, data EmailData) (html, text string, err error) {
	ht, err := htmltpl.New("immo.html").Funcs(tplFuncs()).Parse(htmlTemplate)
	if err != nil {
		return "", "", fmt.Errorf("parsing HTML template: %w", err)
	}
	tt, err := texttpl.New("immo.txt").Funcs(tplFuncs()).Parse(textTemplate)
	if err != nil {
		return "", "", fmt.Errorf("parsing text template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := ht.Execute(&htmlBuf, data); err != nil {
		return "", "", fmt.Errorf("rendering HTML: %w", err)
	}
	var textBuf bytes.Buffer
	if err := tt.Execute(&textBuf, data); err != nil {
		return "", "", fmt.Errorf("rendering text: %w", err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}

// RenderForDigest renders the immo content as an embeddable fragment (no
// outer <!DOCTYPE> or <html>/<body> wrapper) so it can be included as
// "page 2" inside the main digest email. The listing visuals are
// identical to the standalone immo email.
func RenderForDigest(htmlTemplate, textTemplate string, data EmailData) (html, text string, err error) {
	fullHTML, fullText, err := RenderEmail(htmlTemplate, textTemplate, data)
	if err != nil {
		return "", "", err
	}
	// Extract inner body content for embedding.
	if bodyStart := strings.Index(fullHTML, "<body"); bodyStart != -1 {
		if tagEnd := strings.Index(fullHTML[bodyStart:], ">"); tagEnd != -1 {
			bodyStart += tagEnd + 1
			if bodyEnd := strings.LastIndex(fullHTML, "</body>"); bodyEnd != -1 {
				html = strings.TrimSpace(fullHTML[bodyStart:bodyEnd])
			}
		}
	}
	if html == "" {
		html = fullHTML // fallback
	}
	// For text we can just return the full text (caller will prefix separator)
	text = fullText
	return html, text, nil
}

// Subject builds the email subject, e.g.
// "Burrow Immobilien — 3 neue Treffer, 1 Preissenkung".
func Subject(d DiffResult) string {
	var parts []string
	switch n := len(d.New); {
	case n == 1:
		parts = append(parts, "1 neuer Treffer")
	case n > 1:
		parts = append(parts, fmt.Sprintf("%d neue Treffer", n))
	}
	switch n := len(d.PriceDrops); {
	case n == 1:
		parts = append(parts, "1 Preissenkung")
	case n > 1:
		parts = append(parts, fmt.Sprintf("%d Preissenkungen", n))
	}
	return "Burrow Immobilien — " + strings.Join(parts, ", ")
}
