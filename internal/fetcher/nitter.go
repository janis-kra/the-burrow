package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	instances []string
	usernames []string
	limit     int
}

// Default public Nitter mirrors, tried in order when none are configured.
var defaultNitterInstances = []string{
	"https://nitter.net",
	"https://nitter.jaydenha.uk",
	"https://nitter.cz",
	"https://nitter.privacyredirect.com",
}

func NewNitter(client *http.Client, instances []string, usernames []string, limit int) *Nitter {
	if limit <= 0 {
		limit = 5
	}
	cleaned := cleanInstances(instances)
	if len(cleaned) == 0 {
		cleaned = append([]string(nil), defaultNitterInstances...)
	}
	return &Nitter{client: client, instances: cleaned, usernames: usernames, limit: limit}
}

func cleanInstances(instances []string) []string {
	seen := make(map[string]bool, len(instances))
	var out []string
	for _, raw := range instances {
		inst := strings.TrimRight(strings.TrimSpace(raw), "/")
		if inst == "" || seen[inst] {
			continue
		}
		seen[inst] = true
		out = append(out, inst)
	}
	return out
}

func (n *Nitter) Name() string { return "Opinion" }

// lookbackGuarantee is how far back we go to ensure each followed user can
// still contribute one post even if they were quiet in the last day.
const lookbackGuarantee = 7 * 24 * time.Hour

// lookbackFill is the window used for extra (non-guaranteed) slots.
const lookbackFill = 24 * time.Hour

func (n *Nitter) Fetch(ctx context.Context) (any, error) {
	now := time.Now()
	guaranteeCutoff := now.Add(-lookbackGuarantee)
	fillCutoff := now.Add(-lookbackFill)
	var allPosts []NitterPost

	for _, username := range n.usernames {
		posts, err := n.fetchUser(ctx, username, guaranteeCutoff)
		if err != nil {
			log.Printf("nitter: failed to fetch @%s: %v", username, err)
			continue
		}
		allPosts = append(allPosts, posts...)
	}

	return selectNitterPosts(allPosts, n.limit, fillCutoff), nil
}

// selectNitterPosts picks up to limit posts with two rules:
//  1. Every user who has a post is represented at least once (may exceed limit
//     when there are more active users than slots).
//  2. Remaining slots are filled by recency, but only from the fill window
//     (typically last 24h), so one prolific poster cannot crowd others out.
func selectNitterPosts(allPosts []NitterPost, limit int, fillCutoff time.Time) []NitterPost {
	if len(allPosts) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}

	sort.SliceStable(allPosts, func(i, j int) bool {
		return allPosts[i].PubDate.After(allPosts[j].PubDate)
	})

	// Newest post per user (allPosts already newest-first).
	bestByUser := make(map[string]NitterPost, len(allPosts))
	var userOrder []string
	for _, p := range allPosts {
		if _, ok := bestByUser[p.Username]; ok {
			continue
		}
		bestByUser[p.Username] = p
		userOrder = append(userOrder, p.Username)
	}

	guaranteed := make([]NitterPost, 0, len(userOrder))
	for _, user := range userOrder {
		guaranteed = append(guaranteed, bestByUser[user])
	}
	sort.SliceStable(guaranteed, func(i, j int) bool {
		return guaranteed[i].PubDate.After(guaranteed[j].PubDate)
	})

	// More active users than slots: keep one each, sorted by recency.
	if len(guaranteed) >= limit {
		return guaranteed
	}

	included := make(map[string]bool, len(guaranteed))
	result := make([]NitterPost, 0, limit)
	for _, p := range guaranteed {
		result = append(result, p)
		included[p.Link] = true
	}

	// Fill remaining slots from the recent window only.
	for _, p := range allPosts {
		if len(result) >= limit {
			break
		}
		if included[p.Link] {
			continue
		}
		if p.PubDate.Before(fillCutoff) {
			continue
		}
		result = append(result, p)
		included[p.Link] = true
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].PubDate.After(result[j].PubDate)
	})
	return result
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

var (
	imgSrcRe       = regexp.MustCompile(`(?i)<img[^>]+src="([^"]+)"`)
	statusLinkRe   = regexp.MustCompile(`(?i)(?:https?://[^/]+)?/([^/]+)/status/(\d+)`)
	nitterPicPath  = regexp.MustCompile(`(?i)/pic/(.+)$`)
)

