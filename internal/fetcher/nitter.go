package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type NitterPost struct {
	Username  string
	Text      string
	Link      string
	PubDate   time.Time
	Images    []string
	AvatarURL string
	IsRetweet bool
	IsReply   bool
}

type Nitter struct {
	client    *http.Client
	instance  string
	usernames []string
	limit     int
}

func NewNitter(client *http.Client, instance string, usernames []string, limit int) *Nitter {
	if limit <= 0 {
		limit = 5
	}
	return &Nitter{client: client, instance: strings.TrimRight(instance, "/"), usernames: usernames, limit: limit}
}

func (n *Nitter) Name() string { return "Opinion" }

func (n *Nitter) Fetch(ctx context.Context) (any, error) {
	cutoff := time.Now().Add(-24 * time.Hour)
	var allPosts []NitterPost

	for _, username := range n.usernames {
		posts, err := n.fetchUser(ctx, username, cutoff)
		if err != nil {
			log.Printf("nitter: failed to fetch @%s: %v", username, err)
			continue
		}
		allPosts = append(allPosts, posts...)
	}

	sort.Slice(allPosts, func(i, j int) bool {
		return allPosts[i].PubDate.After(allPosts[j].PubDate)
	})

	// Guarantee at least one tweet per user, then fill remaining slots by recency
	seen := make(map[string]bool)
	var guaranteed []NitterPost
	for _, p := range allPosts {
		if !seen[p.Username] {
			seen[p.Username] = true
			guaranteed = append(guaranteed, p)
		}
	}

	if len(guaranteed) >= n.limit {
		// More users than limit — just show the top tweet per user, sorted by time
		sort.Slice(guaranteed, func(i, j int) bool {
			return guaranteed[i].PubDate.After(guaranteed[j].PubDate)
		})
		return guaranteed, nil
	}

	// Fill up to limit from all posts in recency order (guaranteed ones are already
	// the top post per user, so they'll naturally appear first for each user)
	result := make([]NitterPost, 0, n.limit)
	for _, p := range allPosts {
		if len(result) >= n.limit {
			break
		}
		result = append(result, p)
	}

	// Ensure every user has at least one tweet even if it wasn't in the top N by time
	included := make(map[string]bool)
	for _, p := range result {
		included[p.Username] = true
	}
	for _, p := range guaranteed {
		if !included[p.Username] {
			result = append(result, p)
			included[p.Username] = true
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PubDate.After(result[j].PubDate)
	})

	return result, nil
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
}

var imgSrcRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)

func (n *Nitter) fetchUser(ctx context.Context, username string, cutoff time.Time) ([]NitterPost, error) {
	url := fmt.Sprintf("%s/%s/rss", n.instance, username)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Burrow/1.0)")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss rssDocument
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("parsing RSS for @%s: %w", username, err)
	}

	var posts []NitterPost
	for _, item := range rss.Channel.Items {
		pubDate := parseRSSDate(item.PubDate)
		if pubDate.IsZero() || pubDate.Before(cutoff) {
			continue
		}

		text := item.Title
		isRetweet := strings.HasPrefix(text, "RT by ")
		isReply := strings.HasPrefix(text, "R to ")

		if isRetweet {
			if idx := strings.Index(text, ": "); idx != -1 {
				text = text[idx+2:]
			}
		}
		if isReply {
			if idx := strings.Index(text, ": "); idx != -1 {
				text = text[idx+2:]
			}
		}

		images := extractImages(item.Description)

		posts = append(posts, NitterPost{
			Username:  username,
			Text:      text,
			Link:      item.Link,
			PubDate:   pubDate,
			Images:    images,
			AvatarURL: fmt.Sprintf("https://unavatar.io/twitter/%s", username),
			IsRetweet: isRetweet,
			IsReply:   isReply,
		})
	}

	return posts, nil
}

func parseRSSDate(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func extractImages(html string) []string {
	matches := imgSrcRe.FindAllStringSubmatch(html, -1)
	var images []string
	for _, m := range matches {
		src := m[1]
		// Skip emoji images
		if strings.Contains(src, "emoji") || strings.Contains(src, "twemoji") {
			continue
		}
		images = append(images, src)
	}
	return images
}