func (n *Nitter) fetchUser(ctx context.Context, username string, cutoff time.Time) ([]NitterPost, error) {
	var errs []string
	for _, instance := range n.instances {
		posts, err := n.fetchUserFrom(ctx, instance, username, cutoff)
		if err == nil {
			if instance != n.instances[0] {
				log.Printf("nitter: @%s fetched via fallback %s", username, instance)
			}
			return posts, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", instance, err))
	}
	return nil, fmt.Errorf("all instances failed: %s", strings.Join(errs, "; "))
}

func (n *Nitter) fetchUserFrom(ctx context.Context, instance, username string, cutoff time.Time) ([]NitterPost, error) {
	feedURL := fmt.Sprintf("%s/%s/rss", instance, username)

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	// Browser-like headers — some mirrors block bare bot UAs.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Reject non-RSS bodies early (Cloudflare/Anubis challenge pages return 200).
	trimmed := strings.TrimSpace(string(body))
	if !strings.HasPrefix(trimmed, "<?xml") && !strings.HasPrefix(trimmed, "<rss") {
		return nil, fmt.Errorf("non-RSS response from %s", feedURL)
	}

	var rss rssDocument
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("parsing RSS: %w", err)
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

		images := extractImages(item.Description, instance)

		posts = append(posts, NitterPost{
			Username:  username,
			Text:      text,
			Link:      toXStatusLink(item.Link),
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

func extractImages(html, instance string) []string {
	matches := imgSrcRe.FindAllStringSubmatch(html, -1)
	var images []string
	for _, m := range matches {
		src := m[1]
		// Skip emoji images
		if strings.Contains(src, "emoji") || strings.Contains(src, "twemoji") {
			continue
		}
		if rewritten := rewriteNitterMedia(src, instance); rewritten != "" {
			images = append(images, rewritten)
		}
	}
	return images
}

// toXStatusLink rewrites nitter status URLs to durable x.com links.
func toXStatusLink(link string) string {
	if m := statusLinkRe.FindStringSubmatch(link); m != nil {
		return fmt.Sprintf("https://x.com/%s/status/%s", m[1], m[2])
	}
	return link
}

// rewriteNitterMedia turns nitter /pic/ proxy URLs into direct media hosts when
// possible, so images still load after a mirror dies. Falls back to an absolute
// nitter URL.
func rewriteNitterMedia(src, instance string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}

	// Protocol-relative
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}

	// Root-relative nitter path
	if strings.HasPrefix(src, "/") {
		src = strings.TrimRight(instance, "/") + src
	}

	m := nitterPicPath.FindStringSubmatch(src)
	if m == nil {
		// Non-nitter absolute URL (or unknown) — keep https if we can.
		if strings.HasPrefix(src, "http://") {
			return "https://" + strings.TrimPrefix(src, "http://")
		}
		return src
	}

	encoded := m[1]
	// Strip fragment/query if present
	if i := strings.IndexAny(encoded, "?#"); i >= 0 {
		encoded = encoded[:i]
	}
	decoded, err := url.PathUnescape(encoded)
	if err != nil {
		decoded = encoded
	}
	decoded, err = url.QueryUnescape(decoded)
	if err != nil {
		// PathUnescape is usually enough; keep decoded as-is.
	}

	switch {
	case strings.HasPrefix(decoded, "https://"), strings.HasPrefix(decoded, "http://"):
		if strings.HasPrefix(decoded, "http://") {
			return "https://" + strings.TrimPrefix(decoded, "http://")
		}
		return decoded
	case strings.HasPrefix(decoded, "media/"), strings.HasPrefix(decoded, "tweet_video_thumb/"), strings.HasPrefix(decoded, "ext_tw_video_thumb/"):
		return "https://pbs.twimg.com/" + decoded
	case strings.HasPrefix(decoded, "profile_images/"):
		return "https://pbs.twimg.com/" + decoded
	default:
		// Keep absolute nitter URL as last resort (prefer https).
		if strings.HasPrefix(src, "http://") {
			return "https://" + strings.TrimPrefix(src, "http://")
		}
		return src
	}
}
